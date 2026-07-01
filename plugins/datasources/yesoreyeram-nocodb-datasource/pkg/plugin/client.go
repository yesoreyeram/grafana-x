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
	defaultPageLimit = 100
	maxRecords       = 100000
)

// Client is a thin wrapper around the NocoDB REST API.
type Client struct {
	baseURL    string
	apiToken   string
	apiVersion string
	httpClient *http.Client
}

// NewClient creates a NocoDB API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	apiVersion := settings.APIVersion
	if apiVersion == "" {
		apiVersion = "v2"
	}
	return &Client{
		baseURL:    base,
		apiToken:   settings.apiToken,
		apiVersion: apiVersion,
		httpClient: httpClient,
	}, nil
}

func (c *Client) do(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiToken != "" {
		req.Header.Set("xc-token", c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to nocodb failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading nocodb response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("nocodb returned status %d: %s", resp.StatusCode, msg)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("failed parsing nocodb response: %w", err)
		}
	}
	return nil
}

// recordsResponse is the shape of GET /api/v2/tables/{tableId}/records.
type recordsResponse struct {
	List     []map[string]any `json:"list"`
	PageInfo struct {
		TotalRows   int  `json:"totalRows"`
		Page        int  `json:"page"`
		PageSize    int  `json:"pageSize"`
		IsFirstPage bool `json:"isFirstPage"`
		IsLastPage  bool `json:"isLastPage"`
	} `json:"pageInfo"`
}

// recordsResponseV3 is the shape of GET /api/v3/data/{baseId}/{tableId}/records.
type recordsResponseV3 struct {
	Records []struct {
		ID       any            `json:"id"`
		Fields   map[string]any `json:"fields"`
		IDFields map[string]any `json:"id_fields"`
	} `json:"records"`
	Next string `json:"next"`
}

// ListRecords fetches records for a table, transparently following pagination up
// to the requested limit (or maxRecords when no limit is provided).
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	out := make([]map[string]any, 0, defaultPageLimit)
	offset := 0

	for {
		pageSize := defaultPageLimit
		if remaining := hardLimit - len(out); remaining < pageSize {
			pageSize = remaining
		}
		if pageSize <= 0 {
			break
		}

		page, isLast, err := c.fetchRecordPage(ctx, q, pageSize, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)

		if isLast || len(page) == 0 || len(out) >= hardLimit {
			break
		}
		offset += len(page)
	}

	return out, nil
}

// useV3 reports whether the v3 data API should be used for record queries.
func (c *Client) useV3(q QueryModel) bool {
	return c.apiVersion == "v3" && strings.TrimSpace(q.BaseID) != ""
}

func (c *Client) recordsEndpoint(q QueryModel) string {
	// v3 uses a base-scoped path: /api/v3/data/{baseId}/{tableId}/records.
	// Fall back to the v2 path when v3 is selected but no base id is available.
	if c.useV3(q) {
		return fmt.Sprintf("%s/api/v3/data/%s/%s/records", c.baseURL, url.PathEscape(q.BaseID), url.PathEscape(q.TableID))
	}
	return fmt.Sprintf("%s/api/v2/tables/%s/records", c.baseURL, url.PathEscape(q.TableID))
}

// effectiveWhere returns the where clause for a query. The structured filter
// tree (if any) takes priority over a raw override. The `@` quoting prefix is
// only used for the v2 API.
func (c *Client) effectiveWhere(q QueryModel) string {
	where := strings.TrimSpace(q.Where)
	if q.filter != nil {
		if built := BuildWhere(q.filter, !c.useV3(q)); built != "" {
			where = built
		}
	}
	return where
}

func (c *Client) fetchRecordPage(ctx context.Context, q QueryModel, limit, offset int) ([]map[string]any, bool, error) {
	endpoint := c.recordsEndpoint(q)

	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))
	if v := strings.TrimSpace(q.ViewID); v != "" {
		params.Set("viewId", v)
	}
	if where := c.effectiveWhere(q); where != "" {
		params.Set("where", where)
	}
	if v := strings.TrimSpace(q.Sort); v != "" {
		params.Set("sort", v)
	}
	if v := strings.TrimSpace(q.Fields); v != "" {
		params.Set("fields", v)
	}

	if c.useV3(q) {
		var res recordsResponseV3
		if err := c.do(ctx, endpoint+"?"+params.Encode(), &res); err != nil {
			return nil, false, err
		}
		rows := make([]map[string]any, 0, len(res.Records))
		for _, rec := range res.Records {
			row := map[string]any{}
			for k, v := range rec.IDFields {
				row[k] = v
			}
			for k, v := range rec.Fields {
				row[k] = v
			}
			rows = append(rows, row)
		}
		// v3 signals more pages with a `next` URL; absence means last page.
		isLast := strings.TrimSpace(res.Next) == ""
		return rows, isLast, nil
	}

	var res recordsResponse
	if err := c.do(ctx, endpoint+"?"+params.Encode(), &res); err != nil {
		return nil, false, err
	}
	return res.List, res.PageInfo.IsLastPage, nil
}

type countResponse struct {
	Count int64 `json:"count"`
}

// CountRecords returns the number of records matching the query's filter. It
// uses the NocoDB count endpoint, which respects the `where` clause.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return 0, fmt.Errorf("tableId is required")
	}

	var endpoint string
	if c.useV3(q) {
		endpoint = fmt.Sprintf("%s/api/v3/data/%s/%s/count", c.baseURL, url.PathEscape(q.BaseID), url.PathEscape(q.TableID))
	} else {
		endpoint = fmt.Sprintf("%s/api/v2/tables/%s/records/count", c.baseURL, url.PathEscape(q.TableID))
	}

	params := url.Values{}
	if v := strings.TrimSpace(q.ViewID); v != "" {
		params.Set("viewId", v)
	}
	if where := c.effectiveWhere(q); where != "" {
		params.Set("where", where)
	}

	full := endpoint
	if encoded := params.Encode(); encoded != "" {
		full = endpoint + "?" + encoded
	}

	var res countResponse
	if err := c.do(ctx, full, &res); err != nil {
		return 0, err
	}
	return res.Count, nil
}

