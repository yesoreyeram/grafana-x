package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// seatableCloudURL is the default SeaTable server. It is overridable for
// self-hosted deployments via the Server URL config field.
const seatableCloudURL = "https://cloud.seatable.io"

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the Base API Token) is kept out of this struct
// and is read from DecryptedSecureJSONData.
type Settings struct {
	// ServerURL is the root URL of the SeaTable server (e.g.
	// https://cloud.seatable.io or a self-hosted instance). Defaults to the
	// SeaTable cloud URL.
	ServerURL string `json:"serverURL"`
	// apiToken is the SeaTable *Base API Token*. It is NOT used directly on data
	// endpoints; the backend exchanges it for a short-lived Base-Token (access
	// token) plus the base's dtable_uuid (see client.go).
	apiToken string
}

// settingsJSON is the on-disk shape of the non-secret settings. A legacy
// `baseURL` key is accepted so existing provisioning keeps working.
type settingsJSON struct {
	ServerURL string `json:"serverURL"`
	BaseURL   string `json:"baseURL"` // legacy alias for ServerURL
}

// LoadSettings parses the data source instance settings.
func LoadSettings(s backend.DataSourceInstanceSettings) (Settings, error) {
	raw := settingsJSON{}
	if len(s.JSONData) > 0 {
		if err := json.Unmarshal(s.JSONData, &raw); err != nil {
			return Settings{}, fmt.Errorf("invalid settings json: %w", err)
		}
	}

	server := firstNonEmpty(raw.ServerURL, raw.BaseURL, s.URL, seatableCloudURL)

	settings := Settings{ServerURL: server}
	if s.DecryptedSecureJSONData != nil {
		// `apiToken` is the canonical key; `baseToken` is accepted for backward
		// compatibility with earlier provisioning.
		settings.apiToken = firstNonEmpty(
			s.DecryptedSecureJSONData["apiToken"],
			s.DecryptedSecureJSONData["baseToken"],
		)
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists rows; "count" returns the number of
	// matching rows; "sql" runs a raw SeaTable SQL statement.
	QueryType string `json:"queryType"`
	// TableName is the SeaTable table name, required for record/count queries.
	TableName string `json:"tableName"`
	// ViewName is an optional view name. It is only honoured for plain record
	// listings (no filter/sort/fields), which use the rows endpoint. Filtered,
	// sorted or projected queries run via SQL, which has no view concept.
	ViewName string `json:"viewName"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend compiles it into a parameterized
	// SQL WHERE clause.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// Sort is the structured sort, serialized as a JSON string by the editor and
	// compiled into a SQL ORDER BY clause.
	Sort string `json:"sort"`
	// sortItems is the parsed Sort (nil when absent/invalid).
	sortItems []SortItem
	// Fields is an optional comma separated list of column names to include.
	// Empty returns every column.
	Fields string `json:"fields"`
	// Limit caps the number of rows returned. 0 means use default paging.
	Limit int64 `json:"limit"`
	// SQL is the raw SeaTable SQL statement for the "sql" query type.
	SQL string `json:"sql"`
}

// LoadQuery parses the raw query JSON into a QueryModel and parses the structured
// filter and sort (the SQL is built later by the client).
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

// requiresSQL reports whether a record listing must use the SQL endpoint. The
// rows endpoint cannot filter, sort, or project columns, so any of those force
// the SQL path.
func (q QueryModel) requiresSQL() bool {
	return q.filter != nil || len(q.sortItems) > 0 || strings.TrimSpace(q.Fields) != ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
