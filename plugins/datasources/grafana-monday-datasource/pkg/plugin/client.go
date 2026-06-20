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

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

const (
	// monday.com caps items_page at 500; we use a conservative page size that
	// works for all connection types.
	defaultPageSize = 100
	// itemsPageSize is the per-page size for items_page (monday allows up to 500).
	itemsPageSize = 100
	maxRecords    = 100000
)

// Client is a thin wrapper around monday.com's GraphQL API.
type Client struct {
	baseURL    string
	token      string
	bearer     bool
	apiVersion string
	httpClient *http.Client
}

// NewClient creates a monday.com API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimSpace(settings.BaseURL)
	if base == "" {
		base = mondayCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	token, bearer := settings.credential()
	return &Client{
		baseURL:    base,
		token:      token,
		bearer:     bearer,
		apiVersion: strings.TrimSpace(settings.APIVersion),
		httpClient: httpClient,
	}, nil
}

// effectiveAPIVersion returns the API version actually sent on requests: the
// configured one, or the bundled default when unset.
func (c *Client) effectiveAPIVersion() string {
	if c.apiVersion == "" {
		return defaultAPIVersion
	}
	return c.apiVersion
}

// graphqlRequest is the JSON body of a GraphQL POST.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphqlError is a single error entry returned by the GraphQL endpoint.
type graphqlError struct {
	Message string `json:"message"`
}

// graphqlResponse is the envelope returned for every GraphQL request. monday.com
// can return errors either in the standard `errors` array or as a top-level
// `error_message` (for some auth / rate-limit cases).
type graphqlResponse struct {
	Data         json.RawMessage `json:"data"`
	Errors       []graphqlError  `json:"errors"`
	ErrorMessage string          `json:"error_message"`
}

// do issues a GraphQL request and returns the raw `data` object. GraphQL-level
// errors (which arrive with HTTP 200) are surfaced as a Go error.
func (c *Client) do(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	payload, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("failed encoding request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	// Always pin an API version. monday's account default can be older than the
	// version that supports newer features (e.g. `aggregate` with `group_by`),
	// so fall back to a known-good recent version when none is configured.
	req.Header.Set("API-Version", c.effectiveAPIVersion())
	if c.token != "" {
		if c.bearer {
			req.Header.Set("Authorization", "Bearer "+c.token)
		} else {
			// Personal API tokens are sent raw (no Bearer prefix).
			req.Header.Set("Authorization", c.token)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to monday.com failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading monday.com response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("monday.com returned status %d: %s", resp.StatusCode, truncate(string(raw)))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed parsing monday.com response: %w", err)
	}
	if strings.TrimSpace(gqlResp.ErrorMessage) != "" {
		return nil, fmt.Errorf("monday.com error: %s", truncate(gqlResp.ErrorMessage))
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, 0, len(gqlResp.Errors))
		for _, e := range gqlResp.Errors {
			if e.Message != "" {
				msgs = append(msgs, e.Message)
			}
		}
		return nil, fmt.Errorf("monday.com graphql error: %s", truncate(strings.Join(msgs, "; ")))
	}
	return gqlResp.Data, nil
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

// ListRecords runs the appropriate predefined query (or the raw GraphQL
// document) and returns flattened scalar records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	switch q.QueryType {
	case queryTypeRaw:
		return c.listRaw(ctx, q)
	case queryTypeItems:
		if q.isGrouped() {
			return c.listAggregate(ctx, q)
		}
		return c.listItems(ctx, q)
	case queryTypeBoards:
		return c.listBoards(ctx, q)
	case queryTypeGroups:
		return c.listGroups(ctx, q)
	case queryTypeUsers:
		return c.listPaged(ctx, q, usersQuery, "users", false)
	case queryTypeWorkspaces:
		return c.listPaged(ctx, q, workspacesQuery, "workspaces", true)
	case queryTypeTags:
		return c.listTags(ctx, q)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.QueryType)
	}
}

// CountRecords returns the number of records matching the query.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	records, err := c.ListRecords(ctx, q)
	if err != nil {
		return 0, err
	}
	return int64(len(records)), nil
}

// hardLimit returns the effective record cap for a query.
func hardLimit(q QueryModel) int {
	if q.Limit > 0 {
		return q.Limit
	}
	return maxRecords
}

