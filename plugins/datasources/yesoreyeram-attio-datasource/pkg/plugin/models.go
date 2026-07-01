package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// attioCloudURL is the fixed host for the Attio REST API. Attio is a hosted SaaS
// product, so the base URL is constant; it is only overridable to point at a
// proxy or gateway.
const attioCloudURL = "https://api.attio.com"

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secure access token is kept out of this struct and is read
// from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root URL of the Attio API. Defaults to https://api.attio.com;
	// overridable to point at a proxy or gateway.
	BaseURL string `json:"baseURL"`
	// DefaultObjectID is an optional default object api_slug used to prefill the
	// query editor.
	DefaultObjectID string `json:"defaultObjectId"`
	// apiToken is the Attio workspace access token sent as the Bearer credential.
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
		settings.BaseURL = attioCloudURL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists records for an object; "count"
	// returns the number of matching records.
	QueryType string `json:"queryType"`
	// ObjectID is the object api_slug (e.g. people, companies, deals), required
	// for record/count queries.
	ObjectID string `json:"objectId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend compiles it into an Attio filter.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// Fields is an optional comma separated list of attribute slugs to include.
	// Empty returns every attribute.
	Fields string `json:"fields"`
	// HideSystemFields drops the synthetic identity columns (_record_id,
	// _created_at) from the returned frame when true. Defaults to false so the
	// columns are emitted for backwards compatibility.
	HideSystemFields bool `json:"hideSystemFields"`
	// Sort is a JSON-serialized array of structured sort items.
	Sort string `json:"sort"`
	// sortItems is the parsed Sort (empty when absent/invalid).
	sortItems []SortItem
	// Limit caps the number of records returned. 0 returns all (auto-paginated).
	Limit int64 `json:"limit"`
	// Offset is the number of records to skip (offset-based pagination).
	Offset int64 `json:"offset"`
}

// LoadQuery parses the raw query JSON into a QueryModel and resolves the
// structured filter tree and sort items.
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
