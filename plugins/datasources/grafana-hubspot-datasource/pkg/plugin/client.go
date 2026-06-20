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
	searchPageSize = 200
	maxRecords     = 100000
)

// Client is a thin wrapper around HubSpot's REST API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a HubSpot API client.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if base == "" {
		base = hubSpotCloudURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	return &Client{
		baseURL:    base,
		token:      settings.credential(),
		httpClient: httpClient,
	}, nil
}

// hubSpotError is HubSpot's JSON error envelope.
type hubSpotError struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Category    string `json:"category"`
	SubCategory string `json:"subCategory"`
}

func (e hubSpotError) Error() string {
	s := e.Message
	if e.Category != "" {
		s = s + " (" + e.Category + ")"
	}
	return s
}

// doGET issues a GET request and returns the raw response body.
func (c *Client) doGET(ctx context.Context, path string) (json.RawMessage, error) {
	full := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// doPOST issues a POST request with a JSON body and returns the raw response.
func (c *Client) doPOST(ctx context.Context, path string, body any) (json.RawMessage, error) {
	full := c.baseURL + path
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("failed encoding request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

// doRaw issues a GET or POST request with the given optional body.
func (c *Client) doRaw(ctx context.Context, method, path, bodyStr string) (json.RawMessage, error) {
	full := c.baseURL + path
	var bodyReader io.Reader
	if method == http.MethodPost && bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
	if err != nil {
		return nil, err
	}
	if bodyStr != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (json.RawMessage, error) {
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to hubspot failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading hubspot response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e hubSpotError
		if json.Unmarshal(raw, &e) == nil && e.Message != "" {
			return nil, fmt.Errorf("hubspot returned status %d: %s", resp.StatusCode, truncate(e.Error()))
		}
		return nil, fmt.Errorf("hubspot returned status %d: %s", resp.StatusCode, truncate(string(raw)))
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

// ----- Search API types -------------------------------------------------------

// searchRequest is the HubSpot CRM Search API POST body.
type searchRequest struct {
	FilterGroups []filterGroup  `json:"filterGroups,omitempty"`
	Sorts        []sortRequest  `json:"sorts,omitempty"`
	Properties   []string       `json:"properties,omitempty"`
	Limit        int            `json:"limit"`
	After        int            `json:"after,omitempty"`
}

type filterGroup struct {
	Filters []filter `json:"filters"`
}

type filter struct {
	PropertyName string `json:"propertyName"`
	Operator     string `json:"operator"`
	Value        string `json:"value"`
}

type sortRequest struct {
	PropertyName string `json:"propertyName"`
	Direction    string `json:"direction"`
}

// searchResponse is the HubSpot CRM Search API response.
type searchResponse struct {
	Total   int                `json:"total"`
	Results []json.RawMessage  `json:"results"`
	Paging  *pagingInfo        `json:"paging,omitempty"`
}

type pagingInfo struct {
	Next *nextPage `json:"next,omitempty"`
}

type nextPage struct {
	After string `json:"after"`
}

// ----- List API types ----------------------------------------------------------

type listResponse struct {
	Results []json.RawMessage `json:"results"`
	Paging  *pagingInfo       `json:"paging,omitempty"`
}

// ----- Properties API types ----------------------------------------------------

type propertyDefinition struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Type       string `json:"type"`
	FieldType  string `json:"fieldType"`
	GroupName  string `json:"groupName"`
}

type propertiesResponse struct {
	Results []propertyDefinition `json:"results"`
}

// ----- Pipelines API types -----------------------------------------------------

type pipelineInfo struct {
	ID     string        `json:"id"`
	Label  string        `json:"label"`
	Stages []pipelineStage `json:"stages"`
}

type pipelineStage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type pipelinesResponse struct {
	Results []pipelineInfo `json:"results"`
}

// ----- Owners API types --------------------------------------------------------

type ownerInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type ownersResponse struct {
	Results []ownerInfo `json:"results"`
}

// ----- Main dispatcher ---------------------------------------------------------

// ListRecords runs the appropriate HubSpot query and returns flattened records.
func (c *Client) ListRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	switch {
	case q.QueryType == queryTypeRaw:
		return c.listRaw(ctx, q)
	case q.QueryType == queryTypePipelines:
		return c.listPipelines(ctx, q)
	case q.QueryType == queryTypeOwners:
		return c.listOwners(ctx)
	case q.QueryType == queryTypeProperties:
		return c.listProperties(ctx, q)
	case searchableQueryType(q.QueryType):
		return c.searchRecords(ctx, q)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.QueryType)
	}
}

// searchRecords uses the HubSpot CRM Search API to fetch and flatten records.
func (c *Client) searchRecords(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	objectPath, ok := objectTypeToAPIPath[q.QueryType]
	if !ok {
		return nil, fmt.Errorf("unsupported search query type: %s", q.QueryType)
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}

	reqBody := c.buildSearchRequest(q, searchPageSize, 0)

	records := make([]map[string]any, 0, searchPageSize)
	after := 0

	for {
		reqBody.After = after
		raw, err := c.doPOST(ctx, "/crm/v3/objects/"+objectPath+"/search", reqBody)
		if err != nil {
			return nil, err
		}
		var resp searchResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("failed parsing search response: %w", err)
		}
		for _, item := range resp.Results {
			records = append(records, flattenHubSpotRecord(item))
			if len(records) >= hardLimit {
				return records, nil
			}
		}
		if resp.Paging == nil || resp.Paging.Next == nil || resp.Paging.Next.After == "" {
			break
		}
		if _, err := fmt.Sscanf(resp.Paging.Next.After, "%d", &after); err != nil {
			break
		}
	}
	return records, nil
}

// buildSearchRequest constructs a HubSpot Search API request from the query model.
func (c *Client) buildSearchRequest(q QueryModel, limit, after int) searchRequest {
	req := searchRequest{
		Limit: limit,
		After: after,
	}

	// Filter groups from the query editor.
	if len(q.FilterGroups) > 0 {
		req.FilterGroups = make([]filterGroup, 0, len(q.FilterGroups))
		for _, fg := range q.FilterGroups {
			if len(fg.Filters) == 0 {
				continue
			}
			g := filterGroup{Filters: make([]filter, 0, len(fg.Filters))}
			for _, f := range fg.Filters {
				if strings.TrimSpace(f.PropertyName) == "" {
					continue
				}
				g.Filters = append(g.Filters, filter{
					PropertyName: f.PropertyName,
					Operator:     f.Operator,
					Value:        f.Value,
				})
			}
			if len(g.Filters) > 0 {
				req.FilterGroups = append(req.FilterGroups, g)
			}
		}
	}

	// Date filters: createdate
	if q.CreatedMode == dateModeDashboard && !q.TimeRange.From.IsZero() && !q.TimeRange.To.IsZero() {
		req.FilterGroups = append(req.FilterGroups, filterGroup{
			Filters: []filter{
				{PropertyName: "createdate", Operator: "GTE", Value: q.TimeRange.From.UTC().Format("2006-01-02T15:04:05Z")},
				{PropertyName: "createdate", Operator: "LTE", Value: q.TimeRange.To.UTC().Format("2006-01-02T15:04:05Z")},
			},
		})
	} else if q.CreatedMode == dateModeCustom {
		fg := filterGroup{}
		if after := strings.TrimSpace(q.CreatedAfter); after != "" {
			fg.Filters = append(fg.Filters, filter{PropertyName: "createdate", Operator: "GTE", Value: after})
		}
		if before := strings.TrimSpace(q.CreatedBefore); before != "" {
			fg.Filters = append(fg.Filters, filter{PropertyName: "createdate", Operator: "LTE", Value: before})
		}
		if len(fg.Filters) > 0 {
			req.FilterGroups = append(req.FilterGroups, fg)
		}
	}

	// Date filters: hs_lastmodifieddate
	if q.UpdatedMode == dateModeDashboard && !q.TimeRange.From.IsZero() && !q.TimeRange.To.IsZero() {
		req.FilterGroups = append(req.FilterGroups, filterGroup{
			Filters: []filter{
				{PropertyName: "hs_lastmodifieddate", Operator: "GTE", Value: q.TimeRange.From.UTC().Format("2006-01-02T15:04:05Z")},
				{PropertyName: "hs_lastmodifieddate", Operator: "LTE", Value: q.TimeRange.To.UTC().Format("2006-01-02T15:04:05Z")},
			},
		})
	} else if q.UpdatedMode == dateModeCustom {
		fg := filterGroup{}
		if after := strings.TrimSpace(q.UpdatedAfter); after != "" {
			fg.Filters = append(fg.Filters, filter{PropertyName: "hs_lastmodifieddate", Operator: "GTE", Value: after})
		}
		if before := strings.TrimSpace(q.UpdatedBefore); before != "" {
			fg.Filters = append(fg.Filters, filter{PropertyName: "hs_lastmodifieddate", Operator: "LTE", Value: before})
		}
		if len(fg.Filters) > 0 {
			req.FilterGroups = append(req.FilterGroups, fg)
		}
	}

	// Sort
	if sortBy := strings.TrimSpace(q.SortBy); sortBy != "" {
		dir := "ASCENDING"
		if strings.ToUpper(strings.TrimSpace(q.SortDir)) == "DESCENDING" {
			dir = "DESCENDING"
		}
		req.Sorts = []sortRequest{{PropertyName: sortBy, Direction: dir}}
	}

	// Property selection
	if props := nonEmpty(q.Properties); len(props) > 0 {
		req.Properties = props
	}

	return req
}

// listPipelines fetches pipeline definitions for a given object type (deals/tickets).
func (c *Client) listPipelines(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	objectType := strings.TrimSpace(q.ObjectType)
	if objectType == "" {
		objectType = "deals"
	}
	raw, err := c.doGET(ctx, "/crm/v3/pipelines/"+objectType)
	if err != nil {
		return nil, err
	}
	var resp pipelinesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing pipelines response: %w", err)
	}
	records := make([]map[string]any, 0, len(resp.Results))
	for _, p := range resp.Results {
		// Flatten stages into a comma-separated string.
		stages := make([]string, 0, len(p.Stages))
		for _, s := range p.Stages {
			stages = append(stages, s.Label+" ("+s.ID+")")
		}
		records = append(records, map[string]any{
			"id":     p.ID,
			"label":  p.Label,
			"stages": strings.Join(stages, ", "),
		})
		// Also emit one row per stage.
		for _, s := range p.Stages {
			records = append(records, map[string]any{
				"pipeline_id":    p.ID,
				"pipeline_label": p.Label,
				"stage_id":       s.ID,
				"stage_label":    s.Label,
			})
		}
	}
	return records, nil
}

