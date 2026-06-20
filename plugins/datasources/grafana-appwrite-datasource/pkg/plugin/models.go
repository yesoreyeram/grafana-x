package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// appwriteDefaultURL is the Appwrite Cloud API endpoint. Self-hosted instances
// and regional cloud endpoints (for example https://nyc.cloud.appwrite.io/v1)
// are configured by overriding this in the data source settings.
const appwriteDefaultURL = "https://cloud.appwrite.io/v1"

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the API key) is kept out of this struct and is
// read from DecryptedSecureJSONData.
type Settings struct {
	// Endpoint is the root URL of the Appwrite API, including the `/v1` suffix.
	// Defaults to https://cloud.appwrite.io/v1.
	Endpoint string `json:"endpoint"`
	// ProjectID is the Appwrite project id, sent in the `X-Appwrite-Project`
	// header on every request.
	ProjectID string `json:"projectId"`
	// DatabaseID is an optional default Appwrite database id. When set, the query
	// editor lists that database's collections directly; otherwise the user picks
	// a database in the query editor.
	DatabaseID string `json:"databaseId"`
	// apiKey is the Appwrite API key sent in the `X-Appwrite-Key` header.
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

	// The endpoint may also be supplied through the standard data source URL field.
	if settings.Endpoint == "" {
		settings.Endpoint = s.URL
	}
	if settings.Endpoint == "" {
		settings.Endpoint = appwriteDefaultURL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiKey = s.DecryptedSecureJSONData["apiKey"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "documents" (default) lists documents from a collection;
	// "count" returns the number of matching documents.
	QueryType string `json:"queryType"`
	// DatabaseID is the Appwrite database id. When empty the data-source
	// configured database id is used.
	DatabaseID string `json:"databaseId"`
	// CollectionID is the Appwrite collection id.
	CollectionID string `json:"collectionId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend compiles it into Appwrite query
	// strings.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// RawQueries is an optional newline-separated list of raw Appwrite query
	// strings (for example `equal("status", ["active"])`). When set it takes
	// precedence over the structured FilterTree (advanced/escape hatch).
	RawQueries string `json:"rawQueries"`
	// Sort is the structured sort, serialized as a JSON string by the editor and
	// compiled into Appwrite `orderAsc`/`orderDesc` query strings.
	Sort string `json:"sort"`
	// sortItems is the parsed Sort (nil when absent/invalid).
	sortItems []SortItem
	// Attributes is an optional comma separated list of attribute keys to return
	// (compiled into an Appwrite `select` query).
	Attributes string `json:"attributes"`
	// HideSystemFields, when true, drops the Appwrite system columns (the
	// $-prefixed fields such as $id, $permissions, $collectionId, $sequence)
	// from the result frame.
	HideSystemFields bool `json:"hideSystemFields"`
	// Limit caps the number of documents returned. 0 means return all
	// (auto-paginated).
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and parses the structured
// filter and sort (the Appwrite query strings are built later by the client).
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) == 0 {
		return q, nil
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return q, fmt.Errorf("invalid query json: %w", err)
	}
	if q.QueryType == "" {
		q.QueryType = "documents"
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
