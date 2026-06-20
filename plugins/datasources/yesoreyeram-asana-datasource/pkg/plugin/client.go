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
	// Asana paginates with a page size of up to 100 objects.
	pageSize   = 100
	maxRecords = 100000
)

// Client is a thin wrapper around Asana's REST API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates an Asana API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = asanaCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		token:      settings.credential(),
		httpClient: httpClient,
	}, nil
}

// errorResponse is the JSON body Asana returns on error, e.g.
// {"errors":[{"message":"Not Found","help":"..."}]}.
type errorResponse struct {
	Errors []struct {
		Message string `json:"message"`
		Help    string `json:"help"`
	} `json:"errors"`
}

// dataPage is the standard Asana list envelope: results under "data", with an
// optional "next_page" cursor when more results exist.
type dataPage struct {
	Data     []json.RawMessage `json:"data"`
	NextPage *struct {
		Offset string `json:"offset"`
	} `json:"next_page"`
}

// do issues a GET request to the given path (relative to the API root) and
// returns the raw response body. HTTP and Asana-level errors are surfaced as Go
// errors.
func (c *Client) do(ctx context.Context, path string) (json.RawMessage, error) {
	full := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		// Both personal access tokens and OAuth tokens use the Bearer scheme.
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to asana failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading asana response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Asana returns a structured error body; surface its message when present.
		if msg, ok := firstError(raw); ok {
			return nil, fmt.Errorf("asana returned status %d: %s", resp.StatusCode, truncate(msg))
		}
		return nil, fmt.Errorf("asana returned status %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	return raw, nil
}

// firstError extracts the first error message from an Asana error body.
func firstError(raw json.RawMessage) (string, bool) {
	var e errorResponse
	if json.Unmarshal(raw, &e) == nil && len(e.Errors) > 0 && e.Errors[0].Message != "" {
		return e.Errors[0].Message, true
	}
	return "", false
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
	case queryTypeProjects:
		return c.listProjects(ctx, q)
	case queryTypeSections:
		return c.listSections(ctx, q)
	case queryTypeWorkspaces:
		return c.listWorkspaces(ctx, q)
	case queryTypeTeams:
		return c.listTeamsRecords(ctx, q)
	case queryTypeUsers:
		return c.listUsersRecords(ctx, q)
	case queryTypeTags:
		return c.listTagsRecords(ctx, q)
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

// listPaginated GETs a list endpoint, following Asana's next_page offset cursor
// up to the requested hard limit (or a safety cap), and flattens each item.
func (c *Client) listPaginated(ctx context.Context, path string, params url.Values, hardLimit int) ([]map[string]any, error) {
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}
	records := make([]map[string]any, 0, pageSize)
	offset := ""
	for {
		p := cloneValues(params)
		p.Set("limit", strconv.Itoa(pageSize))
		if offset != "" {
			p.Set("offset", offset)
		}
		raw, err := c.do(ctx, path+"?"+p.Encode())
		if err != nil {
			return nil, err
		}
		var page dataPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("failed parsing response: %w", err)
		}
		for _, item := range page.Data {
			records = append(records, flattenItem(item))
			if len(records) >= hardLimit {
				return records, nil
			}
		}
		if page.NextPage == nil || page.NextPage.Offset == "" {
			break
		}
		offset = page.NextPage.Offset
	}
	return records, nil
}

