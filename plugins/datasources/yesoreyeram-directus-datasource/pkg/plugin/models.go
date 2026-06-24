package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type Settings struct {
	BaseURL             string `json:"baseURL"`
	DefaultCollectionID string `json:"defaultCollectionId"`
	apiToken            string
}

func LoadSettings(s backend.DataSourceInstanceSettings) (Settings, error) {
	settings := Settings{}
	if len(s.JSONData) > 0 {
		if err := json.Unmarshal(s.JSONData, &settings); err != nil {
			return settings, fmt.Errorf("invalid settings json: %w", err)
		}
	}

	if settings.BaseURL == "" {
		settings.BaseURL = s.URL
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

type QueryModel struct {
	QueryType    string `json:"queryType"`
	CollectionID string `json:"collectionId"`
	FilterTree   string `json:"filterTree"`
	filter       *FilterNode
	Fields       string `json:"fields"`
	Sort         string `json:"sort"`
	sortItems    []SortItem
	Limit        int64  `json:"limit"`
	Offset       int64  `json:"offset"`
	Search       string `json:"search"`
}

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
