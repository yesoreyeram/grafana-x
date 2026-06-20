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
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	// Plane paginates list endpoints; the server caps per_page at 100.
	pageSize   = 100
	maxRecords = 100000
)

// Client is a thin wrapper around Plane's REST API.
type Client struct {
	baseURL     string
	token       string
	bearer      bool
	defaultSlug string
	httpClient  *http.Client
}

// NewClient creates a Plane API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = planeCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	token, bearer := settings.credential()
	return &Client{
		baseURL:     base,
		token:       token,
		bearer:      bearer,
		defaultSlug: strings.TrimSpace(settings.WorkspaceSlug),
		httpClient:  httpClient,
	}, nil
}

// errorResponse is a best-effort decode of Plane's JSON error body. Plane
// returns errors in a few shapes (e.g. {"error":"..."}, {"detail":"..."},
// {"message":"..."}); any present message is surfaced.
type errorResponse struct {
	Error   string `json:"error"`
	Detail  string `json:"detail"`
	Message string `json:"message"`
}

func (e errorResponse) message() string {
	switch {
	case e.Error != "":
		return e.Error
	case e.Detail != "":
		return e.Detail
	case e.Message != "":
		return e.Message
	default:
		return ""
	}
}

// do issues a GET request to the given path (relative to the API root) and
// returns the raw response body. HTTP and Plane-level errors are surfaced as Go
// errors.
func (c *Client) do(ctx context.Context, path string) (json.RawMessage, error) {
	full := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		if c.bearer {
			req.Header.Set("Authorization", "Bearer "+c.token)
		} else {
			req.Header.Set("X-API-Key", c.token)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to plane failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading plane response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e errorResponse
		if json.Unmarshal(raw, &e) == nil && e.message() != "" {
			return nil, fmt.Errorf("plane returned status %d: %s", resp.StatusCode, truncate(e.message()))
		}
		return nil, fmt.Errorf("plane returned status %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	return raw, nil
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

// ListRecords runs the appropriate predefined query (or the raw REST path) and
// returns flattened scalar records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	switch q.QueryType {
	case queryTypeWorkItems:
		return c.listWorkItems(ctx, q)
	case queryTypeProjects:
		return c.listProjects(ctx, q)
	case queryTypeStates:
		return c.listProjectScoped(ctx, q, "states")
	case queryTypeLabels:
		return c.listProjectScoped(ctx, q, "labels")
	case queryTypeCycles:
		return c.listProjectScoped(ctx, q, "cycles")
	case queryTypeModules:
		return c.listProjectScoped(ctx, q, "modules")
	case queryTypeMembers:
		return c.listMembers(ctx, q)
	case queryTypeRaw:
		return c.listRaw(ctx, q)
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

// pagedResponse is Plane's cursor-paginated envelope.
type pagedResponse struct {
	Results         []json.RawMessage `json:"results"`
	NextCursor      string            `json:"next_cursor"`
	NextPageResults bool              `json:"next_page_results"`
}

// listPaged walks Plane's cursor pagination for the given base path (which must
// already contain its leading "?" or have no query string), applying extra
// query params, and flattens each result into a record. It stops at the
// requested limit or when there are no more pages.
func (c *Client) listPaged(ctx context.Context, basePath string, params url.Values, limit int) ([]map[string]any, error) {
	return c.listPagedFiltered(ctx, basePath, params, limit, nil)
}

// listPagedFiltered is listPaged with an optional predicate applied to each raw
// item before flattening. Items for which keep returns false are skipped. The
// limit is applied AFTER filtering, so it bounds the number of matching rows
// (not the number of rows scanned). Pagination always continues until the API
// is exhausted or the post-filter limit is reached, because Plane's list
// endpoint does not filter server-side.
func (c *Client) listPagedFiltered(ctx context.Context, basePath string, params url.Values, limit int, keep func(json.RawMessage) bool) ([]map[string]any, error) {
	hardLimit := limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}
	if params == nil {
		params = url.Values{}
	}
	if params.Get("per_page") == "" {
		params.Set("per_page", strconv.Itoa(pageSize))
	}

	records := make([]map[string]any, 0, pageSize)
	cursor := ""
	for {
		p := cloneValues(params)
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		path := basePath + "?" + p.Encode()

		raw, err := c.do(ctx, path)
		if err != nil {
			return nil, err
		}
		var resp pagedResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("failed parsing paginated response: %w", err)
		}
		for _, item := range resp.Results {
			if keep != nil && !keep(item) {
				continue
			}
			records = append(records, flattenEntity(item))
			if len(records) >= hardLimit {
				return records, nil
			}
		}
		// Stop when Plane reports no further pages, gives no cursor, or returns
		// an empty page (defensive against missing flags).
		if !resp.NextPageResults || strings.TrimSpace(resp.NextCursor) == "" || len(resp.Results) == 0 {
			break
		}
		cursor = resp.NextCursor
	}
	return records, nil
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vals := range v {
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[k] = cp
	}
	return out
}

// listWorkItems fetches the work items in a project, paginating and flattening
// each item. Filters (priority / state / assignees / labels / created / updated)
// are applied to the raw work item objects in the backend, because Plane's List
// Work Items endpoint does NOT support filtering query parameters — it ignores
// unknown params and returns the full list. Only order_by / expand / per_page
// are sent to the API; everything else is matched here so the filters actually
// take effect.
func (c *Client) listWorkItems(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	slug := q.resolveWorkspace(c.defaultSlug)
	if slug == "" {
		return nil, fmt.Errorf("a Workspace slug is required for work item queries")
	}
	project := strings.TrimSpace(q.ProjectId)
	if project == "" {
		return nil, fmt.Errorf("a Project is required for work item queries")
	}

	params := c.workItemParams(q)
	keep := workItemFilter(q)
	basePath := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/",
		url.PathEscape(slug), url.PathEscape(project))
	return c.listPagedFiltered(ctx, basePath, params, q.Limit, keep)
}

