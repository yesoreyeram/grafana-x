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
	"sync"
)

const (
	// rowsPageSize is the max rows the List Rows endpoint returns per request.
	rowsPageSize = 1000
	// sqlPageSize is the max rows a single SQL query returns.
	sqlPageSize = 10000
	// maxRecords is the safety cap on rows fetched when no limit is given.
	maxRecords = 100000
)

// Client is a SeaTable API gateway client.
//
// SeaTable authentication is two-step. The configured *Base API Token* is NOT
// used on data endpoints. The client first exchanges it (via the seahub
// app-access-token endpoint) for a short-lived *Base-Token* (access token) plus
// the base's dtable_uuid and api-gateway base URL, then uses
// `Authorization: Bearer <access_token>` for all data-gateway calls. The access
// token is cached and transparently re-fetched on a 401.
type Client struct {
	serverURL  string
	apiToken   string
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	dtableUUID  string
	gatewayBase string // e.g. https://cloud.seatable.io/api-gateway
}

// NewClient creates a SeaTable client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.ServerURL), "/")
	if base == "" {
		base = seatableCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid server URL %q: %w", base, err)
	}
	return &Client{
		serverURL:  base,
		apiToken:   strings.TrimSpace(settings.apiToken),
		httpClient: httpClient,
	}, nil
}

// ---------------------------------------------------------------------------
// Low-level transport
// ---------------------------------------------------------------------------

// send performs an HTTP request and returns the status code and raw body. Only
// transport/read errors are returned as err; HTTP status handling is left to the
// caller so it can implement the 401 token-refresh retry.
func (c *Client) send(ctx context.Context, method, fullURL, auth string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("failed encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request to seatable failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed reading seatable response: %w", err)
	}
	return resp.StatusCode, raw, nil
}

// apiError builds a useful error from a non-2xx SeaTable response.
func apiError(status int, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	var apiErr struct {
		ErrorMsg string `json:"error_msg"`
		Detail   string `json:"detail"`
		Error    string `json:"error"`
		Message  string `json:"message"`
	}
	if json.Unmarshal(raw, &apiErr) == nil {
		for _, m := range []string{apiErr.ErrorMsg, apiErr.Detail, apiErr.Error, apiErr.Message} {
			if strings.TrimSpace(m) != "" {
				msg = m
				break
			}
		}
	}
	if hint := statusHint(status); hint != "" {
		return fmt.Errorf("seatable returned status %d: %s. %s", status, truncate(msg, 400), hint)
	}
	return fmt.Errorf("seatable returned status %d: %s", status, truncate(msg, 400))
}

func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The Base API Token was missing or rejected — re-enter the API Token and click Save & test (saved secrets are write-only, so an empty re-save blanks the token)."
	case http.StatusForbidden:
		return "Access denied — ensure the API Token has access to this base with the required permission."
	case http.StatusNotFound:
		return "Not found — verify the Server URL and table name are correct."
	case http.StatusBadRequest:
		return "Bad request — check the table name, column names, SQL statement and filter values."
	default:
		return ""
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// ---------------------------------------------------------------------------
// Token exchange (Base API Token -> Base-Token + dtable_uuid)
// ---------------------------------------------------------------------------

// appAccessToken is the response of GET /api/v2.1/dtable/app-access-token/.
type appAccessToken struct {
	AppName      string `json:"app_name"`
	AccessToken  string `json:"access_token"`
	DtableUUID   string `json:"dtable_uuid"`
	DtableServer string `json:"dtable_server"` // e.g. https://cloud.seatable.io/api-gateway/
	DtableName   string `json:"dtable_name"`
}

// exchangeLocked performs the token exchange. The caller must hold c.mu.
func (c *Client) exchangeLocked(ctx context.Context) error {
	if c.apiToken == "" {
		return fmt.Errorf("SeaTable Base API Token is not configured")
	}
	endpoint := c.serverURL + "/api/v2.1/dtable/app-access-token/"
	status, raw, err := c.send(ctx, http.MethodGet, endpoint, "Token "+c.apiToken, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return apiError(status, raw)
	}
	var res appAccessToken
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("failed parsing seatable token response: %w", err)
	}
	if res.AccessToken == "" || res.DtableUUID == "" {
		return fmt.Errorf("seatable token exchange returned an empty access token or base uuid")
	}
	c.accessToken = res.AccessToken
	c.dtableUUID = res.DtableUUID
	c.gatewayBase = strings.TrimRight(res.DtableServer, "/")
	if c.gatewayBase == "" {
		c.gatewayBase = c.serverURL + "/api-gateway"
	}
	return nil
}

