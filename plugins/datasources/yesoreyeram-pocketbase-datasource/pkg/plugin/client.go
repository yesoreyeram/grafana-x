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
	"sync"
)

const (
	// PocketBase allows up to 500 records per list request; 200 keeps responses
	// reasonable while minimising round trips.
	defaultPageSize = 200
	// Safety cap on the number of records fetched when no limit is given.
	maxRecords = 100000
)

// Client is a thin wrapper around the PocketBase REST API. It authenticates by
// exchanging an identity/password for an auth token (superuser or user mode) or
// by using a pre-issued token verbatim (token mode), and sends the token in the
// `Authorization` header.
type Client struct {
	baseURL        string
	mode           AuthMode
	authCollection string
	identity       string
	password       string
	staticToken    string
	httpClient     *http.Client

	mu    sync.Mutex
	token string
}

// NewClient creates a PocketBase API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.URL), "/")
	if base == "" {
		base = pocketbaseDefaultURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid PocketBase URL %q: %w", base, err)
	}
	mode := settings.AuthMode
	if mode == "" {
		mode = AuthModeSuperuser
	}
	return &Client{
		baseURL:        base,
		mode:           mode,
		authCollection: settings.effectiveAuthCollection(),
		identity:       strings.TrimSpace(settings.Identity),
		password:       settings.password,
		staticToken:    strings.TrimSpace(settings.authToken),
		httpClient:     httpClient,
	}, nil
}

// authResponse is the shape of POST .../auth-with-password.
type authResponse struct {
	Token string `json:"token"`
}

// ensureToken returns a valid auth token, authenticating on first use. For token
// mode the configured static token is returned directly.
func (c *Client) ensureToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mode == AuthModeToken {
		if c.staticToken == "" {
			return "", fmt.Errorf("auth token is not configured (re-enter it; saved secrets are write-only)")
		}
		return c.staticToken, nil
	}
	if c.token != "" {
		return c.token, nil
	}
	if err := c.authenticateLocked(ctx); err != nil {
		return "", err
	}
	return c.token, nil
}

// resetToken clears the cached token so the next request re-authenticates. It is
// a no-op in token mode (there is nothing to refresh).
func (c *Client) resetToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mode != AuthModeToken {
		c.token = ""
	}
}

// authenticateLocked exchanges the identity/password for an auth token. The
// caller must hold c.mu.
func (c *Client) authenticateLocked(ctx context.Context) error {
	if c.identity == "" || c.password == "" {
		return fmt.Errorf("identity and password are required for %s auth", c.mode)
	}
	payload, _ := json.Marshal(map[string]string{
		"identity": c.identity,
		"password": c.password,
	})
	endpoint := fmt.Sprintf("%s/api/collections/%s/auth-with-password",
		c.baseURL, url.PathEscape(c.authCollection))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to PocketBase failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed reading PocketBase auth response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PocketBase authentication failed with status %d: %s. %s",
			resp.StatusCode, truncate(string(body), 300), authHint(resp.StatusCode))
	}

	var ar authResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return fmt.Errorf("failed parsing PocketBase auth response: %w", err)
	}
	if ar.Token == "" {
		return fmt.Errorf("PocketBase authentication succeeded but returned no token")
	}
	c.token = ar.Token
	return nil
}

// do performs an authenticated GET request to baseURL+path with the given query
// and unmarshals the JSON body into out. On a 401 it transparently
// re-authenticates once (for password modes) and retries.
func (c *Client) do(ctx context.Context, path string, query url.Values, out any) error {
	rawURL := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		rawURL += "?" + encoded
	}

	const maxAttempts = 2
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		token, err := c.ensureToken(ctx)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		if token != "" {
			// PocketBase reads the raw token from the Authorization header (the
			// official SDKs send it without a scheme prefix).
			req.Header.Set("Authorization", token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request to PocketBase failed: %w", err)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed reading PocketBase response: %w", err)
		}

		// An expired/invalid cached token yields 401; drop it and retry once.
		if resp.StatusCode == http.StatusUnauthorized && c.mode != AuthModeToken && attempt+1 < maxAttempts {
			c.resetToken()
			lastErr = fmt.Errorf("PocketBase returned status 401: %s", truncate(string(body), 300))
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("PocketBase returned status %d: %s. %s",
				resp.StatusCode, truncate(string(body), 500), statusHint(resp.StatusCode))
		}

		if out != nil {
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("failed parsing PocketBase response: %w", err)
			}
		}
		return nil
	}
	return lastErr
}

