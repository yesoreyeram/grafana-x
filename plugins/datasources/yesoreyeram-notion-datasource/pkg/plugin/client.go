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
)

const (
	// Notion caps page_size at 100 for the database query and search endpoints.
	defaultPageSize = 100
	maxRecords      = 100000
)

// Client is a thin wrapper around the Notion REST API.
type Client struct {
	baseURL       string
	apiToken      string
	notionVersion string
	httpClient    *http.Client
}

// NewClient creates a Notion API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = notionCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	version := settings.NotionVersion
	if version == "" {
		version = defaultNotionVersion
	}
	return &Client{
		baseURL:       base,
		apiToken:      settings.apiToken,
		notionVersion: version,
		httpClient:    httpClient,
	}, nil
}

// do issues a request to Notion. When body is non-nil it is JSON-encoded and the
// method is POST; otherwise a GET is performed.
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
	req.Header.Set("Notion-Version", c.notionVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to notion failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading notion response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		// Notion error bodies carry a useful `message` field.
		var apiErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &apiErr) == nil && apiErr.Message != "" {
			msg = apiErr.Message
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("notion returned status %d: %s", resp.StatusCode, msg)
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("failed parsing notion response: %w", err)
		}
	}
	return nil
}

// queryResponse is the shape of POST /v1/databases/{id}/query.
type queryResponse struct {
	Results    []notionPage `json:"results"`
	NextCursor string       `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

// sortItem is a single Notion sort directive.
type sortItem struct {
	Property  string `json:"property"`
	Direction string `json:"direction"`
}

// parseSort converts the comma-separated sort string (e.g. `-Created,Name`) used
// by the query editor into Notion sort directives. A leading `-` marks a
// descending sort.
func parseSort(sort string) []sortItem {
	items := make([]sortItem, 0)
	for _, token := range strings.Split(sort, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		direction := "ascending"
		if strings.HasPrefix(token, "-") {
			direction = "descending"
			token = strings.TrimSpace(token[1:])
		}
		if token == "" {
			continue
		}
		items = append(items, sortItem{Property: token, Direction: direction})
	}
	return items
}

// buildQueryBody assembles the request body for the database query endpoint.
func (c *Client) buildQueryBody(q QueryModel, pageSize int, cursor string) map[string]any {
	body := map[string]any{"page_size": pageSize}
	if cursor != "" {
		body["start_cursor"] = cursor
	}
	if q.filter != nil {
		if f := BuildFilter(q.filter); f != nil {
			body["filter"] = f
		}
	}
	if sorts := parseSort(q.Sort); len(sorts) > 0 {
		body["sorts"] = sorts
	}
	return body
}

// ListRecords fetches pages for a database, transparently following the
// cursor-based pagination up to the requested limit (or maxRecords when no limit
// is provided). The pages are flattened into scalar records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.DatabaseID) == "" {
		return nil, fmt.Errorf("databaseId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	endpoint := fmt.Sprintf("%s/v1/databases/%s/query", c.baseURL, url.PathEscape(q.DatabaseID))
	fields := splitFields(q.Fields)

	pages := make([]notionPage, 0, defaultPageSize)
	cursor := ""
	for {
		pageSize := defaultPageSize
		if remaining := hardLimit - len(pages); remaining < pageSize {
			pageSize = remaining
		}
		if pageSize <= 0 {
			break
		}

		var res queryResponse
		if err := c.do(ctx, http.MethodPost, endpoint, c.buildQueryBody(q, pageSize, cursor), &res); err != nil {
			return nil, err
		}
		pages = append(pages, res.Results...)

		if !res.HasMore || res.NextCursor == "" || len(res.Results) == 0 || len(pages) >= hardLimit {
			break
		}
		cursor = res.NextCursor
	}

	return flattenPages(pages, fields), nil
}

// CountRecords returns the number of pages matching the query's filter. Notion
// has no count endpoint, so the matching pages are paginated and counted. Only
// page metadata is fetched; the page bodies are discarded.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.DatabaseID) == "" {
		return 0, fmt.Errorf("databaseId is required")
	}

	endpoint := fmt.Sprintf("%s/v1/databases/%s/query", c.baseURL, url.PathEscape(q.DatabaseID))

	var total int64
	cursor := ""
	for {
		var res queryResponse
		if err := c.do(ctx, http.MethodPost, endpoint, c.buildQueryBody(q, defaultPageSize, cursor), &res); err != nil {
			return 0, err
		}
		total += int64(len(res.Results))

		if !res.HasMore || res.NextCursor == "" || len(res.Results) == 0 {
			break
		}
		cursor = res.NextCursor
	}

	return total, nil
}

// DatabaseInfo is a lightweight representation of a Notion database used for the
// resource handler that populates the QueryEditor database dropdown.
type DatabaseInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// searchResponse is the shape of POST /v1/search.
type searchResponse struct {
	Results []struct {
		ID    string     `json:"id"`
		Title []richText `json:"title"`
	} `json:"results"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// ListDatabases returns the databases shared with the integration via the search
// endpoint, filtered to database objects.
func (c *Client) ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	endpoint := fmt.Sprintf("%s/v1/search", c.baseURL)

	databases := make([]DatabaseInfo, 0)
	cursor := ""
	for {
		body := map[string]any{
			"filter":    map[string]any{"value": "database", "property": "object"},
			"page_size": defaultPageSize,
		}
		if cursor != "" {
			body["start_cursor"] = cursor
		}

		var res searchResponse
		if err := c.do(ctx, http.MethodPost, endpoint, body, &res); err != nil {
			return nil, err
		}
		for _, r := range res.Results {
			title := plainTitle(r.Title)
			if title == "" {
				title = r.ID
			}
			databases = append(databases, DatabaseInfo{ID: r.ID, Title: title})
		}

		if !res.HasMore || res.NextCursor == "" || len(res.Results) == 0 {
			break
		}
		cursor = res.NextCursor
	}

	return databases, nil
}

