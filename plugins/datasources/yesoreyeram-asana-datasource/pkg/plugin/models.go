package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// asanaCloudURL is Asana's REST API base. Asana is hosted SaaS; there is no
// self-hosted/local mode. The base includes the version segment so the client
// can build paths like /workspaces and /tasks.
const asanaCloudURL = "https://app.asana.com/api/1.0"

// Date filter modes for the modified-since filter.
const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the personal access token) is kept out of this
// struct and read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the Asana API root (including the version segment). Defaults to
	// https://app.asana.com/api/1.0; overridable only to point at a proxy or
	// gateway.
	BaseURL string `json:"baseURL"`

	// apiKey is the Asana personal access token (PAT) or OAuth access token. Both
	// are sent as the Authorization: Bearer header.
	apiKey string
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
		settings.BaseURL = asanaCloudURL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiKey = s.DecryptedSecureJSONData["apiKey"]
	}
	return settings, nil
}

// credential returns the token to send in the Authorization: Bearer header.
func (s Settings) credential() string {
	return strings.TrimSpace(s.apiKey)
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: "tasks" (default), "projects",
	// "sections", "workspaces", "teams", "users", "tags", or "raw" (a custom
	// REST GET path).
	QueryType string `json:"queryType"`

	// --- Scope (which part of the hierarchy to read) -----------------------

	// Workspace is the Asana workspace/organization gid. Required for projects,
	// teams, users, tags and assignee-scoped task queries. Optional otherwise.
	Workspace string `json:"workspace"`
	// Team is an Asana team gid (organizations only). Scopes the projects list.
	// Optional.
	Team string `json:"team"`
	// Project scopes sections and tasks queries to a single project. Optional.
	Project string `json:"project"`
	// Section scopes tasks queries to a single section. Optional.
	Section string `json:"section"`
	// Assignee scopes tasks queries to a single assignee (a user gid or "me").
	// Requires Workspace. Used only when no Project/Section is selected. Optional.
	Assignee string `json:"assignee"`

	// --- Task filters (used when QueryType == "tasks") ---------------------

	// IncompleteOnly returns only incomplete tasks (completed_since=now). Optional.
	IncompleteOnly bool `json:"incompleteOnly"`
	// ModifiedMode selects the modified_since filter source: "any" (default),
	// "dashboard" (use the panel time range from), or "custom" (use the bound
	// below).
	ModifiedMode string `json:"modifiedMode"`
	// ModifiedSince bounds the task modified_at (ISO-8601). Used when
	// ModifiedMode == "custom".
	ModifiedSince string `json:"modifiedSince"`

	// --- Projects filter (used when QueryType == "projects") ---------------

	// IncludeArchived includes archived projects in the results. Optional.
	IncludeArchived bool `json:"includeArchived"`

	// Fields is the subset of task fields (columns) to return. Empty returns the
	// default field set.
	Fields []string `json:"fields"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`

	// --- Raw query (used when QueryType == "raw") --------------------------

	// RawPath is the REST path (relative to the API root, e.g.
	// "/workspaces") used when QueryType == "raw".
	RawPath string `json:"rawPath"`
	// RawRoot is an optional JSON key in the response whose value (an array of
	// objects, or a single object) should be flattened into rows. When empty the
	// first array of objects found anywhere in the response is used (Asana
	// responses wrap results in a top-level "data" key).
	RawRoot string `json:"rawRoot"`

	// TimeRange is the panel/dashboard time range. It is not part of the query
	// JSON; it is populated from the backend request and used when a date mode is
	// "dashboard".
	TimeRange backend.TimeRange `json:"-"`
}

// LoadQuery parses the raw query JSON into a QueryModel and applies defaults.
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &q); err != nil {
			return q, fmt.Errorf("invalid query json: %w", err)
		}
	}
	if q.QueryType == "" {
		q.QueryType = queryTypeTasks
	}
	if q.ModifiedMode == "" {
		q.ModifiedMode = dateModeAny
	}
	return q, nil
}
