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
	// defaultPageSize is the per-request limit hint. Coda's maximum page size is
	// not guaranteed and may differ per endpoint, so pagination always relies on
	// nextPageToken rather than this value.
	defaultPageSize = 200
	// maxRecords is a safety cap on the number of rows fetched when no limit is
	// supplied, to avoid unbounded paging.
	maxRecords = 100000
)

// Client is a thin wrapper around the Coda Web API.
type Client struct {
	baseURL    string
	apiToken   string
	configDoc  string
	httpClient *http.Client
}

// NewClient creates a Coda API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(codaDefaultURL), "/")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		apiToken:   strings.TrimSpace(settings.apiToken),
		configDoc:  strings.TrimSpace(settings.DocID),
		httpClient: httpClient,
	}, nil
}

// do issues a GET to Coda and decodes the JSON response into out (when non-nil).
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
		return fmt.Errorf("request to coda failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading coda response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := truncate(string(body), 500)
		// Coda error bodies carry useful `message`/`statusMessage` fields.
		var apiErr struct {
			Message       string `json:"message"`
			StatusMessage string `json:"statusMessage"`
		}
		if json.Unmarshal(body, &apiErr) == nil {
			if apiErr.Message != "" {
				msg = truncate(apiErr.Message, 500)
			} else if apiErr.StatusMessage != "" {
				msg = truncate(apiErr.StatusMessage, 500)
			}
		}
		if hint := statusHint(resp.StatusCode); hint != "" {
			return fmt.Errorf("coda returned status %d: %s. %s", resp.StatusCode, msg, hint)
		}
		return fmt.Errorf("coda returned status %d: %s", resp.StatusCode, msg)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("failed parsing coda response: %w", err)
		}
	}
	return nil
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
		return "The API token was missing or rejected — re-enter the API Token and click Save & test."
	case http.StatusForbidden:
		return "Access denied — ensure the token has access to this resource."
	case http.StatusNotFound:
		return "Not found — verify the Doc ID and Table id/name are correct and the token can access them."
	case http.StatusTooManyRequests:
		return "Rate limited by Coda — reduce the query frequency and retry."
	default:
		return ""
	}
}

func (c *Client) resolveDoc(queryDoc string) string {
	if d := strings.TrimSpace(queryDoc); d != "" {
		return d
	}
	return c.configDoc
}

// ---------------------------------------------------------------------------
// Docs
// ---------------------------------------------------------------------------

type docsResponse struct {
	Items         []docItem `json:"items"`
	NextPageToken string    `json:"nextPageToken"`
	NextPageLink  string    `json:"nextPageLink"`
}

type docItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListDocs returns the docs accessible with the configured token, following the
// pageToken cursor until exhausted.
func (c *Client) ListDocs(ctx context.Context) ([]DocInfo, error) {
	docs := make([]DocInfo, 0)
	pageToken := ""
	for {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(defaultPageSize))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		u := fmt.Sprintf("%s/docs?%s", c.baseURL, params.Encode())

		var res docsResponse
		if err := c.do(ctx, u, &res); err != nil {
			return nil, err
		}
		for _, d := range res.Items {
			docs = append(docs, DocInfo{ID: d.ID, Title: d.Name})
		}
		pageToken = strings.TrimSpace(res.NextPageToken)
		if pageToken == "" || len(res.Items) == 0 {
			break
		}
	}
	return docs, nil
}

// ---------------------------------------------------------------------------
// Tables
// ---------------------------------------------------------------------------

type tablesResponse struct {
	Items         []tableItem `json:"items"`
	NextPageToken string      `json:"nextPageToken"`
	NextPageLink  string      `json:"nextPageLink"`
}

type tableItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListTables returns the tables in a doc, following the pageToken cursor.
func (c *Client) ListTables(ctx context.Context, docID string) ([]TableInfo, error) {
	docID = c.resolveDoc(docID)
	if docID == "" {
		return nil, fmt.Errorf("docId is required")
	}
	tables := make([]TableInfo, 0)
	pageToken := ""
	for {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(defaultPageSize))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		u := fmt.Sprintf("%s/docs/%s/tables?%s", c.baseURL, url.PathEscape(docID), params.Encode())

		var res tablesResponse
		if err := c.do(ctx, u, &res); err != nil {
			return nil, err
		}
		for _, t := range res.Items {
			tables = append(tables, TableInfo{ID: t.ID, Title: t.Name})
		}
		pageToken = strings.TrimSpace(res.NextPageToken)
		if pageToken == "" || len(res.Items) == 0 {
			break
		}
	}
	return tables, nil
}