// idArgs converts an ID slice into a []any of trimmed, non-empty IDs (for use as
// GraphQL variables).
func idArgs(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// idStrings returns the trimmed, non-empty entries of an ID slice as []string
// (for splicing into a GraphQL document).
func idStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// buildItemsFilterRules assembles the monday.com ItemsQuery `rules` array from
// the editor's filter inputs (name search + group filter). Returns nil when there
// is nothing to filter. Shared by items_page filtering and the aggregate query.
func buildItemsFilterRules(q QueryModel) []any {
	rules := make([]any, 0)
	if s := strings.TrimSpace(q.SearchQuery); s != "" {
		rules = append(rules, map[string]any{
			"column_id":     "name",
			"compare_value": []any{s},
			"operator":      "contains_text",
		})
	}
	if groups := idArgs(q.GroupIds); len(groups) > 0 {
		rules = append(rules, map[string]any{
			"column_id":     "group",
			"compare_value": groups,
			"operator":      "any_of",
		})
	}
	if len(rules) == 0 {
		return nil
	}
	return rules
}

// buildItemsQueryParams assembles the monday.com `ItemsQuery` object (rules +
// order) for the items query. Returns nil when there is nothing to filter/order.
func buildItemsQueryParams(q QueryModel) map[string]any {
	params := map[string]any{}

	if rules := buildItemsFilterRules(q); rules != nil {
		params["rules"] = rules
	}

	if col := strings.TrimSpace(q.OrderBy); col != "" {
		direction := "asc"
		if strings.EqualFold(q.OrderDir, "desc") {
			direction = "desc"
		}
		params["order_by"] = []any{map[string]any{"column_id": col, "direction": direction}}
	}

	if len(params) == 0 {
		return nil
	}
	return params
}

// isAggregateUnsupportedError reports whether err is monday.com rejecting the
// `aggregate` query at the schema level (i.e. the feature is unavailable on the
// resolved API version). monday returns messages like:
//
//	Cannot query field "aggregate" on type "Query".
//	Unknown type "AggregateBasicAggregationResult".
//	Unknown type "AggregateGroupByResult".
func isAggregateUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "aggregate") &&
		(strings.Contains(msg, "cannot query field") || strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "does not exist")) {
		return true
	}
	return strings.Contains(msg, "unknown type") && strings.Contains(msg, "aggregate")
}

