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
	// Teable caps the record page size (`take`) at 1000 records per request.
	defaultPageSize = 1000
	// Safety cap on the number of records fetched when no limit is given.
	maxRecords = 100000
	// fieldKeyTypeName makes Teable key record.fields, the filter and orderBy by
	// human field names rather than internal field ids.
	fieldKeyTypeName = "name"
)

// Client is a thin wrapper around the Teable API. It authenticates with a
// personal access token sent as `Authorization: Bearer <token>`.
type Client struct {
	baseURL     string
	apiToken    string
	defaultBase string
	httpClient  *http.Client
}

// NewClient creates a Teable API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = teableCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:     base,
		apiToken:    strings.TrimSpace(settings.apiToken),
		defaultBase: strings.TrimSpace(settings.DefaultBaseID),
		httpClient:  httpClient,
	}, nil
}

// do performs a GET request to path (with optional query) and unmarshals the
// JSON body into out.
func (c *Client) do(ctx context.Context, path string, query url.Values, out any) error {
	rawURL := c.baseURL + path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to teable failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading teable response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := truncate(string(body), 500)
		// Teable error bodies carry a useful `message` field.
		var apiErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
			msg = apiErr.Message
		}
		return fmt.Errorf("teable returned status %d: %s. %s",
			resp.StatusCode, msg, statusHint(resp.StatusCode))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("failed parsing teable response: %w", err)
		}
	}
	return nil
}

// truncate shortens s to at most n runes for safe error messages.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// statusHint returns an actionable message for common Teable error statuses.
func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The API token was missing or rejected — re-enter the API Token and click Save & test (saved secrets are write-only, so an empty re-save can blank the token)."
	case http.StatusForbidden:
		return "Access denied — ensure the token has access to this base/table and the required permissions."
	case http.StatusNotFound:
		return "Not found — verify the Server URL and the base/table ids are correct and the token can access them."
	case http.StatusUnprocessableEntity:
		return "Unprocessable request — check the filter conditions, sort fields and field names."
	default:
		return ""
	}
}

// resolveBase returns the base id to use: the query's base id, falling back to
// the data-source-configured default base id.
func (c *Client) resolveBase(queryBase string) string {
	if b := strings.TrimSpace(queryBase); b != "" {
		return b
	}
	return c.defaultBase
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. It calls the current-user endpoint, which only requires a valid
// token (no specific base access).
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, "/api/auth/user", nil, nil)
}

// TableInfo is a lightweight representation of a Teable table used for the
// resource handler that populates the QueryEditor table dropdown.
type TableInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListTables returns the tables of a base. The Teable endpoint returns a bare
// JSON array of tables.
func (c *Client) ListTables(ctx context.Context, baseID string) ([]TableInfo, error) {
	base := c.resolveBase(baseID)
	if base == "" {
		return nil, fmt.Errorf("baseId is required")
	}
	path := fmt.Sprintf("/api/base/%s/table", url.PathEscape(base))
	var res []TableInfo
	if err := c.do(ctx, path, nil, &res); err != nil {
		return nil, err
	}
	tables := make([]TableInfo, 0, len(res))
	for _, t := range res {
		if strings.TrimSpace(t.ID) == "" {
			continue
		}
		tables = append(tables, t)
	}
	return tables, nil
}

// FieldInfo is a lightweight representation of a Teable field used for the
// resource handler that populates the QueryEditor fields multi-select.
type FieldInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsPrimary *bool  `json:"isPrimary,omitempty"`
}