// truncate shortens s to at most n bytes for safe error messages.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// authHint returns an actionable message for common authentication failures.
func authHint(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return "Check the identity (email) and password — and that the auth mode/collection match the account (superusers use the _superusers collection)."
	default:
		return ""
	}
}

// statusHint returns an actionable message for common PocketBase error statuses.
func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The auth token was missing, expired or rejected — re-check the credentials and click Save & test."
	case http.StatusForbidden:
		return "Access denied — listing collections and reading records with restrictive API rules requires superuser auth, or a collection listRule that permits the authenticated user."
	case http.StatusNotFound:
		return "Not found — verify the collection id/name and the PocketBase URL."
	default:
		return ""
	}
}

// recordsResponse is the shape of GET /api/collections/{c}/records.
type recordsResponse struct {
	Page       int              `json:"page"`
	PerPage    int              `json:"perPage"`
	TotalItems int64            `json:"totalItems"`
	TotalPages int64            `json:"totalPages"`
	Items      []map[string]any `json:"items"`
}

// recordsPath builds the list-records path for a collection id or name.
func recordsPath(collectionID string) string {
	return fmt.Sprintf("/api/collections/%s/records", url.PathEscape(collectionID))
}

// baseFilter returns the filter expression shared by ListRecords and
// CountRecords. A raw filter (advanced field) takes precedence over the
// structured filter tree.
func baseFilter(q QueryModel) string {
	if raw := strings.TrimSpace(q.RawFilter); raw != "" {
		return raw
	}
	return BuildFilter(q.filter)
}

// ListRecords fetches records for a collection, transparently following the
// offset pagination (page/perPage) up to the requested limit (or maxRecords when
// no limit is provided). Row order is preserved as returned by PocketBase.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.CollectionID) == "" {
		return nil, fmt.Errorf("collectionId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	path := recordsPath(q.CollectionID)
	filter := baseFilter(q)
	sort := sortParam(q.sortItems)
	fields, hasFields := fieldsParam(q.Fields, q.HideSystemFields)

	out := make([]map[string]any, 0, defaultPageSize)
	page := 1
	for {
		perPage := defaultPageSize
		if remaining := hardLimit - len(out); remaining < perPage {
			perPage = remaining
		}
		if perPage <= 0 {
			break
		}

		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("perPage", strconv.Itoa(perPage))
		// We paginate ourselves and never need the (slower) total count here.
		query.Set("skipTotal", "1")
		if filter != "" {
			query.Set("filter", filter)
		}
		if sort != "" {
			query.Set("sort", sort)
		}
		if hasFields {
			query.Set("fields", fields)
		}

		var res recordsResponse
		if err := c.do(ctx, path, query, &res); err != nil {
			return nil, err
		}
		out = append(out, res.Items...)

		if len(res.Items) < perPage || len(out) >= hardLimit {
			break
		}
		page++
	}

	return out, nil
}

// CountRecords returns the number of records matching the query's filter.
// PocketBase list responses include a filter-aware `totalItems`, so a single
// minimal request (perPage 1, total not skipped) is enough.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.CollectionID) == "" {
		return 0, fmt.Errorf("collectionId is required")
	}

	query := url.Values{}
	query.Set("page", "1")
	query.Set("perPage", "1")
	if filter := baseFilter(q); filter != "" {
		query.Set("filter", filter)
	}

	var res recordsResponse
	if err := c.do(ctx, recordsPath(q.CollectionID), query, &res); err != nil {
		return 0, err
	}
	return res.TotalItems, nil
}