// workItemParams builds the query string sent to Plane's List Work Items
// endpoint. Only parameters the endpoint actually honours are included:
// order_by, expand and per_page (set by the paginator). Filter parameters are
// NOT sent because the endpoint ignores them; filtering happens in
// workItemFilter instead.
func (c *Client) workItemParams(q QueryModel) url.Values {
	v := url.Values{}
	v.Set("order_by", normalizeOrderBy(q.OrderBy))
	if exp := nonEmpty(q.Expand); len(exp) > 0 {
		v.Set("expand", strings.Join(exp, ","))
	}
	return v
}

// workItemFilter returns a predicate that decides whether a raw work item JSON
// object matches all of the query's filters. Returns nil when no filters are
// set (so pagination is not slowed needlessly).
//
// Each filter group is OR within the group (any selected value matches) and AND
// across groups (an item must satisfy every active group). Matching is done on
// the raw API values:
//   - priority: the scalar string field "priority"
//   - state:    the "state" field, which is a UUID string (unexpanded) or an
//     object whose "id" is the UUID (expanded)
//   - assignees/labels: arrays of UUID strings (unexpanded) or arrays of objects
//     whose "id" is the UUID (expanded)
//   - created/updated: the "created_at" / "updated_at" RFC3339 timestamps
//     against the resolved date bounds
func workItemFilter(q QueryModel) func(json.RawMessage) bool {
	priorities := lowerSet(nonEmpty(q.Priorities))
	states := stringSet(nonEmpty(q.States))
	assignees := stringSet(nonEmpty(q.Assignees))
	labels := stringSet(nonEmpty(q.Labels))

	createdFrom, createdTo := resolveDateBounds(q.CreatedMode, q.CreatedAfter, q.CreatedBefore, q.TimeRange)
	updatedFrom, updatedTo := resolveDateBounds(q.UpdatedMode, q.UpdatedAfter, q.UpdatedBefore, q.TimeRange)

	hasDate := !createdFrom.IsZero() || !createdTo.IsZero() || !updatedFrom.IsZero() || !updatedTo.IsZero()
	if len(priorities) == 0 && len(states) == 0 && len(assignees) == 0 && len(labels) == 0 && !hasDate {
		return nil
	}

	return func(raw json.RawMessage) bool {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			return false
		}
		if len(priorities) > 0 {
			p := strings.ToLower(rawScalarString(item["priority"]))
			if !priorities[p] {
				return false
			}
		}
		if len(states) > 0 && !matchesRelation(item["state"], states) {
			return false
		}
		if len(assignees) > 0 && !matchesRelationList(item["assignees"], assignees) {
			return false
		}
		if len(labels) > 0 && !matchesRelationList(item["labels"], labels) {
			return false
		}
		if !withinBounds(item["created_at"], createdFrom, createdTo) {
			return false
		}
		if !withinBounds(item["updated_at"], updatedFrom, updatedTo) {
			return false
		}
		return true
	}
}