func cloneValues(in url.Values) url.Values {
	out := make(url.Values, len(in))
	for k, vs := range in {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

// listTasks fetches tasks, choosing the scope from the query. Asana requires one
// of: a section, a project, or an assignee together with a workspace.
func (c *Client) listTasks(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	params := url.Values{}
	params.Set("opt_fields", taskOptFields(q.Fields))

	switch {
	case strings.TrimSpace(q.Section) != "":
		params.Set("section", strings.TrimSpace(q.Section))
	case strings.TrimSpace(q.Project) != "":
		params.Set("project", strings.TrimSpace(q.Project))
	case strings.TrimSpace(q.Assignee) != "" && strings.TrimSpace(q.Workspace) != "":
		params.Set("assignee", strings.TrimSpace(q.Assignee))
		params.Set("workspace", strings.TrimSpace(q.Workspace))
	default:
		return nil, fmt.Errorf("a Project, a Section, or a Workspace together with an Assignee is required for task queries")
	}

	if q.IncompleteOnly {
		// completed_since=now returns only incomplete tasks.
		params.Set("completed_since", "now")
	}
	addModifiedSince(params, q.ModifiedMode, q.ModifiedSince, q.TimeRange)

	return c.listPaginated(ctx, "/tasks", params, q.Limit)
}

// addModifiedSince appends the modified_since bound based on its mode:
//   - "dashboard": use the panel time range "from"
//   - "custom":    use the explicit ISO-8601 bound
//   - anything else ("any" / empty): no bound added
func addModifiedSince(v url.Values, mode, custom string, tr backend.TimeRange) {
	switch mode {
	case dateModeDashboard:
		if !tr.From.IsZero() {
			v.Set("modified_since", tr.From.UTC().Format(time.RFC3339))
		}
	case dateModeCustom:
		if iso, ok := toISO(custom); ok {
			v.Set("modified_since", iso)
		}
	}
}

// toISO normalises a custom bound to an ISO-8601/RFC3339 string. Recognised
// date/date-time strings are reformatted; anything else is passed through as-is
// so users can supply an exact Asana-accepted value.
func toISO(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if t, ok := toTime(s); ok {
		return t.UTC().Format(time.RFC3339), true
	}
	return s, true
}

// listProjects fetches projects in a workspace and/or team.
func (c *Client) listProjects(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	workspace := strings.TrimSpace(q.Workspace)
	team := strings.TrimSpace(q.Team)
	if workspace == "" && team == "" {
		return nil, fmt.Errorf("a Workspace or Team is required for projects queries")
	}
	params := url.Values{}
	params.Set("opt_fields", projectOptFields)
	if workspace != "" {
		params.Set("workspace", workspace)
	}
	if team != "" {
		params.Set("team", team)
	}
	// Asana's `archived` is an exclusive filter: archived=false returns only
	// active projects, archived=true returns ONLY archived ones, and omitting it
	// returns both. "Include archived" therefore omits the param.
	if !q.IncludeArchived {
		params.Set("archived", "false")
	}
	return c.listPaginated(ctx, "/projects", params, q.Limit)
}

// listSections fetches the sections in a project.
func (c *Client) listSections(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	project := strings.TrimSpace(q.Project)
	if project == "" {
		return nil, fmt.Errorf("a Project is required for sections queries")
	}
	params := url.Values{}
	params.Set("opt_fields", sectionOptFields)
	return c.listPaginated(ctx, fmt.Sprintf("/projects/%s/sections", url.PathEscape(project)), params, q.Limit)
}

// listWorkspaces fetches the workspaces and organizations visible to the user.
func (c *Client) listWorkspaces(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	params := url.Values{}
	params.Set("opt_fields", workspaceOptFields)
	return c.listPaginated(ctx, "/workspaces", params, q.Limit)
}

// listTeamsRecords fetches the teams in a workspace (organizations only).
func (c *Client) listTeamsRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	workspace := strings.TrimSpace(q.Workspace)
	if workspace == "" {
		return nil, fmt.Errorf("a Workspace is required for teams queries")
	}
	params := url.Values{}
	params.Set("opt_fields", "gid,name,resource_type,description")
	return c.listPaginated(ctx, fmt.Sprintf("/workspaces/%s/teams", url.PathEscape(workspace)), params, q.Limit)
}

// listUsersRecords fetches the users in a workspace.
func (c *Client) listUsersRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	workspace := strings.TrimSpace(q.Workspace)
	if workspace == "" {
		return nil, fmt.Errorf("a Workspace is required for users queries")
	}
	params := url.Values{}
	params.Set("opt_fields", userOptFields)
	params.Set("workspace", workspace)
	return c.listPaginated(ctx, "/users", params, q.Limit)
}

// listTagsRecords fetches the tags in a workspace.
func (c *Client) listTagsRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	workspace := strings.TrimSpace(q.Workspace)
	if workspace == "" {
		return nil, fmt.Errorf("a Workspace is required for tags queries")
	}
	params := url.Values{}
	params.Set("opt_fields", tagOptFields)
	params.Set("workspace", workspace)
	return c.listPaginated(ctx, "/tags", params, q.Limit)
}

