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
	// Pipedrive v1 list endpoints allow a maximum page size of 500.
	listPageSize = 500
	// maxRecords is a safety cap on the total number of records fetched across
	// pages when no explicit limit is set.
	maxRecords = 100000
)

// Client is a thin wrapper around Pipedrive's REST API v1.
type Client struct {
	baseURL    string
	authMethod string
	token      string
	httpClient *http.Client
}

// NewClient creates a Pipedrive API client. Validation of the credential and
// company domain is intentionally deferred to CheckHealth so that the data
// source instance can always be created and report a friendly message.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	c := &Client{
		authMethod: settings.authMode(),
		token:      settings.credential(),
		httpClient: httpClient,
	}
	domain := strings.TrimSpace(settings.CompanyDomain)
	if domain != "" {
		baseURL := fmt.Sprintf("https://%s.pipedrive.com/api/v1", domain)
		if _, err := url.ParseRequestURI(baseURL); err != nil {
			return nil, fmt.Errorf("invalid base URL %q: %w", baseURL, err)
		}
		c.baseURL = baseURL
	}
	return c, nil
}

// ----- Response envelope ------------------------------------------------------

// pipedriveResponse is the standard Pipedrive API response envelope.
type pipedriveResponse struct {
	Success        bool            `json:"success"`
	Data           json.RawMessage `json:"data"`
	AdditionalData *additionalData `json:"additional_data,omitempty"`
	Error          string          `json:"error"`
	ErrorInfo      string          `json:"error_info"`
}

type additionalData struct {
	Pagination *paginationInfo `json:"pagination,omitempty"`
}

// paginationInfo is the v1 offset pagination block. The loop must follow
// more_items_in_collection / next_start, NOT blindly increment start.
type paginationInfo struct {
	Start                 int  `json:"start"`
	Limit                 int  `json:"limit"`
	MoreItemsInCollection bool `json:"more_items_in_collection"`
	NextStart             *int `json:"next_start"`
}

func (r *pipedriveResponse) pagination() *paginationInfo {
	if r == nil || r.AdditionalData == nil {
		return nil
	}
	return r.AdditionalData.Pagination
}

// ----- HTTP --------------------------------------------------------------------

// get issues an authenticated GET request and returns the parsed envelope.
// Authentication is applied per the configured mode: api_token as a query
// parameter, or an Authorization: Bearer header for OAuth.
func (c *Client) get(ctx context.Context, path string, params url.Values) (*pipedriveResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("company domain is not configured")
	}

	q := url.Values{}
	for k, vs := range params {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	if c.authMethod != authOAuth && c.token != "" {
		q.Set("api_token", c.token)
	}

	full := c.baseURL + path
	if encoded := q.Encode(); encoded != "" {
		full += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.authMethod == authOAuth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to pipedrive failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading pipedrive response: %w", err)
	}

	var pr pipedriveResponse
	parseErr := json.Unmarshal(raw, &pr)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pipedrive returned status %d: %s", resp.StatusCode, extractError(raw, &pr))
	}
	if parseErr != nil {
		return nil, fmt.Errorf("failed parsing pipedrive response: %w", parseErr)
	}
	if !pr.Success {
		return nil, fmt.Errorf("pipedrive request failed: %s", extractError(raw, &pr))
	}
	return &pr, nil
}

// extractError returns the most useful error message from a Pipedrive response.
func extractError(raw []byte, pr *pipedriveResponse) string {
	if pr != nil {
		msg := strings.TrimSpace(pr.Error)
		if info := strings.TrimSpace(pr.ErrorInfo); info != "" && info != msg {
			if msg != "" {
				msg = msg + " (" + info + ")"
			} else {
				msg = info
			}
		}
		if msg != "" {
			return truncate(msg)
		}
	}
	return truncate(string(raw))
}

// ----- List & pagination -------------------------------------------------------

// ListRecords fetches a paginated entity list, transparently following the v1
// offset pagination, then optionally remaps custom field hash keys to names.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	entity := q.QueryType
	if _, ok := entityListPath[entity]; !ok {
		return nil, fmt.Errorf("unsupported entity: %s", entity)
	}

	records, err := c.listAll(ctx, "/"+entityListPath[entity], entityListParams(entity, q), q.Limit, q.Start)
	if err != nil {
		return nil, err
	}

	if q.shouldMapCustomFields() {
		if fieldMap, ferr := c.fetchFieldMap(ctx, entity); ferr == nil {
			remapCustomFields(records, fieldMap)
		}
		// A failure to fetch the field map is non-fatal: the records are still
		// returned, keyed by their raw custom-field hashes.
	}
	return records, nil
}

