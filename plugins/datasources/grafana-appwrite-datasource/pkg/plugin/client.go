package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Appwrite allows up to 100 documents per list request.
	defaultPageSize = 100
	// Safety cap on the number of documents fetched when no limit is given.
	maxDocuments = 100000
	// responseFormat pins the Appwrite API response format so the document
	// envelope (system attributes, list shape) stays stable across server
	// versions.
	responseFormat = "1.6.0"
)

// Client is a thin wrapper around the Appwrite REST API. It authenticates with a
// project id (`X-Appwrite-Project`) and an API key (`X-Appwrite-Key`).
type Client struct {
	endpoint   string
	projectID  string
	apiKey     string
	configDB   string
	httpClient *http.Client
}

// NewClient creates an Appwrite API client. The provided httpClient is normally
// the SDK-managed client so that proxy, TLS and timeout settings are respected.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(settings.Endpoint), "/")
	if endpoint == "" {
		endpoint = appwriteDefaultURL
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint URL %q: %w", endpoint, err)
	}
	return &Client{
		endpoint:   endpoint,
		projectID:  strings.TrimSpace(settings.ProjectID),
		apiKey:     strings.TrimSpace(settings.apiKey),
		configDB:   strings.TrimSpace(settings.DatabaseID),
		httpClient: httpClient,
	}, nil
}

// do performs a GET request to rawURL and unmarshals the JSON body into out.
func (c *Client) do(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Appwrite-Response-Format", responseFormat)
	if c.projectID != "" {
		req.Header.Set("X-Appwrite-Project", c.projectID)
	}
	if c.apiKey != "" {
		req.Header.Set("X-Appwrite-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to appwrite failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("failed reading appwrite response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("appwrite returned status %d: %s. %s",
			resp.StatusCode, truncate(string(body), 500), statusHint(resp.StatusCode))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("failed parsing appwrite response: %w", err)
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

// statusHint returns an actionable message for common Appwrite error statuses.
func statusHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "The API key or project id was missing or rejected — re-enter the API Key and click Save & test (saved secrets are write-only, so an empty re-save can blank the key)."
	case http.StatusForbidden:
		return "Access denied — ensure the API key has the required scopes (databases.read, collections.read, documents.read) and access to this project."
	case http.StatusNotFound:
		return "Not found — verify the Project ID, Database ID and Collection ID are correct and the key can access them."
	default:
		return ""
	}
}

// resolveDatabase returns the database id to use: the query's database id,
// falling back to the data-source-configured database id.
func (c *Client) resolveDatabase(queryDB string) string {
	if d := strings.TrimSpace(queryDB); d != "" {
		return d
	}
	return c.configDB
}

// documentsResponse is the shape of GET .../documents.
type documentsResponse struct {
	Total     int64            `json:"total"`
	Documents []map[string]any `json:"documents"`
}

// documentsEndpoint builds the list-documents URL for a database/collection.
func (c *Client) documentsEndpoint(databaseID, collectionID string) string {
	return fmt.Sprintf("%s/databases/%s/collections/%s/documents",
		c.endpoint, url.PathEscape(databaseID), url.PathEscape(collectionID))
}

// baseQueries returns the filter and select query strings shared by ListDocuments
// and CountDocuments. Ordering is added only by ListDocuments. A raw queries block
// (advanced field) takes precedence over the structured filter tree.
func (c *Client) baseQueries(q QueryModel) []string {
	queries := make([]string, 0)
	if raw := strings.TrimSpace(q.RawQueries); raw != "" {
		for _, line := range strings.Split(raw, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				queries = append(queries, line)
			}
		}
	} else {
		queries = append(queries, BuildFilterQueries(q.filter)...)
	}
	if sel, ok := selectQuery(q.Attributes, q.HideSystemFields); ok {
		queries = append(queries, sel)
	}
	return queries
}

// ListDocuments fetches documents for a collection, transparently following the
// cursor (cursorAfter on the last document's $id) up to the requested limit (or
// maxDocuments when no limit is provided).
func (c *Client) ListDocuments(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	databaseID := c.resolveDatabase(q.DatabaseID)
	if databaseID == "" {
		return nil, fmt.Errorf("databaseId is required")
	}
	if strings.TrimSpace(q.CollectionID) == "" {
		return nil, fmt.Errorf("collectionId is required")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxDocuments
	}

	endpoint := c.documentsEndpoint(databaseID, q.CollectionID)
	out := make([]map[string]any, 0, defaultPageSize)
	cursor := ""

	for {
		size := defaultPageSize
		if remaining := hardLimit - len(out); remaining < size {
			size = remaining
		}
		if size <= 0 {
			break
		}

		queries := c.baseQueries(q)
		queries = append(queries, orderQueries(q.sortItems)...)
		queries = append(queries, queryString{Method: "limit", Values: []any{size}}.encode())
		if cursor != "" {
			queries = append(queries, queryString{Method: "cursorAfter", Values: []any{cursor}}.encode())
		}

		var res documentsResponse
		if err := c.do(ctx, endpoint+"?"+encodeQueries(queries), &res); err != nil {
			return nil, err
		}
		out = append(out, res.Documents...)

		if len(res.Documents) < size || len(out) >= hardLimit {
			break
		}
		cursor = lastID(res.Documents)
		if cursor == "" {
			break
		}
	}

	return out, nil
}

