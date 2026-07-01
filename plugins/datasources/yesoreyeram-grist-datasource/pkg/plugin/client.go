package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client is a thin wrapper around the Grist REST API.
//
// Grist Cloud team sites live at https://{team}.getgrist.com and self-hosted
// instances at the configured host. In both cases the REST API is served under
// the `/api` path prefix. The configured base URL may include or omit a trailing
// `/api`; NewClient normalizes it away and the client always appends `/api`
// itself (see apiURL).
type Client struct {
	baseURL    string // host root, e.g. https://team.getgrist.com (no /api, no trailing slash)
	apiKey     string
	defaultDoc string
	httpClient *http.Client
}

// NewClient creates a Grist API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	// Accept a base URL that already ends in /api and normalise it away so the
	// client can append the /api prefix uniformly.
	base = strings.TrimSuffix(base, "/api")
	base = strings.TrimRight(base, "/")
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		apiKey:     strings.TrimSpace(settings.apiKey),
		defaultDoc: strings.TrimSpace(settings.DocID),
		httpClient: httpClient,
	}, nil
}

// apiURL builds a full API URL for the given path (which must start with "/").
func (c *Client) apiURL(path string) string {
	return c.baseURL + "/api" + path
}

// do issues an HTTP request to Grist. When body is non-nil it is JSON-encoded.
// Non-2xx responses are turned into errors carrying Grist's `error` message.
func (c *Client) do(ctx context.Context, method, rawURL string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to grist failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading grist response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp.StatusCode, raw)
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("failed parsing grist response: %w", err)
		}
	}
	return nil
}

// apiError builds a useful error from a non-2xx Grist response. Grist error
// bodies carry an `error` field (string), sometimes accompanied by `details`.
func apiError(status int, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	var apiErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &apiErr) == nil {
		if apiErr.Error != "" {
			msg = apiErr.Error
		} else if apiErr.Message != "" {
			msg = apiErr.Message
		}
	}
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if hint := statusHint(status); hint != "" {
		return fmt.Errorf("grist returned status %d: %s. %s", status, msg, hint)
	}
	return fmt.Errorf("grist returned status %d: %s", status, msg)
}

func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The API key was missing or rejected — re-enter the API Key and click Save & test (saved secrets are write-only, so an empty re-save blanks the key)."
	case http.StatusForbidden:
		return "Access denied — ensure the API key has access to this document."
	case http.StatusNotFound:
		return "Not found — verify the base URL, document id and table id are correct."
	default:
		return ""
	}
}

func (c *Client) resolveDoc(queryDoc string) string {
	if d := strings.TrimSpace(queryDoc); d != "" {
		return d
	}
	return c.defaultDoc
}

// ---------------------------------------------------------------------------
// Orgs / workspaces / docs (resource discovery)
// ---------------------------------------------------------------------------

// DocInfo is a lightweight representation of a Grist document used to populate
// the query editor document dropdown.
type DocInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type orgItem struct {
	ID     json.Number `json:"id"`
	Name   string      `json:"name"`
	Domain string      `json:"domain"`
}

