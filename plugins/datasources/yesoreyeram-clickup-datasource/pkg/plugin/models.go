package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// clickUpCloudURL is ClickUp's REST API base. ClickUp is hosted SaaS; there is
// no self-hosted/local mode. The base is the API root WITHOUT the trailing
// `/v2` so the client can build versioned paths (e.g. /v2/team).
const clickUpCloudURL = "https://api.clickup.com/api"

// Auth methods supported by the data source.
const (
	authAPIKey = "apiKey"
	authOAuth  = "oauth"
)

// Date filter modes for created/updated/due dates.
const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secret fields (the personal token / OAuth token) are kept out of
// this struct and read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the ClickUp API root (without the version segment). Defaults to
	// https://api.clickup.com/api; overridable only to point at a proxy or
	// gateway.
	BaseURL string `json:"baseURL"`
	// AuthMethod selects how the Authorization header is built: "apiKey"
	// (personal token, sent raw) or "oauth" (access token, sent as Bearer).
	AuthMethod string `json:"authMethod"`

	// apiKey is the ClickUp personal API token (sent raw as the Authorization
	// header). Used when AuthMethod == "apiKey".
	apiKey string
	// oauthToken is a ClickUp OAuth2 access token (sent as Authorization:
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
		settings.BaseURL = clickUpCloudURL
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
	// QueryType selects what to fetch: "tasks" (default), "spaces", "folders",
	// "lists", "teams" (workspaces), or "raw" (a custom REST GET path).
	QueryType string `json:"queryType"`

	// --- Scope (which part of the hierarchy to read) -----------------------

	// TeamId is the ClickUp Workspace ID. Required for tasks/spaces queries and
	// for raw queries that target a workspace-scoped path. Optional otherwise.
	TeamId string `json:"teamId"`
	// SpaceId scopes folders/lists queries to a Space, and tasks queries to a
	// Space. Optional.
	SpaceId string `json:"spaceId"`
	// FolderId scopes lists queries to a Folder, and tasks queries to a Folder.
	// Optional.
	FolderId string `json:"folderId"`
	// ListId scopes tasks queries to a List. Optional.
	ListId string `json:"listId"`

	// --- Task filters (used when QueryType == "tasks") ---------------------

	// Statuses filters tasks by status names (OR'd). Optional.
	Statuses []string `json:"statuses"`
	// Assignees filters tasks by assignee user IDs (OR'd). Optional.
	Assignees []string `json:"assignees"`
	// Tags filters tasks by tag names (OR'd). Optional.
	Tags []string `json:"tags"`
	// IncludeClosed includes closed tasks in the results. Optional.
	IncludeClosed bool `json:"includeClosed"`
	// IncludeSubtasks includes subtasks in the results. Optional.
	IncludeSubtasks bool `json:"includeSubtasks"`
	// IncludeArchived includes archived tasks in the results. Optional.
	IncludeArchived bool `json:"includeArchived"`

	// CreatedMode selects the date_created filter source: "any" (default),
	// "dashboard" (use the panel time range), or "custom" (use the bounds below).
	CreatedMode string `json:"createdMode"`
	// CreatedAfter/CreatedBefore bound the task date_created (ISO-8601 or Unix
	// millis). Used when CreatedMode == "custom".
	CreatedAfter  string `json:"createdAfter"`
	CreatedBefore string `json:"createdBefore"`
	// UpdatedMode selects the date_updated filter source: "any" (default),
	// "dashboard" (use the panel time range), or "custom" (use the bounds below).
	UpdatedMode string `json:"updatedMode"`
	// UpdatedAfter/UpdatedBefore bound the task date_updated (ISO-8601 or Unix
	// millis). Used when UpdatedMode == "custom".
	UpdatedAfter  string `json:"updatedAfter"`
	UpdatedBefore string `json:"updatedBefore"`
	// DueMode selects the due_date filter source: "any" (default), "dashboard",
	// or "custom".
	DueMode string `json:"dueMode"`
	// DueAfter/DueBefore bound the task due_date. Used when DueMode == "custom".
	DueAfter  string `json:"dueAfter"`
	DueBefore string `json:"dueBefore"`

	// Fields is the subset of task fields (columns) to return. Empty returns all
	// flattened fields.
	Fields []string `json:"fields"`
	// OrderBy is the task ordering: "created" (default), "updated", "due_date" or
	// "id".
	OrderBy string `json:"orderBy"`
	// Reverse reverses the order direction. Optional.
	Reverse bool `json:"reverse"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`

	// --- Raw query (used when QueryType == "raw") --------------------------

	// RawPath is the REST path (relative to the API root, e.g.
	// "/v2/team/123/task?subtasks=true") used when QueryType == "raw".
	RawPath string `json:"rawPath"`
	// RawRoot is an optional JSON key in the response whose value (an array of
	// objects, or a single object) should be flattened into rows. When empty the
	// first array of objects found anywhere in the response is used.
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
	if q.CreatedMode == "" {
		q.CreatedMode = dateModeAny
	}
	if q.UpdatedMode == "" {
		q.UpdatedMode = dateModeAny
	}
	if q.DueMode == "" {
		q.DueMode = dateModeAny
	}
	return q, nil
}
