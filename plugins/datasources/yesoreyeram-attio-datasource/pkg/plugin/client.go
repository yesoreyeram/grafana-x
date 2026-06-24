package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Attio caps the records query `limit` at 500.
	defaultPageSize = 500
	// maxRecords is a safety cap when no explicit limit is requested.
	maxRecords = 100000
)

// Client is a thin wrapper around the Attio REST API.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// NewClient creates an Attio API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = attioCloudURL
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

// do issues a request to Attio. When body is non-nil it is JSON-encoded;
// otherwise the request is sent without a body (typically GET).
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
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
		return fmt.Errorf("request to attio failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading attio response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("attio returned status %d: %s. %s",
			resp.StatusCode, extractError(rawBody), statusHint(resp.StatusCode))
	}

	if out != nil {
		if err := json.Unmarshal(rawBody, out); err != nil {
			return fmt.Errorf("failed parsing attio response: %w", err)
		}
	}
	return nil
}

func (c *Client) doGet(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// extractError pulls the human-readable message out of an Attio error body,
// which has the shape {"status_code","type","code","message"}. When no message
// is present the truncated raw body is returned.
func extractError(raw []byte) string {
	var apiErr struct {
		StatusCode int    `json:"status_code"`
		Type       string `json:"type"`
		Code       string `json:"code"`
		Message    string `json:"message"`
	}
	if json.Unmarshal(raw, &apiErr) == nil && strings.TrimSpace(apiErr.Message) != "" {
		return truncate(apiErr.Message, 500)
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
		return "The access token was missing or rejected — re-enter the API Token and click Save & test (saved secrets are write-only, so an empty re-save can blank the token)."
	case http.StatusForbidden:
		return "Access denied — ensure the token has the required scopes (record_permission:read, object_configuration:read)."
	case http.StatusNotFound:
		return "Not found — verify the object slug is correct and the token can access it."
	case http.StatusTooManyRequests:
		return "Rate limited — Attio throttles requests; reduce the query frequency or limit."
	default:
		return ""
	}
}

// Ping validates connectivity and credentials using the meta identify endpoint,
// which any valid token can call.
func (c *Client) Ping(ctx context.Context) error {
	var res struct {
		Active bool `json:"active"`
	}
	if err := c.doGet(ctx, "/v2/self", &res); err != nil {
		return err
	}
	if !res.Active {
		return fmt.Errorf("the access token is inactive or has been revoked")
	}
	return nil
}

// ObjectInfo is a lightweight representation of an Attio object used to populate
// the QueryEditor object dropdown.
type ObjectInfo struct {
	APISlug      string `json:"api_slug"`
	SingularNoun string `json:"singular_noun"`
	PluralNoun   string `json:"plural_noun"`
}

// ListObjects returns all system-defined and user-defined objects in the
// workspace.
func (c *Client) ListObjects(ctx context.Context) ([]ObjectInfo, error) {
	var res struct {
		Data []ObjectInfo `json:"data"`
	}
	if err := c.doGet(ctx, "/v2/objects", &res); err != nil {
		return nil, err
	}
	objects := make([]ObjectInfo, 0, len(res.Data))
	for _, o := range res.Data {
		if strings.TrimSpace(o.APISlug) == "" {
			continue
		}
		objects = append(objects, o)
	}
	return objects, nil
}

// AttributeInfo is a lightweight representation of an Attio attribute used to
// populate the QueryEditor field multi-select and the filter builder.
type AttributeInfo struct {
	APISlug    string `json:"api_slug"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	IsRequired bool   `json:"is_required"`
}

// ListAttributes returns the attributes (columns) defined on an object.
func (c *Client) ListAttributes(ctx context.Context, objectID string) ([]AttributeInfo, error) {
	if strings.TrimSpace(objectID) == "" {
		return nil, fmt.Errorf("objectId is required")
	}
	path := fmt.Sprintf("/v2/objects/%s/attributes", url.PathEscape(objectID))
	var res struct {
		Data []AttributeInfo `json:"data"`
	}
	if err := c.doGet(ctx, path, &res); err != nil {
		return nil, err
	}
	attrs := make([]AttributeInfo, 0, len(res.Data))
	for _, a := range res.Data {
		if strings.TrimSpace(a.APISlug) == "" {
			continue
		}
		attrs = append(attrs, a)
	}
	return attrs, nil
}

// queryResponse is the shape of POST /v2/objects/{object}/records/query.
type queryResponse struct {
	Data []attioRecord `json:"data"`
}

// buildQueryBody assembles the request body for the records query endpoint.
func (c *Client) buildQueryBody(q QueryModel, limit int, offset int64) map[string]any {
	body := map[string]any{
		"limit":  limit,
		"offset": offset,
	}
	if q.filter != nil {
		if f := BuildFilter(q.filter); f != nil {
			body["filter"] = f
		}
	}
	if sorts := buildSorts(q.sortItems); len(sorts) > 0 {
		body["sorts"] = sorts
	}
	return body
}

// buildSorts converts structured sort items into Attio sort directives.
func buildSorts(items []SortItem) []map[string]any {
	sorts := make([]map[string]any, 0, len(items))
	for _, s := range items {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		direction := "asc"
		if strings.EqualFold(s.Direction, "desc") {
			direction = "desc"
		}
		sorts = append(sorts, map[string]any{
			"attribute": field,
			"direction": direction,
		})
	}
	return sorts
}

func (c *Client) recordsEndpoint(objectID string) string {
	return fmt.Sprintf("/v2/objects/%s/records/query", url.PathEscape(objectID))
}

// QueryRecords fetches records for an object, transparently following Attio's
// offset/limit pagination up to the requested limit (or maxRecords when no limit
// is provided). The records are flattened into scalar maps.
func (c *Client) QueryRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.ObjectID) == "" {
		return nil, fmt.Errorf("objectId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	fields := splitFields(q.Fields)
	out := make([]attioRecord, 0, defaultPageSize)
	offset := q.Offset

	for {
		size := defaultPageSize
		if remaining := hardLimit - int64(len(out)); int64(size) > remaining {
			size = int(remaining)
		}
		if size <= 0 {
			break
		}

		page, err := c.fetchRecordPage(ctx, q, size, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)

		if len(page) < size || int64(len(out)) >= hardLimit {
			break
		}
		offset += int64(size)
	}

	return flattenRecords(out, fields), nil
}

func (c *Client) fetchRecordPage(ctx context.Context, q QueryModel, size int, offset int64) ([]attioRecord, error) {
	var res queryResponse
	if err := c.do(ctx, http.MethodPost, c.recordsEndpoint(q.ObjectID), c.buildQueryBody(q, size, offset), &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// CountRecords returns the number of records matching the query's filter. Attio
// has no count endpoint, so the matching records are paginated and counted.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.ObjectID) == "" {
		return 0, fmt.Errorf("objectId is required")
	}

	var total int64
	var offset int64
	for {
		page, err := c.fetchRecordPage(ctx, q, defaultPageSize, offset)
		if err != nil {
			return 0, err
		}
		total += int64(len(page))
		if len(page) < defaultPageSize {
			break
		}
		offset += defaultPageSize
	}
	return total, nil
}

func splitFields(fields string) []string {
	out := make([]string, 0)
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
