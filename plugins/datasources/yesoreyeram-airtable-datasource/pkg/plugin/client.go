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
	// Airtable caps the page size at 100 records per request.
	defaultPageSize = 100
	// Safety cap on the number of records fetched when no limit is given.
	maxRecords = 100000
)

// Client is a thin wrapper around the Airtable Web API. It authenticates with a
// personal access token (PAT) sent as `Authorization: Bearer <token>`.
type Client struct {
	baseURL    string
	apiToken   string
	configBase string
	httpClient *http.Client
}

// NewClient creates an Airtable API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = airtableDefaultURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		apiToken:   strings.TrimSpace(settings.apiToken),
		configBase: strings.TrimSpace(settings.BaseID),
		httpClient: httpClient,
	}, nil
}

// do performs a GET request to rawURL and unmarshals the JSON body into out.
func (c *Client) do(ctx context.Context, rawURL string, out any) error {
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
		return fmt.Errorf("request to airtable failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading airtable response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("airtable returned status %d: %s. %s",
			resp.StatusCode, truncate(string(body), 500), statusHint(resp.StatusCode))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("failed parsing airtable response: %w", err)
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

// statusHint returns an actionable message for common Airtable error statuses.
func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The personal access token was missing or rejected — re-enter the API Token and click Save & test (saved secrets are write-only, so an empty re-save can blank the token)."
	case http.StatusForbidden:
		return "Access denied — ensure the token has the required scopes (data.records:read, schema.bases:read) and access to this base."
	case http.StatusNotFound:
		return "Not found — verify the Base ID (app...) and Table id/name are correct and the token can access them."
	case http.StatusUnprocessableEntity:
		return "Unprocessable request — check the filter formula, sort fields and field names."
	default:
		return ""
	}
}

// resolveBase returns the base id to use: the query's base id, falling back to
// the data-source-configured base id.
func (c *Client) resolveBase(queryBase string) string {
	if b := strings.TrimSpace(queryBase); b != "" {
		return b
	}
	return c.configBase
}

// recordsResponse is the shape of GET /v0/{baseId}/{tableIdOrName}.
type recordsResponse struct {
	Records []recordItem `json:"records"`
	Offset  string       `json:"offset"`
}

type recordItem struct {
	ID          string         `json:"id"`
	CreatedTime string         `json:"createdTime"`
	Fields      map[string]any `json:"fields"`
}

// ListRecords fetches records for a table, transparently following the offset
// cursor up to the requested limit (or maxRecords when no limit is provided).
//
// Each returned row is a flat map that includes the synthetic `_id` and
// `_createdTime` columns plus the record's user fields.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	baseID := c.resolveBase(q.BaseID)
	if baseID == "" {
		return nil, fmt.Errorf("baseId is required")
	}
	if strings.TrimSpace(q.TableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	out := make([]map[string]any, 0, defaultPageSize)
	offset := ""

	for {
		size := defaultPageSize
		if remaining := hardLimit - len(out); remaining < size {
			size = remaining
		}
		if size <= 0 {
			break
		}

		rows, next, err := c.fetchRecordPage(ctx, baseID, q, size, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)

		if next == "" || len(rows) == 0 || len(out) >= hardLimit {
			break
		}
		offset = next
	}

	return out, nil
}

func (c *Client) recordsEndpoint(baseID, tableID string) string {
	return fmt.Sprintf("%s/v0/%s/%s", c.baseURL, url.PathEscape(baseID), url.PathEscape(tableID))
}

// recordParams builds the shared query parameters used by both ListRecords and
// CountRecords (view, filterByFormula, sort, fields). pageSize/offset are added
// by the caller.
func (c *Client) recordParams(q QueryModel) url.Values {
	params := url.Values{}
	if v := strings.TrimSpace(q.ViewID); v != "" {
		params.Set("view", v)
	}
	if formula := c.effectiveFormula(q); formula != "" {
		params.Set("filterByFormula", formula)
	}
	// sort is an indexed array parameter: sort[0][field], sort[0][direction], ...
	idx := 0
	for _, s := range q.sortItems {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		params.Set(fmt.Sprintf("sort[%d][field]", idx), field)
		dir := "asc"
		if strings.EqualFold(s.Direction, "desc") {
			dir = "desc"
		}
		params.Set(fmt.Sprintf("sort[%d][direction]", idx), dir)
		idx++
	}
	return params
}