// listAll loops over the v1 list endpoint following more_items_in_collection /
// next_start until the hard limit (or the safety cap) is reached.
func (c *Client) listAll(ctx context.Context, path string, params url.Values, hardLimit, startOffset int) ([]map[string]any, error) {
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}
	start := startOffset
	if start < 0 {
		start = 0
	}

	out := make([]map[string]any, 0, listPageSize)
	for {
		remaining := hardLimit - len(out)
		if remaining <= 0 {
			break
		}
		pageLimit := listPageSize
		if remaining < pageLimit {
			pageLimit = remaining
		}

		p := cloneValues(params)
		p.Set("limit", strconv.Itoa(pageLimit))
		p.Set("start", strconv.Itoa(start))

		resp, err := c.get(ctx, path, p)
		if err != nil {
			return nil, err
		}

		items := decodeRecords(resp.Data)
		out = append(out, items...)

		if len(out) >= hardLimit {
			break
		}
		pag := resp.pagination()
		if pag == nil || !pag.MoreItemsInCollection || len(items) == 0 {
			break
		}
		if pag.NextStart != nil {
			start = *pag.NextStart
		} else {
			start += pageLimit
		}
	}

	if len(out) > hardLimit {
		out = out[:hardLimit]
	}
	return out, nil
}

// CountRecords returns the number of records matching the filters for the chosen
// entity. Pipedrive has no count endpoint for list resources, so the records are
// paginated (following more_items_in_collection / next_start) and counted.
// Only the lightweight record metadata is parsed; nothing is retained.
//
// Note: for deals specifically, the GET /deals/summary endpoint also returns a
// total count and is more efficient, but pagination is used here so a single
// implementation correctly counts every entity type.
func (c *Client) CountRecords(ctx context.Context, q QueryModel) (int64, error) {
	entity := q.CountEntity
	if entity == "" {
		entity = queryTypeDeals
	}
	if _, ok := entityListPath[entity]; !ok {
		return 0, fmt.Errorf("unsupported count entity: %s", entity)
	}

	params := entityListParams(entity, q)
	var total int64
	start := 0
	for {
		p := cloneValues(params)
		p.Set("limit", strconv.Itoa(listPageSize))
		p.Set("start", strconv.Itoa(start))

		resp, err := c.get(ctx, "/"+entityListPath[entity], p)
		if err != nil {
			return 0, err
		}
		n := countData(resp.Data)
		total += int64(n)

		pag := resp.pagination()
		if pag == nil || !pag.MoreItemsInCollection || n == 0 {
			break
		}
		if total >= maxRecords {
			break
		}
		if pag.NextStart != nil {
			start = *pag.NextStart
		} else {
			start += listPageSize
		}
	}
	return total, nil
}

// entityListParams builds the Pipedrive list query params for an entity from the
// query model. When FilterId is set it takes precedence (Pipedrive ignores the
// other filters), mirroring the API's documented behaviour.
func entityListParams(entity string, q QueryModel) url.Values {
	p := url.Values{}

	if filterID := strings.TrimSpace(q.FilterId); filterID != "" {
		p.Set("filter_id", filterID)
	} else {
		switch entity {
		case queryTypeDeals:
			if q.Status != "" && q.Status != "all" {
				p.Set("status", q.Status)
			}
			if q.PipelineId != "" {
				p.Set("pipeline_id", q.PipelineId)
			}
			if q.StageId != "" {
				p.Set("stage_id", q.StageId)
			}
			if q.UserId != "" {
				p.Set("user_id", q.UserId)
			}
		case queryTypePersons, queryTypeOrganizations:
			if q.UserId != "" {
				p.Set("user_id", q.UserId)
			}
		}
	}

	if sortBy := strings.TrimSpace(q.SortBy); sortBy != "" {
		dir := "DESC"
		if strings.EqualFold(strings.TrimSpace(q.SortDir), "ASC") {
			dir = "ASC"
		}
		// v1 sort syntax: "field_name ASC" / "field_name DESC".
		p.Set("sort", sortBy+" "+dir)
	}
	return p
}

// ----- Custom field mapping ----------------------------------------------------

// fieldDefinition is the subset of a Pipedrive {entity}Field we need: the
// custom-field hash key and its human-readable name.
type fieldDefinition struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// fetchFieldMap returns a map of custom-field hash key -> field name for the
// given entity, by paginating the corresponding {entity}Fields endpoint.
func (c *Client) fetchFieldMap(ctx context.Context, entity string) (map[string]string, error) {
	fieldsPath, ok := entityFieldsPath[entity]
	if !ok {
		return nil, nil
	}

	out := map[string]string{}
	start := 0
	for {
		p := url.Values{}
		p.Set("limit", strconv.Itoa(listPageSize))
		p.Set("start", strconv.Itoa(start))

		resp, err := c.get(ctx, "/"+fieldsPath, p)
		if err != nil {
			return nil, err
		}
		var defs []fieldDefinition
		if err := json.Unmarshal(resp.Data, &defs); err != nil {
			return nil, fmt.Errorf("failed parsing %s: %w", fieldsPath, err)
		}
		for _, d := range defs {
			key := strings.TrimSpace(d.Key)
			name := strings.TrimSpace(d.Name)
			// Only custom fields use 40-character hash keys; standard fields
			// (e.g. "title", "add_time") are left untouched.
			if isCustomFieldHash(key) && name != "" {
				out[key] = name
			}
		}
		pag := resp.pagination()
		if pag == nil || !pag.MoreItemsInCollection || len(defs) == 0 {
			break
		}
		if pag.NextStart != nil {
			start = *pag.NextStart
		} else {
			start += listPageSize
		}
	}
	return out, nil
}

