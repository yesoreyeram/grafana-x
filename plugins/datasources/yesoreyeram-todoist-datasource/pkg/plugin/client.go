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
	// pageSize is the per-request page size. Todoist's v1 list endpoints accept
	// a `limit` of 1..200 (default 50); 200 minimises the number of round-trips.
	pageSize = 200
	// maxRecords is the safety cap applied when a query requests no explicit
	// limit, so an unbounded account cannot loop forever.
	maxRecords = 100000
)

// Client is a thin wrapper around the Todoist unified API v1.
type Client struct {
	apiToken   string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Todoist API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = todoistCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		apiToken:   settings.credential(),
		httpClient: httpClient,
	}, nil
}

// listResponse is the standard Todoist v1 pagination envelope. Paginated list
// endpoints (/tasks, /tasks/filter, /projects, /sections, /labels, …) return
// results under "results" and an opaque "next_cursor" (null when exhausted).
type listResponse struct {
	Results    []json.RawMessage `json:"results"`
	NextCursor *string           `json:"next_cursor"`
}

// do issues a GET request to the given path (relative to the API root) and
// returns the raw response body. HTTP and Todoist-level errors are surfaced as
// Go errors.
func (c *Client) do(ctx context.Context, path string) (json.RawMessage, error) {
	full := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to todoist failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading todoist response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Todoist v1 returns a structured JSON error body, e.g.
		// {"error":"Task not found","error_code":478,"error_tag":"NOT_FOUND",...}.
		if msg, ok := firstError(raw); ok {
			return nil, fmt.Errorf("todoist returned status %d: %s", resp.StatusCode, truncate(msg))
		}
		return nil, fmt.Errorf("todoist returned status %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	return raw, nil
}

// firstError extracts the human-readable message from a Todoist error body.
func firstError(raw json.RawMessage) (string, bool) {
	var e struct {
		Error    string `json:"error"`
		ErrorTag string `json:"error_tag"`
	}
	if json.Unmarshal(raw, &e) == nil {
		if e.Error != "" {
			return e.Error, true
		}
		if e.ErrorTag != "" {
			return e.ErrorTag, true
		}
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

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. Listing a single project is the cheapest authenticated call.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "/projects?limit=1")
	return err
}

// ListRecords runs the task query and returns flattened scalar records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	switch q.QueryType {
	case queryTypeTasks, queryTypeCount:
		path, params := taskPathParams(q)
		return c.listPaginated(ctx, path, params, q.Limit)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.QueryType)
	}
}

// CountRecords returns the number of tasks matching the query. Todoist has no
// native count endpoint, so the matching tasks are paginated and counted (only
// the page envelopes are inspected; task bodies are not flattened). The optional
// Limit caps how many tasks are scanned.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	path, params := taskPathParams(q)

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	var total int64
	cursor := ""
	for {
		p := cloneValues(params)
		pageLimit := pageSize
		if remaining := hardLimit - int(total); remaining < pageLimit {
			pageLimit = remaining
		}
		if pageLimit <= 0 {
			break
		}
		p.Set("limit", strconv.Itoa(pageLimit))
		if cursor != "" {
			p.Set("cursor", cursor)
		}

		raw, err := c.do(ctx, path+"?"+p.Encode())
		if err != nil {
			return 0, err
		}
		var page listResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return 0, fmt.Errorf("failed parsing todoist response: %w", err)
		}
		total += int64(len(page.Results))

		if page.NextCursor == nil || *page.NextCursor == "" || len(page.Results) == 0 {
			break
		}
		cursor = *page.NextCursor
	}
	if int64(hardLimit) < total {
		total = int64(hardLimit)
	}
	return total, nil
}

