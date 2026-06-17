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
	// Baserow caps the page size at 200 rows per request.
	defaultPageLimit = 200
	maxRecords       = 100000
)

// Client is a thin wrapper around the Baserow REST API. It supports two auth
// modes: a static database token, or email/password which it exchanges for a
// (cached, auto-refreshed) JWT.
type Client struct {
	baseURL    string
	authMode   string
	apiToken   string
	email      string
	password   string
	databaseID string
	httpClient *http.Client

	mu  sync.Mutex
	jwt string // cached JWT for the password auth mode
}

// NewClient creates a Baserow API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	authMode := settings.AuthMode
	if authMode == "" {
		authMode = AuthToken
	}

	// Go's http.Client strips the Authorization header when a redirect changes
	// host (e.g. Baserow redirecting an internal host to its canonical
	// BASEROW_PUBLIC_URL), which surfaces as a 401 "credentials were not
	// provided". Re-attach the original Authorization header across redirects so
	// authenticated requests keep working. We never follow more than a handful of
	// hops.
	if httpClient != nil {
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if len(via) > 0 {
				if auth := via[0].Header.Get("Authorization"); auth != "" && req.Header.Get("Authorization") == "" {
					req.Header.Set("Authorization", auth)
				}
			}
			return nil
		}
	}

	return &Client{
		baseURL:    base,
		authMode:   authMode,
		apiToken:   strings.TrimSpace(settings.apiToken),
		email:      strings.TrimSpace(settings.Email),
		password:   settings.password,
		databaseID: strings.TrimSpace(settings.DatabaseID),
		httpClient: httpClient,
	}, nil
}

// usePassword reports whether the client authenticates via email/password (JWT).
func (c *Client) usePassword() bool {
	return c.authMode == AuthPassword
}

// authHeader returns the Authorization header value for the current auth mode.
// For the password mode it lazily obtains (and caches) a JWT.
func (c *Client) authHeader(ctx context.Context) (string, error) {
	if c.usePassword() {
		token, err := c.ensureJWT(ctx, false)
		if err != nil {
			return "", err
		}
		return "JWT " + token, nil
	}
	if c.apiToken == "" {
		return "", nil
	}
	return "Token " + c.apiToken, nil
}

type tokenAuthResponse struct {
	Token string `json:"token"`
}