// effectiveFormula returns the Airtable filterByFormula for a query. A raw
// formula (advanced field) takes precedence over the structured tree.
func (c *Client) effectiveFormula(q QueryModel) string {
	if raw := strings.TrimSpace(q.FilterByFormula); raw != "" {
		return raw
	}
	return BuildFormula(q.filter)
}

func (c *Client) fetchRecordPage(ctx context.Context, baseID string, q QueryModel, size int, offset string) ([]map[string]any, string, error) {
	params := c.recordParams(q)
	params.Set("pageSize", strconv.Itoa(size))
	if offset != "" {
		params.Set("offset", offset)
	}
	if v := strings.TrimSpace(q.Fields); v != "" {
		for _, f := range strings.Split(v, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				params.Add("fields[]", f)
			}
		}
	}

	var res recordsResponse
	if err := c.do(ctx, c.recordsEndpoint(baseID, q.TableID)+"?"+params.Encode(), &res); err != nil {
		return nil, "", err
	}

	rows := make([]map[string]any, 0, len(res.Records))
	for _, rec := range res.Records {
		row := make(map[string]any, len(rec.Fields)+2)
		row["_id"] = rec.ID
		if rec.CreatedTime != "" {
			row["_createdTime"] = rec.CreatedTime
		}
		for k, v := range rec.Fields {
			row[k] = v
		}
		rows = append(rows, row)
	}
	return rows, strings.TrimSpace(res.Offset), nil
}

// CountRecords returns the number of records matching the query's filter.
// Airtable has no count endpoint, so it paginates with a minimal payload
// (requesting no user fields) and counts the records.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	baseID := c.resolveBase(q.BaseID)
	if baseID == "" {
		return 0, fmt.Errorf("baseId is required")
	}
	if strings.TrimSpace(q.TableID) == "" {
		return 0, fmt.Errorf("tableId is required")
	}

	var count int64
	offset := ""
	for {
		params := c.recordParams(q)
		params.Set("pageSize", strconv.Itoa(defaultPageSize))
		// Request only the record id metadata; ask for no user fields to keep the
		// payload minimal. An empty fields[] selection still returns record ids.
		params.Set("fields[]", "")
		if offset != "" {
			params.Set("offset", offset)
		}

		var res recordsResponse
		if err := c.do(ctx, c.recordsEndpoint(baseID, q.TableID)+"?"+params.Encode(), &res); err != nil {
			return 0, err
		}
		count += int64(len(res.Records))
		next := strings.TrimSpace(res.Offset)
		if next == "" || len(res.Records) == 0 {
			break
		}
		offset = next
	}
	return count, nil
}

// BaseInfo is a lightweight representation of an Airtable base used to populate
// the base dropdown in the query editor.
type BaseInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type basesResponse struct {
	Bases []struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		PermissionLevel string `json:"permissionLevel"`
	} `json:"bases"`
	Offset string `json:"offset"`
}

// ListBases returns every base accessible to the token (via the metadata API,
// following pagination). Requires the schema.bases:read scope.
func (c *Client) ListBases(ctx context.Context) ([]BaseInfo, error) {
	bases := make([]BaseInfo, 0)
	offset := ""
	for {
		endpoint := c.baseURL + "/v0/meta/bases"
		if offset != "" {
			endpoint += "?offset=" + url.QueryEscape(offset)
		}
		var res basesResponse
		if err := c.do(ctx, endpoint, &res); err != nil {
			return nil, err
		}
		for _, b := range res.Bases {
			bases = append(bases, BaseInfo{ID: b.ID, Title: b.Name})
		}
		if strings.TrimSpace(res.Offset) == "" {
			break
		}
		offset = res.Offset
	}
	return bases, nil
}

