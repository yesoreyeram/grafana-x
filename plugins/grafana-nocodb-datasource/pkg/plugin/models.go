package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// NocoDB Cloud always lives at this host.
const nocodbCloudURL = "https://app.nocodb.com"

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secure fields (the API token) are kept out of this struct and
// are read from DecryptedSecureJSONData.
type Settings struct {
	// Platform is "cloud" or "selfhosted". Cloud forces the app.nocodb.com URL.
	Platform string `json:"platform"`
	// BaseURL is the root URL of the NocoDB instance, e.g. https://app.nocodb.com
	BaseURL string `json:"baseURL"`
	// APIVersion is "v2" (default) or "v3".
	APIVersion string `json:"apiVersion"`
	// apiToken is the NocoDB API token sent as the xc-token header.
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

	// Cloud platform always uses the NocoDB cloud host, regardless of any
	// configured base URL.
	if settings.Platform == "cloud" {
		settings.BaseURL = nocodbCloudURL
	}
	// URL may also be supplied through the standard data source URL field.
	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}
	if settings.APIVersion == "" {
		settings.APIVersion = "v2"
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists records from a table.
	QueryType string `json:"queryType"`
	// TableID is the NocoDB table id (prefixed with m), required for record queries.
	TableID string `json:"tableId"`
	// BaseID is the NocoDB base id (prefixed with p). Required for the v3 data API.
	BaseID string `json:"baseId"`
	// ViewID is an optional NocoDB view id (prefixed with v).
	ViewID string `json:"viewId"`
	// Where is an optional raw NocoDB where clause. Used as a manual override
	// when no structured filter tree is provided.
	Where string `json:"where"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend builds the where clause from it.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// Sort is an optional comma separated list of fields (prefix - for desc).
	Sort string `json:"sort"`
	// Fields is an optional comma separated list of fields to include.
	Fields string `json:"fields"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and resolves the
// effective where clause (built server-side from the structured filter tree
// when present, otherwise the raw where override).
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) == 0 {
		return q, nil
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return q, fmt.Errorf("invalid query json: %w", err)
	}
	if q.QueryType == "" {
		q.QueryType = "records"
	}

	// Parse the structured filter tree. The where clause is built later by the
	// client, which knows the API version (the `@` quoting prefix is v2-only).
	if strings.TrimSpace(q.FilterTree) != "" {
		var root FilterNode
		if err := json.Unmarshal([]byte(q.FilterTree), &root); err != nil {
			return q, fmt.Errorf("invalid filterTree json: %w", err)
		}
		q.filter = &root
	}

	return q, nil
}