// ensureJWT returns a cached JWT, fetching a fresh one if none is cached or
// force is set. It authenticates with the configured email/password.
func (c *Client) ensureJWT(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	if !force && c.jwt != "" {
		token := c.jwt
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	if c.email == "" || c.password == "" {
		return "", fmt.Errorf("email and password are required for password auth")
	}

	body, _ := json.Marshal(map[string]string{"email": c.email, "password": c.password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/user/token-auth/", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("baserow authentication failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if hint := htmlResponseHint(raw); hint != "" {
			return "", fmt.Errorf("baserow authentication failed (status %d): %s", resp.StatusCode, hint)
		}
		return "", fmt.Errorf("baserow authentication failed (status %d): %s", resp.StatusCode, truncate(string(raw), 500))
	}
	var ta tokenAuthResponse
	if err := json.Unmarshal(raw, &ta); err != nil || ta.Token == "" {
		if hint := htmlResponseHint(raw); hint != "" {
			return "", fmt.Errorf("baserow authentication returned an unexpected response: %s", hint)
		}
		return "", fmt.Errorf("baserow authentication returned no token")
	}

	c.mu.Lock()
	c.jwt = ta.Token
	c.mu.Unlock()
	return ta.Token, nil
}

func (c *Client) do(ctx context.Context, rawURL string, out any) error {
	return c.doWithRetry(ctx, rawURL, out, true)
}

// doWithRetry performs a GET. In password mode, a 401 (expired/invalid JWT)
// triggers a single re-authentication and retry.
func (c *Client) doWithRetry(ctx context.Context, rawURL string, out any, retry bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if auth, err := c.authHeader(ctx); err != nil {
		return err
	} else if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to baserow failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading baserow response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized && retry && c.usePassword() {
		// The JWT likely expired; force a refresh and retry once.
		if _, err := c.ensureJWT(ctx, true); err != nil {
			return err
		}
		return c.doWithRetry(ctx, rawURL, out, false)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if hint := htmlResponseHint(body); hint != "" {
			return fmt.Errorf("baserow returned status %d: %s", resp.StatusCode, hint)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("baserow returned status 401 (unauthorized): %s. %s",
				truncate(string(body), 300), unauthorizedHint(c.authMode))
		}
		return fmt.Errorf("baserow returned status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			if hint := htmlResponseHint(body); hint != "" {
				return fmt.Errorf("unexpected baserow response: %s", hint)
			}
			return fmt.Errorf("failed parsing baserow response: %w", err)
		}
	}
	return nil
}

// truncate shortens s to at most n runes for safe error messages.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// htmlResponseHint detects when Baserow returned its web frontend (HTML) instead
// of a JSON API response — typically because the Base URL points at the web app
// rather than the API, or (self-hosted all-in-one) the request host isn't a
// recognised Baserow URL so Caddy serves the application-builder 404. It returns
// an actionable message, or "" when the body is not HTML.
func htmlResponseHint(body []byte) string {
	head := strings.ToLower(strings.TrimSpace(string(body)))
	if len(head) > 200 {
		head = head[:200]
	}
	if !strings.HasPrefix(head, "<!doctype html") && !strings.HasPrefix(head, "<html") {
		return ""
	}
	return "the API returned an HTML page, not JSON. Check the Base URL points at the Baserow API " +
		"(e.g. https://api.baserow.io for Cloud, or your instance root for self-hosted). For a self-hosted " +
		"all-in-one container, ensure the URL Grafana uses is registered via BASEROW_PUBLIC_URL or " +
		"BASEROW_EXTRA_PUBLIC_URLS so /api/* routes to the backend."
}

// unauthorizedHint returns an actionable message for a 401 from Baserow,
// tailored to the configured auth mode.
func unauthorizedHint(authMode string) string {
	if authMode == AuthPassword {
		return "Email/password auth was rejected — verify the email and password are correct for this Baserow instance."
	}
	return "The database token was missing or rejected — verify the API Token is correct and not expired. " +
		"After (re)entering it in the data source config, click Save & test (saved secrets are write-only, " +
		"so an empty re-save can blank the token). Database tokens also need read access to the database."
}

// recordsResponse is the shape of GET /api/database/rows/table/{table_id}/.
type recordsResponse struct {
	Count    int              `json:"count"`
	Next     string           `json:"next"`
	Previous string           `json:"previous"`
	Results  []map[string]any `json:"results"`
}

// ListRecords fetches rows for a table, transparently following pagination up to
// the requested limit (or maxRecords when no limit is provided).
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	out := make([]map[string]any, 0, defaultPageLimit)
	page := 1

	for {
		size := defaultPageLimit
		if remaining := hardLimit - len(out); remaining < size {
			size = remaining
		}
		if size <= 0 {
			break
		}

		rows, hasNext, err := c.fetchRecordPage(ctx, q, size, page)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)

		if !hasNext || len(rows) == 0 || len(out) >= hardLimit {
			break
		}
		page++
	}

	return out, nil
}

func (c *Client) recordsEndpoint(tableID string) string {
	return fmt.Sprintf("%s/api/database/rows/table/%s/", c.baseURL, url.PathEscape(tableID))
}

