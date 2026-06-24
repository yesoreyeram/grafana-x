package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// gristDefaultURL is the default Grist server used when no base URL is
// configured. It targets a typical self-hosted instance; Grist Cloud team sites
// live at https://{team}.getgrist.com.
const gristDefaultURL = "http://localhost:8484"

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the API key) is kept out of this struct and is
// read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root URL of the Grist server, e.g.
	// https://{team}.getgrist.com (Cloud) or https://grist.example.com
	// (self-hosted). A trailing "/api" is accepted and normalised away by the
	// client, which always appends the "/api" prefix itself.
	BaseURL string `json:"baseURL"`
	// DocID is an optional default Grist document id. When set, the query editor
	// lists this doc's tables directly instead of enumerating orgs/workspaces.
	DocID string `json:"docId"`
	// apiKey is the Grist API key sent as the Bearer credential.
	apiKey string
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
	if strings.TrimSpace(settings.BaseURL) == "" {
		settings.BaseURL = s.URL
	}
	if strings.TrimSpace(settings.BaseURL) == "" {
		settings.BaseURL = gristDefaultURL
	}
	settings.DocID = strings.TrimSpace(settings.DocID)

	if s.DecryptedSecureJSONData != nil {
		settings.apiKey = s.DecryptedSecureJSONData["apiKey"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists rows; "count" returns the number of
	// matching rows via SQL COUNT(*); "sql" runs a raw read-only SELECT.
	QueryType string `json:"queryType"`
	// DocID is the Grist document id. Overrides the configured default doc id.
	DocID string `json:"docId"`
	// TableID is the Grist table id/name, required for record/count queries.
	TableID string `json:"tableId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. Simple equality/membership filters compile to the Grist
	// records `filter` JSON; richer operators compile to a parameterized SQL
	// WHERE clause (see filter.go).
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// Sort is the structured sort, serialized as a JSON string by the editor.
	// It compiles to the Grist records `sort` csv (records path) or a SQL
	// ORDER BY clause (SQL path).
	Sort string `json:"sort"`
	// sortItems is the parsed Sort (nil when absent/invalid).
	sortItems []SortItem
	// Fields is an optional comma separated list of column names to include.
	// Empty returns every column. Grist's records endpoint has no projection
	// param, so a non-empty Fields forces the SQL path.
	Fields string `json:"fields"`
	// Limit caps the number of records returned. 0 means no limit (Grist returns
	// every row).
	Limit int `json:"limit"`
	// SQL is the raw Grist SQL SELECT statement for the "sql" query type.
	SQL string `json:"sql"`
}

// LoadQuery parses the raw query JSON into a QueryModel and parses the structured
// filter and sort (the Grist filter / SQL is built later by the client).
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) == 0 {
		return q, nil
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return q, fmt.Errorf("invalid query json: %w", err)
	}
	if q.QueryType == "" {
		q.QueryType = QueryTypeRecords
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

// requiresSQL reports whether a record listing must use the SQL endpoint rather
// than the records endpoint. The Grist records endpoint can filter only by
// simple equality/membership (`{"col":[vals]}`), cannot project columns
// (no fields param), so any of the following force the SQL path:
//   - a non-empty Fields projection;
//   - a filter that is not expressible as a simple membership filter (rich
//     operators such as gt/lt/contains/neq, OR logic, nested groups, or the
//     same column used more than once).
func (q QueryModel) requiresSQL() bool {
	if strings.TrimSpace(q.Fields) != "" {
		return true
	}
	if q.filter == nil {
		return false
	}
	_, ok := simpleGristFilter(q.filter)
	return !ok
}