// resolveDateBounds returns the [from, to] time bounds for a date filter mode.
// Zero times mean "no bound on that side".
func resolveDateBounds(mode, after, before string, tr backend.TimeRange) (from, to time.Time) {
	switch mode {
	case dateModeDashboard:
		return tr.From, tr.To
	case dateModeCustom:
		if t, ok := toTime(strings.TrimSpace(after)); ok {
			from = t
		}
		if t, ok := toTime(strings.TrimSpace(before)); ok {
			to = t
		}
	}
	return from, to
}

// withinBounds reports whether a raw RFC3339 timestamp value falls within
// [from, to]. A zero bound is open on that side. A missing or invalid value
// passes only when there are no bounds at all (so items without the timestamp
// are excluded when a bound is active).
func withinBounds(rawVal json.RawMessage, from, to time.Time) bool {
	if from.IsZero() && to.IsZero() {
		return true
	}
	t, ok := toTime(rawScalarString(rawVal))
	if !ok {
		return false
	}
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && t.After(to) {
		return false
	}
	return true
}

// matchesRelation reports whether a relation field (UUID string or object with
// an "id") matches any value in the set.
func matchesRelation(rawVal json.RawMessage, set map[string]bool) bool {
	for _, id := range relationIDs(rawVal) {
		if set[id] {
			return true
		}
	}
	return false
}

// matchesRelationList reports whether any element of a relation array (UUID
// strings or objects with an "id") matches any value in the set.
func matchesRelationList(rawVal json.RawMessage, set map[string]bool) bool {
	trimmed := strings.TrimSpace(string(rawVal))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(rawVal, &items); err != nil {
		// Not an array; treat it as a single relation.
		return matchesRelation(rawVal, set)
	}
	for _, item := range items {
		for _, id := range relationIDs(item) {
			if set[id] {
				return true
			}
		}
	}
	return false
}

// relationIDs returns the candidate match values for a relation value: the raw
// scalar (a UUID string) and, when it is an object, its "id" field.
func relationIDs(rawVal json.RawMessage) []string {
	trimmed := strings.TrimSpace(string(rawVal))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		if id := rawScalarString(pickRaw(rawVal, "id")); id != "" {
			return []string{id}
		}
		return nil
	}
	if s := rawScalarString(rawVal); s != "" {
		return []string{s}
	}
	return nil
}

// pickRaw returns the raw value of a named field in a JSON object, or nil.
func pickRaw(rawVal json.RawMessage, field string) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(rawVal, &obj); err != nil {
		return nil
	}
	return obj[field]
}

// rawScalarString returns the string form of a raw JSON scalar (string or
// number); objects/arrays/null yield "".
func rawScalarString(rawVal json.RawMessage) string {
	trimmed := strings.TrimSpace(string(rawVal))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if strings.HasPrefix(trimmed, `"`) {
		var s string
		if err := json.Unmarshal(rawVal, &s); err == nil {
			return s
		}
		return ""
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return ""
	}
	return trimmed
}

// stringSet returns a set of the given values.
func stringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

// lowerSet returns a set of the lower-cased values.
func lowerSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[strings.ToLower(v)] = true
	}
	return out
}

// listProjects fetches the projects in a workspace.
func (c *Client) listProjects(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	slug := q.resolveWorkspace(c.defaultSlug)
	if slug == "" {
		return nil, fmt.Errorf("a Workspace slug is required for projects queries")
	}
	params := url.Values{}
	params.Set("order_by", normalizeOrderBy(q.OrderBy))
	basePath := fmt.Sprintf("/api/v1/workspaces/%s/projects/", url.PathEscape(slug))
	return c.listPaged(ctx, basePath, params, q.Limit)
}

// listProjectScoped fetches a project-scoped collection (states / labels /
// cycles / modules).
func (c *Client) listProjectScoped(ctx context.Context, q QueryModel, collection string) ([]map[string]any, error) {
	slug := q.resolveWorkspace(c.defaultSlug)
	if slug == "" {
		return nil, fmt.Errorf("a Workspace slug is required for %s queries", collection)
	}
	project := strings.TrimSpace(q.ProjectId)
	if project == "" {
		return nil, fmt.Errorf("a Project is required for %s queries", collection)
	}
	basePath := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/%s/",
		url.PathEscape(slug), url.PathEscape(project), collection)
	return c.listPaged(ctx, basePath, url.Values{}, q.Limit)
}