// listAggregate runs monday.com's server-side `aggregate` query to group items by
// a column and compute an aggregation (count/sum/avg/min/max/count_distinct),
// without downloading the raw items. The `aggregate` query operates on a single
// board, so it is run once per selected board; when multiple boards are selected
// a `board` column is added to each row to disambiguate.
func (c *Client) listAggregate(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	boards := idStrings(q.BoardIds)
	if len(boards) == 0 {
		return nil, fmt.Errorf("at least one board is required for grouped/aggregated item queries")
	}

	resultCol := aggregationColumnName(q.Aggregation, q.AggregationColumn)
	filterRules := buildItemsFilterRules(q)
	multiBoard := len(boards) > 1
	limit := hardLimit(q)

	out := make([]map[string]any, 0)
	for _, boardID := range boards {
		doc, variables, err := buildAggregateQuery(boardID, q.GroupBy, q.Aggregation, q.AggregationColumn, filterRules)
		if err != nil {
			return nil, err
		}
		data, err := c.do(ctx, doc, variables)
		if err != nil {
			if isAggregateUnsupportedError(err) {
				return nil, fmt.Errorf(
					"grouping uses monday.com's aggregate API, which is not available on API version %q. "+
						"Set a newer API version (e.g. 2026-01 or later) in the data source settings, or remove the Group by. (%w)",
					c.effectiveAPIVersion(), err)
			}
			return nil, err
		}
		// Log the raw aggregate response so the group-by value shape can be
		// inspected (visible with GF_LOG_LEVEL=debug).
		log.DefaultLogger.Debug("monday aggregate response", "board", boardID, "groupBy", q.GroupBy, "data", string(data))
		rows, err := parseAggregateResults(data, q.GroupBy, q.GroupBy, resultCol)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if multiBoard {
				row["board_id"] = boardID
			}
			out = append(out, row)
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// listItems paginates items across the requested boards using cursor-based
// `items_page` / `next_items_page`, flattening each item (including its column
// values) into a scalar record.
func (c *Client) listItems(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	boards := idArgs(q.BoardIds)
	if len(boards) == 0 {
		return nil, fmt.Errorf("at least one board is required for the items query")
	}

	withColumns := q.includeColumns()
	limit := hardLimit(q)
	queryParams := buildItemsQueryParams(q)

	records := make([]map[string]any, 0, itemsPageSize)

	// First page: one items_page per board.
	firstQuery := buildItemsQuery(withColumns, idStrings(q.ColumnIds))
	pageSize := itemsPageSize
	if remaining := limit - len(records); remaining < pageSize {
		pageSize = remaining
	}
	variables := map[string]any{"boardIds": boards, "limit": pageSize}
	if queryParams != nil {
		variables["queryParams"] = queryParams
	}
	data, err := c.do(ctx, firstQuery, variables)
	if err != nil {
		return nil, err
	}

	var firstResp struct {
		Boards []struct {
			ItemsPage struct {
				Cursor string            `json:"cursor"`
				Items  []json.RawMessage `json:"items"`
			} `json:"items_page"`
		} `json:"boards"`
	}
	if err := json.Unmarshal(data, &firstResp); err != nil {
		return nil, fmt.Errorf("failed parsing items response: %w", err)
	}

	cursors := make([]string, 0, len(firstResp.Boards))
	for _, b := range firstResp.Boards {
		for _, item := range b.ItemsPage.Items {
			if len(records) >= limit {
				break
			}
			records = append(records, flattenItem(item, withColumns, q.HideSystemColumns))
		}
		if b.ItemsPage.Cursor != "" {
			cursors = append(cursors, b.ItemsPage.Cursor)
		}
	}

	// Follow each board's cursor for subsequent pages.
	nextQuery := buildNextItemsQuery(withColumns, idStrings(q.ColumnIds))
	for _, cursor := range cursors {
		for cursor != "" && len(records) < limit {
			pageSize := itemsPageSize
			if remaining := limit - len(records); remaining < pageSize {
				pageSize = remaining
			}
			nextData, err := c.do(ctx, nextQuery, map[string]any{"cursor": cursor, "limit": pageSize})
			if err != nil {
				return nil, err
			}
			var nextResp struct {
				NextItemsPage struct {
					Cursor string            `json:"cursor"`
					Items  []json.RawMessage `json:"items"`
				} `json:"next_items_page"`
			}
			if err := json.Unmarshal(nextData, &nextResp); err != nil {
				return nil, fmt.Errorf("failed parsing next items response: %w", err)
			}
			for _, item := range nextResp.NextItemsPage.Items {
				if len(records) >= limit {
					break
				}
				records = append(records, flattenItem(item, withColumns, q.HideSystemColumns))
			}
			if len(nextResp.NextItemsPage.Items) == 0 {
				break
			}
			cursor = nextResp.NextItemsPage.Cursor
		}
	}

	return records, nil
}

// listBoards paginates boards using page/limit and flattens each board node.
func (c *Client) listBoards(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	limit := hardLimit(q)
	state := q.State
	if !validState(state) {
		state = stateActive
	}
	records := make([]map[string]any, 0, defaultPageSize)
	page := 1
	for len(records) < limit {
		pageSize := defaultPageSize
		if remaining := limit - len(records); remaining < pageSize {
			pageSize = remaining
		}
		variables := map[string]any{"limit": pageSize, "page": page, "state": state}
		if ids := idArgs(q.BoardIds); len(ids) > 0 {
			variables["ids"] = ids
		}
		if ws := idArgs(q.WorkspaceIds); len(ws) > 0 {
			variables["workspaceIds"] = ws
		}
		data, err := c.do(ctx, boardsQuery, variables)
		if err != nil {
			return nil, err
		}
		nodes, err := decodeArray(data, "boards")
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			records = append(records, flattenNode(node))
		}
		// ids/state pagination: monday returns all matching ids on page 1.
		if len(nodes) < pageSize || len(idArgs(q.BoardIds)) > 0 {
			break
		}
		page++
	}
	return records, nil
}

// listGroups fetches the groups of the requested boards. Groups are not
// paginated; one request returns them all.
func (c *Client) listGroups(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	boards := idArgs(q.BoardIds)
	if len(boards) == 0 {
		return nil, fmt.Errorf("at least one board is required for the groups query")
	}
	data, err := c.do(ctx, groupsQuery, map[string]any{"boardIds": boards})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Boards []struct {
			ID     json.RawMessage   `json:"id"`
			Name   json.RawMessage   `json:"name"`
			Groups []json.RawMessage `json:"groups"`
		} `json:"boards"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing groups response: %w", err)
	}
	records := make([]map[string]any, 0)
	limit := hardLimit(q)
	for _, b := range resp.Boards {
		boardID := flattenValue(b.ID)
		boardName := flattenValue(b.Name)
		for _, g := range b.Groups {
			if len(records) >= limit {
				return records, nil
			}
			row := flattenNode(g)
			row["board"] = boardName
			row["board_id"] = boardID
			records = append(records, row)
		}
	}
	return records, nil
}

// listTags fetches the account tags (not paginated).
func (c *Client) listTags(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	data, err := c.do(ctx, tagsQuery, nil)
	if err != nil {
		return nil, err
	}
	nodes, err := decodeArray(data, "tags")
	if err != nil {
		return nil, err
	}
	limit := hardLimit(q)
	records := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		if len(records) >= limit {
			break
		}
		records = append(records, flattenNode(node))
	}
	return records, nil
}

// listPaged paginates a simple page/limit collection (users, workspaces) and
// flattens each node. When withState is true the State variable is sent.
func (c *Client) listPaged(ctx context.Context, q QueryModel, query, field string, withState bool) ([]map[string]any, error) {
	limit := hardLimit(q)
	state := q.State
	if !validState(state) {
		state = stateActive
	}
	records := make([]map[string]any, 0, defaultPageSize)
	page := 1
	for len(records) < limit {
		pageSize := defaultPageSize
		if remaining := limit - len(records); remaining < pageSize {
			pageSize = remaining
		}
		variables := map[string]any{"limit": pageSize, "page": page}
		if withState {
			variables["state"] = state
		}
		data, err := c.do(ctx, query, variables)
		if err != nil {
			return nil, err
		}
		nodes, err := decodeArray(data, field)
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			records = append(records, flattenNode(node))
		}
		if len(nodes) < pageSize {
			break
		}
		page++
	}
	return records, nil
}

// listRaw executes the user-provided GraphQL document, finds the first array of
// objects (a collection) in the response, and flattens it. When no collection is
// found, the whole response is flattened into one row.
func (c *Client) listRaw(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.RawQuery) == "" {
		return nil, fmt.Errorf("rawQuery is required for raw queries")
	}
	var variables map[string]any
	if strings.TrimSpace(q.RawVariables) != "" {
		if err := json.Unmarshal([]byte(q.RawVariables), &variables); err != nil {
			return nil, fmt.Errorf("invalid rawVariables json: %w", err)
		}
	}

	data, err := c.do(ctx, q.RawQuery, variables)
	if err != nil {
		return nil, err
	}

	if nodes, ok := findCollection(data); ok {
		records := make([]map[string]any, 0, len(nodes))
		for _, node := range nodes {
			records = append(records, flattenNode(node))
		}
		return records, nil
	}

	// No collection found: flatten the top-level data object into a single row.
	return []map[string]any{flattenNode(data)}, nil
}

// decodeArray extracts a named top-level array from a GraphQL `data` object.
func decodeArray(data json.RawMessage, field string) ([]json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("failed parsing response data: %w", err)
	}
	raw, ok := top[field]
	if !ok {
		return nil, fmt.Errorf("response missing %q field", field)
	}
	var nodes []json.RawMessage
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, fmt.Errorf("failed parsing %q array: %w", field, err)
	}
	return nodes, nil
}

// findCollection recursively searches a GraphQL response for an array of objects
// and returns its elements. It prefers the deepest collection (e.g. the `items`
// inside `boards[].items_page` rather than the outer `boards` array), so nested
// item responses flatten to the most useful rows. Arrays of scalars are skipped.
func findCollection(raw json.RawMessage) ([]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}

	// Prefer a deeper collection found by recursing into nested objects and the
	// elements of object-arrays. Only fall back to an array on this object when
	// nothing deeper exists.
	for _, v := range obj {
		if nested, ok := findCollection(v); ok {
			return nested, true
		}
		if arr, ok := asObjectArray(v); ok {
			for _, el := range arr {
				if nested, ok := findCollection(el); ok {
					return nested, true
				}
			}
		}
	}

	// No deeper collection: use the first object array on this object.
	for _, v := range obj {
		if arr, ok := asObjectArray(v); ok {
			return arr, true
		}
	}
	return nil, false
}

// asObjectArray returns the elements of raw if it is a JSON array whose first
// element is an object.
func asObjectArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed[0] != '[' {
		return nil, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return nil, false
	}
	if first := strings.TrimSpace(string(arr[0])); first == "" || first[0] != '{' {
		return nil, false
	}
	return arr, true
}

// Ping performs a minimal authenticated request (the `me` query) to validate
// connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, `query { me { id name } }`, nil)
	return err
}

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns.
// ---------------------------------------------------------------------------

// BoardInfo is a lightweight board representation for the board dropdown.
type BoardInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListBoards returns the boards in the account for the board picker.
func (c *Client) ListBoards(ctx context.Context) ([]BoardInfo, error) {
	query := `query Boards($limit: Int!, $page: Int!) {
  boards(limit: $limit, page: $page) {
    id
    name
  }
}`
	boards := make([]BoardInfo, 0)
	page := 1
	for {
		data, err := c.do(ctx, query, map[string]any{"limit": defaultPageSize, "page": page})
		if err != nil {
			return nil, err
		}
		nodes, err := decodeArray(data, "boards")
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			var b BoardInfo
			if err := json.Unmarshal(node, &b); err == nil && b.ID != "" {
				boards = append(boards, b)
			}
		}
		if len(nodes) < defaultPageSize {
			break
		}
		page++
	}
	return boards, nil
}

// GroupInfo is a group representation for the group multi-select.
type GroupInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListGroups returns the groups of the given boards for the group picker.
func (c *Client) ListGroups(ctx context.Context, boardIDs []string) ([]GroupInfo, error) {
	boards := idArgs(boardIDs)
	if len(boards) == 0 {
		return []GroupInfo{}, nil
	}
	data, err := c.do(ctx, groupsQuery, map[string]any{"boardIds": boards})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Boards []struct {
			Groups []GroupInfo `json:"groups"`
		} `json:"boards"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing groups response: %w", err)
	}
	seen := map[string]bool{}
	groups := make([]GroupInfo, 0)
	for _, b := range resp.Boards {
		for _, g := range b.Groups {
			if g.ID == "" || seen[g.ID] {
				continue
			}
			seen[g.ID] = true
			groups = append(groups, g)
		}
	}
	return groups, nil
}