// taskPathParams builds the endpoint path and query parameters for a task query.
//
// A non-empty Filter routes to the dedicated GET /tasks/filter endpoint (the
// Todoist v1 API removed the `filter`/`lang` parameters from /tasks and exposes
// filtering through this separate endpoint). The id-based scope parameters
// cannot be combined with the filter endpoint, so they are ignored when Filter
// is set. Otherwise the standard GET /tasks endpoint is used with the
// project_id/section_id/label/parent_id scope parameters.
func taskPathParams(q QueryModel) (string, url.Values) {
	params := url.Values{}

	if filter := strings.TrimSpace(q.Filter); filter != "" {
		params.Set("query", filter)
		if lang := strings.TrimSpace(q.Lang); lang != "" {
			params.Set("lang", lang)
		}
		return "/tasks/filter", params
	}

	if id := strings.TrimSpace(q.ProjectId); id != "" {
		params.Set("project_id", id)
	}
	if id := strings.TrimSpace(q.SectionId); id != "" {
		params.Set("section_id", id)
	}
	// The Todoist `label` parameter filters by label NAME, not ID.
	if name := strings.TrimSpace(q.Label); name != "" {
		params.Set("label", name)
	}
	if id := strings.TrimSpace(q.ParentId); id != "" {
		params.Set("parent_id", id)
	}
	return "/tasks", params
}

// listPaginated GETs a paginated list endpoint, following the v1 cursor
// (`next_cursor`) up to the requested hard limit (or a safety cap), and flattens
// each item.
func (c *Client) listPaginated(ctx context.Context, path string, params url.Values, hardLimit int) ([]map[string]any, error) {
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}
	records := make([]map[string]any, 0, pageSize)
	cursor := ""
	for {
		p := cloneValues(params)
		pageLimit := pageSize
		if remaining := hardLimit - len(records); remaining < pageLimit {
			pageLimit = remaining
		}
		if pageLimit <= 0 {
			break
		}
		p.Set("limit", strconv.Itoa(pageLimit))
		if cursor != "" {
			p.Set("cursor", cursor)
		}

		raw, err := c.do(ctx, path+"?"+p.Encode())
		if err != nil {
			return nil, err
		}
		var page listResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("failed parsing todoist response: %w", err)
		}

		for _, item := range page.Results {
			records = append(records, flattenItem(item))
			if len(records) >= hardLimit {
				return records, nil
			}
		}

		if page.NextCursor == nil || *page.NextCursor == "" || len(page.Results) == 0 {
			break
		}
		cursor = *page.NextCursor
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

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns.
// ---------------------------------------------------------------------------

// ResourceInfo is a lightweight Todoist resource (id + name) for the editor
// dropdowns.
type ResourceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// listResource GETs a paginated list endpoint and returns its id/name records,
// following the cursor across pages so accounts with many projects/labels are
// fully covered.
func (c *Client) listResource(ctx context.Context, path string) ([]ResourceInfo, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}

	result := make([]ResourceInfo, 0)
	cursor := ""
	for {
		full := path + sep + "limit=" + strconv.Itoa(pageSize)
		if cursor != "" {
			full += "&cursor=" + url.QueryEscape(cursor)
		}
		raw, err := c.do(ctx, full)
		if err != nil {
			return nil, err
		}
		var page struct {
			Results []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"results"`
			NextCursor *string `json:"next_cursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("failed parsing response: %w", err)
		}
		for _, item := range page.Results {
			result = append(result, ResourceInfo{ID: item.ID, Name: item.Name})
		}
		if page.NextCursor == nil || *page.NextCursor == "" || len(page.Results) == 0 {
			break
		}
		cursor = *page.NextCursor
	}
	return result, nil
}

// ListProjects returns the active projects in the account.
func (c *Client) ListProjects(ctx context.Context) ([]ResourceInfo, error) {
	return c.listResource(ctx, "/projects")
}

// ListSections returns the sections in a project.
func (c *Client) ListSections(ctx context.Context, projectId string) ([]ResourceInfo, error) {
	if strings.TrimSpace(projectId) == "" {
		return nil, fmt.Errorf("projectId is required")
	}
	return c.listResource(ctx, "/sections?project_id="+url.QueryEscape(strings.TrimSpace(projectId)))
}

// ListLabels returns the personal labels in the account.
func (c *Client) ListLabels(ctx context.Context) ([]ResourceInfo, error) {
	return c.listResource(ctx, "/labels")
}