// CountDocuments returns the number of documents matching the query's filter.
// Appwrite list responses include a `total` field computed for the filter, so a
// single minimal request (limit 1) is enough.
func (c *Client) CountDocuments(ctx context.Context, q QueryModel) (int64, error) {
	databaseID := c.resolveDatabase(q.DatabaseID)
	if databaseID == "" {
		return 0, fmt.Errorf("databaseId is required")
	}
	if strings.TrimSpace(q.CollectionID) == "" {
		return 0, fmt.Errorf("collectionId is required")
	}

	// Reuse the filter, but never the select projection (irrelevant to a count).
	count := QueryModel{filter: q.filter, RawQueries: q.RawQueries}
	queries := c.baseQueries(count)
	queries = append(queries, queryString{Method: "limit", Values: []any{1}}.encode())

	var res documentsResponse
	endpoint := c.documentsEndpoint(databaseID, q.CollectionID)
	if err := c.do(ctx, endpoint+"?"+encodeQueries(queries), &res); err != nil {
		return 0, err
	}
	return res.Total, nil
}

// lastID returns the $id of the last document in the slice, used as the
// pagination cursor.
func lastID(docs []map[string]any) string {
	if len(docs) == 0 {
		return ""
	}
	if id, ok := docs[len(docs)-1]["$id"].(string); ok {
		return id
	}
	return ""
}

// encodeQueries encodes a slice of Appwrite query strings into the repeated
// `queries[]` query parameter form.
func encodeQueries(queries []string) string {
	params := url.Values{}
	for _, qstr := range queries {
		if qstr != "" {
			params.Add("queries[]", qstr)
		}
	}
	return params.Encode()
}

// DatabaseInfo is a lightweight representation of an Appwrite database used to
// populate the database dropdown in the query editor.
type DatabaseInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type databasesResponse struct {
	Total     int64 `json:"total"`
	Databases []struct {
		ID   string `json:"$id"`
		Name string `json:"name"`
	} `json:"databases"`
}

// ListDatabases returns every database in the project, following pagination with
// a cursor on the last database's $id.
//
// Appwrite has two database APIs: the legacy `/databases` and the newer
// `/tablesdb`. Databases created via TablesDB are not returned by the legacy
// list endpoint (it responds with an empty array), so when `/databases` yields
// nothing we fall back to `/tablesdb`, which lists databases from both worlds.
func (c *Client) ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	out, err := c.listDatabasesAt(ctx, c.endpoint+"/databases")
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		fallback, fbErr := c.listDatabasesAt(ctx, c.endpoint+"/tablesdb")
		if fbErr == nil && len(fallback) > 0 {
			return fallback, nil
		}
	}
	return out, nil
}

// listDatabasesAt lists databases from a specific endpoint (the legacy
// `/databases` or the TablesDB `/tablesdb`), following the cursor.
func (c *Client) listDatabasesAt(ctx context.Context, endpoint string) ([]DatabaseInfo, error) {
	out := make([]DatabaseInfo, 0)
	cursor := ""
	for {
		queries := []string{queryString{Method: "limit", Values: []any{defaultPageSize}}.encode()}
		if cursor != "" {
			queries = append(queries, queryString{Method: "cursorAfter", Values: []any{cursor}}.encode())
		}
		var res databasesResponse
		if err := c.do(ctx, endpoint+"?"+encodeQueries(queries), &res); err != nil {
			return nil, err
		}
		for _, d := range res.Databases {
			out = append(out, DatabaseInfo{ID: d.ID, Name: d.Name})
		}
		if len(res.Databases) < defaultPageSize {
			break
		}
		cursor = res.Databases[len(res.Databases)-1].ID
	}
	return out, nil
}

// CollectionInfo is a lightweight representation of an Appwrite collection used
// for the resource handler that populates the QueryEditor collection dropdown.
type CollectionInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// collectionsResponse covers both the legacy `/collections` envelope and the
// TablesDB `/tables` envelope; only one of the arrays is populated per response.
type collectionsResponse struct {
	Total       int64        `json:"total"`
	Collections []idNameItem `json:"collections"`
	Tables      []idNameItem `json:"tables"`
}

type idNameItem struct {
	ID   string `json:"$id"`
	Name string `json:"name"`
}