// CollectionInfo is a lightweight representation of a PocketBase collection used
// to populate the collection dropdown in the query editor.
type CollectionInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// FieldInfo is a lightweight representation of a collection field used for the
// QueryEditor fields multi-select and the type-aware filter operators.
type FieldInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// collectionItem mirrors the relevant parts of a PocketBase collection. The
// schema is exposed as `fields` (PocketBase >= 0.23) or `schema` (older); both
// are parsed and whichever is populated is used.
type collectionItem struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Type   string      `json:"type"`
	System bool        `json:"system"`
	Fields []fieldItem `json:"fields"`
	Schema []fieldItem `json:"schema"`
}

func (ci collectionItem) fieldList() []fieldItem {
	if len(ci.Fields) > 0 {
		return ci.Fields
	}
	return ci.Schema
}

type fieldItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	System bool   `json:"system"`
	Hidden bool   `json:"hidden"`
}

type collectionsResponse struct {
	Page       int              `json:"page"`
	PerPage    int              `json:"perPage"`
	TotalItems int64            `json:"totalItems"`
	TotalPages int64            `json:"totalPages"`
	Items      []collectionItem `json:"items"`
}

// ListCollections returns every non-system collection (id, name, type),
// following pagination. Listing collections requires superuser auth in
// PocketBase; in user/token mode this may return a 403.
func (c *Client) ListCollections(ctx context.Context) ([]CollectionInfo, error) {
	out := make([]CollectionInfo, 0)
	page := 1
	for {
		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("perPage", strconv.Itoa(defaultPageSize))

		var res collectionsResponse
		if err := c.do(ctx, "/api/collections", query, &res); err != nil {
			return nil, err
		}
		for _, col := range res.Items {
			// Skip PocketBase's internal/system collections (e.g. _superusers,
			// _authOrigins) to keep the picker focused on user data.
			if col.System {
				continue
			}
			out = append(out, CollectionInfo{ID: col.ID, Name: col.Name, Type: col.Type})
		}
		if len(res.Items) < defaultPageSize {
			break
		}
		page++
	}
	return out, nil
}

// CollectionFields returns the queryable fields of a collection (name + type).
// Hidden fields and password fields are omitted. Requires superuser auth.
func (c *Client) CollectionFields(ctx context.Context, collectionID string) ([]FieldInfo, error) {
	if strings.TrimSpace(collectionID) == "" {
		return nil, fmt.Errorf("collectionId is required")
	}

	var col collectionItem
	path := fmt.Sprintf("/api/collections/%s", url.PathEscape(collectionID))
	if err := c.do(ctx, path, nil, &col); err != nil {
		return nil, err
	}

	out := make([]FieldInfo, 0)
	for _, f := range col.fieldList() {
		if strings.TrimSpace(f.Name) == "" || f.Hidden || f.Type == "password" {
			continue
		}
		out = append(out, FieldInfo{Name: f.Name, Type: f.Type})
	}
	return out, nil
}

// healthResponse is the shape of GET /api/health.
type healthResponse struct {
	Code int `json:"code"`
}

// Ping validates connectivity and credentials. It first checks the public
// /api/health endpoint for reachability, then (for password modes)
// authenticates to validate the identity/password.
func (c *Client) Ping(ctx context.Context) error {
	// /api/health is public (no auth) — verify the URL is reachable first.
	rawURL := c.baseURL + "/api/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to PocketBase failed: %w", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PocketBase health check returned status %d: %s",
			resp.StatusCode, truncate(string(body), 300))
	}
	var hr healthResponse
	_ = json.Unmarshal(body, &hr)

	// For password modes, authenticating validates the credentials.
	if c.mode != AuthModeToken {
		if _, err := c.ensureToken(ctx); err != nil {
			return err
		}
	} else if c.staticToken == "" {
		return fmt.Errorf("auth token is not configured (re-enter it; saved secrets are write-only)")
	}
	return nil
}
