package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	// PostgREST has no hard page size, but a generous page keeps the number of
	// round-trips low while bounding memory per request.
	defaultPageSize = 1000
	// Safety cap on the number of records fetched when no limit is given.
	maxRecords = 100000
)

// Client is a thin wrapper around the Supabase PostgREST API.
//
// Supabase authenticates every request with the project API key sent in BOTH
// the `apikey` header AND the `Authorization: Bearer <key>` header, using the
// same value (the `anon` key, `service_role` key, or a user JWT).
type Client struct {
	baseURL    string
	apiKey     string
	schema     string
	httpClient *http.Client
}

// NewClient creates a Supabase PostgREST client. The provided httpClient is
// normally the SDK-managed client so that proxy, TLS and timeout settings are
// respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.APIURL), "/")
	if base == "" {
		return nil, fmt.Errorf("API URL is required (e.g. https://<project-ref>.supabase.co/rest/v1)")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid API URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		apiKey:     strings.TrimSpace(settings.apiKey),
		schema:     strings.TrimSpace(settings.Schema),
		httpClient: httpClient,
	}, nil
}

// do issues a request to PostgREST and returns the response headers. When out is
// non-nil and a JSON body is returned, it is unmarshalled into out.
//
// The dual Supabase auth headers (`apikey` + `Authorization: Bearer`) are set on
// every request. Both 200 OK and 206 Partial Content (returned for ranged reads)
// are treated as success.
func (c *Client) do(ctx context.Context, method, rawPath string, params url.Values, headers map[string]string, out any) (http.Header, error) {
	endpoint := c.baseURL + rawPath
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		// Supabase requires BOTH headers, set to the same key value.
		req.Header.Set("apikey", c.apiKey)
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.schema != "" {
		// Select the Postgres schema for read requests (PostgREST default: public).
		req.Header.Set("Accept-Profile", c.schema)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to supabase failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return resp.Header, fmt.Errorf("failed reading supabase response: %w", err)
	}

	// PostgREST returns 200 (full) or 206 (partial/ranged) on success.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Header, fmt.Errorf("supabase returned status %d: %s%s",
			resp.StatusCode, extractError(body), statusHint(resp.StatusCode))
	}

	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return resp.Header, fmt.Errorf("failed parsing supabase response: %w", err)
		}
	}
	return resp.Header, nil
}