// ListCollections returns the collections (a.k.a. tables) of a database
// (id + name), following pagination. It tries the legacy `/collections`
// endpoint first and falls back to the TablesDB `/tables` endpoint when the
// legacy one returns nothing (databases created via TablesDB).
func (c *Client) ListCollections(ctx context.Context, databaseID string) ([]CollectionInfo, error) {
	db := c.resolveDatabase(databaseID)
	if db == "" {
		return nil, fmt.Errorf("databaseId is required")
	}

	legacy := fmt.Sprintf("%s/databases/%s/collections", c.endpoint, url.PathEscape(db))
	out, err := c.listCollectionsAt(ctx, legacy)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		tablesEndpoint := fmt.Sprintf("%s/tablesdb/%s/tables", c.endpoint, url.PathEscape(db))
		if fallback, fbErr := c.listCollectionsAt(ctx, tablesEndpoint); fbErr == nil && len(fallback) > 0 {
			return fallback, nil
		}
	}
	return out, nil
}

// listCollectionsAt lists collections/tables from a specific endpoint, reading
// whichever of the `collections`/`tables` arrays the server populates.
func (c *Client) listCollectionsAt(ctx context.Context, endpoint string) ([]CollectionInfo, error) {
	out := make([]CollectionInfo, 0)
	cursor := ""
	for {
		queries := []string{queryString{Method: "limit", Values: []any{defaultPageSize}}.encode()}
		if cursor != "" {
			queries = append(queries, queryString{Method: "cursorAfter", Values: []any{cursor}}.encode())
		}
		var res collectionsResponse
		if err := c.do(ctx, endpoint+"?"+encodeQueries(queries), &res); err != nil {
			return nil, err
		}
		items := res.Collections
		if len(items) == 0 {
			items = res.Tables
		}
		for _, col := range items {
			out = append(out, CollectionInfo{ID: col.ID, Name: col.Name})
		}
		if len(items) < defaultPageSize {
			break
		}
		cursor = items[len(items)-1].ID
	}
	return out, nil
}

// AttributeInfo is a lightweight representation of an Appwrite attribute used for
// the resource handler that populates the QueryEditor attributes multi-select and
// the type-aware filter operators.
type AttributeInfo struct {
	Key  string `json:"key"`
	Type string `json:"type"`
}

// attributesResponse covers both the legacy `/attributes` envelope and the
// TablesDB `/columns` envelope; only one of the arrays is populated per response.
type attributesResponse struct {
	Total      int64         `json:"total"`
	Attributes []keyTypeItem `json:"attributes"`
	Columns    []keyTypeItem `json:"columns"`
}

type keyTypeItem struct {
	Key  string `json:"key"`
	Type string `json:"type"`
}

// ListAttributes returns the attributes (a.k.a. columns) of a collection within
// a database, following pagination. It tries the legacy `/attributes` endpoint
// first and falls back to the TablesDB `/columns` endpoint when the legacy one
// returns nothing. Appwrite attribute types are: string, integer, double,
// boolean, datetime, email, ip, url, enum, relationship.
func (c *Client) ListAttributes(ctx context.Context, databaseID, collectionID string) ([]AttributeInfo, error) {
	if strings.TrimSpace(collectionID) == "" {
		return nil, fmt.Errorf("collectionId is required")
	}
	db := c.resolveDatabase(databaseID)
	if db == "" {
		return nil, fmt.Errorf("databaseId is required")
	}

	legacy := fmt.Sprintf("%s/databases/%s/collections/%s/attributes",
		c.endpoint, url.PathEscape(db), url.PathEscape(collectionID))
	out, err := c.listAttributesAt(ctx, legacy)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		columnsEndpoint := fmt.Sprintf("%s/tablesdb/%s/tables/%s/columns",
			c.endpoint, url.PathEscape(db), url.PathEscape(collectionID))
		if fallback, fbErr := c.listAttributesAt(ctx, columnsEndpoint); fbErr == nil && len(fallback) > 0 {
			return fallback, nil
		}
	}
	return out, nil
}

// listAttributesAt lists attributes/columns from a specific endpoint, reading
// whichever of the `attributes`/`columns` arrays the server populates.
func (c *Client) listAttributesAt(ctx context.Context, endpoint string) ([]AttributeInfo, error) {
	out := make([]AttributeInfo, 0)
	offset := 0
	for {
		queries := []string{
			queryString{Method: "limit", Values: []any{defaultPageSize}}.encode(),
			queryString{Method: "offset", Values: []any{offset}}.encode(),
		}
		var res attributesResponse
		if err := c.do(ctx, endpoint+"?"+encodeQueries(queries), &res); err != nil {
			return nil, err
		}
		items := res.Attributes
		if len(items) == 0 {
			items = res.Columns
		}
		for _, a := range items {
			if strings.TrimSpace(a.Key) == "" {
				continue
			}
			out = append(out, AttributeInfo{Key: a.Key, Type: a.Type})
		}
		if len(items) < defaultPageSize {
			break
		}
		offset += defaultPageSize
	}
	return out, nil
}

// Ping performs a minimal authenticated request to validate connectivity and
// credentials. It lists databases with limit 1, which only requires a valid
// project id and API key with databases.read access.
func (c *Client) Ping(ctx context.Context) error {
	queries := []string{queryString{Method: "limit", Values: []any{1}}.encode()}
	return c.do(ctx, c.endpoint+"/databases?"+encodeQueries(queries), nil)
}