// remapCustomFields renames record keys that are custom-field hashes (or
// hash-derived subfields like "{hash}_currency") to their human-readable names.
func remapCustomFields(records []map[string]any, fieldMap map[string]string) {
	if len(fieldMap) == 0 {
		return
	}
	for _, rec := range records {
		type rename struct {
			from string
			to   string
			val  any
		}
		var renames []rename
		for key, val := range rec {
			if newKey, ok := mappedKey(key, fieldMap); ok && newKey != key {
				renames = append(renames, rename{from: key, to: newKey, val: val})
			}
		}
		for _, r := range renames {
			// Never clobber an existing column; keep the hash key if the mapped
			// name already exists to avoid silent data loss.
			if _, exists := rec[r.to]; exists {
				continue
			}
			delete(rec, r.from)
			rec[r.to] = r.val
		}
	}
}

// mappedKey resolves a record key to its mapped name. It handles both bare
// hashes and hash-derived subfields ("{hash}_currency", "{hash}_until", ...).
func mappedKey(key string, fieldMap map[string]string) (string, bool) {
	if name, ok := fieldMap[key]; ok {
		return name, true
	}
	if len(key) > 41 && key[40] == '_' && isCustomFieldHash(key[:40]) {
		if name, ok := fieldMap[key[:40]]; ok {
			return name + "_" + key[41:], true
		}
	}
	return key, false
}

// isCustomFieldHash reports whether s is a 40-character lowercase hex string,
// the format Pipedrive uses for custom field keys.
func isCustomFieldHash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ----- Resource helpers for the QueryEditor dropdowns --------------------------

// Ping validates connectivity and credentials via GET /users/me.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.get(ctx, "/users/me", nil)
	return err
}

// PipelineDTO is a lightweight pipeline representation for the frontend.
type PipelineDTO struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	OrderNr int    `json:"order_nr"`
}

// ListPipelines returns all pipelines.
func (c *Client) ListPipelines(ctx context.Context) ([]PipelineDTO, error) {
	resp, err := c.get(ctx, "/pipelines", nil)
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		return nil, fmt.Errorf("failed parsing pipelines: %w", err)
	}
	out := make([]PipelineDTO, 0, len(items))
	for _, item := range items {
		out = append(out, PipelineDTO{
			ID:      intFromAny(item["id"]),
			Name:    stringFromAny(item["name"]),
			OrderNr: intFromAny(item["order_nr"]),
		})
	}
	return out, nil
}

// StageDTO is a lightweight stage representation for the frontend.
type StageDTO struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	PipelineID int    `json:"pipeline_id"`
	OrderNr    int    `json:"order_nr"`
}

// ListStages returns stages, optionally filtered by pipelineId.
func (c *Client) ListStages(ctx context.Context, pipelineID string) ([]StageDTO, error) {
	params := url.Values{}
	if pipelineID != "" {
		params.Set("pipeline_id", pipelineID)
	}
	resp, err := c.get(ctx, "/stages", params)
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		return nil, fmt.Errorf("failed parsing stages: %w", err)
	}
	out := make([]StageDTO, 0, len(items))
	for _, item := range items {
		out = append(out, StageDTO{
			ID:         intFromAny(item["id"]),
			Name:       stringFromAny(item["name"]),
			PipelineID: intFromAny(item["pipeline_id"]),
			OrderNr:    intFromAny(item["order_nr"]),
		})
	}
	return out, nil
}

// UserDTO is a lightweight user representation for the frontend.
type UserDTO struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ListUsers returns all Pipedrive users.
func (c *Client) ListUsers(ctx context.Context) ([]UserDTO, error) {
	resp, err := c.get(ctx, "/users", nil)
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		return nil, fmt.Errorf("failed parsing users: %w", err)
	}
	out := make([]UserDTO, 0, len(items))
	for _, item := range items {
		out = append(out, UserDTO{
			ID:    intFromAny(item["id"]),
			Name:  stringFromAny(item["name"]),
			Email: stringFromAny(item["email"]),
		})
	}
	return out, nil
}

// ----- Helpers -----------------------------------------------------------------

// decodeRecords parses a list "data" payload into flattened records. The payload
// may be an array, a single object, or null.
func decodeRecords(raw json.RawMessage) []map[string]any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return []map[string]any{}
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return []map[string]any{}
		}
		records := make([]map[string]any, 0, len(items))
		for _, item := range items {
			records = append(records, flattenRecord(item))
		}
		return records
	}
	if strings.HasPrefix(trimmed, "{") {
		return []map[string]any{flattenRecord(raw)}
	}
	return []map[string]any{}
}

// countData returns the number of records in a list "data" payload.
func countData(raw json.RawMessage) int {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return 0
		}
		return len(items)
	}
	if strings.HasPrefix(trimmed, "{") {
		return 1
	}
	return 0
}

func cloneValues(in url.Values) url.Values {
	out := url.Values{}
	for k, vs := range in {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case float32:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		if i, err := strconv.Atoi(n.String()); err == nil {
			return i
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i
		}
	}
	return 0
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := toString(v); ok {
		return s
	}
	return ""
}