// listMembers fetches the members of a workspace.
func (c *Client) listMembers(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	slug := q.resolveWorkspace(c.defaultSlug)
	if slug == "" {
		return nil, fmt.Errorf("a Workspace slug is required for members queries")
	}
	basePath := fmt.Sprintf("/api/v1/workspaces/%s/members/", url.PathEscape(slug))
	// The members endpoint may return either a bare array or a paginated
	// envelope; handle both.
	raw, err := c.do(ctx, basePath)
	if err != nil {
		return nil, err
	}
	return flattenListResponse(raw), nil
}

// listRaw executes a user-provided REST GET path and flattens the response into
// rows. If RawRoot is set, that key's value is flattened; otherwise the
// "results" array (Plane's envelope) is used when present, falling back to the
// first array of objects found anywhere in the response. When no array is found,
// the top-level object becomes a single row.
func (c *Client) listRaw(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	path := strings.TrimSpace(q.RawPath)
	if path == "" {
		return nil, fmt.Errorf("rawPath is required for raw queries")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	raw, err := c.do(ctx, path)
	if err != nil {
		return nil, err
	}

	if root := strings.TrimSpace(q.RawRoot); root != "" {
		var top map[string]json.RawMessage
		if err := json.Unmarshal(raw, &top); err != nil {
			return nil, fmt.Errorf("failed parsing response: %w", err)
		}
		val, ok := top[root]
		if !ok {
			return nil, fmt.Errorf("response has no %q key", root)
		}
		return flattenAny(val), nil
	}
	return flattenListResponse(raw), nil
}

// flattenListResponse flattens a Plane list response that may be a bare array, a
// paginated envelope with a "results" array, an object with the first array of
// objects found, or a single object.
func flattenListResponse(raw json.RawMessage) []map[string]any {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		return flattenAny(raw)
	}
	// Prefer Plane's "results" envelope key.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err == nil {
		if results, ok := top["results"]; ok {
			if strings.HasPrefix(strings.TrimSpace(string(results)), "[") {
				return flattenAny(results)
			}
		}
	}
	if items, ok := findArray(raw); ok {
		records := make([]map[string]any, 0, len(items))
		for _, item := range items {
			records = append(records, flattenEntity(item))
		}
		return records
	}
	// No array found: flatten the top-level object into a single row.
	return []map[string]any{flattenEntity(raw)}
}

// flattenAny flattens a value that is either an array of objects (-> many rows)
// or a single object (-> one row).
func flattenAny(raw json.RawMessage) []map[string]any {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil {
			records := make([]map[string]any, 0, len(items))
			for _, item := range items {
				records = append(records, flattenEntity(item))
			}
			return records
		}
	}
	return []map[string]any{flattenEntity(raw)}
}

// findArray recursively searches a JSON response for the first array of objects
// and returns its items.
func findArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	for _, v := range obj {
		trimmed := strings.TrimSpace(string(v))
		if strings.HasPrefix(trimmed, "[") {
			var items []json.RawMessage
			if err := json.Unmarshal(v, &items); err == nil && len(items) > 0 {
				if strings.HasPrefix(strings.TrimSpace(string(items[0])), "{") {
					return items, true
				}
			}
		}
	}
	// Recurse into nested objects.
	for _, v := range obj {
		if strings.HasPrefix(strings.TrimSpace(string(v)), "{") {
			if items, ok := findArray(v); ok {
				return items, true
			}
		}
	}
	return nil, false
}

// Ping performs a minimal authenticated request (the current-user endpoint) to
// validate connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "/api/v1/users/me/")
	return err
}

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns.
// ---------------------------------------------------------------------------

// ProjectInfo is a lightweight project representation for the project dropdown.
type ProjectInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

// ListProjects returns the projects in a workspace, used to populate the project
// dropdown.
func (c *Client) ListProjects(ctx context.Context, slug string) ([]ProjectInfo, error) {
	slug = c.resolveSlug(slug)
	if slug == "" {
		return nil, fmt.Errorf("workspace slug is required")
	}
	records, err := c.listPaged(ctx,
		fmt.Sprintf("/api/v1/workspaces/%s/projects/", url.PathEscape(slug)),
		url.Values{}, maxRecords)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectInfo, 0, len(records))
	for _, r := range records {
		out = append(out, ProjectInfo{
			ID:         asString(r["id"]),
			Name:       asString(r["name"]),
			Identifier: asString(r["identifier"]),
		})
	}
	return out, nil
}

