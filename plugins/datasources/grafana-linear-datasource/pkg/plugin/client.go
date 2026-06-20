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
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	// Linear caps connection page sizes at 250.
	defaultPageSize = 250
	maxRecords      = 100000
)

// Supported query types.
const (
	queryTypeIssues   = "issues"
	queryTypeProjects = "projects"
	queryTypeTeams    = "teams"
	queryTypeUsers    = "users"
	queryTypeCycles   = "cycles"
	queryTypeRaw      = "raw"
)

// Client is a thin wrapper around Linear's GraphQL API.
type Client struct {
	baseURL    string
	token      string
	bearer     bool
	httpClient *http.Client
}

// NewClient creates a Linear API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimSpace(settings.BaseURL)
	if base == "" {
		base = linearCloudURL
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

// graphqlRequest is the JSON body of a GraphQL POST.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphqlError is a single error entry returned by the GraphQL endpoint.
type graphqlError struct {
	Message string `json:"message"`
}

// graphqlResponse is the envelope returned for every GraphQL request.
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors"`
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
	if c.token != "" {
		if c.bearer {
			req.Header.Set("Authorization", "Bearer "+c.token)
		} else {
			// Personal API keys are sent raw (no Bearer prefix).
			req.Header.Set("Authorization", c.token)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to linear failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading linear response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("linear returned status %d: %s", resp.StatusCode, truncate(string(raw)))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed parsing linear response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, 0, len(gqlResp.Errors))
		for _, e := range gqlResp.Errors {
			if e.Message != "" {
				msgs = append(msgs, e.Message)
			}
		}
		return nil, fmt.Errorf("linear graphql error: %s", truncate(strings.Join(msgs, "; ")))
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

// connection is the generic shape of a Linear GraphQL connection.
type connection struct {
	Nodes    []json.RawMessage `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

// ListRecords runs the appropriate predefined query (or the raw GraphQL
// document) and returns flattened scalar records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if q.QueryType == queryTypeRaw {
		return c.listRaw(ctx, q)
	}
	return c.listConnection(ctx, q)
}

// listConnection paginates a predefined entity connection, following the
// cursor-based pagination up to the requested limit (or maxRecords when no limit
// is provided), and flattens each node into a scalar record.
func (c *Client) listConnection(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	root, err := connectionField(q.QueryType)
	if err != nil {
		return nil, err
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	query := buildConnectionQuery(q)
	filter := buildFilter(q)
	orderBy := normalizeOrderBy(q.OrderBy)

	records := make([]map[string]any, 0, defaultPageSize)
	cursor := ""
	for {
		pageSize := defaultPageSize
		if remaining := hardLimit - len(records); remaining < pageSize {
			pageSize = remaining
		}
		if pageSize <= 0 {
			break
		}

		variables := map[string]any{"first": pageSize, "orderBy": orderBy}
		if cursor != "" {
			variables["after"] = cursor
		}
		if filter != nil {
			variables["filter"] = filter
		}
		// includeArchived is only meaningful for the issues query (the only one
		// that declares the variable); harmlessly ignored otherwise.
		if q.QueryType == queryTypeIssues {
			variables["includeArchived"] = q.IncludeArchived
		}

		data, err := c.do(ctx, query, variables)
		if err != nil {
			return nil, err
		}

		conn, err := decodeConnection(data, root)
		if err != nil {
			return nil, err
		}
		for _, node := range conn.Nodes {
			records = append(records, flattenNode(node))
		}

		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" || len(conn.Nodes) == 0 || len(records) >= hardLimit {
			break
		}
		cursor = conn.PageInfo.EndCursor
	}

	return records, nil
}

// listRaw executes the user-provided GraphQL document, finds the first
// connection (object with a `nodes` array) in the response, and flattens it.
// When no connection is found, the whole response is flattened into one row.
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

	if nodes, ok := findNodes(data); ok {
		records := make([]map[string]any, 0, len(nodes))
		for _, node := range nodes {
			records = append(records, flattenNode(node))
		}
		return records, nil
	}

	// No connection found: flatten the top-level data object into a single row.
	return []map[string]any{flattenNode(data)}, nil
}

// CountRecords returns the number of records matching the query.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	records, err := c.ListRecords(ctx, q)
	if err != nil {
		return 0, err
	}
	return int64(len(records)), nil
}

// connectionField maps a query type to the top-level connection field name.
func connectionField(queryType string) (string, error) {
	switch queryType {
	case queryTypeIssues:
		return "issues", nil
	case queryTypeProjects:
		return "projects", nil
	case queryTypeTeams:
		return "teams", nil
	case queryTypeUsers:
		return "users", nil
	case queryTypeCycles:
		return "cycles", nil
	default:
		return "", fmt.Errorf("unsupported query type: %s", queryType)
	}
}

// decodeConnection extracts the named connection from a GraphQL `data` object.
func decodeConnection(data json.RawMessage, field string) (connection, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return connection{}, fmt.Errorf("failed parsing response data: %w", err)
	}
	raw, ok := top[field]
	if !ok {
		return connection{}, fmt.Errorf("response missing %q connection", field)
	}
	var conn connection
	if err := json.Unmarshal(raw, &conn); err != nil {
		return connection{}, fmt.Errorf("failed parsing %q connection: %w", field, err)
	}
	return conn, nil
}

// findNodes recursively searches a GraphQL response for the first object that
// has a `nodes` array (i.e. a connection) and returns those node objects.
func findNodes(raw json.RawMessage) ([]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	if nodesRaw, ok := obj["nodes"]; ok {
		var nodes []json.RawMessage
		if err := json.Unmarshal(nodesRaw, &nodes); err == nil {
			return nodes, true
		}
	}
	for _, v := range obj {
		if nodes, ok := findNodes(v); ok {
			return nodes, true
		}
	}
	return nil, false
}

// normalizeOrderBy maps the editor value to Linear's PaginationOrderBy enum.
func normalizeOrderBy(orderBy string) string {
	if orderBy == "updatedAt" {
		return "updatedAt"
	}
	return "createdAt"
}

// buildFilter assembles the GraphQL filter object for predefined issue/cycle
// queries from the editor inputs. Returns nil when no filter applies.
//
// Multi-value inputs become `in` conditions (or, for matching by email-or-name,
// an `or` group per value). Multiple distinct filter fields are implicitly AND'd
// by Linear's filter semantics.
func buildFilter(q QueryModel) map[string]any {
	if q.QueryType != queryTypeIssues && q.QueryType != queryTypeCycles {
		return nil
	}
	filter := map[string]any{}

	if t := strings.TrimSpace(q.TeamId); t != "" {
		filter["team"] = map[string]any{"id": map[string]any{"eq": t}}
	}

	if q.QueryType == queryTypeIssues {
		buildIssueFilter(q, filter)
	}

	if len(filter) == 0 {
		return nil
	}
	return filter
}

// buildIssueFilter populates the issue-specific filter conditions.
func buildIssueFilter(q QueryModel, filter map[string]any) {
	if names := nonEmpty(q.States); len(names) > 0 {
		filter["state"] = map[string]any{"name": map[string]any{"in": names}}
	}

	if people := nonEmptyStrings(q.Assignees); len(people) > 0 {
		filter["assignee"] = personFilter(people)
	}

	if c := strings.TrimSpace(q.Creator); c != "" {
		filter["creator"] = personFilter([]string{c})
	}

	if names := nonEmpty(q.Labels); len(names) > 0 {
		// An issue matches if it has any label whose name is in the set.
		filter["labels"] = map[string]any{"some": map[string]any{"name": map[string]any{"in": names}}}
	}

	if len(q.Priorities) > 0 {
		priorityValues := make([]any, 0, len(q.Priorities))
		for _, p := range q.Priorities {
			priorityValues = append(priorityValues, p)
		}
		filter["priority"] = map[string]any{"in": priorityValues}
	}

	if names := nonEmpty(q.Projects); len(names) > 0 {
		filter["project"] = map[string]any{"name": map[string]any{"in": names}}
	}

	if s := strings.TrimSpace(q.SearchQuery); s != "" {
		filter["title"] = map[string]any{"containsIgnoreCase": s}
	}

	if created := dateModeFilter(q.CreatedMode, q.CreatedAfter, q.CreatedBefore, q.TimeRange); created != nil {
		filter["createdAt"] = created
	}
	if updated := dateModeFilter(q.UpdatedMode, q.UpdatedAfter, q.UpdatedBefore, q.TimeRange); updated != nil {
		filter["updatedAt"] = updated
	}
}

// personFilter matches a user comparator (assignee/creator) against a set of
// values, each of which may be an email or a (partial) name. It builds an `or`
// group so any value and either field can match.
func personFilter(values []string) map[string]any {
	or := make([]any, 0, len(values)*2)
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		or = append(or,
			map[string]any{"email": map[string]any{"eqIgnoreCase": v}},
			map[string]any{"name": map[string]any{"containsIgnoreCase": v}},
		)
	}
	return map[string]any{"or": or}
}

// dateModeFilter builds a DateComparator for a created/updated filter based on
// its mode:
//   - "dashboard": use the panel time range (gte=From, lte=To)
//   - "custom":    use the explicit after/before bounds
//   - anything else ("any" / empty): no filter
//
// Returns nil when the mode yields no usable bounds.
func dateModeFilter(mode, after, before string, tr backend.TimeRange) map[string]any {
	switch mode {
	case dateModeDashboard:
		if tr.From.IsZero() || tr.To.IsZero() {
			return nil
		}
		return map[string]any{
			"gte": tr.From.UTC().Format(time.RFC3339),
			"lte": tr.To.UTC().Format(time.RFC3339),
		}
	case dateModeCustom:
		cmp := map[string]any{}
		if a := strings.TrimSpace(after); a != "" {
			cmp["gte"] = a
		}
		if b := strings.TrimSpace(before); b != "" {
			cmp["lte"] = b
		}
		if len(cmp) == 0 {
			return nil
		}
		return cmp
	default:
		return nil
	}
}

// nonEmpty returns the trimmed, non-empty entries of a string slice as []any,
// suitable for use as a GraphQL `in` list value.
func nonEmpty(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// nonEmptyStrings returns the trimmed, non-empty entries of a string slice.
func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Ping performs a minimal authenticated request (the `viewer` query) to validate
// connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, `query { viewer { id name } }`, nil)
	return err
}

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns.
// ---------------------------------------------------------------------------

// TeamInfo is a lightweight team representation for the team dropdown.
type TeamInfo struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ListTeams returns the teams in the workspace.
func (c *Client) ListTeams(ctx context.Context) ([]TeamInfo, error) {
	query := `query Teams($first: Int!, $after: String) {
  teams(first: $first, after: $after) {
    nodes { id key name }
    pageInfo { hasNextPage endCursor }
  }
}`
	teams := make([]TeamInfo, 0)
	cursor := ""
	for {
		variables := map[string]any{"first": defaultPageSize}
		if cursor != "" {
			variables["after"] = cursor
		}
		data, err := c.do(ctx, query, variables)
		if err != nil {
			return nil, err
		}
		conn, err := decodeConnection(data, "teams")
		if err != nil {
			return nil, err
		}
		for _, node := range conn.Nodes {
			var t TeamInfo
			if err := json.Unmarshal(node, &t); err == nil && t.ID != "" {
				teams = append(teams, t)
			}
		}
		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" || len(conn.Nodes) == 0 {
			break
		}
		cursor = conn.PageInfo.EndCursor
	}
	return teams, nil
}

// StateInfo is a workflow state name plus its team key, used for the state
// dropdown.
type StateInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	TeamKey string `json:"teamKey"`
}

// ListStates returns the workflow states, optionally restricted to a team.
func (c *Client) ListStates(ctx context.Context, teamID string) ([]StateInfo, error) {
	query := `query States($first: Int!, $after: String, $filter: WorkflowStateFilter) {
  workflowStates(first: $first, after: $after, filter: $filter) {
    nodes { name type team { key } }
    pageInfo { hasNextPage endCursor }
  }
}`
	var filter map[string]any
	if strings.TrimSpace(teamID) != "" {
		filter = map[string]any{"team": map[string]any{"id": map[string]any{"eq": teamID}}}
	}

	seen := map[string]bool{}
	states := make([]StateInfo, 0)
	cursor := ""
	for {
		variables := map[string]any{"first": defaultPageSize}
		if cursor != "" {
			variables["after"] = cursor
		}
		if filter != nil {
			variables["filter"] = filter
		}
		data, err := c.do(ctx, query, variables)
		if err != nil {
			return nil, err
		}
		conn, err := decodeConnection(data, "workflowStates")
		if err != nil {
			return nil, err
		}
		for _, node := range conn.Nodes {
			var raw struct {
				Name string `json:"name"`
				Type string `json:"type"`
				Team struct {
					Key string `json:"key"`
				} `json:"team"`
			}
			if err := json.Unmarshal(node, &raw); err != nil || raw.Name == "" {
				continue
			}
			if seen[raw.Name] {
				continue
			}
			seen[raw.Name] = true
			states = append(states, StateInfo{Name: raw.Name, Type: raw.Type, TeamKey: raw.Team.Key})
		}
		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" || len(conn.Nodes) == 0 {
			break
		}
		cursor = conn.PageInfo.EndCursor
	}
	return states, nil
}

// LabelInfo is a label name used for the labels multi-select.
type LabelInfo struct {
	Name string `json:"name"`
}

// ListLabels returns the workspace label names, optionally restricted to a team.
// Names are de-duplicated since labels can be defined per team and at workspace
// level.
func (c *Client) ListLabels(ctx context.Context, teamID string) ([]LabelInfo, error) {
	query := `query Labels($first: Int!, $after: String, $filter: IssueLabelFilter) {
  issueLabels(first: $first, after: $after, filter: $filter) {
    nodes { name }
    pageInfo { hasNextPage endCursor }
  }
}`
	var filter map[string]any
	if strings.TrimSpace(teamID) != "" {
		filter = map[string]any{"team": map[string]any{"id": map[string]any{"eq": teamID}}}
	}

	seen := map[string]bool{}
	labels := make([]LabelInfo, 0)
	cursor := ""
	for {
		variables := map[string]any{"first": defaultPageSize}
		if cursor != "" {
			variables["after"] = cursor
		}
		if filter != nil {
			variables["filter"] = filter
		}
		data, err := c.do(ctx, query, variables)
		if err != nil {
			return nil, err
		}
		conn, err := decodeConnection(data, "issueLabels")
		if err != nil {
			return nil, err
		}
		for _, node := range conn.Nodes {
			var l LabelInfo
			if err := json.Unmarshal(node, &l); err != nil || strings.TrimSpace(l.Name) == "" {
				continue
			}
			if seen[l.Name] {
				continue
			}
			seen[l.Name] = true
			labels = append(labels, l)
		}
		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" || len(conn.Nodes) == 0 {
			break
		}
		cursor = conn.PageInfo.EndCursor
	}
	return labels, nil
}

// ProjectInfo is a lightweight project representation for the project
// multi-select.
type ProjectInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListProjects returns the workspace projects.
func (c *Client) ListProjects(ctx context.Context) ([]ProjectInfo, error) {
	query := `query Projects($first: Int!, $after: String) {
  projects(first: $first, after: $after) {
    nodes { id name }
    pageInfo { hasNextPage endCursor }
  }
}`
	projects := make([]ProjectInfo, 0)
	cursor := ""
	for {
		variables := map[string]any{"first": defaultPageSize}
		if cursor != "" {
			variables["after"] = cursor
		}
		data, err := c.do(ctx, query, variables)
		if err != nil {
			return nil, err
		}
		conn, err := decodeConnection(data, "projects")
		if err != nil {
			return nil, err
		}
		for _, node := range conn.Nodes {
			var p ProjectInfo
			if err := json.Unmarshal(node, &p); err == nil && p.Name != "" {
				projects = append(projects, p)
			}
		}
		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" || len(conn.Nodes) == 0 {
			break
		}
		cursor = conn.PageInfo.EndCursor
	}
	return projects, nil
}

// UserInfo is a lightweight user representation for the assignee/creator
// multi-selects.
type UserInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ListUsers returns the active users in the workspace.
func (c *Client) ListUsers(ctx context.Context) ([]UserInfo, error) {
	query := `query Users($first: Int!, $after: String) {
  users(first: $first, after: $after) {
    nodes { name email active }
    pageInfo { hasNextPage endCursor }
  }
}`
	users := make([]UserInfo, 0)
	cursor := ""
	for {
		variables := map[string]any{"first": defaultPageSize}
		if cursor != "" {
			variables["after"] = cursor
		}
		data, err := c.do(ctx, query, variables)
		if err != nil {
			return nil, err
		}
		conn, err := decodeConnection(data, "users")
		if err != nil {
			return nil, err
		}
		for _, node := range conn.Nodes {
			var u struct {
				Name   string `json:"name"`
				Email  string `json:"email"`
				Active bool   `json:"active"`
			}
			if err := json.Unmarshal(node, &u); err != nil || u.Name == "" {
				continue
			}
			users = append(users, UserInfo{Name: u.Name, Email: u.Email})
		}
		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" || len(conn.Nodes) == 0 {
			break
		}
		cursor = conn.PageInfo.EndCursor
	}
	return users, nil
}
