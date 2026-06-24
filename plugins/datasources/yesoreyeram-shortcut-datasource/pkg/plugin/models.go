package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// shortcutCloudURL is Shortcut's API host (WITHOUT the version prefix). Shortcut
// is a hosted SaaS, but the host is overridable in the config editor so the
// plugin can be pointed at a proxy. The client appends the version prefix
// (/api/v3) and resource paths itself.
const shortcutCloudURL = "https://api.app.shortcut.com"

// apiPrefix is the versioned API path prefix appended to the base host.
const apiPrefix = "/api/v3"

// Supported query types.
const (
	queryTypeStories = "stories"
	queryTypeCount   = "count"
)

// Date filter modes.
const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Date fields the dashboard time range can be applied to. These map to the
// Shortcut search date operators created: / updated: / due:.
const (
	dateFieldCreated  = "created"
	dateFieldUpdated  = "updated"
	dateFieldDeadline = "deadline"
)

// Archived filter modes.
const (
	archivedAny     = "any"     // do not constrain on archived state
	archivedOnly    = "only"    // is:archived
	archivedExclude = "exclude" // !is:archived
)

// Story detail levels accepted by the search endpoint.
const (
	detailFull = "full"
	detailSlim = "slim"
)

// Settings holds the data source instance settings. The API token is a secret
// kept out of JSONData and read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the Shortcut API host (without a trailing slash and without the
	// /api/v3 prefix). Defaults to https://api.app.shortcut.com. Override only to
	// point at a proxy.
	BaseURL string `json:"baseURL"`

	// apiToken is the Shortcut personal API token (sent as the Shortcut-Token
	// header).
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
	// The host may also be supplied through the standard data source URL field.
	if strings.TrimSpace(settings.BaseURL) == "" {
		settings.BaseURL = s.URL
	}
	if strings.TrimSpace(settings.BaseURL) == "" {
		settings.BaseURL = shortcutCloudURL
	}
	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = strings.TrimSpace(s.DecryptedSecureJSONData["apiToken"])
	}
	return settings, nil
}

// QueryModel is the per-query payload sent from the QueryEditor. Structured
// filters carry the values used by Shortcut's search query language: names for
// projects/states/epics/iterations/labels/teams and mention names for owners.
type QueryModel struct {
	// QueryType selects what to return: "stories" (default) or "count".
	QueryType string `json:"queryType"`

	// Query is a raw Shortcut search query (operators + free text). It is
	// combined (AND) with the structured filters below.
	Query string `json:"query"`

	// --- Structured filters (compiled into the search query string) ---

	// StoryType filters by story type (feature/bug/chore) -> type:.
	StoryType string `json:"storyType"`
	// Projects filters by project name -> project:. Search is AND-only, so
	// multiple projects rarely match (a story has one project).
	Projects []string `json:"projects"`
	// WorkflowStates filters by workflow state name -> state:. AND-only.
	WorkflowStates []string `json:"workflowStates"`
	// Epic filters by epic name -> epic:.
	Epic string `json:"epic"`
	// Iteration filters by iteration name -> iteration:.
	Iteration string `json:"iteration"`
	// Labels filters by label name -> label:. Multiple labels AND (a story may
	// carry several labels).
	Labels []string `json:"labels"`
	// Owners filters by owner mention name -> owner:. Multiple owners AND.
	Owners []string `json:"owners"`
	// Teams filters by team (group) name -> team:.
	Teams []string `json:"teams"`
	// Archived selects the archived constraint: "any", "only" (is:archived) or
	// "exclude" (!is:archived).
	Archived string `json:"archived"`

	// --- Date filters ---

	// DateMode selects the date filter source: "any", "dashboard" (panel range)
	// or "custom".
	DateMode string `json:"dateMode"`
	// DateField selects which date operator the dashboard range applies to:
	// "created" (default), "updated" or "deadline" (due:).
	DateField string `json:"dateField"`
	// Custom date bounds (ISO-8601 or YYYY-MM-DD; only the date part is used).
	CreatedAfter   string `json:"createdAfter"`
	CreatedBefore  string `json:"createdBefore"`
	UpdatedAfter   string `json:"updatedAfter"`
	UpdatedBefore  string `json:"updatedBefore"`
	DeadlineAfter  string `json:"deadlineAfter"`
	DeadlineBefore string `json:"deadlineBefore"`

	// Detail selects the amount of detail per story: "full" (default) or "slim".
	Detail string `json:"detail"`
	// Fields restricts the returned columns. Empty returns the default story
	// field catalog.
	Fields []string `json:"fields"`
	// Limit caps the number of returned stories. 0 returns all matches (subject
	// to Shortcut's 1000-result search cap).
	Limit int `json:"limit"`

	// TimeRange is the panel time range, populated from the backend request and
	// used when DateMode is "dashboard". Not part of the query JSON.
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
		q.QueryType = queryTypeStories
	}
	if q.DateMode == "" {
		q.DateMode = dateModeAny
	}
	if q.DateField == "" {
		q.DateField = dateFieldCreated
	}
	if q.Archived == "" {
		q.Archived = archivedAny
	}
	if q.Detail == "" {
		q.Detail = detailFull
	}
	return q, nil
}
