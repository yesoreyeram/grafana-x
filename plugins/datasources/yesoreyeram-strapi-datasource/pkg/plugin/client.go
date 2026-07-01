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
	// defaultPageSize is Strapi's own default page size.
	defaultPageSize = 25
	// maxPageSize is Strapi's default maximum page size (api.rest.maxLimit). The
	// client never requests more than this in a single page; larger requests are
	// satisfied by following pages.
	maxPageSize = 100
)

type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("strapi base URL is required")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		apiToken:   strings.TrimSpace(settings.apiToken),
		httpClient: httpClient,
	}, nil
}

// apiURL builds a URL under the public content REST API (`/api`). path must start
// with a slash, e.g. "/articles".
func (c *Client) apiURL(path string) string {
	return c.baseURL + "/api" + path
}

// adminURL builds a URL under the admin/plugin namespace (no `/api` prefix), used
// by the content-type-builder endpoints. path must start with a slash.
func (c *Client) adminURL(path string) string {
	return c.baseURL + path
}

func (c *Client) do(ctx context.Context, method, rawURL string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to Strapi failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading Strapi response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("strapi returned status %d: %s. %s",
			resp.StatusCode, extractAPIError(rawBody), statusHint(resp.StatusCode))
	}

	if out != nil {
		if err := json.Unmarshal(rawBody, out); err != nil {
			return fmt.Errorf("failed parsing Strapi response: %w", err)
		}
	}
	return nil
}

// extractAPIError pulls the human-readable message out of a Strapi error body,
// which has the shape {"error":{"status":N,"name":"...","message":"..."}}. It
// falls back to the (truncated) raw body when the structure is absent.
func extractAPIError(raw []byte) string {
	var apiErr struct {
		Error struct {
			Status  int    `json:"status"`
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &apiErr) == nil && strings.TrimSpace(apiErr.Error.Message) != "" {
		return apiErr.Error.Message
	}
	return truncate(string(raw), 500)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The API token was missing or rejected — re-enter the API Token and click Save & test (saved secrets are write-only, so an empty re-save can blank the token)."
	case http.StatusForbidden:
		return "Access denied — ensure the API token's permissions grant access to this content type (Settings > API Tokens in the Strapi admin)."
	case http.StatusNotFound:
		return "Not found — verify the Base URL and content type plural API id (e.g. \"articles\") are correct and the token can access them."
	default:
		return ""
	}
}

// Ping validates connectivity and credentials. Strapi has no universal ping
// endpoint, so when a default content type is configured we issue a minimal
// request against it (a 2xx confirms both reachability and that the token is
// accepted). When no content type is configured we fall back to a lightweight
// reachability check that only fails on auth rejections or network errors.
func (c *Client) Ping(ctx context.Context, contentType string) error {
	contentType = strings.TrimSpace(contentType)
	if contentType != "" {
		params := url.Values{}
		params.Set("pagination[pageSize]", "1")
		return c.do(ctx, http.MethodGet, c.apiURL(c.contentTypeEndpoint(contentType))+"?"+params.Encode(), nil, nil)
	}
	return c.checkReachable(ctx)
}

// checkReachable issues a bare request to the content API root. Strapi returns
// 404 for that path, which still proves the base URL is reachable and the token
// was not rejected, so only auth failures (401/403) and network errors are
// treated as unhealthy.
func (c *Client) checkReachable(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL("/"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to Strapi failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("strapi returned status %d: %s. %s",
			resp.StatusCode, extractAPIError(raw), statusHint(resp.StatusCode))
	}
	return nil
}

// ContentTypeInfo represents a Strapi content type from the content-type-builder
// API. Note: this endpoint requires an admin JWT and is NOT accessible with a
// regular API token, so discovery commonly returns nothing — the query editor
// always allows typing the content type plural API id directly.
type ContentTypeInfo struct {
	UID    string `json:"uid"`
	Schema struct {
		DisplayName  string         `json:"displayName"`
		SingularName string         `json:"singularName"`
		PluralName   string         `json:"pluralName"`
		Attributes   map[string]any `json:"attributes"`
	} `json:"schema"`
}