// ensure returns a valid access token, base uuid and gateway base, performing
// the token exchange on first use.
func (c *Client) ensure(ctx context.Context) (token, uuid, base string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken == "" {
		if err := c.exchangeLocked(ctx); err != nil {
			return "", "", "", err
		}
	}
	return c.accessToken, c.dtableUUID, c.gatewayBase, nil
}

// resetToken clears the cached access token so the next call re-exchanges.
func (c *Client) resetToken() {
	c.mu.Lock()
	c.accessToken = ""
	c.mu.Unlock()
}

// gatewayJSON issues a data-gateway request, transparently refreshing the access
// token once on a 401. subpath is appended after the base's dtables/{uuid}/ path.
func (c *Client) gatewayJSON(ctx context.Context, method, subpath string, query url.Values, body any, out any) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		token, uuid, base, err := c.ensure(ctx)
		if err != nil {
			return err
		}
		endpoint := fmt.Sprintf("%s/api/v2/dtables/%s/%s", base, url.PathEscape(uuid), subpath)
		if len(query) > 0 {
			endpoint += "?" + query.Encode()
		}
		status, raw, err := c.send(ctx, method, endpoint, "Bearer "+token, body)
		if err != nil {
			return err
		}
		if status == http.StatusUnauthorized && attempt == 0 {
			// The cached Base-Token expired; drop it and retry once.
			c.resetToken()
			lastErr = apiError(status, raw)
			continue
		}
		if status < 200 || status >= 300 {
			return apiError(status, raw)
		}
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				return fmt.Errorf("failed parsing seatable response: %w", err)
			}
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("seatable authentication failed after refreshing the access token")
}

// Ping validates connectivity and credentials by forcing a fresh token
// exchange, which authenticates the Base API Token against the base.
func (c *Client) Ping(ctx context.Context) error {
	c.resetToken()
	_, _, _, err := c.ensure(ctx)
	return err
}

// ---------------------------------------------------------------------------
// Records (rows endpoint and SQL endpoint)
// ---------------------------------------------------------------------------

// rowsResponse is the shape of GET .../rows/.
type rowsResponse struct {
	Rows []map[string]any `json:"rows"`
}

// sqlResponse is the shape of POST .../sql/.
type sqlResponse struct {
	Results  []map[string]any `json:"results"`
	Metadata []sqlColumn      `json:"metadata"`
	Success  bool             `json:"success"`
}

type sqlColumn struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListRecords fetches rows for a table. Plain listings (no filter/sort/fields)
// use the rows endpoint (and may target a view); filtered, sorted or projected
// listings use the SQL endpoint. Both auto-paginate up to the requested limit
// (or a safety cap) and return rows normalized to keep only the `_id`, `_ctime`
// and `_mtime` identity columns alongside the user columns.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.TableName) == "" {
		return nil, fmt.Errorf("tableName is required")
	}
	if q.requiresSQL() {
		return c.listViaSQL(ctx, q)
	}
	return c.listViaRows(ctx, q)
}

// listViaRows paginates the List Rows endpoint via start/limit.
func (c *Client) listViaRows(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	out := make([]map[string]any, 0, rowsPageSize)
	var start int64
	for {
		page := int64(rowsPageSize)
		if remaining := hardLimit - int64(len(out)); remaining < page {
			page = remaining
		}
		if page <= 0 {
			break
		}

		params := url.Values{}
		params.Set("table_name", q.TableName)
		params.Set("convert_keys", "true")
		params.Set("limit", strconv.FormatInt(page, 10))
		params.Set("start", strconv.FormatInt(start, 10))
		if v := strings.TrimSpace(q.ViewName); v != "" {
			params.Set("view_name", v)
		}

		var res rowsResponse
		if err := c.gatewayJSON(ctx, http.MethodGet, "rows/", params, nil, &res); err != nil {
			return nil, err
		}
		for _, row := range res.Rows {
			out = append(out, normalizeRow(row))
		}

		if int64(len(res.Rows)) < page || int64(len(out)) >= hardLimit {
			break
		}
		start += int64(len(res.Rows))
	}
	return out, nil
}

