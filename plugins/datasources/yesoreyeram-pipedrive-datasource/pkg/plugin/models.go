package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Authentication methods. Pipedrive supports two credential types:
//   - apiToken: an API token (Settings > Personal preferences > API), sent as
//     the `api_token` query parameter on every request.
//   - oauth: an OAuth2 access token, sent as `Authorization: Bearer <token>`.
const (
	authAPIToken = "apiToken"
	authOAuth    = "oauth"
)

// Settings represents the data source instance settings from the ConfigEditor.
// Secret credentials are kept out of this struct and read from
// DecryptedSecureJSONData.
type Settings struct {
	// CompanyDomain is the Pipedrive company subdomain (e.g. "mycompany"
	// from mycompany.pipedrive.com). The API base URL is built as
	// https://{companyDomain}.pipedrive.com/api/v1
	CompanyDomain string `json:"companyDomain"`

	// AuthMethod selects the credential type: "apiToken" (default) or "oauth".
	// apiToken is sent as the `api_token` query parameter; oauth is sent as an
	// `Authorization: Bearer` header.
	AuthMethod string `json:"authMethod"`

	// apiToken is the Pipedrive API token (sent as ?api_token=).
	apiToken string
	// oauthToken is a Pipedrive OAuth2 access token (sent as Bearer).
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
	settings.CompanyDomain = strings.TrimSpace(settings.CompanyDomain)
	if settings.AuthMethod == "" {
		settings.AuthMethod = authAPIToken
	}
	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = strings.TrimSpace(s.DecryptedSecureJSONData["apiToken"])
		settings.oauthToken = strings.TrimSpace(s.DecryptedSecureJSONData["oauthToken"])
	}
	return settings, nil
}

// authMode returns the resolved authentication mode. When the configured method
// is apiToken but only an OAuth token is present (or vice versa), it falls back
// to whichever credential is actually configured so the connection still works.
func (s Settings) authMode() string {
	switch s.AuthMethod {
	case authOAuth:
		if s.oauthToken == "" && s.apiToken != "" {
			return authAPIToken
		}
		return authOAuth
	default:
		if s.apiToken == "" && s.oauthToken != "" {
			return authOAuth
		}
		return authAPIToken
	}
}

// credential returns the token to send for the resolved auth mode.
func (s Settings) credential() string {
	if s.authMode() == authOAuth {
		return s.oauthToken
	}
	return s.apiToken
}

// Filter represents a single client-side filter condition.
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// FilterGroup is a group of filters combined with AND logic. Multiple filter
// groups are combined with OR logic.
type FilterGroup struct {
	Filters []Filter `json:"filters"`
}

// QueryModel represents the per-query payload from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: deals, persons, organizations, products,
	// or count.
	QueryType string `json:"queryType"`

	// ----- Server-side filters (Pipedrive list query params) -------------------
	// PipelineId / StageId apply to deals; UserId applies to deals/persons/orgs.
	PipelineId string `json:"pipelineId"`
	StageId    string `json:"stageId"`
	UserId     string `json:"userId"`
	// Status filters deals (all|open|won|lost|deleted).
	Status string `json:"statusFilter"`
	// FilterId selects a saved Pipedrive filter. When set it takes precedence
	// over the other server-side filters (Pipedrive ignores them).
	FilterId string `json:"filterId"`

	// ----- Count -----------------------------------------------------------------
	// CountEntity selects which entity to count for the "count" query type
	// (deals|persons|organizations|products). Defaults to deals.
	CountEntity string `json:"countEntity"`

	// ----- Fields selection -----------------------------------------------------
	Fields []string `json:"fields"`

	// ----- Custom field mapping -------------------------------------------------
	// MapCustomFields, when nil or true, remaps 40-character custom field hash
	// keys to their human-readable names using the {entity}Fields endpoint.
	MapCustomFields *bool `json:"mapCustomFields"`

	// ----- Client-side filter groups -------------------------------------------
	FilterGroups []FilterGroup `json:"filterGroups"`

	// ----- Sort ----------------------------------------------------------------
	SortBy  string `json:"sortBy"`
	SortDir string `json:"sortDir"`

	// ----- Pagination ----------------------------------------------------------
	// Limit caps the total number of records returned across pages. 0 (or
	// negative) fetches all matching records up to the safety cap.
	Limit int `json:"limit"`
	// Start is the initial pagination offset.
	Start int `json:"start"`
}

// shouldMapCustomFields reports whether custom field hash keys should be remapped
// to their names. Defaults to true when unset.
func (q QueryModel) shouldMapCustomFields() bool {
	return q.MapCustomFields == nil || *q.MapCustomFields
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
		q.QueryType = queryTypeDeals
	}
	if q.Status == "" {
		q.Status = "all"
	}
	if q.SortDir == "" {
		q.SortDir = "DESC"
	}
	if q.CountEntity == "" {
		q.CountEntity = queryTypeDeals
	}
	return q, nil
}