// listOwners fetches HubSpot account owners/users.
func (c *Client) listOwners(ctx context.Context) ([]map[string]any, error) {
	raw, err := c.doGET(ctx, "/crm/v3/owners")
	if err != nil {
		return nil, err
	}
	var resp ownersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing owners response: %w", err)
	}
	records := make([]map[string]any, 0, len(resp.Results))
	for _, o := range resp.Results {
		records = append(records, map[string]any{
			"id":         o.ID,
			"email":      o.Email,
			"first_name": o.FirstName,
			"last_name":  o.LastName,
		})
	}
	return records, nil
}

// listProperties fetches property definitions for a given object type.
func (c *Client) listProperties(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	objectType := strings.TrimSpace(q.ObjectType)
	if objectType == "" {
		objectType = "contacts"
	}
	raw, err := c.doGET(ctx, "/crm/v3/properties/"+objectType)
	if err != nil {
		return nil, err
	}
	var resp propertiesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing properties response: %w", err)
	}
	records := make([]map[string]any, 0, len(resp.Results))
	for _, p := range resp.Results {
		records = append(records, map[string]any{
			"name":       p.Name,
			"label":      p.Label,
			"type":       p.Type,
			"field_type": p.FieldType,
			"group_name": p.GroupName,
		})
	}
	return records, nil
}