// tableDetail is the subset of GET /docs/{docId}/tables/{tableId} used here.
type tableDetail struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RowCount int64  `json:"rowCount"`
}

// GetTable returns metadata for a single table, including its rowCount (the
// total, unfiltered number of rows).
func (c *Client) GetTable(ctx context.Context, docID, tableID string) (tableDetail, error) {
	docID = c.resolveDoc(docID)
	if docID == "" {
		return tableDetail{}, fmt.Errorf("docId is required")
	}
	if strings.TrimSpace(tableID) == "" {
		return tableDetail{}, fmt.Errorf("tableId is required")
	}
	u := fmt.Sprintf("%s/docs/%s/tables/%s", c.baseURL, url.PathEscape(docID), url.PathEscape(tableID))
	var res tableDetail
	if err := c.do(ctx, u, &res); err != nil {
		return tableDetail{}, err
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// Columns
// ---------------------------------------------------------------------------

type columnsResponse struct {
	Items         []columnItem `json:"items"`
	NextPageToken string       `json:"nextPageToken"`
	NextPageLink  string       `json:"nextPageLink"`
}

// columnItem mirrors a Coda column. The resource-level `type` is always
// "column"; the data type lives in `format.type`.
type columnItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Format struct {
		Type string `json:"type"`
	} `json:"format"`
}

// ListColumns returns the columns of a table, following the pageToken cursor.
func (c *Client) ListColumns(ctx context.Context, docID, tableID string) ([]ColumnInfo, error) {
	docID = c.resolveDoc(docID)
	if docID == "" {
		return nil, fmt.Errorf("docId is required")
	}
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	columns := make([]ColumnInfo, 0)
	pageToken := ""
	for {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(defaultPageSize))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		u := fmt.Sprintf("%s/docs/%s/tables/%s/columns?%s", c.baseURL, url.PathEscape(docID), url.PathEscape(tableID), params.Encode())

		var res columnsResponse
		if err := c.do(ctx, u, &res); err != nil {
			return nil, err
		}
		for _, col := range res.Items {
			if strings.TrimSpace(col.Name) == "" {
				continue
			}
			columns = append(columns, ColumnInfo{Title: col.Name, Type: col.Format.Type})
		}
		pageToken = strings.TrimSpace(res.NextPageToken)
		if pageToken == "" || len(res.Items) == 0 {
			break
		}
	}
	return columns, nil
}

// ---------------------------------------------------------------------------
// Rows
// ---------------------------------------------------------------------------

type rowsResponse struct {
	Items         []rowItem `json:"items"`
	NextPageToken string    `json:"nextPageToken"`
	NextPageLink  string    `json:"nextPageLink"`
}

