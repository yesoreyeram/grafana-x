package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret field (the API key) is kept out of this struct and is
// read from DecryptedSecureJSONData.
type Settings struct {
	// APIURL is the Supabase PostgREST endpoint.
	// e.g. https://<project-ref>.supabase.co/rest/v1
	APIURL string `json:"apiUrl"`
	// Schema is the optional Postgres schema to expose, sent via the
	// `Accept-Profile` header. Empty uses the PostgREST default (public).
	Schema string `json:"schema"`
	// apiKey is the Supabase anon or service_role key (or a user JWT). Sent as
	// BOTH the `apikey` header and `Authorization: Bearer <key>`.
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

	if settings.APIURL == "" {
		settings.APIURL = s.URL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiKey = s.DecryptedSecureJSONData["serviceKey"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	QueryType  string `json:"queryType"`
	TableID    string `json:"tableId"`
	Select     string `json:"select"`
	FilterTree string `json:"filterTree"`
	filter     *FilterNode
	Sort       string `json:"sort"`
	sortItems  []SortItem
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
}

// LoadQuery parses the raw query JSON into a QueryModel.
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) == 0 {
		q.QueryType = "records"
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
