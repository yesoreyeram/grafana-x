package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Notion's public API lives at this host.
const notionCloudURL = "https://api.notion.com"

// defaultNotionVersion is the Notion-Version header sent with every request. The
// Notion API is versioned by date and requires this header on all calls.
const defaultNotionVersion = "2022-06-28"

// Settings represents the data source instance settings configured in the
// ConfigEditor. Secure fields (the integration token) are kept out of this
// struct and are read from DecryptedSecureJSONData.
type Settings struct {
	// BaseURL is the root URL of the Notion API. Defaults to api.notion.com;
	// overridable to point at a proxy or gateway.
	BaseURL string `json:"baseURL"`
	// NotionVersion is the value of the Notion-Version header (e.g. 2022-06-28).
	NotionVersion string `json:"notionVersion"`
	// DatabaseID is an optional default Notion database id used to populate the
	// database dropdown when search is restricted.
	DatabaseID string `json:"databaseId"`
	// apiToken is the Notion integration token sent as the Bearer credential.
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
		settings.BaseURL = notionCloudURL
	}
	if settings.NotionVersion == "" {
		settings.NotionVersion = defaultNotionVersion
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists pages from a database; "count"
	// returns the number of matching pages.
	QueryType string `json:"queryType"`
	// DatabaseID is the Notion database id, required for record/count queries.
	DatabaseID string `json:"databaseId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend builds the Notion filter from it.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// Sort is an optional comma separated list of properties (prefix - for desc).
	Sort string `json:"sort"`
	// Fields is an optional comma separated list of property names to include.
	// Empty returns every property.
	Fields string `json:"fields"`
	// Limit caps the number of pages returned. 0 means use default paging.
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and resolves the
// structured filter tree (built server-side into the Notion filter object when
// present).
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
