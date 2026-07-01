package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// intercomCloudURL is the default Intercom REST API host (US region).
const intercomCloudURL = "https://api.intercom.io"

// defaultIntercomVersion is the value of the Intercom-Version header sent with
// every request. Intercom is versioned by date; pinning a version keeps response
// shapes stable.
const defaultIntercomVersion = "2.11"

// Region base URLs. Intercom hosts data in three regions and each has a distinct
// API host. The region is chosen in the ConfigEditor; an explicit Base URL
// always takes precedence.
const (
	regionUS = "us"
	regionEU = "eu"
	regionAU = "au"

	baseURLUnitedStates = "https://api.intercom.io"
	baseURLEurope       = "https://api.eu.intercom.io"
	baseURLAustralia    = "https://api.au.intercom.io"
)

// regionBaseURL maps a region code to its Intercom API host.
var regionBaseURL = map[string]string{
	regionUS: baseURLUnitedStates,
	regionEU: baseURLEurope,
	regionAU: baseURLAustralia,
}

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secure fields (the access token) are kept out of this struct and
// read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root URL of the Intercom API. When empty it is derived from
	// Region (defaulting to the US host). An explicit value wins over Region.
	BaseURL string `json:"baseURL"`
	// Region selects the Intercom data residency region (us/eu/au). Used only to
	// derive BaseURL when BaseURL is empty.
	Region string `json:"region"`
	// IntercomVersion is the value of the Intercom-Version header (e.g. 2.11).
	IntercomVersion string `json:"intercomVersion"`
	// apiToken is the Intercom access token sent as the Bearer credential.
	apiToken string
}

// LoadSettings parses the data source instance settings, applying defaults for
// the base URL (from region) and the Intercom-Version header.
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
	// Derive the base URL from the chosen region when not set explicitly.
	if settings.BaseURL == "" {
		if u, ok := regionBaseURL[strings.ToLower(strings.TrimSpace(settings.Region))]; ok {
			settings.BaseURL = u
		}
	}
	if settings.BaseURL == "" {
		settings.BaseURL = intercomCloudURL
	}
	if settings.IntercomVersion == "" {
		settings.IntercomVersion = defaultIntercomVersion
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

// SearchFilter is a single Intercom Search API condition (field/operator/value).
type SearchFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType selects the entity to fetch: conversations, contacts, tickets,
	// articles, companies, admins, teams, tags, or count.
	QueryType string `json:"queryType"`
	// HideSystemFields drops metadata-style columns (see system_fields.go) from
	// the returned frame when true. Defaults to false.
	HideSystemFields bool `json:"hideSystemFields"`

	// CountOf is the entity to count when QueryType == "count". Defaults to
	// conversations.
	CountOf string `json:"countOf"`

	// ----- Structured pickers (compiled into the search query server-side) -----
	// StatusFilter is a conversation state filter (open/closed/snoozed).
	StatusFilter string `json:"statusFilter"`
	// Role is a contact role filter (user/lead).
	Role string `json:"role"`
	// AssigneeID filters by the admin assignee id (admin_assignee_id).
	AssigneeID string `json:"assigneeId"`
	// TeamID filters by the team assignee id (team_assignee_id).
	TeamID string `json:"teamId"`
	// TagID filters by tag id (tag_ids contains).
	TagID string `json:"tagId"`
	// SearchQuery is a free-text contains match. It is applied to the entity's
	// primary text field (e.g. email for contacts, name for companies).
	SearchQuery string `json:"searchQuery"`

	// ----- Generic filter rows (Intercom Search API conditions) ----------------
	Filters []SearchFilter `json:"filters"`

	// ----- Sort -----------------------------------------------------------------
	// Sort is a field name, optionally prefixed with `-` for descending,
	// e.g. `-created_at`.
	Sort string `json:"sort"`

	// ----- Limit -----------------------------------------------------------------
	// Limit caps the number of records returned. 0 means use default paging up to
	// the safety cap.
	Limit int `json:"limit"`
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
		q.QueryType = queryTypeConversations
	}
	if q.CountOf == "" {
		q.CountOf = queryTypeConversations
	}
	return q, nil
}

// hasSearch reports whether the query carries any criteria that require the
// Intercom Search API (as opposed to a plain list). When false, a searchable
// entity is fetched via its cheaper list endpoint.
func (q QueryModel) hasSearch() bool {
	if strings.TrimSpace(q.StatusFilter) != "" ||
		strings.TrimSpace(q.Role) != "" ||
		strings.TrimSpace(q.AssigneeID) != "" ||
		strings.TrimSpace(q.TeamID) != "" ||
		strings.TrimSpace(q.TagID) != "" ||
		strings.TrimSpace(q.SearchQuery) != "" {
		return true
	}
	for _, f := range q.Filters {
		if strings.TrimSpace(f.Field) != "" {
			return true
		}
	}
	return false
}
