package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// mondayCloudURL is monday.com's single GraphQL endpoint. monday.com is hosted
// SaaS; there is no self-hosted/local mode.
const mondayCloudURL = "https://api.monday.com/v2"

// defaultAPIVersion is the monday.com API version sent when the user has not
// configured one. It must be recent enough to expose the `aggregate` query with
// `group_by` (used for grouping/aggregation); older versions reject it with a
// schema error. monday resolves a non-existent version to the Current default,
// so this should track a recent, GA version.
const defaultAPIVersion = "2026-01"

// Auth methods supported by the data source.
const (
	authAPIKey = "apiKey"
	authOAuth  = "oauth"
)

// Date filter modes for created/updated bounds.
const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secret fields (the API token / OAuth token) are kept out of this
// struct and read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the GraphQL endpoint. Defaults to https://api.monday.com/v2;
	// overridable only to point at a proxy or gateway.
	BaseURL string `json:"baseURL"`
	// AuthMethod selects how the Authorization header is built: "apiKey"
	// (personal API token, sent raw) or "oauth" (access token, sent as Bearer).
	AuthMethod string `json:"authMethod"`
	// APIVersion is the optional monday.com API version sent as the API-Version
	// header (e.g. "2024-10"). Empty omits the header (monday uses its current
	// default).
	APIVersion string `json:"apiVersion"`

	// apiToken is the monday.com personal API token (sent raw as the
	// Authorization header). Used when AuthMethod == "apiKey".
	apiToken string
	// oauthToken is a monday.com OAuth2 access token (sent as Authorization:
	// Bearer). Used when AuthMethod == "oauth".
	oauthToken string
}

// LoadSettings parses the data source instance settings.
func LoadSettings(s backend.DataSourceInstanceSettings) (Settings, error) {
	settings := Settings{}
	if len(s.JSONData) > 0 {
		if err := json.Unmarshal(s.JSONData, &settings); err != nil {
			return settings, fmt.Errorf("invalid settings json: %w", err)
		}
	}

	// URL may also be supplied through the standard data source URL field.
	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}
	if settings.BaseURL == "" {
		settings.BaseURL = mondayCloudURL
	}
	if settings.AuthMethod == "" {
		settings.AuthMethod = authAPIKey
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
		settings.oauthToken = s.DecryptedSecureJSONData["oauthToken"]
	}
	return settings, nil
}

// credential returns the value to send in the Authorization header and whether
// the OAuth Bearer scheme should be used.
func (s Settings) credential() (token string, bearer bool) {
	if s.AuthMethod == authOAuth {
		return strings.TrimSpace(s.oauthToken), true
	}
	return strings.TrimSpace(s.apiToken), false
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: "items" (default), "boards", "groups",
	// "users", "workspaces", "tags", or "raw" (a custom GraphQL document).
	QueryType string `json:"queryType"`
	// BoardIds restricts boards/items/groups queries to specific board IDs.
	// Required for the items query. Optional otherwise.
	BoardIds []string `json:"boardIds"`
	// GroupIds restricts the items query to specific group IDs within the board.
	// Optional.
	GroupIds []string `json:"groupIds"`
	// WorkspaceIds restricts the boards query to specific workspaces. Optional.
	WorkspaceIds []string `json:"workspaceIds"`
	// ColumnIds restricts the column_values selection for items to specific
	// column IDs. Empty returns all columns. Optional.
	ColumnIds []string `json:"columnIds"`
	// SearchQuery filters items by their name (containsText rule). Optional.
	SearchQuery string `json:"searchQuery"`
	// State filters boards/items/groups by lifecycle state: "active" (default),
	// "all", "archived" or "deleted". Optional.
	State string `json:"state"`
	// IncludeColumnValues includes flattened column values for items. Defaults
	// to true. When false, only the core item fields are returned.
	IncludeColumnValues *bool `json:"includeColumnValues"`
	// HideSystemColumns omits monday.com's built-in/system column values (e.g.
	// the auto-generated subitems, last-updated, creation-log columns) from the
	// flattened item rows. Optional; defaults to false.
	HideSystemColumns bool `json:"hideSystemColumns"`
	// OrderBy is the column ID to order items by. Optional (monday default order).
	OrderBy string `json:"orderBy"`
	// OrderDir is the order direction for items: "asc" or "desc". Optional.
	OrderDir string `json:"orderDir"`
	// GroupBy is the board column ID to group items by (e.g. "status", "person").
	// When set, the query is routed to monday.com's server-side `aggregate` API:
	// one row per distinct value of this column. Optional.
	GroupBy string `json:"groupBy"`
	// Aggregation is the aggregation applied within each group: "count" (default),
	// "count_distinct", "sum", "avg", "min" or "max". Used only when GroupBy is
	// set.
	Aggregation string `json:"aggregation"`
	// AggregationColumn is the board column ID whose values are aggregated for
	// numeric aggregations (sum/avg/min/max) and for count_distinct. Ignored for
	// plain "count". Used only when GroupBy is set.
	AggregationColumn string `json:"aggregationColumn"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`
	// RawQuery is the GraphQL document used when QueryType == "raw".
	RawQuery string `json:"rawQuery"`
	// RawVariables is an optional JSON object of variables for the raw query.
	RawVariables string `json:"rawVariables"`

	// TimeRange is the panel/dashboard time range. It is not part of the query
	// JSON; it is populated from the backend request. Reserved for future date
	// filters.
	TimeRange backend.TimeRange `json:"-"`
}

// includeColumns reports whether column values should be flattened for items.
// Defaults to true when unset.
func (q QueryModel) includeColumns() bool {
	if q.IncludeColumnValues == nil {
		return true
	}
	return *q.IncludeColumnValues
}

// LoadQuery parses the raw query JSON into a QueryModel and applies defaults.
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) == 0 {
		q.QueryType = queryTypeItems
		q.State = stateActive
		return q, nil
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return q, fmt.Errorf("invalid query json: %w", err)
	}
	if q.QueryType == "" {
		q.QueryType = queryTypeItems
	}
	if q.State == "" {
		q.State = stateActive
	}
	if strings.TrimSpace(q.GroupBy) != "" && q.Aggregation == "" {
		q.Aggregation = aggCount
	}
	return q, nil
}

// isGrouped reports whether the query requests grouped/aggregated output.
func (q QueryModel) isGrouped() bool {
	return strings.TrimSpace(q.GroupBy) != ""
}