type workspaceItem struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
	Docs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"docs"`
}

// ListDocs returns all documents accessible to the API key by enumerating
// orgs -> workspaces -> docs. Grist has no flat "list all docs" endpoint.
func (c *Client) ListDocs(ctx context.Context) ([]DocInfo, error) {
	var orgs []orgItem
	if err := c.do(ctx, http.MethodGet, c.apiURL("/orgs"), nil, &orgs); err != nil {
		return nil, err
	}

	docs := make([]DocInfo, 0)
	seen := map[string]bool{}
	for _, org := range orgs {
		orgID := org.ID.String()
		if orgID == "" {
			orgID = org.Domain
		}
		if orgID == "" {
			continue
		}
		var workspaces []workspaceItem
		endpoint := c.apiURL("/orgs/" + url.PathEscape(orgID) + "/workspaces")
		if err := c.do(ctx, http.MethodGet, endpoint, nil, &workspaces); err != nil {
			// An org may be inaccessible; skip it rather than failing the whole
			// listing.
			continue
		}
		for _, ws := range workspaces {
			for _, d := range ws.Docs {
				if d.ID == "" || seen[d.ID] {
					continue
				}
				seen[d.ID] = true
				title := d.Name
				if title == "" {
					title = d.ID
				}
				docs = append(docs, DocInfo{ID: d.ID, Title: title})
			}
		}
	}
	return docs, nil
}

// ---------------------------------------------------------------------------
// Tables + columns metadata
// ---------------------------------------------------------------------------

// TableInfo is a lightweight representation of a Grist table. The Grist table
// `id` doubles as its display name.
type TableInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type tablesResponse struct {
	Tables []struct {
		ID string `json:"id"`
	} `json:"tables"`
}

// ListTables returns the tables in a document.
func (c *Client) ListTables(ctx context.Context, docID string) ([]TableInfo, error) {
	docID = c.resolveDoc(docID)
	if docID == "" {
		return nil, fmt.Errorf("docId is required")
	}
	endpoint := c.apiURL("/docs/" + url.PathEscape(docID) + "/tables")
	var res tablesResponse
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &res); err != nil {
		return nil, err
	}
	tables := make([]TableInfo, 0, len(res.Tables))
	for _, t := range res.Tables {
		if strings.TrimSpace(t.ID) == "" {
			continue
		}
		tables = append(tables, TableInfo{ID: t.ID, Title: t.ID})
	}
	return tables, nil
}

// gristColumn is a single entry of GET .../columns. The display label and Grist
// type live under `fields`.
type gristColumn struct {
	ID     string `json:"id"`
	Fields struct {
		Label string `json:"label"`
		Type  string `json:"type"`
	} `json:"fields"`
}

type columnsResponse struct {
	Columns []gristColumn `json:"columns"`
}

// FieldInfo is a lightweight representation of a Grist column/field used to
// populate the QueryEditor fields multi-select and filter editor.
type FieldInfo struct {
	Title string `json:"title"`
	Type  string `json:"type"`
}

// listColumns fetches the raw column metadata for a table.
func (c *Client) listColumns(ctx context.Context, docID, tableID string) ([]gristColumn, error) {
	docID = c.resolveDoc(docID)
	if docID == "" {
		return nil, fmt.Errorf("docId is required")
	}
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	endpoint := c.apiURL("/docs/" + url.PathEscape(docID) + "/tables/" + url.PathEscape(tableID) + "/columns")
	var res columnsResponse
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &res); err != nil {
		return nil, err
	}
	return res.Columns, nil
}

// ListFields returns the user-facing columns/fields of a table. The Grist column
// `id` (the stable colId used by filters/SQL) is returned as the title, and the
// Grist type (e.g. Text, Numeric, Date, "DateTime:America/New_York") as the type.
func (c *Client) ListFields(ctx context.Context, docID, tableID string) ([]FieldInfo, error) {
	cols, err := c.listColumns(ctx, docID, tableID)
	if err != nil {
		return nil, err
	}
	fields := make([]FieldInfo, 0, len(cols))
	for _, col := range cols {
		if strings.TrimSpace(col.ID) == "" {
			continue
		}
		fields = append(fields, FieldInfo{Title: col.ID, Type: col.Fields.Type})
	}
	return fields, nil
}

// isDateType reports whether a Grist column type denotes a Date or DateTime
// column. DateTime types carry a timezone suffix, e.g. "DateTime:America/New_York".
func isDateType(t string) bool {
	t = strings.TrimSpace(t)
	return t == "Date" || strings.HasPrefix(t, "DateTime")
}

// dateColumnSet returns the set of column ids whose Grist type is Date/DateTime.
// Grist serves those columns as Unix epoch seconds, so the frame builder uses
// this set to convert them to time fields.
func dateColumnSet(cols []gristColumn) map[string]bool {
	set := map[string]bool{}
	for _, col := range cols {
		if isDateType(col.Fields.Type) {
			set[col.ID] = true
		}
	}
	return set
}

// ---------------------------------------------------------------------------
// Records (records endpoint + SQL endpoint)
// ---------------------------------------------------------------------------

// recordsResponse is the shape of GET .../records.
type recordsResponse struct {
	Records []struct {
		ID     int            `json:"id"`
		Fields map[string]any `json:"fields"`
	} `json:"records"`
}

// sqlResponse is the shape of GET/POST .../sql.
type sqlResponse struct {
	Statement string `json:"statement"`
	Records   []struct {
		Fields map[string]any `json:"fields"`
	} `json:"records"`
}

// ListRecords fetches records for a table and returns the rows plus the set of
// Date/DateTime columns (so the frame builder can convert epoch seconds to time
// fields). Simple equality/membership filters use the records endpoint; richer
// filters or a column projection use the SQL endpoint (see QueryModel.requiresSQL).
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, map[string]bool, error) {
	docID := c.resolveDoc(q.DocID)
	if docID == "" {
		return nil, nil, fmt.Errorf("docId is required")
	}
	if strings.TrimSpace(q.TableID) == "" {
		return nil, nil, fmt.Errorf("tableId is required")
	}

	cols, err := c.listColumns(ctx, docID, q.TableID)
	if err != nil {
		return nil, nil, err
	}
	dateCols := dateColumnSet(cols)

	var rows []map[string]any
	if q.requiresSQL() {
		rows, err = c.listViaSQL(ctx, docID, q)
	} else {
		rows, err = c.listViaRecords(ctx, docID, q)
	}
	if err != nil {
		return nil, nil, err
	}
	return rows, dateCols, nil
}

// listViaRecords fetches rows from the records endpoint. The records endpoint
// has NO offset/cursor pagination: `limit` caps the result and, when omitted,
// every matching row is returned in a single response.
func (c *Client) listViaRecords(ctx context.Context, docID string, q QueryModel) ([]map[string]any, error) {
	endpoint := c.apiURL("/docs/" + url.PathEscape(docID) + "/tables/" + url.PathEscape(q.TableID) + "/records")

	params := url.Values{}
	if filterJSON, ok := simpleGristFilter(q.filter); ok && filterJSON != "" {
		params.Set("filter", filterJSON)
	}
	if s := sortCSV(q.sortItems); s != "" {
		params.Set("sort", s)
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}

	full := endpoint
	if encoded := params.Encode(); encoded != "" {
		full = endpoint + "?" + encoded
	}

	var res recordsResponse
	if err := c.do(ctx, http.MethodGet, full, nil, &res); err != nil {
		return nil, err
	}

	rows := make([]map[string]any, 0, len(res.Records))
	for _, rec := range res.Records {
		row := make(map[string]any, len(rec.Fields)+1)
		row["id"] = float64(rec.ID)
		for k, v := range rec.Fields {
			row[k] = v
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// listViaSQL fetches rows via a compiled read-only SELECT on the SQL endpoint.
func (c *Client) listViaSQL(ctx context.Context, docID string, q QueryModel) ([]map[string]any, error) {
	sql, args := BuildSelectSQL(q)
	return c.querySQL(ctx, docID, sql, args)
}

// CountRecords returns the number of records matching the query's filter via a
// SQL COUNT(*). Grist has no count endpoint, and the records endpoint exposes
// neither a count nor offset pagination, so SQL is the only correct option.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	docID := c.resolveDoc(q.DocID)
	if docID == "" {
		return 0, fmt.Errorf("docId is required")
	}
	if strings.TrimSpace(q.TableID) == "" {
		return 0, fmt.Errorf("tableId is required")
	}
	sql, args := BuildCountSQL(q)
	rows, err := c.querySQL(ctx, docID, sql, args)
	if err != nil {
		return 0, err
	}
	return extractCount(rows), nil
}

// RunSQL executes a raw read-only Grist SQL SELECT and returns its rows as-is.
// Trailing semicolons are stripped (Grist rejects them).
func (c *Client) RunSQL(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	docID := c.resolveDoc(q.DocID)
	if docID == "" {
		return nil, fmt.Errorf("docId is required")
	}
	sql := strings.TrimRight(strings.TrimSpace(q.SQL), ";")
	if sql == "" {
		return nil, fmt.Errorf("sql is required for the SQL query type")
	}
	return c.querySQL(ctx, docID, sql, nil)
}

// querySQL posts a read-only SQL statement (with optional positional args) to the
// SQL endpoint and returns the result rows (each record's `fields` map).
func (c *Client) querySQL(ctx context.Context, docID, sql string, args []any) ([]map[string]any, error) {
	endpoint := c.apiURL("/docs/" + url.PathEscape(docID) + "/sql")
	body := map[string]any{"sql": sql}
	if len(args) > 0 {
		body["args"] = args
	}
	var res sqlResponse
	if err := c.do(ctx, http.MethodPost, endpoint, body, &res); err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(res.Records))
	for _, rec := range res.Records {
		row := make(map[string]any, len(rec.Fields))
		for k, v := range rec.Fields {
			row[k] = v
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// extractCount pulls the single numeric value out of a COUNT(*) result row.
func extractCount(rows []map[string]any) int64 {
	if len(rows) == 0 {
		return 0
	}
	if v, ok := rows[0]["count"]; ok {
		if f, ok := toFloat(v); ok {
			return int64(f)
		}
	}
	for _, v := range rows[0] {
		if f, ok := toFloat(v); ok {
			return int64(f)
		}
	}
	return 0
}

// Ping performs a minimal authenticated request (list orgs) to validate
// connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, c.apiURL("/orgs"), nil, nil)
}