// listViaSQL paginates a compiled SELECT via LIMIT/OFFSET.
func (c *Client) listViaSQL(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	base, params := BuildSelectSQL(q)
	out := make([]map[string]any, 0, sqlPageSize)
	var offset int64
	for {
		page := int64(sqlPageSize)
		if remaining := hardLimit - int64(len(out)); remaining < page {
			page = remaining
		}
		if page <= 0 {
			break
		}

		sql := fmt.Sprintf("%s LIMIT %d OFFSET %d", base, page, offset)
		rows, err := c.querySQL(ctx, sql, params)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, normalizeRow(row))
		}

		if int64(len(rows)) < page || int64(len(out)) >= hardLimit {
			break
		}
		offset += int64(len(rows))
	}
	return out, nil
}

// CountRecords returns the number of rows matching the query's filter via a SQL
// COUNT(*).
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.TableName) == "" {
		return 0, fmt.Errorf("tableName is required")
	}
	sql, params := BuildCountSQL(q)
	rows, err := c.querySQL(ctx, sql, params)
	if err != nil {
		return 0, err
	}
	return extractCount(rows), nil
}

// RunSQL executes a raw SeaTable SQL statement and returns its rows as-is (no
// identity-column normalization, so the caller sees exactly what they selected).
func (c *Client) RunSQL(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	sql := strings.TrimSpace(q.SQL)
	if sql == "" {
		return nil, fmt.Errorf("sql is required for the SQL query type")
	}
	return c.querySQL(ctx, sql, nil)
}

// querySQL posts a SQL statement (with optional parameters) to the SQL endpoint
// and returns the result rows keyed by column name (convert_keys=true).
func (c *Client) querySQL(ctx context.Context, sql string, params []any) ([]map[string]any, error) {
	body := map[string]any{
		"sql":          sql,
		"convert_keys": true,
	}
	if len(params) > 0 {
		body["parameters"] = params
	}
	var res sqlResponse
	if err := c.gatewayJSON(ctx, http.MethodPost, "sql/", nil, body, &res); err != nil {
		return nil, err
	}
	return res.Results, nil
}

// extractCount pulls the single numeric value out of a COUNT(*) result row.
func extractCount(rows []map[string]any) int64 {
	if len(rows) == 0 {
		return 0
	}
	for _, v := range rows[0] {
		if f, ok := toFloat(v); ok {
			return int64(f)
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Metadata (tables + columns) for the query editor
// ---------------------------------------------------------------------------

// ColumnInfo is a SeaTable column descriptor.
type ColumnInfo struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TableInfo is a SeaTable table with its columns.
type TableInfo struct {
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
}

type metadataResponse struct {
	Metadata struct {
		Tables []struct {
			ID      string       `json:"_id"`
			Name    string       `json:"name"`
			Columns []ColumnInfo `json:"columns"`
		} `json:"tables"`
	} `json:"metadata"`
}

// ListTables returns the base's tables (each with its columns) from the metadata
// endpoint.
func (c *Client) ListTables(ctx context.Context) ([]TableInfo, error) {
	var res metadataResponse
	if err := c.gatewayJSON(ctx, http.MethodGet, "metadata/", nil, nil, &res); err != nil {
		return nil, err
	}
	tables := make([]TableInfo, 0, len(res.Metadata.Tables))
	for _, t := range res.Metadata.Tables {
		cols := make([]ColumnInfo, 0, len(t.Columns))
		for _, col := range t.Columns {
			if strings.TrimSpace(col.Name) == "" {
				continue
			}
			cols = append(cols, col)
		}
		tables = append(tables, TableInfo{Name: t.Name, Columns: cols})
	}
	return tables, nil
}

// ---------------------------------------------------------------------------
// Row normalization
// ---------------------------------------------------------------------------

// identityColumns are the SeaTable row-metadata columns kept on normalized rows.
var identityColumns = map[string]bool{"_id": true, "_ctime": true, "_mtime": true}

// normalizeRow keeps the user columns plus the `_id`, `_ctime` and `_mtime`
// identity columns, dropping the other internal metadata fields (`_creator`,
// `_last_modifier`, `_locked`, `_locked_by`, `_archived`, …) so the rows endpoint
// and SQL `SELECT *` produce consistent frames.
func normalizeRow(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		if strings.HasPrefix(k, "_") && !identityColumns[k] {
			continue
		}
		out[k] = v
	}
	return out
}
