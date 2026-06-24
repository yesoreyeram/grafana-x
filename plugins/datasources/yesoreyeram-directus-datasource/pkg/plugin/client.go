package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// Directus defaults the per-request limit to 100; we page in 100s and stop
	// at maxRecords as a safety cap when no explicit limit is given.
	defaultPageSize = 100
	maxRecords      = 100000
)

// Client is a thin wrapper around the Directus REST API.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a Directus API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("Directus base URL is required")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		apiToken:   strings.TrimSpace(settings.apiToken),
		httpClient: httpClient,
	}, nil
}

// do issues a request to Directus. The static API token is sent as the
// `Authorization: Bearer <token>` header — the preferred of Directus' two static
// token methods (the other being a `?access_token=` query parameter, which we
// avoid so the token is never placed in URLs/logs).
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to directus failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading directus response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := extractError(rawBody)
		if hint := statusHint(resp.StatusCode); hint != "" {
			return fmt.Errorf("directus returned status %d: %s. %s", resp.StatusCode, msg, hint)
		}
		return fmt.Errorf("directus returned status %d: %s", resp.StatusCode, msg)
	}

	if out != nil {
		if err := json.Unmarshal(rawBody, out); err != nil {
			return fmt.Errorf("failed parsing directus response: %w", err)
		}
	}
	return nil
}

func (c *Client) doGet(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// extractError pulls the human-readable message out of a Directus error body.
// Directus errors are shaped as {"errors":[{"message":"...","extensions":{...}}]}
// — fall back to the (truncated) raw body when that shape is absent.
func extractError(raw []byte) string {
	var apiErr struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(raw, &apiErr) == nil && len(apiErr.Errors) > 0 {
		if msg := strings.TrimSpace(apiErr.Errors[0].Message); msg != "" {
			return truncate(msg, 500)
		}
	}
	return truncate(string(raw), 500)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The API token was missing or rejected — re-enter the API Token and click Save & test (saved secrets are write-only, so an empty re-save can blank the token)."
	case http.StatusForbidden:
		return "Access denied — ensure the token has the required permissions to access this Directus instance."
	case http.StatusNotFound:
		return "Not found — verify the Base URL and collection name are correct and the token can access them."
	default:
		return ""
	}
}

// Ping validates connectivity AND credentials. It calls /users/me, which
// requires a valid token — unlike /server/ping, which needs no auth and would
// mask a missing/invalid token with a false success.
func (c *Client) Ping(ctx context.Context) error {
	return c.doGet(ctx, "/users/me?fields=id", nil)
}

// CollectionInfo is a lightweight representation of a Directus collection.
type CollectionInfo struct {
	Collection string `json:"collection"`
	Meta       *struct {
		Icon string `json:"icon,omitempty"`
		Note string `json:"note,omitempty"`
	} `json:"meta,omitempty"`
}

// ListCollections returns the user-defined (non-system) collections. Directus
// exposes its internal collections (directus_users, directus_files, …) through
// the same endpoint; these are always prefixed with `directus_` and are filtered
// out because they are not part of the user's data model.
func (c *Client) ListCollections(ctx context.Context) ([]CollectionInfo, error) {
	var res struct {
		Data []CollectionInfo `json:"data"`
	}
	if err := c.doGet(ctx, "/collections", &res); err != nil {
		return nil, err
	}
	out := make([]CollectionInfo, 0, len(res.Data))
	for _, col := range res.Data {
		if strings.TrimSpace(col.Collection) == "" || isSystemCollection(col.Collection) {
			continue
		}
		out = append(out, col)
	}
	return out, nil
}

// isSystemCollection reports whether a collection is a Directus internal
// collection (always prefixed with `directus_`).
func isSystemCollection(name string) bool {
	return strings.HasPrefix(name, "directus_")
}

// FieldInfo is a lightweight representation of a Directus field.
type FieldInfo struct {
	Field string `json:"field"`
	Type  string `json:"type"`
}