func (c *Client) fetchRecordPage(ctx context.Context, q QueryModel, size, page int) ([]map[string]any, bool, error) {
	params := url.Values{}
	// user_field_names returns rows keyed by the human field names rather than
	// the internal field_<id> identifiers.
	params.Set("user_field_names", "true")
	params.Set("size", strconv.Itoa(size))
	params.Set("page", strconv.Itoa(page))
	if v := strings.TrimSpace(q.ViewID); v != "" {
		params.Set("view_id", v)
	}
	if filters := BuildFilters(q.filter); filters != "" {
		params.Set("filters", filters)
	}
	if v := strings.TrimSpace(q.Sort); v != "" {
		params.Set("order_by", v)
	}
	if v := strings.TrimSpace(q.Fields); v != "" {
		params.Set("include", v)
	}

	var res recordsResponse
	if err := c.do(ctx, c.recordsEndpoint(q.TableID)+"?"+params.Encode(), &res); err != nil {
		return nil, false, err
	}
	return res.Results, strings.TrimSpace(res.Next) != "", nil
}

// CountRecords returns the number of rows matching the query's filter. Baserow
// has no dedicated count endpoint, so it requests a single row and reads the
// `count` field from the paginated response (which respects `filters`).
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	if strings.TrimSpace(q.TableID) == "" {
		return 0, fmt.Errorf("tableId is required")
	}

	params := url.Values{}
	params.Set("user_field_names", "true")
	params.Set("size", "1")
	params.Set("page", "1")
	if v := strings.TrimSpace(q.ViewID); v != "" {
		params.Set("view_id", v)
	}
	if filters := BuildFilters(q.filter); filters != "" {
		params.Set("filters", filters)
	}

	var res recordsResponse
	if err := c.do(ctx, c.recordsEndpoint(q.TableID)+"?"+params.Encode(), &res); err != nil {
		return 0, err
	}
	return int64(res.Count), nil
}

// TableInfo is a lightweight representation of a Baserow table used for the
// resource handler that populates the QueryEditor table dropdown.
type TableInfo struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	DatabaseID string `json:"databaseId"`
}

type tableListItem struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}

// allTablesItem is the shape of GET /api/database/tables/all-tables/ which, unlike
// the per-database endpoint, accepts a database token.
type allTablesItem struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	DatabaseID json.Number `json:"database_id"`
}

// ListTables returns the tables available to the configured credentials,
// optionally scoped to a single database.
//
// The endpoint differs by auth mode because Baserow's per-database tables
// endpoint only accepts a JWT, while a database token must use the token-aware
// "all-tables" endpoint:
//   - password (JWT): GET /api/database/tables/database/{id}/ (database id required)
//   - token:          GET /api/database/tables/all-tables/    (filtered client-side)
func (c *Client) ListTables(ctx context.Context, databaseID string) ([]TableInfo, error) {
	db := strings.TrimSpace(databaseID)
	if db == "" {
		db = c.databaseID
	}

	if c.usePassword() {
		if db == "" {
			return nil, fmt.Errorf("databaseId is required")
		}
		endpoint := fmt.Sprintf("%s/api/database/tables/database/%s/", c.baseURL, url.PathEscape(db))
		var res []tableListItem
		if err := c.do(ctx, endpoint, &res); err != nil {
			return nil, err
		}
		tables := make([]TableInfo, 0, len(res))
		for _, t := range res {
			tables = append(tables, TableInfo{ID: t.ID.String(), Title: t.Name, DatabaseID: db})
		}
		return tables, nil
	}

	// Database token mode: list every table the token can access, then optionally
	// filter to the configured database.
	var res []allTablesItem
	if err := c.do(ctx, c.baseURL+"/api/database/tables/all-tables/", &res); err != nil {
		return nil, err
	}
	tables := make([]TableInfo, 0, len(res))
	for _, t := range res {
		dbID := t.DatabaseID.String()
		if db != "" && dbID != db {
			continue
		}
		tables = append(tables, TableInfo{ID: t.ID.String(), Title: t.Name, DatabaseID: dbID})
	}
	return tables, nil
}

// DatabaseInfo is a lightweight representation of a Baserow database
// (application) used to populate the database dropdown in the password auth mode.
type DatabaseInfo struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName"`
}

type workspaceListItem struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}

type applicationListItem struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
	Type string      `json:"type"`
}