// listRaw executes a user-provided REST path and flattens the response.
func (c *Client) listRaw(ctx context.Context, q QueryModel) ([]map[string]any, error) {
	path := strings.TrimSpace(q.RawPath)
	if path == "" {
		return nil, fmt.Errorf("rawPath is required for raw queries")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	method := strings.ToUpper(strings.TrimSpace(q.RawMethod))
	if method == "" {
		method = http.MethodGet
	}

	var raw json.RawMessage
	var err error
	if method == http.MethodPost {
		raw, err = c.doPOST(ctx, path, json.RawMessage(q.RawBody))
	} else {
		raw, err = c.doGET(ctx, path)
	}
	if err != nil {
		return nil, err
	}

	if root := strings.TrimSpace(q.RawRoot); root != "" {
		var top map[string]json.RawMessage
		if err := json.Unmarshal(raw, &top); err != nil {
			return nil, fmt.Errorf("failed parsing response: %w", err)
		}
		val, ok := top[root]
		if !ok {
			return nil, fmt.Errorf("response has no %q key", root)
		}
		return flattenAny(val), nil
	}
	return flattenListResponse(raw), nil
}

// flattenListResponse flattens a list response that may be a bare array, an
// envelope with a "results" array, the first array of objects found, or a
// single object.
func flattenListResponse(raw json.RawMessage) []map[string]any {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		return flattenAny(raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err == nil {
		if results, ok := top["results"]; ok {
			if strings.HasPrefix(strings.TrimSpace(string(results)), "[") {
				return flattenAny(results)
			}
		}
	}
	if items, ok := findArray(raw); ok {
		records := make([]map[string]any, 0, len(items))
		for _, item := range items {
			records = append(records, flattenHubSpotRecord(item))
		}
		return records
	}
	return []map[string]any{flattenHubSpotRecord(raw)}
}

func flattenAny(raw json.RawMessage) []map[string]any {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil {
			records := make([]map[string]any, 0, len(items))
			for _, item := range items {
				records = append(records, flattenHubSpotRecord(item))
			}
			return records
		}
	}
	return []map[string]any{flattenHubSpotRecord(raw)}
}

func findArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	for _, v := range obj {
		trimmed := strings.TrimSpace(string(v))
		if strings.HasPrefix(trimmed, "[") {
			var items []json.RawMessage
			if err := json.Unmarshal(v, &items); err == nil && len(items) > 0 {
				if strings.HasPrefix(strings.TrimSpace(string(items[0])), "{") {
					return items, true
				}
			}
		}
	}
	for _, v := range obj {
		if strings.HasPrefix(strings.TrimSpace(string(v)), "{") {
			if items, ok := findArray(v); ok {
				return items, true
			}
		}
	}
	return nil, false
}

