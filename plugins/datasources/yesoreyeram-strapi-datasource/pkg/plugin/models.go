package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// APIVersionV4 and APIVersionV5 are the supported Strapi major versions. The
// REST response shape differs between them (see frame.go::flattenRecord), so the
// configured version is surfaced to users; the backend additionally
// auto-detects the shape per record for robustness.
const (
	APIVersionV4 = "v4"
	APIVersionV5 = "v5"
)

type Settings struct {
	BaseURL              string `json:"baseURL"`
	DefaultContentTypeID string `json:"defaultContentTypeId"`
	// APIVersion is the Strapi major version ("v4" or "v5"). Defaults to "v5".
	// It selects the expected response shape; the backend still auto-detects the
	// shape per record so a misconfigured version degrades gracefully.
	APIVersion string `json:"apiVersion"`
	apiToken   string
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

	switch settings.APIVersion {
	case APIVersionV4, APIVersionV5:
		// keep
	default:
		settings.APIVersion = APIVersionV5
	}

	if s.DecryptedSecureJSONData != nil {
		settings.apiToken = s.DecryptedSecureJSONData["apiToken"]
	}
	return settings, nil
}

type QueryModel struct {
	QueryType string `json:"queryType"`
	// HideSystemFields drops metadata-style columns (see system_fields.go) from
	// the returned frame when true. Defaults to false.
	HideSystemFields bool   `json:"hideSystemFields"`
	ContentTypeID    string `json:"contentTypeId"`
	FilterTree       string `json:"filterTree"`
	filter           *FilterNode
	Fields           string `json:"fields"`
	Sort             string `json:"sort"`
	sortItems        []SortItem
	Page             int    `json:"page"`
	PageSize         int    `json:"pageSize"`
	Populate         string `json:"populate"`
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
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 25
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
