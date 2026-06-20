package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Baserow Cloud always lives at this host.
const baserowCloudURL = "https://api.baserow.io"

// Auth modes supported by the data source.
const (
	// AuthToken uses a Baserow database token (Authorization: Token <token>).
	// Database tokens are scoped to a single database.
	AuthToken = "token"
	// AuthPassword uses email + password to obtain a JWT
	// (Authorization: JWT <jwt>), which can enumerate workspaces and databases.
	AuthPassword = "password"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secure fields (the API token / password) are kept out of this
// struct and are read from DecryptedSecureJSONData.
type Settings struct {
	// Platform is "cloud" or "selfhosted". Cloud forces the api.baserow.io URL.
	Platform string `json:"platform"`
	// BaseURL is the root URL of the Baserow instance, e.g. https://api.baserow.io
	BaseURL string `json:"baseURL"`
	// AuthMode selects how requests are authenticated: "token" (database token)
	// or "password" (email + password -> JWT). Defaults to "token".
	AuthMode string `json:"authMode"`
	// DatabaseID is the Baserow database (application) id used to list tables.
	// Required for the token auth mode (database tokens are scoped to a single
	// database); optional for the password mode where databases are discoverable.
	DatabaseID string `json:"databaseId"`
	// Email is the Baserow account email for the password auth mode.
	Email string `json:"email"`
	// apiToken is the Baserow database token sent in the Authorization header.
	apiToken string
	// password is the Baserow account password for the password auth mode.
	password string
}

// LoadSettings parses the data source instance settings.
func LoadSettings(s backend.DataSourceInstanceSettings) (Settings, error) {
	settings := Settings{}
	if len(s.JSONData) > 0 {
		if err := json.Unmarshal(s.JSONData, &settings); err != nil {
			return settings, fmt.Errorf("invalid settings json: %w", err)
		}
	}

	// Cloud platform always uses the Baserow cloud host, regardless of any
	// configured base URL.
	if settings.Platform == "cloud" {
		settings.BaseURL = baserowCloudURL
	}
	// URL may also be supplied through the standard data source URL field.
	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}
	if settings.AuthMode == "" {
		settings.AuthMode = AuthToken
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
		settings.password = s.DecryptedSecureJSONData["password"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists rows from a table; "count" returns
	// the number of matching rows.
	QueryType string `json:"queryType"`
	// TableID is the Baserow table id (numeric), required for record queries.
	TableID string `json:"tableId"`
	// ViewID is an optional Baserow view id (numeric). When set, the view's own
	// filters and sorts are applied by Baserow.
	ViewID string `json:"viewId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend builds the Baserow `filters` tree
	// from it.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// Sort is an optional comma separated list of field names (prefix - for desc).
	Sort string `json:"sort"`
	// Fields is an optional comma separated list of field names to include.
	Fields string `json:"fields"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and parses the structured
// filter tree (the Baserow `filters` clause is built later by the client).
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

	if strings.TrimSpace(q.FilterTree) != "" {
		var root FilterNode
		if err := json.Unmarshal([]byte(q.FilterTree), &root); err != nil {
			return q, fmt.Errorf("invalid filterTree json: %w", err)
		}
		q.filter = &root
	}

	return q, nil
}
