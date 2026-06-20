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

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	// ClickUp paginates task endpoints in pages of up to 100.
	tasksPageSize = 100
	maxRecords    = 100000
)

// Client is a thin wrapper around ClickUp's REST API.
type Client struct {
	baseURL    string
	token      string
	bearer     bool
	httpClient *http.Client
}

// NewClient creates a ClickUp API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = clickUpCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	token, bearer := settings.credential()
	return &Client{
		baseURL:    base,
		token:      token,
		bearer:     bearer,
		httpClient: httpClient,
	}, nil
}

// errorResponse is the JSON body ClickUp returns on error (e.g.
// {"err":"Team not found","ECODE":"TEAM_001"}).
type errorResponse struct {
	Err   string `json:"err"`
	ECODE string `json:"ECODE"`
}

// do issues a GET request to the given path (relative to the API root) and
// returns the raw response body. HTTP and ClickUp-level errors are surfaced as
// Go errors.
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
			// Personal tokens are sent raw (no Bearer prefix).
			req.Header.Set("Authorization", c.token)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to clickup failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading clickup response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// ClickUp returns a structured error body; surface its message when present.
		var e errorResponse
		if json.Unmarshal(raw, &e) == nil && e.Err != "" {
			return nil, fmt.Errorf("clickup returned status %d: %s", resp.StatusCode, truncate(e.Err))
		}
		return nil, fmt.Errorf("clickup returned status %d: %s", resp.StatusCode, truncate(string(raw)))
	}

	// A 2xx response can still carry an error body in some edge cases.
	var e errorResponse
	if json.Unmarshal(raw, &e) == nil && e.Err != "" {
		return nil, fmt.Errorf("clickup error: %s", truncate(e.Err))
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
	case queryTypeTasks:
		return c.listTasks(ctx, q)
	case queryTypeTeams:
		return c.listTeams(ctx)
	case queryTypeSpaces:
		return c.listSpaces(ctx, q)
	case queryTypeFolders:
		return c.listFolders(ctx, q)
	case queryTypeLists:
		return c.listLists(ctx, q)
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

// taskListResponse is the wrapper ClickUp returns for task list endpoints.
type taskListResponse struct {
	Tasks    []json.RawMessage `json:"tasks"`
	LastPage bool              `json:"last_page"`
}

// listTasks fetches tasks, choosing the most specific endpoint available from
// the query scope, paginates through pages, applies the task filters as query
// parameters, and flattens each task into a scalar record.
//
// Endpoint selection:
//   - ListId set    -> GET /v2/list/{list_id}/task        (tasks in a List)
//   - otherwise     -> GET /v2/team/{team_id}/task        (filtered team tasks),
//     scoped by space_ids/project_ids(folder)/list_ids when provided.
func (c *Client) listTasks(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	useList := strings.TrimSpace(q.ListId) != ""
	if !useList && strings.TrimSpace(q.TeamId) == "" {
		return nil, fmt.Errorf("a Workspace (team) or List is required for task queries")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	records := make([]map[string]any, 0, tasksPageSize)
	for page := 0; ; page++ {
		params := c.taskParams(q, page, useList)
		var path string
		if useList {
			path = fmt.Sprintf("/v2/list/%s/task?%s", url.PathEscape(q.ListId), params.Encode())
		} else {
			path = fmt.Sprintf("/v2/team/%s/task?%s", url.PathEscape(q.TeamId), params.Encode())
		}

		raw, err := c.do(ctx, path)
		if err != nil {
			return nil, err
		}
		var resp taskListResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("failed parsing tasks response: %w", err)
		}
		for _, t := range resp.Tasks {
			records = append(records, flattenTask(t))
			if len(records) >= hardLimit {
				return records, nil
			}
		}
		// Stop when ClickUp signals the last page, or returns fewer than a full
		// page, or returns nothing (defensive against missing last_page).
		if resp.LastPage || len(resp.Tasks) < tasksPageSize || len(resp.Tasks) == 0 {
			break
		}
	}
	return records, nil
}

// taskParams builds the query string for a task request from the editor inputs.
func (c *Client) taskParams(q QueryModel, page int, useList bool) url.Values {
	v := url.Values{}
	v.Set("page", strconv.Itoa(page))
	v.Set("order_by", normalizeOrderBy(q.OrderBy))
	if q.Reverse {
		v.Set("reverse", "true")
	}
	if q.IncludeSubtasks {
		v.Set("subtasks", "true")
	}
	if q.IncludeClosed {
		v.Set("include_closed", "true")
	}
	if q.IncludeArchived {
		v.Set("archived", "true")
	}

	// Scope filters only apply to the filtered-team-tasks endpoint.
	if !useList {
		if s := strings.TrimSpace(q.SpaceId); s != "" {
			v.Add("space_ids[]", s)
		}
		if f := strings.TrimSpace(q.FolderId); f != "" {
			v.Add("project_ids[]", f)
		}
		if l := strings.TrimSpace(q.ListId); l != "" {
			v.Add("list_ids[]", l)
		}
	}

	for _, s := range nonEmpty(q.Statuses) {
		v.Add("statuses[]", s)
	}
	for _, a := range nonEmpty(q.Assignees) {
		v.Add("assignees[]", a)
	}
	for _, t := range nonEmpty(q.Tags) {
		v.Add("tags[]", t)
	}

	addDateMode(v, "date_created_gt", "date_created_lt", q.CreatedMode, q.CreatedAfter, q.CreatedBefore, q.TimeRange)
	addDateMode(v, "date_updated_gt", "date_updated_lt", q.UpdatedMode, q.UpdatedAfter, q.UpdatedBefore, q.TimeRange)
	addDateMode(v, "due_date_gt", "due_date_lt", q.DueMode, q.DueAfter, q.DueBefore, q.TimeRange)

	return v
}

// addDateMode appends gt/lt Unix-millisecond bounds for a date filter based on
// its mode:
//   - "dashboard": use the panel time range (gt=From, lt=To)
//   - "custom":    use the explicit after/before bounds
//   - anything else ("any" / empty): no bounds added
func addDateMode(v url.Values, gtKey, ltKey, mode, after, before string, tr backend.TimeRange) {
	switch mode {
	case dateModeDashboard:
		if !tr.From.IsZero() {
			v.Set(gtKey, strconv.FormatInt(tr.From.UnixMilli(), 10))
		}
		if !tr.To.IsZero() {
			v.Set(ltKey, strconv.FormatInt(tr.To.UnixMilli(), 10))
		}
	case dateModeCustom:
		if ms, ok := toUnixMillis(after); ok {
			v.Set(gtKey, strconv.FormatInt(ms, 10))
		}
		if ms, ok := toUnixMillis(before); ok {
			v.Set(ltKey, strconv.FormatInt(ms, 10))
		}
	}
}

// toUnixMillis parses a bound that may be given either as Unix milliseconds or
// as an ISO-8601 / RFC3339 / date-only string, returning Unix milliseconds.
func toUnixMillis(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Already Unix millis?
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ms, true
	}
	if t, ok := toTime(s); ok {
		return t.UnixMilli(), true
	}
	return 0, false
}