// PropertyInfo is a lightweight representation of a Notion database property used
// for the resource handler that populates the QueryEditor fields multi-select.
// Type is the raw Notion property type; Category is the logical category used by
// the filter builder.
type PropertyInfo struct {
	Title    string `json:"title"`
	Type     string `json:"type"`
	Category string `json:"category"`
}

type databaseResponse struct {
	Title      []richText `json:"title"`
	Properties map[string]struct {
		Type string `json:"type"`
	} `json:"properties"`
}

// ListProperties returns the properties (columns) of a database via the retrieve
// endpoint.
func (c *Client) ListProperties(ctx context.Context, databaseID string) ([]PropertyInfo, error) {
	if strings.TrimSpace(databaseID) == "" {
		return nil, fmt.Errorf("databaseId is required")
	}
	endpoint := fmt.Sprintf("%s/v1/databases/%s", c.baseURL, url.PathEscape(databaseID))

	var res databaseResponse
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &res); err != nil {
		return nil, err
	}
	props := make([]PropertyInfo, 0, len(res.Properties))
	for name, p := range res.Properties {
		if strings.TrimSpace(name) == "" {
			continue
		}
		props = append(props, PropertyInfo{
			Title:    name,
			Type:     p.Type,
			Category: categoryForNotionType(p.Type),
		})
	}
	return props, nil
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. It uses the bot-user endpoint, which any valid integration token
// can call.
func (c *Client) Ping(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/v1/users/me", c.baseURL)
	return c.do(ctx, http.MethodGet, endpoint, nil, nil)
}

// categoryForNotionType maps a raw Notion property type to the logical category
// used by the filter builder and value coercion.
func categoryForNotionType(t string) string {
	switch t {
	case "number", "unique_id":
		return "number"
	case "checkbox":
		return "checkbox"
	case "date", "created_time", "last_edited_time":
		return "date"
	case "select":
		return "select"
	case "status":
		return "status"
	case "multi_select":
		return "multi_select"
	case "people", "created_by", "last_edited_by":
		return "people"
	case "files":
		return "files"
	default: // title, rich_text, email, phone_number, url, formula, rollup, relation
		return "text"
	}
}

func plainTitle(items []richText) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString(it.PlainText)
	}
	return strings.TrimSpace(b.String())
}

func splitFields(fields string) []string {
	out := make([]string, 0)
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