// ListContentTypes attempts to enumerate content types via the
// content-type-builder admin endpoint. This requires an admin JWT (an API token
// is insufficient), so callers should treat failures as "discovery unavailable"
// rather than fatal errors.
func (c *Client) ListContentTypes(ctx context.Context) ([]ContentTypeInfo, error) {
	var res struct {
		Data []ContentTypeInfo `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, c.adminURL("/content-type-builder/content-types"), nil, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// FieldInfo is a lightweight representation of a Strapi field.
type FieldInfo struct {
	Field string `json:"field"`
	Type  string `json:"type"`
}

// ListFields derives the field list for a content type from the
// content-type-builder schema. Like ListContentTypes it needs an admin JWT and
// degrades to an error (handled gracefully by the caller) when only an API token
// is available.
func (c *Client) ListFields(ctx context.Context, contentTypeID string) ([]FieldInfo, error) {
	if strings.TrimSpace(contentTypeID) == "" {
		return nil, fmt.Errorf("contentTypeId is required")
	}

	contentTypes, err := c.ListContentTypes(ctx)
	if err != nil {
		return nil, err
	}

	for _, ct := range contentTypes {
		if ct.Schema.PluralName == contentTypeID || ct.Schema.SingularName == contentTypeID || ct.UID == contentTypeID {
			fields := make([]FieldInfo, 0, len(ct.Schema.Attributes))
			for name, attr := range ct.Schema.Attributes {
				typ := "string"
				if attrMap, ok := attr.(map[string]any); ok {
					if t, ok := attrMap["type"].(string); ok && t != "" {
						typ = t
					}
				}
				fields = append(fields, FieldInfo{Field: name, Type: typ})
			}
			return fields, nil
		}
	}

	return nil, fmt.Errorf("content type %q not found", contentTypeID)
}

// listResponse is the shape of GET /api/{pluralApiId} for BOTH Strapi v4 and v5.
// The elements of Data differ between versions (v4 wraps fields in `attributes`,
// v5 is flat with a `documentId`), but the envelope and `meta.pagination` are
// identical, so a single struct handles both. flattenRecord (frame.go) reconciles
// the per-element shape.
type listResponse struct {
	Data []map[string]any `json:"data"`
	Meta *struct {
		Pagination *struct {
			Page      int   `json:"page"`
			PageSize  int   `json:"pageSize"`
			PageCount int   `json:"pageCount"`
			Total     int64 `json:"total"`
		} `json:"pagination,omitempty"`
	} `json:"meta,omitempty"`
}

func (c *Client) contentTypeEndpoint(contentTypeID string) string {
	return "/" + url.PathEscape(contentTypeID)
}

// buildQueryParams compiles the shared, non-pagination query parameters
// (fields, filters, sort, populate) for a records request.
func (c *Client) buildQueryParams(q QueryModel) url.Values {
	params := url.Values{}

	for i, f := range splitCSV(q.Fields) {
		params.Set(fmt.Sprintf("fields[%d]", i), f)
	}

	if q.filter != nil {
		for k, vals := range BuildFilter(q.filter) {
			for _, v := range vals {
				params.Add(k, v)
			}
		}
	}

	i := 0
	for _, s := range q.sortItems {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		dir := "asc"
		if strings.EqualFold(s.Direction, "desc") {
			dir = "desc"
		}
		params.Set(fmt.Sprintf("sort[%d]", i), field+":"+dir)
		i++
	}

	addPopulateParams(params, q.Populate)

	return params
}

// addPopulateParams encodes the populate option. A bare "*" populates every
// first-level relation (`populate=*`); otherwise each comma-separated relation
// is indexed (`populate[0]=author`).
func addPopulateParams(params url.Values, populate string) {
	populate = strings.TrimSpace(populate)
	if populate == "" {
		return
	}
	if populate == "*" {
		params.Set("populate", "*")
		return
	}
	for i, p := range splitCSV(populate) {
		params.Set(fmt.Sprintf("populate[%d]", i), p)
	}
}

func clampPageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultPageSize
	}
	if pageSize > maxPageSize {
		return maxPageSize
	}
	return pageSize
}

// ListRecords fetches a single page of records for a content type using
// page-based pagination (`pagination[page]` / `pagination[pageSize]`). The data
// array is returned unmodified; per-version flattening happens in frame.go.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.ContentTypeID) == "" {
		return nil, fmt.Errorf("contentTypeId is required")
	}

	params := c.buildQueryParams(q)

	page := q.Page
	if page <= 0 {
		page = 1
	}
	params.Set("pagination[page]", strconv.Itoa(page))
	params.Set("pagination[pageSize]", strconv.Itoa(clampPageSize(q.PageSize)))

	endpoint := c.apiURL(c.contentTypeEndpoint(q.ContentTypeID)) + "?" + params.Encode()

	var res listResponse
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// CountRecords returns the number of records matching the query's filters. It
// requests a single-row page and reads meta.pagination.total, falling back to
// the returned data length when pagination metadata is unavailable.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.ContentTypeID) == "" {
		return 0, fmt.Errorf("contentTypeId is required")
	}

	params := url.Values{}
	params.Set("pagination[page]", "1")
	params.Set("pagination[pageSize]", "1")
	params.Set("pagination[withCount]", "true")

	if q.filter != nil {
		for k, vals := range BuildFilter(q.filter) {
			for _, v := range vals {
				params.Add(k, v)
			}
		}
	}

	endpoint := c.apiURL(c.contentTypeEndpoint(q.ContentTypeID)) + "?" + params.Encode()

	var res listResponse
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &res); err != nil {
		return 0, err
	}

	if res.Meta != nil && res.Meta.Pagination != nil {
		return res.Meta.Pagination.Total, nil
	}
	return int64(len(res.Data)), nil
}

func splitCSV(s string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