// ColumnInfo is a board column for the columns multi-select.
type ColumnInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// ListColumns returns the columns of the given boards for the column picker.
func (c *Client) ListColumns(ctx context.Context, boardIDs []string) ([]ColumnInfo, error) {
	boards := idArgs(boardIDs)
	if len(boards) == 0 {
		return []ColumnInfo{}, nil
	}
	query := `query Columns($boardIds: [ID!]) {
  boards(ids: $boardIds) {
    columns { id title type }
  }
}`
	data, err := c.do(ctx, query, map[string]any{"boardIds": boards})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Boards []struct {
			Columns []ColumnInfo `json:"columns"`
		} `json:"boards"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing columns response: %w", err)
	}
	seen := map[string]bool{}
	columns := make([]ColumnInfo, 0)
	for _, b := range resp.Boards {
		for _, col := range b.Columns {
			if col.ID == "" || seen[col.ID] {
				continue
			}
			seen[col.ID] = true
			columns = append(columns, col)
		}
	}
	return columns, nil
}

// WorkspaceInfo is a lightweight workspace representation for the workspace
// multi-select.
type WorkspaceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListWorkspaces returns the workspaces in the account.
func (c *Client) ListWorkspaces(ctx context.Context) ([]WorkspaceInfo, error) {
	query := `query Workspaces($limit: Int!, $page: Int!) {
  workspaces(limit: $limit, page: $page) {
    id
    name
  }
}`
	workspaces := make([]WorkspaceInfo, 0)
	page := 1
	for {
		data, err := c.do(ctx, query, map[string]any{"limit": defaultPageSize, "page": page})
		if err != nil {
			return nil, err
		}
		nodes, err := decodeArray(data, "workspaces")
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			var w WorkspaceInfo
			if err := json.Unmarshal(node, &w); err == nil && w.ID != "" {
				workspaces = append(workspaces, w)
			}
		}
		if len(nodes) < defaultPageSize {
			break
		}
		page++
	}
	return workspaces, nil
}