// extractError pulls a human-readable message out of a PostgREST error body,
// which has the shape {message, details, hint, code}. It falls back to the raw
// (truncated) body when the JSON cannot be parsed.
func extractError(body []byte) string {
	var apiErr struct {
		Message string `json:"message"`
		Details string `json:"details"`
		Hint    string `json:"hint"`
		Code    string `json:"code"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
		msg := apiErr.Message
		if apiErr.Details != "" {
			msg += " (" + apiErr.Details + ")"
		}
		if apiErr.Hint != "" {
			msg += " hint: " + apiErr.Hint
		}
		return truncate(msg, 500)
	}
	return truncate(string(body), 500)
}

// truncate shortens s to at most n bytes for safe error messages.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// statusHint returns an actionable suffix for common PostgREST/Supabase error
// statuses.
func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return ". The API key was missing or rejected — re-enter the API key and click Save & test (saved secrets are write-only, so an empty re-save can blank the key)."
	case http.StatusForbidden:
		return ". Access denied — with the anon key, row-level security (RLS) policies apply; use the service_role key to bypass RLS, or grant the role access."
	case http.StatusNotFound:
		return ". Not found — verify the Project URL (…/rest/v1) and that the table/view exists in the selected schema."
	case http.StatusRequestedRangeNotSatisfiable:
		return ". The requested range/offset is beyond the available rows."
	default:
		return ""
	}
}

// parseContentRangeTotal extracts the total row count from a Content-Range header
// of the form "0-24/3573" (the part after "/"). It returns -1 when the total is
// unknown ("*") or the header is missing/unparseable.
func parseContentRangeTotal(h http.Header) int64 {
	cr := h.Get("Content-Range")
	if cr == "" {
		return -1
	}
	idx := strings.LastIndex(cr, "/")
	if idx < 0 {
		return -1
	}
	total := strings.TrimSpace(cr[idx+1:])
	if total == "" || total == "*" {
		return -1
	}
	n, err := strconv.ParseInt(total, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

// ---------------------------------------------------------------------------
// Records
// ---------------------------------------------------------------------------

// ListRecords fetches rows from a table/view, transparently following
// limit/offset pagination up to the requested limit (or maxRecords when no limit
// is provided). Each row is the table's flat JSON object.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	out := make([]map[string]any, 0, defaultPageSize)
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	for {
		size := defaultPageSize
		if remaining := hardLimit - len(out); remaining < size {
			size = remaining
		}
		if size <= 0 {
			break
		}

		rows, err := c.fetchRecordPage(ctx, q, offset, size)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)

		// A short page means we have reached the end of the result set.
		if len(rows) < size || len(out) >= hardLimit {
			break
		}
		offset += size
	}

	return out, nil
}

// recordParams builds the shared query parameters used to read records: select,
// the compiled filter parameters, and ordering.
func (c *Client) recordParams(q QueryModel) url.Values {
	params := url.Values{}
	if v := strings.TrimSpace(q.Select); v != "" {
		params.Set("select", v)
	}
	if q.filter != nil {
		for _, p := range BuildParams(*q.filter) {
			params.Add(p.Key, p.Value)
		}
	}
	for _, s := range q.sortItems {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		dir := "asc"
		if strings.EqualFold(s.Direction, "desc") {
			dir = "desc"
		}
		params.Add("order", field+"."+dir)
	}
	return params
}

func (c *Client) recordsEndpoint(tableID string) string {
	return "/" + url.PathEscape(tableID)
}

func (c *Client) fetchRecordPage(ctx context.Context, q QueryModel, offset, size int) ([]map[string]any, error) {
	params := c.recordParams(q)
	if size > 0 {
		params.Set("limit", strconv.Itoa(size))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	var records []map[string]any
	if _, err := c.do(ctx, http.MethodGet, c.recordsEndpoint(q.TableID), params, nil, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// CountRecords returns the number of rows matching the query's filters. It issues
// a HEAD request with `Prefer: count=exact` and reads the total from the
// Content-Range response header (no row data is transferred).
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return 0, fmt.Errorf("tableId is required")
	}

	params := url.Values{}
	if q.filter != nil {
		for _, p := range BuildParams(*q.filter) {
			params.Add(p.Key, p.Value)
		}
	}
	// Keep the page minimal; count=exact is computed over the full match set
	// regardless, but a small range avoids any row materialisation.
	params.Set("limit", "1")

	headers := map[string]string{"Prefer": "count=exact"}
	respHeader, err := c.do(ctx, http.MethodHead, c.recordsEndpoint(q.TableID), params, headers, nil)
	if err != nil {
		return 0, err
	}
	total := parseContentRangeTotal(respHeader)
	if total < 0 {
		return 0, nil
	}
	return total, nil
}

// ---------------------------------------------------------------------------
// Tables (schema discovery)
// ---------------------------------------------------------------------------

// TableInfo is a lightweight representation of a PostgREST table/view used to
// populate the QueryEditor table picker.
type TableInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// openAPIResponse is the subset of the PostgREST OpenAPI (Swagger 2.0) document
// served at the API root that we use to enumerate tables/views. `definitions`
// keys are table/view names; `paths` keys are `/`, `/{table}` and `/rpc/{fn}`.
type openAPIResponse struct {
	Definitions map[string]json.RawMessage `json:"definitions"`
	Paths       map[string]json.RawMessage `json:"paths"`
}

// ListTables returns the available tables/views by fetching the PostgREST
// OpenAPI document from the API root and parsing it. RPC functions and synthetic
// entries are excluded. It degrades gracefully: when the document cannot be
// parsed for table names an empty list is returned (the editor allows custom
// values).
func (c *Client) ListTables(ctx context.Context) ([]TableInfo, error) {
	var schemaResp openAPIResponse
	if _, err := c.do(ctx, http.MethodGet, "/", nil, nil, &schemaResp); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
	}

	// Prefer the `definitions` block (keys are exactly the table/view names).
	for name := range schemaResp.Definitions {
		add(name)
	}
	// Fall back to / augment with the `paths` block.
	for path := range schemaResp.Paths {
		if path == "/" || strings.HasPrefix(path, "/rpc/") {
			continue
		}
		add(strings.TrimPrefix(path, "/"))
	}

	tables := make([]TableInfo, 0, len(seen))
	for name := range seen {
		tables = append(tables, TableInfo{ID: name, Title: name})
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].ID < tables[j].ID })
	return tables, nil
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. The PostgREST API root returns the OpenAPI document and requires
// a valid key, making it a good auth-validating health check.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/", nil, nil, nil)
	return err
}
