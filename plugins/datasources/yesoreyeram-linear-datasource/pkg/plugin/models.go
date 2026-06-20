package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// linearCloudURL is Linear's single GraphQL endpoint. Linear is hosted SaaS;
// there is no self-hosted/local mode.
const linearCloudURL = "https://api.linear.app/graphql"

// Auth methods supported by the data source.
const (
	authAPIKey = "apiKey"
	authOAuth  = "oauth"
)

// Date filter modes for createdAt/updatedAt.
const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secret fields (the API key / OAuth token) are kept out of this
// struct and read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the GraphQL endpoint. Defaults to api.linear.app/graphql;
	// overridable only to point at a proxy or gateway.
	BaseURL string `json:"baseURL"`
	// AuthMethod selects how the Authorization header is built: "apiKey"
	// (personal API key, sent raw) or "oauth" (access token, sent as Bearer).
	AuthMethod string `json:"authMethod"`

	// apiKey is the Linear personal API key (sent raw as the Authorization
	// header). Used when AuthMethod == "apiKey".
	apiKey string
	// oauthToken is a Linear OAuth2 access token (sent as Authorization: Bearer).
	// Used when AuthMethod == "oauth".
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
		settings.BaseURL = linearCloudURL
	}
	if settings.AuthMethod == "" {
		settings.AuthMethod = authAPIKey
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiKey = s.DecryptedSecureJSONData["apiKey"]
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
	return strings.TrimSpace(s.apiKey), false
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: "issues" (default), "projects", "teams",
	// "users", "cycles", or "raw" (a custom GraphQL document).
	QueryType string `json:"queryType"`
	// TeamId filters issues/cycles to a team (the team's UUID). Optional.
	TeamId string `json:"teamId"`
	// States filters issues by workflow state names (OR'd). Optional.
	States []string `json:"states"`
	// Assignees filters issues by assignee email or name (OR'd). Optional.
	Assignees []string `json:"assignees"`
	// Labels filters issues having any of these label names. Optional.
	Labels []string `json:"labels"`
	// Priorities filters issues by priority value (0-4). Optional.
	Priorities []int `json:"priorities"`
	// Projects filters issues by project name (OR'd). Optional.
	Projects []string `json:"projects"`
	// Creator filters issues by creator email or name. Optional.
	Creator string `json:"creator"`
	// SearchQuery is a free-text filter applied to issue titles. Optional.
	SearchQuery string `json:"searchQuery"`
	// CreatedMode selects the createdAt filter source: "any" (default),
	// "dashboard" (use the panel time range), or "custom" (use the bounds below).
	CreatedMode string `json:"createdMode"`
	// CreatedAfter/CreatedBefore bound the issue createdAt (ISO-8601). Used when
	// CreatedMode == "custom".
	CreatedAfter  string `json:"createdAfter"`
	CreatedBefore string `json:"createdBefore"`
	// UpdatedMode selects the updatedAt filter source: "any" (default),
	// "dashboard" (use the panel time range), or "custom" (use the bounds below).
	UpdatedMode string `json:"updatedMode"`
	// UpdatedAfter/UpdatedBefore bound the issue updatedAt (ISO-8601). Used when
	// UpdatedMode == "custom".
	UpdatedAfter  string `json:"updatedAfter"`
	UpdatedBefore string `json:"updatedBefore"`
	// IncludeArchived includes archived issues in the results. Optional.
	IncludeArchived bool `json:"includeArchived"`
	// Fields is the subset of issue fields (columns) to return. Empty returns
	// the default set.
	Fields []string `json:"fields"`
	// OrderBy is the connection ordering: "createdAt" (default) or "updatedAt".
	OrderBy string `json:"orderBy"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`
	// RawQuery is the GraphQL document used when QueryType == "raw".
	RawQuery string `json:"rawQuery"`
	// RawVariables is an optional JSON object of variables for the raw query.
	RawVariables string `json:"rawVariables"`

	// TimeRange is the panel/dashboard time range. It is not part of the query
	// JSON; it is populated from the backend request and used when CreatedMode or
	// UpdatedMode is "dashboard".
	TimeRange backend.TimeRange `json:"-"`
}

// LoadQuery parses the raw query JSON into a QueryModel and applies defaults.
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) == 0 {
		q.QueryType = queryTypeIssues
		return q, nil
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return q, fmt.Errorf("invalid query json: %w", err)
	}
	if q.QueryType == "" {
		q.QueryType = queryTypeIssues
	}
	if q.CreatedMode == "" {
		q.CreatedMode = dateModeAny
	}
	if q.UpdatedMode == "" {
		q.UpdatedMode = dateModeAny
	}
	return q, nil
}
