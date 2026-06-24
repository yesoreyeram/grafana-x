package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// codaDefaultURL is the Coda Web API base path. Coda is SaaS-only; all
// resource paths are appended to this (e.g. baseURL + "/docs"), so the base
// already includes the "/v1" version segment.
const codaDefaultURL = "https://coda.io/apis/v1"

// defaultValueFormat is the Coda rows valueFormat used when none is requested.
// "simple" returns scalar cell values (string/number/boolean) and renders array
// values (e.g. multi-selects) as comma-delimited strings, which keeps frame
// cells clean.
const defaultValueFormat = "simple"

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secure fields (the API token) are kept out of this struct and
// are read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root of the Coda Web API. Defaults to codaDefaultURL;
	// overridable to point at a proxy or gateway.
	BaseURL string `json:"baseURL"`
	// DocID is an optional default Coda doc id used when a query omits one.
	DocID string `json:"docId"`
	// apiToken is the Coda API token sent as the Bearer credential.
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

	// URL may also be supplied through the standard data source URL field.
	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}
	if settings.BaseURL == "" {
		settings.BaseURL = codaDefaultURL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "rows" (default) lists rows from a table; "count" returns the
	// number of rows.
	QueryType string `json:"queryType"`
	// DocID is the Coda doc id. Overrides the configured default doc id.
	DocID string `json:"docId"`
	// TableID is the Coda table id or name, required for row/count queries.
	TableID string `json:"tableId"`
	// Columns is an optional comma separated list of column names to include.
	// Coda's rows endpoint has no column-projection parameter, so this is
	// applied client-side after fetching. Empty returns every column.
	Columns string `json:"columns"`
	// Query is an optional raw Coda `query` parameter, of the form
	// `<columnIdOrName>:<value>`. When set it takes precedence over the
	// structured FilterColumn/FilterValue.
	Query string `json:"query"`
	// FilterColumn / FilterValue form a single-column equality filter compiled
	// into Coda's `query` parameter. Coda's rows endpoint only supports
	// filtering by a single column, so richer filtering must be done with
	// Grafana transformations.
	FilterColumn string `json:"filterColumn"`
	FilterValue  string `json:"filterValue"`
	// SortBy is one of "createdAt", "updatedAt" or "natural" (Coda's RowsSortBy
	// enum). Any other value is ignored. "natural" implies visibleOnly.
	SortBy string `json:"sortBy"`
	// VisibleOnly, when true, returns only visible rows/columns for the table.
	VisibleOnly bool `json:"visibleOnly"`
	// ValueFormat is the Coda valueFormat: "simple" (default), "simpleWithArrays"
	// or "rich".
	ValueFormat string `json:"valueFormat"`
	// Limit caps the number of rows returned. 0 means use default paging.
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and applies defaults.
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) == 0 {
		q.QueryType = QueryTypeRows
		q.ValueFormat = defaultValueFormat
		return q, nil
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return q, fmt.Errorf("invalid query json: %w", err)
	}
	if q.QueryType == "" {
		q.QueryType = QueryTypeRows
	}
	if q.ValueFormat == "" {
		q.ValueFormat = defaultValueFormat
	}
	return q, nil
}

// DocInfo is a lightweight representation of a Coda doc.
type DocInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// TableInfo is a lightweight representation of a Coda table.
type TableInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ColumnInfo is a lightweight representation of a Coda column. Type is the
// column's data type (from the column's `format.type`, e.g. "text", "number",
// "checkbox"), used by the query editor to offer sensible defaults.
type ColumnInfo struct {
	Title string `json:"title"`
	Type  string `json:"type"`
}
