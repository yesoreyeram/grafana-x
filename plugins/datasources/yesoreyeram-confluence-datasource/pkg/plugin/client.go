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
	// Confluence caps the page size at 250 on the v2 listing endpoints.
	maxLimit = 250
	// defaultPageSize is the per-request page size used when auto-paginating.
	defaultPageSize = 100
	// maxRecords is a safety cap on the total number of records fetched when no
	// explicit limit is set on the query.
	maxRecords = 100000
)

// Client is a thin wrapper around the Confluence REST API (v2 content endpoints
// plus the v1 CQL search endpoint).
type Client struct {
	// baseURL is the wiki base, e.g. https://site.atlassian.net/wiki (no trailing slash).
	baseURL string
	// origin is the scheme://host of baseURL, used to resolve the relative
	// `_links.next` pagination URLs returned by the API.
	origin     string
	authHeader string
	httpClient *http.Client
}

// NewClient creates a Confluence API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	u, err := url.ParseRequestURI(base)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	origin := u.Scheme + "://" + u.Host
	return &Client{
		baseURL:    base,
		origin:     origin,
		authHeader: settings.authHeader(),
		httpClient: httpClient,
	}, nil
}

// confluenceError is the union of the v2 (`errors[]`) and v1 (`message`) error
// envelopes returned by the API.
type confluenceError struct {
	Message string `json:"message"`
	Errors  []struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

func (e confluenceError) message() string {
	if len(e.Errors) > 0 {
		first := e.Errors[0]
		msg := strings.TrimSpace(first.Title)
		if first.Detail != "" {
			if msg != "" {
				msg += ": "
			}
			msg += first.Detail
		}
		if msg != "" {
			return msg
		}
	}
	return strings.TrimSpace(e.Message)
}

// do issues a GET request to the given absolute URL and decodes the JSON
// response body into out (when non-nil).
func (c *Client) do(ctx context.Context, method, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to confluence failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading confluence response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		var apiErr confluenceError
		if json.Unmarshal(raw, &apiErr) == nil {
			if m := apiErr.message(); m != "" {
				msg = m
			}
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("confluence returned status %d: %s", resp.StatusCode, msg)
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("failed parsing confluence response: %w", err)
		}
	}
	return nil
}

// multiEntityResult is the common envelope returned by Confluence list endpoints
// (both v2 content listings and the v1 search endpoint).
type multiEntityResult struct {
	Results []json.RawMessage `json:"results"`
	Links   struct {
		Next string `json:"next"`
		Base string `json:"base"`
	} `json:"_links"`
}

// resolveNext resolves a relative `_links.next` URL against the site origin. The
// API returns next links such as `/wiki/api/v2/pages?limit=5&cursor=<token>`
// which are relative to the origin (scheme://host), not the wiki base.
func (c *Client) resolveNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return ""
	}
	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {
		return next
	}
	if strings.HasPrefix(next, "/") {
		return c.origin + next
	}
	return c.origin + "/" + next
}

// eachPage walks the cursor-based pagination starting at startURL, invoking fn
// with each page of raw result items. It stops when fn returns false, when there
// are no more pages, or when hardLimit (when > 0) items have been seen.
func (c *Client) eachPage(ctx context.Context, startURL string, hardLimit int, fn func(items []json.RawMessage) bool) error {
	next := startURL
	seen := 0
	for next != "" {
		var res multiEntityResult
		if err := c.do(ctx, http.MethodGet, next, &res); err != nil {
			return err
		}
		if !fn(res.Results) {
			return nil
		}
		seen += len(res.Results)
		if len(res.Results) == 0 {
			break
		}
		if hardLimit > 0 && seen >= hardLimit {
			break
		}
		next = c.resolveNext(res.Links.Next)
	}
	return nil
}

