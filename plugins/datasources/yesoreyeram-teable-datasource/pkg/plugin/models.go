package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// teableCloudURL is the default Teable API host. Teable can also be self-hosted,
// so BaseURL is configurable; this is only the default for the hosted cloud.
const teableCloudURL = "https://app.teable.io"

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the API token) is kept out of this struct and
// is read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root URL of the Teable API. Defaults to
	// https://app.teable.io; set to your own domain for a self-hosted instance.
	BaseURL string `json:"baseURL"`
	// DefaultBaseID is an optional default Teable base id. When set, the query
	// editor lists that base's tables without the user typing a base id.
	DefaultBaseID string `json:"defaultBaseId"`
	// apiToken is the Teable personal access token sent in the Authorization
	// header as "Bearer <token>".
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

	// The URL may also be supplied through the standard data source URL field.
	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}
	if settings.BaseURL == "" {
		settings.BaseURL = teableCloudURL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists records from a table; "count" returns
	// the number of matching records.
	QueryType string `json:"queryType"`
	// HideSystemFields drops metadata-style columns (see system_fields.go) from
	// the returned frame when true. Defaults to false.
	HideSystemFields bool `json:"hideSystemFields"`
	// BaseID is the Teable base id. It is only needed by the query editor to list
	// tables; the record/count endpoints are addressed by table id alone.
	BaseID string `json:"baseId"`
	// TableID is the Teable table id (starts with "tbl..."). Required.
	TableID string `json:"tableId"`
	// ViewID is an optional Teable view id (starts with "viw..."). When set, the
	// query honours that view's filter/sort settings.
	ViewID string `json:"viewId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend compiles it into the Teable JSON
	// `filter` object ({conjunction, filterSet:[...]}).
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// Sort is the structured sort, serialized as a JSON string by the editor and
	// compiled into the Teable `orderBy` parameter.
	Sort string `json:"sort"`
	// sortItems is the parsed Sort (nil when absent/invalid).
	sortItems []SortItem
	// Fields is an optional comma separated list of field names to include
	// (compiled into the `projection` parameter, keyed by name).
	Fields string `json:"fields"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and parses the structured
// filter and sort (the Teable parameters are built later by the client).
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

	if strings.TrimSpace(q.Sort) != "" {
		var items []SortItem
		if err := json.Unmarshal([]byte(q.Sort), &items); err != nil {
			return q, fmt.Errorf("invalid sort json: %w", err)
		}
		q.sortItems = items
	}

	return q, nil
}