// rowItem mirrors a Coda row. Cell data lives in the `values` map, keyed by
// column name when useColumnNames=true (which the client always sends).
type rowItem struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Index       *float64       `json:"index"`
	Href        string         `json:"href"`
	BrowserLink string         `json:"browserLink"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	Values      map[string]any `json:"values"`
}

// ListRows fetches rows for a table, transparently following the pageToken
// cursor up to the requested limit (or maxRecords when no limit is provided).
func (c *Client) ListRows(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	docID := c.resolveDoc(q.DocID)
	if docID == "" {
		return nil, fmt.Errorf("docId is required")
	}
	if strings.TrimSpace(q.TableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	keep := splitColumns(q.Columns)

	out := make([]map[string]any, 0, defaultPageSize)
	pageToken := ""
	for {
		size := defaultPageSize
		if remaining := hardLimit - len(out); remaining < size {
			size = remaining
		}
		if size <= 0 {
			break
		}

		items, next, err := c.fetchRowPage(ctx, docID, q, size, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, flattenRows(items, keep)...)

		if next == "" || len(items) == 0 || len(out) >= hardLimit {
			break
		}
		pageToken = next
	}

	if len(out) > hardLimit {
		out = out[:hardLimit]
	}
	return out, nil
}

// rowParams builds the query parameters shared by ListRows and CountRows.
func rowParams(q QueryModel, size int) url.Values {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(size))
	// Always request column names so cell values are keyed by readable column
	// names rather than opaque column ids.
	params.Set("useColumnNames", "true")

	valueFormat := defaultValueFormat
	if validValueFormat[q.ValueFormat] {
		valueFormat = q.ValueFormat
	}
	params.Set("valueFormat", valueFormat)

	if filter := effectiveFilterQuery(q); filter != "" {
		params.Set("query", filter)
	}
	if validSortBy[q.SortBy] {
		params.Set("sortBy", q.SortBy)
	}
	// Only ever send visibleOnly=true; sending visibleOnly=false alongside
	// sortBy=natural is rejected by Coda.
	if q.VisibleOnly {
		params.Set("visibleOnly", "true")
	}
	return params
}

func (c *Client) fetchRowPage(ctx context.Context, docID string, q QueryModel, size int, pageToken string) ([]rowItem, string, error) {
	params := rowParams(q, size)
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	u := fmt.Sprintf("%s/docs/%s/tables/%s/rows?%s",
		c.baseURL, url.PathEscape(docID), url.PathEscape(q.TableID), params.Encode())

	var res rowsResponse
	if err := c.do(ctx, u, &res); err != nil {
		return nil, "", err
	}
	return res.Items, strings.TrimSpace(res.NextPageToken), nil
}

// flattenRows converts Coda rows into flat records keyed by column name. The
// row's metadata (id, name, index, createdAt, updatedAt, href, browserLink) is
// preserved as synthetic columns. When keep is non-empty, only those data
// columns are retained (metadata columns are always kept) — Coda's rows endpoint
// has no column-projection parameter, so projection is done here.
func flattenRows(items []rowItem, keep map[string]bool) []map[string]any {
	hasProjection := len(keep) > 0
	out := make([]map[string]any, 0, len(items))
	for _, r := range items {
		row := make(map[string]any, len(r.Values)+7)
		if r.ID != "" {
			row["id"] = r.ID
		}
		if r.Name != "" {
			row["name"] = r.Name
		}
		if r.Index != nil {
			row["index"] = *r.Index
		}
		if r.CreatedAt != "" {
			row["createdAt"] = r.CreatedAt
		}
		if r.UpdatedAt != "" {
			row["updatedAt"] = r.UpdatedAt
		}
		if r.Href != "" {
			row["href"] = r.Href
		}
		if r.BrowserLink != "" {
			row["browserLink"] = r.BrowserLink
		}
		for col, val := range r.Values {
			if hasProjection && !keep[col] {
				continue
			}
			row[col] = val
		}
		out = append(out, row)
	}
	return out
}

// CountRows returns the number of rows in a table.
//
// When a filter (query) is present, Coda offers no filtered-count endpoint, so
// the matching rows are paginated and counted. When no filter is present, the
// table's rowCount (the total, unfiltered row count) is read directly — a single
// request instead of paginating every row.
func (c *Client) CountRows(ctx context.Context, q QueryModel) (int64, error) {
	docID := c.resolveDoc(q.DocID)
	if docID == "" {
		return 0, fmt.Errorf("docId is required")
	}
	if strings.TrimSpace(q.TableID) == "" {
		return 0, fmt.Errorf("tableId is required")
	}

	if effectiveFilterQuery(q) != "" {
		return c.countByPaginating(ctx, docID, q)
	}

	table, err := c.GetTable(ctx, docID, q.TableID)
	if err != nil {
		return 0, err
	}
	return table.RowCount, nil
}

// countByPaginating counts filtered rows by following the pageToken cursor.
func (c *Client) countByPaginating(ctx context.Context, docID string, q QueryModel) (int64, error) {
	var count int64
	pageToken := ""
	for {
		params := rowParams(q, defaultPageSize)
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		u := fmt.Sprintf("%s/docs/%s/tables/%s/rows?%s",
			c.baseURL, url.PathEscape(docID), url.PathEscape(q.TableID), params.Encode())

		var res rowsResponse
		if err := c.do(ctx, u, &res); err != nil {
			return 0, err
		}
		count += int64(len(res.Items))
		pageToken = strings.TrimSpace(res.NextPageToken)
		if pageToken == "" || len(res.Items) == 0 {
			break
		}
	}
	return count, nil
}

// Ping validates connectivity and credentials via the whoami endpoint, which any
// valid token can call.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, c.baseURL+"/whoami", nil)
}

// splitColumns parses a comma-separated column list into a lookup set.
func splitColumns(columns string) map[string]bool {
	out := map[string]bool{}
	for _, col := range strings.Split(columns, ",") {
		col = strings.TrimSpace(col)
		if col != "" {
			out[col] = true
		}
	}
	return out
}
