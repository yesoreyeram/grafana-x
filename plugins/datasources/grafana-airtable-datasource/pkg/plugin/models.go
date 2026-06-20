package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// airtableDefaultURL is the Airtable Web API host. It is fixed (Airtable is a
// hosted SaaS); it is overridable only to support proxies/testing.
const airtableDefaultURL = "https://api.airtable.com"

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the personal access token) is kept out of this
// struct and is read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root URL of the Airtable API. Defaults to
	// https://api.airtable.com; usually only changed to route through a proxy.
	BaseURL string `json:"baseURL"`
	// BaseID is an optional default Airtable base id (starts with "app...").
	// When set, the query editor lists that base's tables directly; otherwise
	// the user picks a base in the query editor.
	BaseID string `json:"baseId"`
	// apiToken is the Airtable personal access token (PAT) sent in the
	// Authorization header as "Bearer <token>".
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
		settings.BaseURL = airtableDefaultURL
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
	// BaseID is the Airtable base id (starts with "app..."). When empty the
	// data-source-configured base id is used.
	BaseID string `json:"baseId"`
	// TableID is the Airtable table id (starts with "tbl...") or table name.
	TableID string `json:"tableId"`
	// ViewID is an optional Airtable view id (starts with "viw...") or name.
	// When set, only records in that view are returned, ordered by the view.
	ViewID string `json:"viewId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend compiles it into an Airtable
	// `filterByFormula` expression.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// FilterByFormula is an optional raw Airtable formula. When set it takes
	// precedence over the structured FilterTree (advanced/escape hatch).
	FilterByFormula string `json:"filterByFormula"`
	// Sort is the structured sort, serialized as a JSON string by the editor and
	// compiled into Airtable `sort[i][field]`/`sort[i][direction]` parameters.
	Sort string `json:"sort"`
	// sortItems is the parsed Sort (nil when absent/invalid).
	sortItems []SortItem
	// Fields is an optional comma separated list of field names to include.
	Fields string `json:"fields"`
	// Limit caps the number of records returned. 0 means use default paging.
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and parses the structured
// filter and sort (the Airtable parameters are built later by the client).
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