// ListWorkspaces returns the workspaces accessible to the authenticated user.
// Only available in the password (JWT) auth mode.
func (c *Client) ListWorkspaces(ctx context.Context) ([]workspaceListItem, error) {
	if !c.usePassword() {
		return nil, fmt.Errorf("listing workspaces requires email/password auth")
	}
	var res []workspaceListItem
	if err := c.do(ctx, c.baseURL+"/api/workspaces/", &res); err != nil {
		return nil, err
	}
	return res, nil
}

// ListDatabases returns every database (application of type "database") across
// the user's workspaces. Only available in the password (JWT) auth mode.
func (c *Client) ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	if !c.usePassword() {
		return nil, fmt.Errorf("listing databases requires email/password auth")
	}
	workspaces, err := c.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	databases := make([]DatabaseInfo, 0)
	for _, ws := range workspaces {
		endpoint := fmt.Sprintf("%s/api/applications/workspace/%s/", c.baseURL, url.PathEscape(ws.ID.String()))
		var apps []applicationListItem
		if err := c.do(ctx, endpoint, &apps); err != nil {
			return nil, fmt.Errorf("failed listing databases for workspace %q: %w", ws.Name, err)
		}
		for _, app := range apps {
			if app.Type != "database" {
				continue
			}
			databases = append(databases, DatabaseInfo{
				ID:            app.ID.String(),
				Title:         app.Name,
				WorkspaceID:   ws.ID.String(),
				WorkspaceName: ws.Name,
			})
		}
	}
	return databases, nil
}

// FieldInfo is a lightweight representation of a Baserow field used for the
// resource handler that populates the QueryEditor fields multi-select.
type FieldInfo struct {
	Title string `json:"title"`
	Type  string `json:"type"`
}

type fieldListItem struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
	Type string      `json:"type"`
}

// ListFields returns the user-defined fields of a table.
func (c *Client) ListFields(ctx context.Context, tableID string) ([]FieldInfo, error) {
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	endpoint := fmt.Sprintf("%s/api/database/fields/table/%s/", c.baseURL, url.PathEscape(tableID))

	var res []fieldListItem
	if err := c.do(ctx, endpoint, &res); err != nil {
		return nil, err
	}
	fields := make([]FieldInfo, 0, len(res))
	for _, f := range res {
		if strings.TrimSpace(f.Name) == "" {
			continue
		}
		fields = append(fields, FieldInfo{Title: f.Name, Type: f.Type})
	}
	return fields, nil
}

// ViewInfo is a lightweight representation of a Baserow view used for the
// resource handler that populates the QueryEditor view dropdown.
type ViewInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type viewListItem struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}

// ListViews returns the views of a table.
//
// Baserow's views endpoint does not accept a database token (only a JWT), so in
// token mode this returns an empty list rather than erroring — the View picker
// is optional and a raw view id can still be typed manually.
func (c *Client) ListViews(ctx context.Context, tableID string) ([]ViewInfo, error) {
	if strings.TrimSpace(tableID) == "" {
		return nil, fmt.Errorf("tableId is required")
	}
	if !c.usePassword() {
		return []ViewInfo{}, nil
	}
	endpoint := fmt.Sprintf("%s/api/database/views/table/%s/", c.baseURL, url.PathEscape(tableID))

	var res []viewListItem
	if err := c.do(ctx, endpoint, &res); err != nil {
		return nil, err
	}
	views := make([]ViewInfo, 0, len(res))
	for _, v := range res {
		views = append(views, ViewInfo{ID: v.ID.String(), Title: v.Name})
	}
	return views, nil
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. In password mode it lists workspaces; in token mode it validates
// the database token via the token check endpoint (the only token-aware,
// table-agnostic endpoint).
func (c *Client) Ping(ctx context.Context) error {
	if c.usePassword() {
		return c.do(ctx, c.baseURL+"/api/workspaces/", nil)
	}
	return c.do(ctx, c.baseURL+"/api/database/tokens/check/", nil)
}