// schemaResponse is the shape of GET /v0/meta/bases/{baseId}/tables.
type schemaResponse struct {
	Tables []schemaTable `json:"tables"`
}

type schemaTable struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	PrimaryFieldID string        `json:"primaryFieldId"`
	Fields         []schemaField `json:"fields"`
	Views          []schemaView  `json:"views"`
}

type schemaField struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type schemaView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// getSchema fetches the full schema (tables, fields, views) for a base.
func (c *Client) getSchema(ctx context.Context, baseID string) (schemaResponse, error) {
	var res schemaResponse
	if strings.TrimSpace(baseID) == "" {
		return res, fmt.Errorf("baseId is required")
	}
	endpoint := fmt.Sprintf("%s/v0/meta/bases/%s/tables", c.baseURL, url.PathEscape(baseID))
	if err := c.do(ctx, endpoint, &res); err != nil {
		return res, err
	}
	return res, nil
}

// TableInfo is a lightweight representation of an Airtable table used for the
// resource handler that populates the QueryEditor table dropdown.
type TableInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListTables returns the tables of a base (id + name). Requires the
// schema.bases:read scope.
func (c *Client) ListTables(ctx context.Context, baseID string) ([]TableInfo, error) {
	schema, err := c.getSchema(ctx, c.resolveBase(baseID))
	if err != nil {
		return nil, err
	}
	tables := make([]TableInfo, 0, len(schema.Tables))
	for _, t := range schema.Tables {
		tables = append(tables, TableInfo{ID: t.ID, Title: t.Name})
	}
	return tables, nil
}

// FieldInfo is a lightweight representation of an Airtable field used for the
// resource handler that populates the QueryEditor fields multi-select.
type FieldInfo struct {
	Title string `json:"title"`
	Type  string `json:"type"`
}

// ListFields returns the fields of a table within a base. The tableID may be a
// table id (tbl...) or a table name.
func (c *Client) ListFields(ctx context.Context, baseID, tableID string) ([]FieldInfo, error) {
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	schema, err := c.getSchema(ctx, c.resolveBase(baseID))
	if err != nil {
		return nil, err
	}
	table := findTable(schema, tableID)
	if table == nil {
		return []FieldInfo{}, nil
	}
	fields := make([]FieldInfo, 0, len(table.Fields))
	for _, f := range table.Fields {
		if strings.TrimSpace(f.Name) == "" {
			continue
		}
		fields = append(fields, FieldInfo{Title: f.Name, Type: f.Type})
	}
	return fields, nil
}

// ViewInfo is a lightweight representation of an Airtable view used for the
// resource handler that populates the QueryEditor view dropdown.
type ViewInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListViews returns the views of a table within a base.
func (c *Client) ListViews(ctx context.Context, baseID, tableID string) ([]ViewInfo, error) {
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	schema, err := c.getSchema(ctx, c.resolveBase(baseID))
	if err != nil {
		return nil, err
	}
	table := findTable(schema, tableID)
	if table == nil {
		return []ViewInfo{}, nil
	}
	views := make([]ViewInfo, 0, len(table.Views))
	for _, v := range table.Views {
		views = append(views, ViewInfo{ID: v.ID, Title: v.Name})
	}
	return views, nil
}

// findTable returns the table whose id or name matches idOrName, or nil.
func findTable(schema schemaResponse, idOrName string) *schemaTable {
	for i := range schema.Tables {
		if schema.Tables[i].ID == idOrName || schema.Tables[i].Name == idOrName {
			return &schema.Tables[i]
		}
	}
	return nil
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. It calls the metadata `whoami` endpoint, which only requires a
// valid token (no specific base access).
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, c.baseURL+"/v0/meta/whoami", nil)
}