// Ping validates connectivity by fetching owners (a lightweight endpoint).
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.doGET(ctx, "/crm/v3/owners")
	return err
}

// ----- Resource helpers for the QueryEditor dropdowns ---------------------------

// PropertyInfo is a lightweight property definition for the frontend.
type PropertyInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

// ListPropertiesForObject returns property definitions for a HubSpot object type.
func (c *Client) ListPropertiesForObject(ctx context.Context, objectType string) ([]PropertyInfo, error) {
	if objectType == "" {
		objectType = "contacts"
	}
	raw, err := c.doGET(ctx, "/crm/v3/properties/"+objectType)
	if err != nil {
		return nil, err
	}
	var resp propertiesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing properties response: %w", err)
	}
	out := make([]PropertyInfo, 0, len(resp.Results))
	for _, p := range resp.Results {
		out = append(out, PropertyInfo{Name: p.Name, Label: p.Label, Type: p.Type})
	}
	return out, nil
}

// PipelineDef is a lightweight pipeline representation for the frontend.
type PipelineDef struct {
	ID     string           `json:"id"`
	Label  string           `json:"label"`
	Stages []PipelineStage  `json:"stages"`
}

type PipelineStage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ListPipelinesForObject returns pipeline definitions for a HubSpot object type.
func (c *Client) ListPipelinesForObject(ctx context.Context, objectType string) ([]PipelineDef, error) {
	if objectType == "" {
		objectType = "deals"
	}
	raw, err := c.doGET(ctx, "/crm/v3/pipelines/"+objectType)
	if err != nil {
		return nil, err
	}
	var resp pipelinesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing pipelines response: %w", err)
	}
	out := make([]PipelineDef, 0, len(resp.Results))
	for _, p := range resp.Results {
		stages := make([]PipelineStage, 0, len(p.Stages))
		for _, s := range p.Stages {
			stages = append(stages, PipelineStage{ID: s.ID, Label: s.Label})
		}
		out = append(out, PipelineDef{ID: p.ID, Label: p.Label, Stages: stages})
	}
	return out, nil
}

// OwnerInfo is a lightweight owner representation for the frontend.
type OwnerInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// ListOwners returns all HubSpot account owners.
func (c *Client) ListOwners(ctx context.Context) ([]OwnerInfo, error) {
	raw, err := c.doGET(ctx, "/crm/v3/owners")
	if err != nil {
		return nil, err
	}
	var resp ownersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing owners response: %w", err)
	}
	out := make([]OwnerInfo, 0, len(resp.Results))
	for _, o := range resp.Results {
		out = append(out, OwnerInfo{
			ID: o.ID, Email: o.Email,
			FirstName: o.FirstName, LastName: o.LastName,
		})
	}
	return out, nil
}