// collect gathers all raw result items from startURL, following pagination up to
// hardLimit (or maxRecords when hardLimit is 0).
func (c *Client) collect(ctx context.Context, startURL string, hardLimit int) ([]json.RawMessage, error) {
	limit := hardLimit
	if limit <= 0 {
		limit = maxRecords
	}
	out := make([]json.RawMessage, 0)
	err := c.eachPage(ctx, startURL, limit, func(items []json.RawMessage) bool {
		out = append(out, items...)
		return true
	})
	if err != nil {
		return nil, err
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// count returns the number of items reachable from startURL, following
// pagination to the end.
func (c *Client) count(ctx context.Context, startURL string) (int64, error) {
	var total int64
	err := c.eachPage(ctx, startURL, 0, func(items []json.RawMessage) bool {
		total += int64(len(items))
		return true
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

// requestLimit returns the per-request page size, capped at maxLimit and not
// exceeding the user's hard limit.
func requestLimit(hardLimit int) int {
	size := defaultPageSize
	if hardLimit > 0 && hardLimit < size {
		size = hardLimit
	}
	if size > maxLimit {
		size = maxLimit
	}
	return size
}

// ListRecords dispatches to the appropriate listing based on the query type and
// returns flattened records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	switch q.QueryType {
	case queryTypeSearch:
		return c.Search(ctx, q)
	case queryTypeBlogposts:
		return c.listContent(ctx, "/api/v2/blogposts", q)
	case queryTypePages, "":
		return c.listContent(ctx, "/api/v2/pages", q)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.QueryType)
	}
}

// contentStartURL builds the first request URL for the pages/blogposts endpoints.
func (c *Client) contentStartURL(path string, q QueryModel) string {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(requestLimit(q.Limit)))
	if id := strings.TrimSpace(q.SpaceID); id != "" {
		params.Set("space-id", id)
	}
	if sort := strings.TrimSpace(q.Sort); sort != "" {
		params.Set("sort", sort)
	}
	if cursor := strings.TrimSpace(q.Cursor); cursor != "" {
		params.Set("cursor", cursor)
	}
	return c.baseURL + path + "?" + params.Encode()
}

// listContent lists pages or blog posts and flattens them into records.
func (c *Client) listContent(ctx context.Context, path string, q QueryModel) ([]map[string]any, error) {
	items, err := c.collect(ctx, c.contentStartURL(path, q), q.Limit)
	if err != nil {
		return nil, err
	}
	return flattenContentItems(items, c.origin), nil
}

// searchStartURL builds the first request URL for the CQL search endpoint.
func (c *Client) searchStartURL(cql string, hardLimit int) string {
	params := url.Values{}
	params.Set("cql", cql)
	params.Set("limit", strconv.Itoa(requestLimit(hardLimit)))
	return c.baseURL + "/rest/api/search?" + params.Encode()
}

// Search runs a CQL search via the v1 search endpoint and flattens the results.
func (c *Client) Search(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	cql := BuildCQL(q.CQL)
	if cql == "" {
		return nil, fmt.Errorf("cql is required for search queries")
	}
	items, err := c.collect(ctx, c.searchStartURL(cql, q.Limit), q.Limit)
	if err != nil {
		return nil, err
	}
	return flattenSearchItems(items, c.origin), nil
}

// CountRecords returns the number of items matching the query. When a CQL string
// is supplied it counts CQL search results; otherwise it counts pages (scoped to
// the space when set).
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if cql := BuildCQL(q.CQL); cql != "" {
		return c.count(ctx, c.searchStartURL(cql, 0))
	}
	return c.count(ctx, c.contentStartURL("/api/v2/pages", QueryModel{SpaceID: q.SpaceID, Sort: q.Sort}))
}

// SpaceInfo is a lightweight representation of a Confluence space used by the
// resource handler that populates the QueryEditor space picker.
type SpaceInfo struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type spaceResult struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// ListSpaces returns the spaces visible to the authenticated user, following
// pagination.
func (c *Client) ListSpaces(ctx context.Context) ([]SpaceInfo, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(maxLimit))
	startURL := c.baseURL + "/api/v2/spaces?" + params.Encode()

	items, err := c.collect(ctx, startURL, maxRecords)
	if err != nil {
		return nil, err
	}
	spaces := make([]SpaceInfo, 0, len(items))
	for _, raw := range items {
		var s spaceResult
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		spaces = append(spaces, SpaceInfo{
			ID:     s.ID,
			Key:    s.Key,
			Name:   s.Name,
			Type:   s.Type,
			Status: s.Status,
		})
	}
	return spaces, nil
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, c.baseURL+"/api/v2/spaces?limit=1", nil)
}
