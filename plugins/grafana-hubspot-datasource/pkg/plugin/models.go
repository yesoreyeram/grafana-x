package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const hubSpotCloudURL = "https://api.hubapi.com"

const (
	authPrivateApp = "privateApp"
	authOAuth      = "oauth"
)

const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Settings represents the data source instance settings from the ConfigEditor.
type Settings struct {
	// BaseURL is the HubSpot API root (without trailing slash). Defaults to
	// https://api.hubapi.com; set to https://api.hubapi.eu for EU data residency.
	BaseURL string `json:"baseURL"`
	// AuthMethod selects how the request is authenticated: "privateApp" (private
	// app access token) or "oauth" (OAuth2 access token). Both are sent as
	// Authorization: Bearer but the distinction aids frontend UX.
	AuthMethod string `json:"authMethod"`

	// privateAppToken is a HubSpot private app access token (Bearer auth).
	privateAppToken string
	// oauthToken is a HubSpot OAuth2 access token (Bearer auth).
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
	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}
	if settings.BaseURL == "" {
		settings.BaseURL = hubSpotCloudURL
	}
	if settings.AuthMethod == "" {
		settings.AuthMethod = authPrivateApp
	}
	if s.DecryptedSecureJSONData != nil {
		settings.privateAppToken = s.DecryptedSecureJSONData["privateAppToken"]
		settings.oauthToken = s.DecryptedSecureJSONData["oauthToken"]
	}
	return settings, nil
}

// credential returns the token to send. Both private app and OAuth tokens use
// Bearer auth in HubSpot.
func (s Settings) credential() string {
	if s.AuthMethod == authOAuth {
		return strings.TrimSpace(s.oauthToken)
	}
	return strings.TrimSpace(s.privateAppToken)
}

// Filter represents a single HubSpot CRM search filter.
type Filter struct {
	PropertyName string `json:"propertyName"`
	Operator     string `json:"operator"`
	Value        string `json:"value"`
}

// FilterGroup is a group of filters combined with AND logic. Multiple filter
// groups are combined with OR logic.
type FilterGroup struct {
	Filters []Filter `json:"filters"`
}

// QueryModel represents the per-query payload from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: contacts, companies, deals, tickets,
	// products, line_items, meetings, calls, tasks, notes, emails, pipelines,
	// owners, properties, or raw.
	QueryType string `json:"queryType"`

	// ----- Filter groups (for CRM Search API queries) -------------------------
	FilterGroups []FilterGroup `json:"filterGroups"`

	// ----- Sort -----------------------------------------------------------------
	// SortBy is the property name to sort by.
	SortBy string `json:"sortBy"`
	// SortDir is "ASCENDING" or "DESCENDING".
	SortDir string `json:"sortDir"`

	// ----- Property selection ---------------------------------------------------
	// Properties is the subset of properties to return. Empty returns all default
	// properties.
	Properties []string `json:"properties"`

	// ----- Pipeline / stage (for deals, tickets) --------------------------------
	PipelineId string `json:"pipelineId"`
	StageId    string `json:"stageId"`

	// ----- Date filters ----------------------------------------------------------
	// Created/createdAt: HubSpot uses "createdate" / "hs_lastmodifieddate"
	CreatedMode    string `json:"createdMode"`
	CreatedAfter   string `json:"createdAfter"`
	CreatedBefore  string `json:"createdBefore"`
	UpdatedMode    string `json:"updatedMode"`
	UpdatedAfter   string `json:"updatedAfter"`
	UpdatedBefore  string `json:"updatedBefore"`

	// ----- Limit -----------------------------------------------------------------
	Limit int `json:"limit"`

	// ----- Object type (for pipelines/properties queries) -----------------------
	ObjectType string `json:"objectType"`

	// ----- Raw query ------------------------------------------------------------
	RawPath   string `json:"rawPath"`
	RawMethod string `json:"rawMethod"`
	RawBody   string `json:"rawBody"`
	RawRoot   string `json:"rawRoot"`

	// TimeRange is populated from the backend request, not query JSON.
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
		q.QueryType = queryTypeContacts
	}
	if q.SortDir == "" {
		q.SortDir = "DESCENDING"
	}
	if q.CreatedMode == "" {
		q.CreatedMode = dateModeAny
	}
	if q.UpdatedMode == "" {
		q.UpdatedMode = dateModeAny
	}
	if q.RawMethod == "" {
		q.RawMethod = "GET"
	}
	return q, nil
}
