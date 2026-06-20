package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// planeCloudURL is Plane's REST API base. Plane Cloud is hosted SaaS, but Plane
// can also be self-hosted, so the base URL is overridable in the config editor.
// The base is the API root WITHOUT a trailing slash; the client builds versioned
// paths (e.g. /api/v1/workspaces/{slug}/projects/).
const planeCloudURL = "https://api.plane.so"

// Auth methods supported by the data source.
const (
	authAPIKey = "apiKey"
	authOAuth  = "oauth"
)

// Date filter modes for created/updated dates.
const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secret fields (the API key / OAuth token) are kept out of this
// struct and read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the Plane API root (without a trailing slash). Defaults to
	// https://api.plane.so; override it to point at a self-hosted instance or a
	// proxy.
	BaseURL string `json:"baseURL"`
	// WorkspaceSlug is the default workspace slug used when a query does not set
	// one. It is the unique workspace identifier from the Plane URL (e.g. the
	// "my-team" in https://app.plane.so/my-team/projects/). Optional.
	WorkspaceSlug string `json:"workspaceSlug"`
	// AuthMethod selects how the request is authenticated: "apiKey" (personal
	// API key, sent as the X-API-Key header) or "oauth" (access token, sent as
	// Authorization: Bearer).
	AuthMethod string `json:"authMethod"`

	// apiKey is the Plane personal API key (sent as the X-API-Key header). Used
	// when AuthMethod == "apiKey".
	apiKey string
	// oauthToken is a Plane OAuth2 access token (sent as Authorization: Bearer).
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
		settings.BaseURL = planeCloudURL
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

// credential returns the token to send and whether the OAuth Bearer scheme
// should be used (true) or the X-API-Key header (false).
func (s Settings) credential() (token string, bearer bool) {
	if s.AuthMethod == authOAuth {
		return strings.TrimSpace(s.oauthToken), true
	}
	return strings.TrimSpace(s.apiKey), false
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: "workitems" (default), "projects",
	// "states", "labels", "cycles", "modules", "members", or "raw" (a custom
	// REST GET path).
	QueryType string `json:"queryType"`

	// --- Scope (which part of the hierarchy to read) -----------------------

	// WorkspaceSlug is the Plane workspace slug. Required for every query type
	// except "raw" (which targets an arbitrary path). Falls back to the
	// data source's default workspace slug when empty.
	WorkspaceSlug string `json:"workspaceSlug"`
	// ProjectId is the Plane project UUID. Required for work items / states /
	// labels / cycles / modules queries.
	ProjectId string `json:"projectId"`

	// --- Work item filters (used when QueryType == "workitems") ------------

	// Priorities filters work items by priority (urgent/high/medium/low/none).
	// Multiple values match any. Optional.
	Priorities []string `json:"priorities"`
	// States filters work items by state UUID. Multiple values match any.
	// Optional.
	States []string `json:"states"`
	// Assignees filters work items by assignee user UUID. Multiple values match
	// any. Optional.
	Assignees []string `json:"assignees"`
	// Labels filters work items by label UUID. Multiple values match any.
	// Optional.
	Labels []string `json:"labels"`

	// CreatedMode selects the created_at filter source: "any" (default),
	// "dashboard" (use the panel time range), or "custom" (use the bounds below).
	CreatedMode string `json:"createdMode"`
	// CreatedAfter/CreatedBefore bound created_at (ISO-8601 / RFC3339). Used when
	// CreatedMode == "custom".
	CreatedAfter  string `json:"createdAfter"`
	CreatedBefore string `json:"createdBefore"`
	// UpdatedMode selects the updated_at filter source: "any" (default),
	// "dashboard", or "custom".
	UpdatedMode string `json:"updatedMode"`
	// UpdatedAfter/UpdatedBefore bound updated_at. Used when UpdatedMode ==
	// "custom".
	UpdatedAfter  string `json:"updatedAfter"`
	UpdatedBefore string `json:"updatedBefore"`

	// Expand requests Plane to inline related objects (e.g. "assignees", "state",
	// "labels") so they flatten to readable columns. Optional.
	Expand []string `json:"expand"`

	// Fields is the subset of columns to return. Empty returns all flattened
	// fields.
	Fields []string `json:"fields"`
	// OrderBy is the field to order results by. Prefix with "-" for descending.
	// Defaults to "-created_at".
	OrderBy string `json:"orderBy"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`

	// --- Raw query (used when QueryType == "raw") --------------------------

	// RawPath is the REST path (relative to the API root, e.g.
	// "/api/v1/workspaces/my-team/projects/") used when QueryType == "raw".
	RawPath string `json:"rawPath"`
	// RawRoot is an optional JSON key in the response whose value (an array of
	// objects, or a single object) should be flattened into rows. Defaults to
	// "results" (Plane's paginated envelope) when empty; if "results" is absent
	// the first array of objects found anywhere in the response is used.
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
		q.QueryType = queryTypeWorkItems
	}
	if q.CreatedMode == "" {
		q.CreatedMode = dateModeAny
	}
	if q.UpdatedMode == "" {
		q.UpdatedMode = dateModeAny
	}
	return q, nil
}

// resolveWorkspace returns the workspace slug to use for the query, falling back
// to the data source default when the query does not set one.
func (q QueryModel) resolveWorkspace(defaultSlug string) string {
	if s := strings.TrimSpace(q.WorkspaceSlug); s != "" {
		return s
	}
	return strings.TrimSpace(defaultSlug)
}