// ListFields returns the fields (columns) of a collection via the schema API.
func (c *Client) ListFields(ctx context.Context, collectionID string) ([]FieldInfo, error) {
	if strings.TrimSpace(collectionID) == "" {
		return nil, fmt.Errorf("collectionId is required")
	}
	path := fmt.Sprintf("/fields/%s", url.PathEscape(collectionID))
	var res struct {
		Data []FieldInfo `json:"data"`
	}
	if err := c.doGet(ctx, path, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// itemsResponse is the shape of GET /items/{collection}.
type itemsResponse struct {
	Data []map[string]any `json:"data"`
}

func (c *Client) itemsEndpoint(collectionID string) string {
	return fmt.Sprintf("/items/%s", url.PathEscape(collectionID))
}

func (c *Client) effectiveFilter(q QueryModel) map[string]any {
	if q.filter == nil {
		return nil
	}
	return BuildFilter(q.filter)
}

// buildQueryParams assembles the shared query parameters (fields, filter, sort,
// search) used by the records query. Pagination (limit/offset) is added by the
// pagination loop, not here.
func (c *Client) buildQueryParams(q QueryModel) url.Values {
	params := url.Values{}

	if v := strings.TrimSpace(q.Fields); v != "" {
		params.Set("fields", v)
	}

	if filter := c.effectiveFilter(q); filter != nil {
		if b, err := json.Marshal(filter); err == nil {
			params.Set("filter", string(b))
		}
	}

	if sort := buildSortParam(q.sortItems); sort != "" {
		params.Set("sort", sort)
	}

	if v := strings.TrimSpace(q.Search); v != "" {
		params.Set("search", v)
	}

	return params
}

// buildSortParam converts structured sort items into a Directus `sort` value,
// e.g. `-views,title` (leading `-` for descending).
func buildSortParam(items []SortItem) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		if strings.EqualFold(s.Direction, "desc") {
			field = "-" + field
		}
		parts = append(parts, field)
	}
	return strings.Join(parts, ",")
}

// ListRecords fetches records for a collection, transparently following Directus'
// offset/limit pagination up to the requested limit (or maxRecords when no limit
// is provided).
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.CollectionID) == "" {
		return nil, fmt.Errorf("collectionId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	out := make([]map[string]any, 0, defaultPageSize)
	offset := q.Offset

	for {
		size := defaultPageSize
		if remaining := hardLimit - int64(len(out)); int64(size) > remaining {
			size = int(remaining)
		}
		if size <= 0 {
			break
		}

		rows, err := c.fetchRecordPage(ctx, q, size, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)

		// A short page (fewer than requested) means there are no more records.
		if len(rows) < size || int64(len(out)) >= hardLimit {
			break
		}
		offset += int64(len(rows))
	}

	return out, nil
}

func (c *Client) fetchRecordPage(ctx context.Context, q QueryModel, size int, offset int64) ([]map[string]any, error) {
	params := c.buildQueryParams(q)
	params.Set("limit", strconv.Itoa(size))
	if offset > 0 {
		params.Set("offset", strconv.FormatInt(offset, 10))
	}

	var res itemsResponse
	if err := c.doGet(ctx, c.itemsEndpoint(q.CollectionID)+"?"+params.Encode(), &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// aggregateResponse is the shape of GET /items/{collection}?aggregate[count]=*.
// The count value comes back either as a JSON number or a numeric string,
// depending on the underlying database driver.
type aggregateResponse struct {
	Data []map[string]any `json:"data"`
}

// CountRecords returns the number of records matching the query's filter (and
// search) using the Directus aggregate API: `aggregate[count]=*`. This respects
// the same `filter`/`search` as the records query and returns
// {"data":[{"count": N}]}.
//
// NOTE: the `meta=filter_count` approach also works, but `meta=total_count`
// ignores the filter entirely; using aggregate avoids that whole class of bug.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.CollectionID) == "" {
		return 0, fmt.Errorf("collectionId is required")
	}

	params := url.Values{}
	params.Set("aggregate[count]", "*")
	if filter := c.effectiveFilter(q); filter != nil {
		if b, err := json.Marshal(filter); err == nil {
			params.Set("filter", string(b))
		}
	}
	if v := strings.TrimSpace(q.Search); v != "" {
		params.Set("search", v)
	}

	var res aggregateResponse
	if err := c.doGet(ctx, c.itemsEndpoint(q.CollectionID)+"?"+params.Encode(), &res); err != nil {
		return 0, err
	}
	if len(res.Data) == 0 {
		return 0, nil
	}
	if n, ok := toInt64(res.Data[0]["count"]); ok {
		return n, nil
	}
	return 0, nil
}

// toInt64 coerces the aggregate count value (number or numeric string) to int64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		if f, err := n.Float64(); err == nil {
			return int64(f), true
		}
		return 0, false
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), true
		}
		return 0, false
	default:
		return 0, false
	}
}