// listRaw executes a user-provided REST GET path and flattens the response into
// rows. If RawRoot is set, that key's value is flattened; otherwise the first
// array of objects found anywhere in the response is used (Asana wraps results
// under a top-level "data" key). When no array is found, the top-level object
// becomes a single row.
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
			records = append(records, flattenItem(item))
		}
		return records, nil
	}
	// No array found: flatten the top-level object into a single row. Asana wraps
	// single resources under "data", so prefer that when present.
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) == nil {
		if d, ok := top["data"]; ok && strings.HasPrefix(strings.TrimSpace(string(d)), "{") {
			return []map[string]any{flattenItem(d)}, nil
		}
	}
	return []map[string]any{flattenItem(raw)}, nil
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
				records = append(records, flattenItem(item))
			}
			return records
		}
	}
	return []map[string]any{flattenItem(raw)}
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

// Ping performs a minimal authenticated request (the authorized user endpoint)
// to validate connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "/users/me")
	return err
}

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns.
// ---------------------------------------------------------------------------

// ResourceInfo is a lightweight Asana resource (gid + name) for the editor
// dropdowns.
type ResourceInfo struct {
	Gid  string `json:"gid"`
	Name string `json:"name"`
}

// listResource GETs a compact list endpoint and returns its gid/name records.
func (c *Client) listResource(ctx context.Context, path string) ([]ResourceInfo, error) {
	raw, err := c.do(ctx, path)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []ResourceInfo `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing response: %w", err)
	}
	return resp.Data, nil
}

// ListWorkspaces returns the workspaces available to the authenticated user.
func (c *Client) ListWorkspaces(ctx context.Context) ([]ResourceInfo, error) {
	return c.listResource(ctx, "/workspaces?limit=100&opt_fields=name")
}

// ListTeams returns the teams in a workspace (organizations only).
func (c *Client) ListTeams(ctx context.Context, workspace string) ([]ResourceInfo, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	return c.listResource(ctx, fmt.Sprintf("/workspaces/%s/teams?limit=100&opt_fields=name", url.PathEscape(workspace)))
}

// ListProjects returns the projects in a workspace and/or team.
func (c *Client) ListProjects(ctx context.Context, workspace, team string) ([]ResourceInfo, error) {
	params := url.Values{}
	params.Set("limit", "100")
	params.Set("opt_fields", "name")
	params.Set("archived", "false")
	if w := strings.TrimSpace(workspace); w != "" {
		params.Set("workspace", w)
	}
	if t := strings.TrimSpace(team); t != "" {
		params.Set("team", t)
	}
	if params.Get("workspace") == "" && params.Get("team") == "" {
		return nil, fmt.Errorf("workspace or team is required")
	}
	return c.listResource(ctx, "/projects?"+params.Encode())
}

// ListSections returns the sections in a project.
func (c *Client) ListSections(ctx context.Context, project string) ([]ResourceInfo, error) {
	if strings.TrimSpace(project) == "" {
		return nil, fmt.Errorf("project is required")
	}
	return c.listResource(ctx, fmt.Sprintf("/projects/%s/sections?limit=100&opt_fields=name", url.PathEscape(project)))
}

// ListUsers returns the users in a workspace, used to populate the assignee
// picker.
func (c *Client) ListUsers(ctx context.Context, workspace string) ([]ResourceInfo, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	params := url.Values{}
	params.Set("limit", "100")
	params.Set("opt_fields", "name")
	params.Set("workspace", strings.TrimSpace(workspace))
	return c.listResource(ctx, "/users?"+params.Encode())
}

// ListTags returns the tags in a workspace.
func (c *Client) ListTags(ctx context.Context, workspace string) ([]ResourceInfo, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	params := url.Values{}
	params.Set("limit", "100")
	params.Set("opt_fields", "name")
	params.Set("workspace", strings.TrimSpace(workspace))
	return c.listResource(ctx, "/tags?"+params.Encode())
}
