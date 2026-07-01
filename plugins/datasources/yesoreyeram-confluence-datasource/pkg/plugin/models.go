package plugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Authentication modes supported by the Confluence data source.
const (
	// authBasic uses HTTP Basic auth with email + API token
	// (Atlassian Cloud: token from id.atlassian.com/manage-profile/security/api-tokens).
	authBasic = "basic"
	// authBearer uses a Bearer token (OAuth2 access token or a Data Center
	// Personal Access Token).
	authBearer = "bearer"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secure fields (the API token / bearer token) are kept out of
// this struct and are read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root URL of the Confluence wiki, e.g.
	// https://your-site.atlassian.net/wiki for Cloud. The v2 API path
	// (/api/v2/...) and the CQL search path (/rest/api/search) are appended to
	// this base.
	BaseURL string `json:"baseURL"`
	// AuthMode selects how requests are authenticated: "basic" (email + API
	// token) or "bearer" (OAuth2 / Personal Access Token).
	AuthMode string `json:"authMode"`
	// Email is the Atlassian account email, used with Basic auth.
	Email string `json:"email"`

	// apiToken is the Atlassian API token used with Basic auth.
	apiToken string
	// bearerToken is the OAuth2 access token / PAT used with Bearer auth.
	bearerToken string
}

// LoadSettings parses the data source instance settings.
func LoadSettings(s backend.DataSourceInstanceSettings) (Settings, error) {
	settings := Settings{}
	if len(s.JSONData) > 0 {
		if err := json.Unmarshal(s.JSONData, &settings); err != nil {
			return settings, fmt.Errorf("invalid settings json: %w", err)
		}
	}

	// The URL may also be supplied through the standard data source URL field.
	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}
	settings.BaseURL = strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if settings.AuthMode == "" {
		settings.AuthMode = authBasic
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
		settings.bearerToken = s.DecryptedSecureJSONData["bearerToken"]
	}
	return settings, nil
}

// authHeader returns the value of the Authorization header to send, based on the
// configured auth mode. It returns an empty string when no usable credential is
// configured.
func (s Settings) authHeader() string {
	switch s.AuthMode {
	case authBearer:
		token := strings.TrimSpace(s.bearerToken)
		if token == "" {
			return ""
		}
		return "Bearer " + token
	default: // authBasic
		email := strings.TrimSpace(s.Email)
		token := strings.TrimSpace(s.apiToken)
		if email == "" || token == "" {
			return ""
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
		return "Basic " + encoded
	}
}

// hasCredential reports whether a usable credential is configured for the
// selected auth mode.
func (s Settings) hasCredential() bool {
	return s.authHeader() != ""
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "pages" (default) | "blogposts" | "search" | "count".
	QueryType string `json:"queryType"`
	// HideSystemFields drops metadata-style columns (see system_fields.go) from
	// the returned frame when true. Defaults to false.
	HideSystemFields bool `json:"hideSystemFields"`
	// SpaceID scopes pages/blogposts (and the default count) to a space. It is
	// the numeric space id returned by the spaces endpoint.
	SpaceID string `json:"spaceId"`
	// CQL is the Confluence Query Language string for "search" queries. When set
	// on a "count" query, the count is over the CQL search results.
	CQL string `json:"cql"`
	// Sort is an optional sort order for pages/blogposts (e.g. "-modified-date",
	// "title"). Ignored for search queries (order by the CQL itself).
	Sort string `json:"sort"`
	// Fields is an optional comma-separated list of column names to return.
	// Empty returns every flattened column.
	Fields string `json:"fields"`
	// Cursor is an optional pagination cursor to start from (advanced). Empty
	// starts from the first page.
	Cursor string `json:"cursor"`
	// Limit caps the number of records returned. 0 means auto-paginate up to a
	// safety cap.
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
		q.QueryType = queryTypePages
	}
	return q, nil
}