// TableInfo is a lightweight representation of a NocoDB table used for the
// resource handler that populates the QueryEditor table dropdown.
type TableInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	BaseID    string `json:"baseId"`
	BaseTitle string `json:"baseTitle"`
}

type tableListResponse struct {
	List []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"list"`
}

type baseListResponse struct {
	List []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"list"`
}

// ListBases returns all bases accessible with the configured token via the meta API.
func (c *Client) ListBases(ctx context.Context) ([]TableInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v2/meta/bases", c.baseURL)
	var res baseListResponse
	if err := c.do(ctx, endpoint, &res); err != nil {
		return nil, err
	}
	bases := make([]TableInfo, 0, len(res.List))
	for _, b := range res.List {
		bases = append(bases, TableInfo{ID: b.ID, Title: b.Title})
	}
	return bases, nil
}

// ListTables returns the tables in a single base via the meta API.
func (c *Client) ListTables(ctx context.Context, baseID string) ([]TableInfo, error) {
	if strings.TrimSpace(baseID) == "" {
		return nil, fmt.Errorf("baseId is required")
	}
	endpoint := fmt.Sprintf("%s/api/v2/meta/bases/%s/tables", c.baseURL, url.PathEscape(baseID))

	var res tableListResponse
	if err := c.do(ctx, endpoint, &res); err != nil {
		return nil, err
	}
	tables := make([]TableInfo, 0, len(res.List))
	for _, t := range res.List {
		tables = append(tables, TableInfo{ID: t.ID, Title: t.Title, BaseID: baseID})
	}
	return tables, nil
}

// ListAllTables returns the tables across every accessible base. The base title
// is attached so the UI can disambiguate tables with the same name.
func (c *Client) ListAllTables(ctx context.Context) ([]TableInfo, error) {
	bases, err := c.ListBases(ctx)
	if err != nil {
		return nil, err
	}

	tables := make([]TableInfo, 0)
	for _, base := range bases {
		baseTables, err := c.ListTables(ctx, base.ID)
		if err != nil {
			return nil, fmt.Errorf("failed listing tables for base %q: %w", base.Title, err)
		}
		for i := range baseTables {
			baseTables[i].BaseTitle = base.Title
		}
		tables = append(tables, baseTables...)
	}
	return tables, nil
}

// FieldInfo is a lightweight representation of a NocoDB column/field used for
// the resource handler that populates the QueryEditor fields multi-select.
type FieldInfo struct {
	Title string `json:"title"`
	Type  string `json:"type"`
}

type tableMetaResponse struct {
	Columns []struct {
		Title  string          `json:"title"`
		UIDT   string          `json:"uidt"`
		System json.RawMessage `json:"system"`
		PK     json.RawMessage `json:"pk"`
	} `json:"columns"`
}

// systemFieldUIDTs are NocoDB column types that represent internal/system fields
// (auto-managed columns and the record id) which are not useful to select.
var systemFieldUIDTs = map[string]bool{
	"ID":               true,
	"ForeignKey":       true,
	"CreatedTime":      true,
	"LastModifiedTime": true,
	"CreatedBy":        true,
	"LastModifiedBy":   true,
	"Order":            true,
	"Deleted":          true,
}

// isTruthyJSON reports whether a raw JSON value represents a truthy flag. NocoDB
// returns these flags inconsistently as 1/0, true/false or null.
func isTruthyJSON(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	switch s {
	case "", "null", "0", "false", `"0"`, `""`, `"false"`:
		return false
	default:
		return true
	}
}

// ListFields returns the user-defined columns/fields of a table via the meta
// API. System/internal fields (record id, created/updated metadata, ordering,
// soft-delete markers, etc.) are excluded.
func (c *Client) ListFields(ctx context.Context, tableID string) ([]FieldInfo, error) {
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	endpoint := fmt.Sprintf("%s/api/v2/meta/tables/%s", c.baseURL, url.PathEscape(tableID))

	var res tableMetaResponse
	if err := c.do(ctx, endpoint, &res); err != nil {
		return nil, err
	}
	fields := make([]FieldInfo, 0, len(res.Columns))
	for _, col := range res.Columns {
		if strings.TrimSpace(col.Title) == "" {
			continue
		}
		if isTruthyJSON(col.System) || systemFieldUIDTs[col.UIDT] {
			continue
		}
		fields = append(fields, FieldInfo{Title: col.Title, Type: col.UIDT})
	}
	return fields, nil
}

// ViewInfo is a lightweight representation of a NocoDB view used for the
// resource handler that populates the QueryEditor view dropdown.
type ViewInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type viewListResponse struct {
	List []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"list"`
}

// ListViews returns the views of a table via the meta API.
func (c *Client) ListViews(ctx context.Context, tableID string) ([]ViewInfo, error) {
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	endpoint := fmt.Sprintf("%s/api/v2/meta/tables/%s/views", c.baseURL, url.PathEscape(tableID))

	var res viewListResponse
	if err := c.do(ctx, endpoint, &res); err != nil {
		return nil, err
	}
	views := make([]ViewInfo, 0, len(res.List))
	for _, v := range res.List {
		views = append(views, ViewInfo{ID: v.ID, Title: v.Title})
	}
	return views, nil
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. It uses the bases list meta endpoint.
func (c *Client) Ping(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/api/v2/meta/bases", c.baseURL)
	return c.do(ctx, endpoint, nil)
}