// listRaw executes a user-provided REST GET path and flattens the response into
// rows. If RawRoot is set, that key's value is flattened; otherwise the first
// array of objects found anywhere in the response is used. When no array is
// found, the top-level object becomes a single row.
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

	if items, ok := findArray(raw); ok {
		records := make([]map[string]any, 0, len(items))
		for _, item := range items {
			records = append(records, flattenTask(item))
		}
		return records, nil
	}
	// No array found: flatten the top-level object into a single row.
	return []map[string]any{flattenTask(raw)}, nil
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
				records = append(records, flattenTask(item))
			}
			return records
		}
	}
	return []map[string]any{flattenTask(raw)}
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

// ---------------------------------------------------------------------------
// Predefined hierarchy queries: spaces / folders / lists / teams.
// ---------------------------------------------------------------------------

// listTeams fetches the authorized workspaces and flattens them into rows.
func (c *Client) listTeams(ctx context.Context) ([]map[string]any, error) {
	raw, err := c.do(ctx, "/v2/team")
	if err != nil {
		return nil, err
	}
	return flattenWrapped(raw, "teams")
}

// listSpaces fetches the spaces in a workspace.
func (c *Client) listSpaces(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	team := strings.TrimSpace(q.TeamId)
	if team == "" {
		return nil, fmt.Errorf("a Workspace (team) is required for spaces queries")
	}
	params := url.Values{}
	if q.IncludeArchived {
		params.Set("archived", "true")
	}
	raw, err := c.do(ctx, fmt.Sprintf("/v2/team/%s/space?%s", url.PathEscape(team), params.Encode()))
	if err != nil {
		return nil, err
	}
	return flattenWrapped(raw, "spaces")
}

// listFolders fetches the folders in a space.
func (c *Client) listFolders(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	space := strings.TrimSpace(q.SpaceId)
	if space == "" {
		return nil, fmt.Errorf("a Space is required for folders queries")
	}
	params := url.Values{}
	if q.IncludeArchived {
		params.Set("archived", "true")
	}
	raw, err := c.do(ctx, fmt.Sprintf("/v2/space/%s/folder?%s", url.PathEscape(space), params.Encode()))
	if err != nil {
		return nil, err
	}
	return flattenWrapped(raw, "folders")
}

