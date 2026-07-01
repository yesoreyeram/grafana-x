package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// todoistCloudURL is the Todoist unified API v1 base. Todoist is hosted SaaS;
// there is no self-hosted/local mode. The base includes the version segment so
// the client can build paths like /tasks, /projects, /sections and /labels.
//
// The unified v1 API (https://api.todoist.com/api/v1) supersedes the older REST
// v2 API (https://api.todoist.com/rest/v2). v1 adds cursor-based pagination to
// the list endpoints, whereas REST v2 returned all active tasks in a single
// un-paginated array (and ignored offset/limit). v1 is therefore the correct,
// scalable target.
const todoistCloudURL = "https://api.todoist.com/api/v1"

// Supported query types.
const (
	// queryTypeTasks returns task records.
	queryTypeTasks = "tasks"
	// queryTypeCount returns a single count of matching tasks (Todoist has no
	// native count endpoint, so matching tasks are paginated and counted).
	queryTypeCount = "count"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the API token) is kept out of this struct and
// read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the Todoist API root (including the version segment). Defaults
	// to https://api.todoist.com/api/v1; overridable only to point at a proxy
	// or gateway.
	BaseURL string `json:"baseURL"`

	// apiToken is the Todoist API token, sent as the Authorization: Bearer
	// header. Create one at Todoist Settings > Integrations > Developer.
	apiToken string
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
		settings.BaseURL = todoistCloudURL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

// credential returns the token to send in the Authorization: Bearer header.
func (s Settings) credential() string {
	return strings.TrimSpace(s.apiToken)
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: "tasks" (default) or "count".
	QueryType string `json:"queryType"`
	// HideSystemFields drops metadata-style columns (see system_fields.go) from
	// the returned frame when true. Defaults to false.
	HideSystemFields bool `json:"hideSystemFields"`

	// --- Task scope / filters -------------------------------------------------
	//
	// ProjectId, SectionId, Label and ParentId scope the standard
	// GET /tasks endpoint. Filter routes to the dedicated GET /tasks/filter
	// endpoint instead and takes precedence over the id-based filters (the two
	// endpoints cannot be combined in the Todoist API).

	// ProjectId scopes tasks to a single project (the project's string ID).
	ProjectId string `json:"projectId"`
	// SectionId scopes tasks to a single section (the section's string ID).
	SectionId string `json:"sectionId"`
	// Label scopes tasks to a single label. NOTE: the Todoist `label` parameter
	// filters by label NAME, not ID.
	Label string `json:"label"`
	// ParentId scopes tasks to the sub-tasks of a single parent task (its ID).
	ParentId string `json:"parentId"`
	// Filter is a Todoist filter query (e.g. "today | overdue", "#Work & p1").
	// When set, the query is sent to GET /tasks/filter as the `query` parameter
	// and the id-based scope fields above are ignored.
	Filter string `json:"filter"`
	// Lang is the optional IETF language tag used to parse the Filter string
	// (e.g. "en", "de"). Only used with Filter.
	Lang string `json:"lang"`

	// Limit caps the number of records returned (and, for count queries, the
	// number of tasks scanned). 0 means use default paging up to a safety cap.
	Limit int `json:"limit"`

	// TimeRange is the panel/dashboard time range. It is not part of the query
	// JSON; it is populated from the backend request.
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
	return q, nil
}
