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
	// defaultPageSize is the search page size requested per page. The Shortcut
	// search endpoint accepts 1..250.
	defaultPageSize = 25
	// maxPageSize is the maximum page size the search endpoint accepts.
	maxPageSize = 250
	// maxRecords bounds an unbounded query. Shortcut search itself caps results
	// at 1000, so this is only a defensive upper limit.
	maxRecords = 100000
)

// Client is a thin wrapper around Shortcut's REST API v3.
type Client struct {
	apiToken   string
	baseURL    string // host only, e.g. https://api.app.shortcut.com (no /api/v3)
	httpClient *http.Client
}

// NewClient creates a Shortcut API client. The provided httpClient is normally
// the SDK-managed client so proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = shortcutCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		apiToken:   settings.apiToken,
		baseURL:    base,
		httpClient: httpClient,
	}, nil
}

// errorResponse is a best-effort decode of Shortcut's JSON error body. Shortcut
// returns errors as {"message":"..."} (and sometimes a structured "errors"
// field); any present message is surfaced.
type errorResponse struct {
	Message string `json:"message"`
}

// do issues a request to the given API sub-path (relative to /api/v3) and
// returns the raw response body.
func (c *Client) do(ctx context.Context, method, apiSubPath string, query url.Values, body any) ([]byte, error) {
	full := c.baseURL + apiPrefix + apiSubPath
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	return c.request(ctx, method, full, body)
}

// request performs an HTTP request against a fully-formed URL, applying the
// Shortcut-Token auth header and surfacing HTTP/Shortcut errors.
func (c *Client) request(ctx context.Context, method, fullURL string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed encoding request body: %w", err)
		}
		reader = strings.NewReader(string(payload))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Shortcut authenticates via the Shortcut-Token header (NOT Authorization:
	// Bearer, and NOT the deprecated `token` query parameter).
	if c.apiToken != "" {
		req.Header.Set("Shortcut-Token", c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to shortcut failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading shortcut response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e errorResponse
		if json.Unmarshal(raw, &e) == nil && strings.TrimSpace(e.Message) != "" {
			return nil, fmt.Errorf("shortcut returned status %d: %s", resp.StatusCode, truncate(e.Message))
		}
		return nil, fmt.Errorf("shortcut returned status %d: %s", resp.StatusCode, truncate(string(raw)))
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

// searchPage is the envelope returned by GET /api/v3/search/stories. `Next` is a
// relative path + query string (including a `next` page token) used to fetch the
// following page, or null/empty on the last page. `Total` is the full match
// count regardless of paging.
type searchPage struct {
	Data  []json.RawMessage `json:"data"`
	Next  *string           `json:"next"`
	Total int               `json:"total"`
}

// ListStories runs a story search built from the query's filters, following the
// `next` page tokens until exhausted or the limit is reached, and returns the
// flattened story records plus the total match count reported by the API.
func (c *Client) ListStories(ctx context.Context, q QueryModel) ([]map[string]any, int, error) {
	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	params := c.searchParams(q)
	stories := make([]map[string]any, 0, defaultPageSize)
	total := 0
	next := ""

	for {
		var raw []byte
		var err error
		if next == "" {
			raw, err = c.do(ctx, http.MethodGet, "/search/stories", params, nil)
		} else {
			raw, err = c.followNext(ctx, params, next)
		}
		if err != nil {
			return nil, 0, err
		}

		var page searchPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, 0, fmt.Errorf("failed parsing stories response: %w", err)
		}
		total = page.Total
		for _, s := range page.Data {
			stories = append(stories, flattenStory(s))
		}

		if len(stories) >= hardLimit {
			break
		}
		if page.Next == nil || strings.TrimSpace(*page.Next) == "" || len(page.Data) == 0 {
			break
		}
		next = strings.TrimSpace(*page.Next)
	}

	if len(stories) > hardLimit {
		stories = stories[:hardLimit]
	}
	if total < len(stories) {
		total = len(stories)
	}
	return stories, total, nil
}

// CountStories returns the number of stories matching the query using a single
// search request and the `total` field in the response — no pagination needed.
func (c *Client) CountStories(ctx context.Context, q QueryModel) (int64, error) {
	params := c.searchParams(q)
	params.Set("page_size", "1")
	params.Set("detail", detailSlim)

	raw, err := c.do(ctx, http.MethodGet, "/search/stories", params, nil)
	if err != nil {
		return 0, err
	}
	var page searchPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return 0, fmt.Errorf("failed parsing stories response: %w", err)
	}
	return int64(page.Total), nil
}

// searchParams builds the query string for the search endpoint: the compiled
// Shortcut query, the page size, and the detail level.
func (c *Client) searchParams(q QueryModel) url.Values {
	v := url.Values{}
	v.Set("query", buildSearchQuery(q))
	v.Set("page_size", strconv.Itoa(clampPageSize(q.Limit)))
	v.Set("detail", normalizeDetail(q.Detail))
	return v
}

// followNext fetches the next page. The API documents `next` as a relative path
// and query string ("/api/v3/search/stories?...&next=<token>"); it is resolved
// against the host. Absolute URLs and bare tokens are also handled defensively.
func (c *Client) followNext(ctx context.Context, params url.Values, next string) ([]byte, error) {
	switch {
	case strings.HasPrefix(next, "http://"), strings.HasPrefix(next, "https://"):
		return c.request(ctx, http.MethodGet, next, nil)
	case strings.HasPrefix(next, "/"):
		return c.request(ctx, http.MethodGet, c.baseURL+next, nil)
	default:
		// Bare token: re-issue the search with the next token added.
		p := cloneValues(params)
		p.Set("next", next)
		return c.do(ctx, http.MethodGet, "/search/stories", p, nil)
	}
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

// clampPageSize returns a search page size honouring the limit (when smaller)
// and the API's 1..250 bounds.
func clampPageSize(limit int) int {
	size := defaultPageSize
	if limit > 0 && limit < size {
		size = limit
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	if size < 1 {
		size = 1
	}
	return size
}

func normalizeDetail(detail string) string {
	if detail == detailSlim {
		return detailSlim
	}
	return detailFull
}

// Ping performs a minimal authenticated request (the current member endpoint) to
// validate connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/member", nil, nil)
	return err
}

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns. The query language
// matches by name (mention name for owners/teams), so the DTOs expose names.
// ---------------------------------------------------------------------------

// ShortcutProject is a project for the project filter (search project: by name).
type ShortcutProject struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ListProjects returns all projects. The projects list endpoint returns a plain
// array (no pagination).
func (c *Client) ListProjects(ctx context.Context) ([]ShortcutProject, error) {
	raw, err := c.do(ctx, http.MethodGet, "/projects", nil, nil)
	if err != nil {
		return nil, err
	}
	var items []ShortcutProject
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("failed parsing projects: %w", err)
	}
	return items, nil
}

// ShortcutEpic is an epic for the epic filter (search epic: by name).
type ShortcutEpic struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ListEpics returns all epics (plain array, no pagination).
func (c *Client) ListEpics(ctx context.Context) ([]ShortcutEpic, error) {
	raw, err := c.do(ctx, http.MethodGet, "/epics", nil, nil)
	if err != nil {
		return nil, err
	}
	var items []ShortcutEpic
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("failed parsing epics: %w", err)
	}
	return items, nil
}