// listLists fetches the lists in a folder, or the folderless lists in a space
// when only a space is provided.
func (c *Client) listLists(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	folder := strings.TrimSpace(q.FolderId)
	space := strings.TrimSpace(q.SpaceId)
	params := url.Values{}
	if q.IncludeArchived {
		params.Set("archived", "true")
	}
	var path string
	switch {
	case folder != "":
		path = fmt.Sprintf("/v2/folder/%s/list?%s", url.PathEscape(folder), params.Encode())
	case space != "":
		path = fmt.Sprintf("/v2/space/%s/list?%s", url.PathEscape(space), params.Encode())
	default:
		return nil, fmt.Errorf("a Folder or Space is required for lists queries")
	}
	raw, err := c.do(ctx, path)
	if err != nil {
		return nil, err
	}
	return flattenWrapped(raw, "lists")
}

// flattenWrapped extracts the named array from a ClickUp wrapper object (e.g.
// {"spaces":[...]}) and flattens each item into a record.
func flattenWrapped(raw json.RawMessage, key string) ([]map[string]any, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("failed parsing response: %w", err)
	}
	arrRaw, ok := top[key]
	if !ok {
		return nil, fmt.Errorf("response missing %q array", key)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(arrRaw, &items); err != nil {
		return nil, fmt.Errorf("failed parsing %q array: %w", key, err)
	}
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		records = append(records, flattenTask(item))
	}
	return records, nil
}

// Ping performs a minimal authenticated request (the authorized user endpoint)
// to validate connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "/v2/user")
	return err
}

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns.
// ---------------------------------------------------------------------------

// TeamInfo is a lightweight workspace representation for the workspace dropdown.
type TeamInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListTeams returns the workspaces available to the authenticated user.
func (c *Client) ListTeams(ctx context.Context) ([]TeamInfo, error) {
	raw, err := c.do(ctx, "/v2/team")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Teams []TeamInfo `json:"teams"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing teams: %w", err)
	}
	return resp.Teams, nil
}

// SpaceInfo is a lightweight space representation for the space dropdown.
type SpaceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListSpaces returns the spaces in a workspace.
func (c *Client) ListSpaces(ctx context.Context, teamID string) ([]SpaceInfo, error) {
	if strings.TrimSpace(teamID) == "" {
		return nil, fmt.Errorf("teamId is required")
	}
	raw, err := c.do(ctx, fmt.Sprintf("/v2/team/%s/space", url.PathEscape(teamID)))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Spaces []SpaceInfo `json:"spaces"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing spaces: %w", err)
	}
	return resp.Spaces, nil
}

// FolderInfo is a lightweight folder representation for the folder dropdown.
type FolderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListFolders returns the folders in a space.
func (c *Client) ListFolders(ctx context.Context, spaceID string) ([]FolderInfo, error) {
	if strings.TrimSpace(spaceID) == "" {
		return nil, fmt.Errorf("spaceId is required")
	}
	raw, err := c.do(ctx, fmt.Sprintf("/v2/space/%s/folder", url.PathEscape(spaceID)))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Folders []FolderInfo `json:"folders"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing folders: %w", err)
	}
	return resp.Folders, nil
}

// ClickUpListInfo is a lightweight list representation for the list dropdown.
type ClickUpListInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListLists returns the lists in a folder, or the folderless lists in a space
// when no folder is provided.
func (c *Client) ListLists(ctx context.Context, spaceID, folderID string) ([]ClickUpListInfo, error) {
	folder := strings.TrimSpace(folderID)
	space := strings.TrimSpace(spaceID)
	var path string
	switch {
	case folder != "":
		path = fmt.Sprintf("/v2/folder/%s/list", url.PathEscape(folder))
	case space != "":
		path = fmt.Sprintf("/v2/space/%s/list", url.PathEscape(space))
	default:
		return nil, fmt.Errorf("spaceId or folderId is required")
	}
	raw, err := c.do(ctx, path)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Lists []ClickUpListInfo `json:"lists"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing lists: %w", err)
	}
	return resp.Lists, nil
}

// MemberInfo is a lightweight workspace member representation for the assignee
// multi-select. The assignee filter on tasks uses the numeric user id.
type MemberInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// ListMembers returns the members of a workspace, used to populate the assignee
// multi-select. ClickUp returns members under each team's `members` array.
func (c *Client) ListMembers(ctx context.Context, teamID string) ([]MemberInfo, error) {
	raw, err := c.do(ctx, "/v2/team")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Teams []struct {
			ID      string `json:"id"`
			Members []struct {
				User struct {
					ID       json.Number `json:"id"`
					Username string      `json:"username"`
					Email    string      `json:"email"`
				} `json:"user"`
			} `json:"members"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing members: %w", err)
	}

	team := strings.TrimSpace(teamID)
	seen := map[string]bool{}
	members := make([]MemberInfo, 0)
	for _, t := range resp.Teams {
		if team != "" && t.ID != team {
			continue
		}
		for _, m := range t.Members {
			id := m.User.ID.String()
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			members = append(members, MemberInfo{ID: id, Username: m.User.Username, Email: m.User.Email})
		}
	}
	return members, nil
}