// StateInfo is a lightweight state representation for the state multi-select.
type StateInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

// ListStates returns the states in a project, used to populate the state filter.
func (c *Client) ListStates(ctx context.Context, slug, projectID string) ([]StateInfo, error) {
	slug = c.resolveSlug(slug)
	if slug == "" || strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("workspace slug and projectId are required")
	}
	records, err := c.listPaged(ctx,
		fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/states/", url.PathEscape(slug), url.PathEscape(projectID)),
		url.Values{}, maxRecords)
	if err != nil {
		return nil, err
	}
	out := make([]StateInfo, 0, len(records))
	for _, r := range records {
		out = append(out, StateInfo{
			ID:    asString(r["id"]),
			Name:  asString(r["name"]),
			Group: asString(r["state_group"]),
		})
	}
	return out, nil
}

// LabelInfo is a lightweight label representation for the label multi-select.
type LabelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListLabels returns the labels in a project, used to populate the label filter.
func (c *Client) ListLabels(ctx context.Context, slug, projectID string) ([]LabelInfo, error) {
	slug = c.resolveSlug(slug)
	if slug == "" || strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("workspace slug and projectId are required")
	}
	records, err := c.listPaged(ctx,
		fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/labels/", url.PathEscape(slug), url.PathEscape(projectID)),
		url.Values{}, maxRecords)
	if err != nil {
		return nil, err
	}
	out := make([]LabelInfo, 0, len(records))
	for _, r := range records {
		out = append(out, LabelInfo{ID: asString(r["id"]), Name: asString(r["name"])})
	}
	return out, nil
}

// MemberInfo is a lightweight workspace member representation for the assignee
// multi-select. The assignee filter on work items uses the member user UUID.
type MemberInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

// ListMembers returns the members of a workspace, used to populate the assignee
// multi-select. Plane returns each membership with a nested user; the filter
// needs the user's id.
func (c *Client) ListMembers(ctx context.Context, slug string) ([]MemberInfo, error) {
	slug = c.resolveSlug(slug)
	if slug == "" {
		return nil, fmt.Errorf("workspace slug is required")
	}
	raw, err := c.do(ctx, fmt.Sprintf("/api/v1/workspaces/%s/members/", url.PathEscape(slug)))
	if err != nil {
		return nil, err
	}

	items := extractArray(raw)
	out := make([]MemberInfo, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		m := parseMember(item)
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	return out, nil
}

// parseMember extracts a MemberInfo from a workspace membership object. The user
// fields may be inlined (member_id / display_name / email) or nested under a
// "member" object.
func parseMember(raw json.RawMessage) MemberInfo {
	var obj struct {
		ID          string `json:"id"`
		MemberID    string `json:"member_id"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Member      *struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"member"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return MemberInfo{}
	}
	m := MemberInfo{ID: obj.MemberID, DisplayName: obj.DisplayName, Email: obj.Email}
	if obj.Member != nil {
		if obj.Member.ID != "" {
			m.ID = obj.Member.ID
		}
		if obj.Member.DisplayName != "" {
			m.DisplayName = obj.Member.DisplayName
		}
		if obj.Member.Email != "" {
			m.Email = obj.Member.Email
		}
	}
	// Fall back to the membership id only if no user id was found.
	if m.ID == "" {
		m.ID = obj.ID
	}
	return m
}

// extractArray returns the array of items from a list response that may be a
// bare array or a paginated envelope.
func extractArray(raw json.RawMessage) []json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil {
			return items
		}
		return nil
	}
	var resp pagedResponse
	if err := json.Unmarshal(raw, &resp); err == nil && resp.Results != nil {
		return resp.Results
	}
	if items, ok := findArray(raw); ok {
		return items
	}
	return nil
}

// resolveSlug returns the provided slug or the data source default.
func (c *Client) resolveSlug(slug string) string {
	if s := strings.TrimSpace(slug); s != "" {
		return s
	}
	return c.defaultSlug
}

// asString coerces a flattened value to a string for resource DTOs.
func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := toString(v); ok {
		return strings.Trim(s, `"`)
	}
	return ""
}