// ShortcutIteration is an iteration for the iteration filter.
type ShortcutIteration struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ListIterations returns all iterations (plain array, no pagination).
func (c *Client) ListIterations(ctx context.Context) ([]ShortcutIteration, error) {
	raw, err := c.do(ctx, http.MethodGet, "/iterations", nil, nil)
	if err != nil {
		return nil, err
	}
	var items []ShortcutIteration
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("failed parsing iterations: %w", err)
	}
	return items, nil
}

// ShortcutMember is a member for the owner filter. Shortcut nests the name and
// mention name under "profile"; the owner: search operator uses the mention
// name.
type ShortcutMember struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MentionName string `json:"mention_name"`
}

// memberEnvelope mirrors the Member object: id at the top level, name and
// mention_name nested under profile.
type memberEnvelope struct {
	ID      string `json:"id"`
	Profile struct {
		Name        string `json:"name"`
		MentionName string `json:"mention_name"`
	} `json:"profile"`
}

// ListMembers returns the workspace members (plain array, no pagination),
// flattening the nested profile into name / mention name.
func (c *Client) ListMembers(ctx context.Context) ([]ShortcutMember, error) {
	raw, err := c.do(ctx, http.MethodGet, "/members", nil, nil)
	if err != nil {
		return nil, err
	}
	var items []memberEnvelope
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("failed parsing members: %w", err)
	}
	out := make([]ShortcutMember, 0, len(items))
	for _, m := range items {
		out = append(out, ShortcutMember{
			ID:          m.ID,
			Name:        m.Profile.Name,
			MentionName: m.Profile.MentionName,
		})
	}
	return out, nil
}

// ShortcutTeam is a team (group) for the team filter. The team: search operator
// uses the team Name (not the mention name).
type ShortcutTeam struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MentionName string `json:"mention_name"`
}

// ListTeams returns the workspace teams via the Groups endpoint (plain array).
func (c *Client) ListTeams(ctx context.Context) ([]ShortcutTeam, error) {
	raw, err := c.do(ctx, http.MethodGet, "/groups", nil, nil)
	if err != nil {
		return nil, err
	}
	var items []ShortcutTeam
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("failed parsing teams: %w", err)
	}
	return items, nil
}

// ShortcutLabel is a label for the label filter (search label: by name).
type ShortcutLabel struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ListLabels returns all labels (plain array, no pagination).
func (c *Client) ListLabels(ctx context.Context) ([]ShortcutLabel, error) {
	raw, err := c.do(ctx, http.MethodGet, "/labels", nil, nil)
	if err != nil {
		return nil, err
	}
	var items []ShortcutLabel
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("failed parsing labels: %w", err)
	}
	return items, nil
}

// ShortcutWorkflow is a workflow with its workflow states.
type ShortcutWorkflow struct {
	ID     int                     `json:"id"`
	Name   string                  `json:"name"`
	States []ShortcutWorkflowState `json:"states"`
}

// ShortcutWorkflowState is a workflow state for the state filter (search state:
// by name).
type ShortcutWorkflowState struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListWorkflows returns the de-duplicated set of workflow states across all
// workflows (plain array, no pagination). State names drive the state: filter.
func (c *Client) ListWorkflows(ctx context.Context) ([]ShortcutWorkflowState, error) {
	raw, err := c.do(ctx, http.MethodGet, "/workflows", nil, nil)
	if err != nil {
		return nil, err
	}
	var workflows []ShortcutWorkflow
	if err := json.Unmarshal(raw, &workflows); err != nil {
		return nil, fmt.Errorf("failed parsing workflows: %w", err)
	}
	seen := map[string]bool{}
	states := make([]ShortcutWorkflowState, 0)
	for _, wf := range workflows {
		for _, s := range wf.States {
			key := strconv.Itoa(s.ID)
			if seen[key] {
				continue
			}
			seen[key] = true
			states = append(states, s)
		}
	}
	return states, nil
}