// ListFields returns the fields of a table. The Teable endpoint returns a bare
// JSON array of fields.
func (c *Client) ListFields(ctx context.Context, tableID string) ([]FieldInfo, error) {
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	path := fmt.Sprintf("/api/table/%s/field", url.PathEscape(tableID))
	var res []FieldInfo
	if err := c.do(ctx, path, nil, &res); err != nil {
		return nil, err
	}
	fields := make([]FieldInfo, 0, len(res))
	for _, f := range res {
		if strings.TrimSpace(f.Name) == "" {
			continue
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// teableRecord is the minimal record shape consumed from the list-records
// result. With fieldKeyType=name the Fields map is keyed by human field names.
type teableRecord struct {
	ID               string         `json:"id"`
	Fields           map[string]any `json:"fields"`
	CreatedTime      string         `json:"createdTime"`
	LastModifiedTime string         `json:"lastModifiedTime"`
}

type recordsResponse struct {
	Records []teableRecord `json:"records"`
}

// ListRecords fetches records for a table using offset-based (`skip`/`take`)
// pagination up to the requested limit (or maxRecords when no limit is given).
//
// Teable's list endpoint returns a flat page of records with no cursor; the next
// page is requested by advancing `skip` until a short page (fewer than `take`
// records) is returned.
//
// Each returned row is a flat map keyed by field name, plus the synthetic `_id`,
// `_createdTime` and `_lastModifiedTime` columns.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	return c.listRecordsWithPageSize(ctx, q, defaultPageSize)
}

// listRecordsWithPageSize is ListRecords with a configurable page size, used by
// tests to exercise multi-page pagination without 1000+ fixture records.
func (c *Client) listRecordsWithPageSize(ctx context.Context, q QueryModel, pageSize int) ([]map[string]any, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	out := make([]map[string]any, 0, pageSize)
	skip := 0

	for {
		take := pageSize
		if remaining := hardLimit - len(out); remaining < take {
			take = remaining
		}
		if take <= 0 {
			break
		}

		page, err := c.fetchRecordPage(ctx, q, take, skip)
		if err != nil {
			return nil, err
		}
		for _, rec := range page {
			row := make(map[string]any, len(rec.Fields)+3)
			for k, v := range rec.Fields {
				row[k] = v
			}
			row["_id"] = rec.ID
			if rec.CreatedTime != "" {
				row["_createdTime"] = rec.CreatedTime
			}
			if rec.LastModifiedTime != "" {
				row["_lastModifiedTime"] = rec.LastModifiedTime
			}
			out = append(out, row)
		}

		// A short page (fewer than requested) means there are no more records.
		if len(page) < take || len(out) >= hardLimit {
			break
		}
		skip += take
	}

	return out, nil
}

func (c *Client) recordsEndpoint(tableID string) string {
	return fmt.Sprintf("/api/table/%s/record", url.PathEscape(tableID))
}

func (c *Client) fetchRecordPage(ctx context.Context, q QueryModel, take, skip int) ([]teableRecord, error) {
	params := url.Values{}
	params.Set("fieldKeyType", fieldKeyTypeName)
	params.Set("take", strconv.Itoa(take))
	if skip > 0 {
		params.Set("skip", strconv.Itoa(skip))
	}
	if v := strings.TrimSpace(q.ViewID); v != "" {
		params.Set("viewId", v)
	}
	filter, err := filterParam(q.filter)
	if err != nil {
		return nil, err
	}
	if filter != "" {
		params.Set("filter", filter)
	}
	orderBy, err := orderByParam(q.sortItems)
	if err != nil {
		return nil, err
	}
	if orderBy != "" {
		params.Set("orderBy", orderBy)
	}
	// projection is a repeated query parameter, keyed by name (fieldKeyType=name).
	for _, f := range splitFields(q.Fields) {
		params.Add("projection", f)
	}

	var res recordsResponse
	if err := c.do(ctx, c.recordsEndpoint(q.TableID), params, &res); err != nil {
		return nil, err
	}
	return res.Records, nil
}

// rowCountResponse is the shape of GET /api/table/{tableId}/aggregation/row-count.
type rowCountResponse struct {
	RowCount int64 `json:"rowCount"`
}

// CountRecords returns the number of records matching the query's filter using
// Teable's dedicated row-count aggregation endpoint (no pagination required).
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return 0, fmt.Errorf("tableId is required")
	}

	params := url.Values{}
	if v := strings.TrimSpace(q.ViewID); v != "" {
		params.Set("viewId", v)
	}
	filter, err := filterParam(q.filter)
	if err != nil {
		return 0, err
	}
	if filter != "" {
		params.Set("filter", filter)
	}

	path := fmt.Sprintf("/api/table/%s/aggregation/row-count", url.PathEscape(q.TableID))
	var res rowCountResponse
	if err := c.do(ctx, path, params, &res); err != nil {
		return 0, err
	}
	return res.RowCount, nil
}

// filterParam compiles the structured filter tree into the Teable JSON `filter`
// query parameter. It returns "" when there is no usable filter.
func filterParam(filter *FilterNode) (string, error) {
	if filter == nil {
		return "", nil
	}
	f := BuildFilter(filter)
	if f == nil {
		return "", nil
	}
	b, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("failed encoding filter: %w", err)
	}
	return string(b), nil
}

// orderByParam compiles the structured sort items into the Teable `orderBy`
// query parameter (a JSON array of {fieldId, order}). With fieldKeyType=name the
// fieldId slot accepts field names. Returns "" when there is no usable sort.
func orderByParam(items []SortItem) (string, error) {
	out := make([]map[string]string, 0, len(items))
	for _, s := range items {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		order := "asc"
		if strings.EqualFold(s.Direction, "desc") {
			order = "desc"
		}
		out = append(out, map[string]string{"fieldId": field, "order": order})
	}
	if len(out) == 0 {
		return "", nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// splitFields splits a comma-separated field list into trimmed, non-empty names.
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
