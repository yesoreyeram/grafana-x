package plugin

import (
	"bytes"
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
	// Intercom caps per_page at 150 on list and search endpoints.
	defaultPerPage = 150
	maxRecords     = 100000
)

// Client is a thin wrapper around the Intercom REST API.
type Client struct {
	baseURL    string
	apiToken   string
	version    string
	httpClient *http.Client
}

// NewClient creates an Intercom API client. The provided httpClient is normally
// the SDK-managed client so proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = intercomCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	version := strings.TrimSpace(settings.IntercomVersion)
	if version == "" {
		version = defaultIntercomVersion
	}
	return &Client{
		baseURL:    base,
		apiToken:   settings.apiToken,
		version:    version,
		httpClient: httpClient,
	}, nil
}

// intercomError is Intercom's JSON error envelope:
//
//	{"type":"error.list","request_id":"...","errors":[{"code":"...","message":"..."}]}
type intercomError struct {
	Type   string `json:"type"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func (e intercomError) message() string {
	parts := make([]string, 0, len(e.Errors))
	for _, er := range e.Errors {
		switch {
		case er.Message != "" && er.Code != "":
			parts = append(parts, er.Code+": "+er.Message)
		case er.Message != "":
			parts = append(parts, er.Message)
		case er.Code != "":
			parts = append(parts, er.Code)
		}
	}
	return strings.Join(parts, "; ")
}

// do issues a request to Intercom. When body is non-nil it is JSON-encoded;
// method is honoured as given. The decoded JSON response is written to out when
// non-nil.
func (c *Client) do(ctx context.Context, method, rawURL string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Intercom-Version", c.version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to intercom failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading intercom response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		var apiErr intercomError
		if json.Unmarshal(raw, &apiErr) == nil {
			if m := apiErr.message(); m != "" {
				msg = m
			}
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("intercom returned status %d: %s", resp.StatusCode, msg)
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("failed parsing intercom response: %w", err)
		}
	}
	return nil
}

// ListRecords dispatches to the appropriate Intercom endpoint for the query type
// and returns flattened records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	switch q.QueryType {
	case queryTypeConversations, queryTypeContacts:
		if q.hasSearch() {
			return c.searchEntity(ctx, q, q.QueryType)
		}
		return c.listEntity(ctx, q, q.QueryType)
	case queryTypeTickets:
		return c.searchEntity(ctx, q, q.QueryType)
	case queryTypeArticles, queryTypeCompanies:
		return c.listEntity(ctx, q, q.QueryType)
	case queryTypeAdmins, queryTypeTeams, queryTypeTags:
		return c.simpleList(ctx, q.QueryType)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.QueryType)
	}
}

// searchEntity runs the POST /{entity}/search endpoint, following cursor
// pagination via pages.next.starting_after up to the requested limit.
func (c *Client) searchEntity(ctx context.Context, q QueryModel, entity string) ([]map[string]any, error) {
	dataKey := dataKeyFor(entity)
	endpoint := fmt.Sprintf("%s/%s/search", c.baseURL, entity)

	query := BuildSearchQuery(q, entity)
	if query == nil {
		query = matchAllQuery()
	}
	sortObj := buildSort(q.Sort)

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	records := make([]map[string]any, 0, defaultPerPage)
	cursor := ""
	for {
		perPage := defaultPerPage
		if remaining := hardLimit - len(records); remaining < perPage {
			perPage = remaining
		}
		if perPage <= 0 {
			break
		}

		pagination := map[string]any{"per_page": perPage}
		if cursor != "" {
			pagination["starting_after"] = cursor
		}
		body := map[string]any{"query": query, "pagination": pagination}
		if sortObj != nil {
			body["sort"] = sortObj
		}

		var resp json.RawMessage
		if err := c.do(ctx, http.MethodPost, endpoint, body, &resp); err != nil {
			return nil, err
		}
		items, _, pages := extractItems(resp, dataKey)
		for _, item := range items {
			records = append(records, flattenIntercomRecord(item, len(records)))
			if len(records) >= hardLimit {
				return records, nil
			}
		}

		next, _ := nextPagination(pages)
		if next == "" || len(items) == 0 {
			break
		}
		cursor = next
	}
	return records, nil
}

// listEntity runs a GET list endpoint, following cursor (starting_after) or
// page-based pagination up to the requested limit.
func (c *Client) listEntity(ctx context.Context, q QueryModel, entity string) ([]map[string]any, error) {
	dataKey := dataKeyFor(entity)
	sortField, sortOrder := buildSortParams(q.Sort)

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	records := make([]map[string]any, 0, defaultPerPage)
	cursor := ""
	page := 0
	for {
		perPage := defaultPerPage
		if remaining := hardLimit - len(records); remaining < perPage {
			perPage = remaining
		}
		if perPage <= 0 {
			break
		}

		params := url.Values{}
		params.Set("per_page", strconv.Itoa(perPage))
		if cursor != "" {
			params.Set("starting_after", cursor)
		}
		if page > 0 {
			params.Set("page", strconv.Itoa(page))
		}
		if sortField != "" {
			params.Set("sort", sortField)
			params.Set("order", sortOrder)
		}
		endpoint := fmt.Sprintf("%s/%s?%s", c.baseURL, entity, params.Encode())

		var resp json.RawMessage
		if err := c.do(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
			return nil, err
		}
		items, _, pages := extractItems(resp, dataKey)
		for _, item := range items {
			records = append(records, flattenIntercomRecord(item, len(records)))
			if len(records) >= hardLimit {
				return records, nil
			}
		}

		next, nextPage := nextPagination(pages)
		if next == "" && nextPage <= 0 {
			break
		}
		if len(items) == 0 {
			break
		}
		cursor = next
		page = nextPage
	}
	return records, nil
}

// simpleList runs a single GET against a non-paginated list endpoint (admins,
// teams, tags) and flattens the result.
func (c *Client) simpleList(ctx context.Context, entity string) ([]map[string]any, error) {
	dataKey := dataKeyFor(entity)
	endpoint := fmt.Sprintf("%s/%s", c.baseURL, entity)

	var resp json.RawMessage
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	items, _, _ := extractItems(resp, dataKey)
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		records = append(records, flattenIntercomRecord(item, len(records)))
	}
	return records, nil
}

// CountRecords returns the number of records matching the query for the entity
// selected by CountOf. Intercom returns total_count on list and search
// responses; for the simple list endpoints (which lack total_count) the array
// length is returned.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	entity := strings.TrimSpace(q.CountOf)
	if entity == "" {
		entity = queryTypeConversations
	}
	dataKey := dataKeyFor(entity)

	switch {
	case searchableEntity(entity):
		query := BuildSearchQuery(q, entity)
		if query == nil {
			query = matchAllQuery()
		}
		body := map[string]any{"query": query, "pagination": map[string]any{"per_page": 1}}
		var resp json.RawMessage
		if err := c.do(ctx, http.MethodPost, fmt.Sprintf("%s/%s/search", c.baseURL, entity), body, &resp); err != nil {
			return 0, err
		}
		_, total, _ := extractItems(resp, dataKey)
		return total, nil
	case cursorListEntity(entity):
		endpoint := fmt.Sprintf("%s/%s?per_page=1", c.baseURL, entity)
		var resp json.RawMessage
		if err := c.do(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
			return 0, err
		}
		_, total, _ := extractItems(resp, dataKey)
		return total, nil
	default: // admins, teams, tags
		var resp json.RawMessage
		if err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", c.baseURL, entity), nil, &resp); err != nil {
			return 0, err
		}
		items, total, _ := extractItems(resp, dataKey)
		if total > 0 {
			return total, nil
		}
		return int64(len(items)), nil
	}
}

// Ping performs a minimal authenticated request (GET /me) to validate
// connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, c.baseURL+"/me", nil, nil)
}

// extractItems pulls the records array (under dataKey), total_count and pages
// envelope out of an Intercom list/search response.
func extractItems(raw json.RawMessage, dataKey string) ([]json.RawMessage, int64, json.RawMessage) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, 0, nil
	}
	var items []json.RawMessage
	if v, ok := top[dataKey]; ok {
		_ = json.Unmarshal(v, &items)
	}
	var total int64
	if v, ok := top["total_count"]; ok {
		_ = json.Unmarshal(v, &total)
	}
	var pages json.RawMessage
	if v, ok := top["pages"]; ok {
		pages = v
	}
	return items, total, pages
}

// nextPagination extracts the next-page cursor (starting_after) and/or page
// number from the `pages` envelope. Intercom returns pages.next either as an
// object {page, starting_after} or, on some legacy endpoints, as a URL string.
func nextPagination(pages json.RawMessage) (string, int) {
	if len(pages) == 0 {
		return "", 0
	}
	var p struct {
		Next json.RawMessage `json:"next"`
	}
	if err := json.Unmarshal(pages, &p); err != nil || len(p.Next) == 0 {
		return "", 0
	}
	trimmed := strings.TrimSpace(string(p.Next))
	if trimmed == "" || trimmed == "null" {
		return "", 0
	}
	if trimmed[0] == '"' {
		// Legacy URL form, e.g. "https://api.intercom.io/companies?per_page=50&page=2".
		var u string
		if json.Unmarshal(p.Next, &u) != nil {
			return "", 0
		}
		return paginationFromURL(u)
	}
	var n struct {
		StartingAfter string `json:"starting_after"`
		Page          int    `json:"page"`
	}
	if json.Unmarshal(p.Next, &n) != nil {
		return "", 0
	}
	return n.StartingAfter, n.Page
}

func paginationFromURL(raw string) (string, int) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0
	}
	q := u.Query()
	page := 0
	if p := q.Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	return q.Get("starting_after"), page
}

// ----- Resource helpers for the QueryEditor dropdowns ---------------------------

// AdminInfo is a lightweight admin representation for the frontend.
type AdminInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type adminsResponse struct {
	Admins []AdminInfo `json:"admins"`
}

// ListAdmins returns the workspace admins (teammates).
func (c *Client) ListAdmins(ctx context.Context) ([]AdminInfo, error) {
	var resp adminsResponse
	if err := c.do(ctx, http.MethodGet, c.baseURL+"/admins", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Admins, nil
}

// TeamInfo is a lightweight team representation for the frontend.
type TeamInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type teamsResponse struct {
	Teams []TeamInfo `json:"teams"`
}

// ListTeams returns the workspace teams.
func (c *Client) ListTeams(ctx context.Context) ([]TeamInfo, error) {
	var resp teamsResponse
	if err := c.do(ctx, http.MethodGet, c.baseURL+"/teams", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Teams, nil
}

// TagInfo is a lightweight tag representation for the frontend.
type TagInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type tagsResponse struct {
	Data []TagInfo `json:"data"`
}

// ListTags returns the workspace tags.
func (c *Client) ListTags(ctx context.Context) ([]TagInfo, error) {
	var resp tagsResponse
	if err := c.do(ctx, http.MethodGet, c.baseURL+"/tags", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
