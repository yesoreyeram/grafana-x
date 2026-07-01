package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// trelloAPIBase is Trello's REST API root. Trello is hosted SaaS; there is no
// self-hosted/local mode, so the base URL is fixed and not configurable.
const trelloAPIBase = "https://api.trello.com"

// Supported query types.
const (
	queryTypeCards = "cards"
	queryTypeCount = "count"
)

// Card visibility filters accepted by Trello's cards endpoints.
const (
	cardFilterAll    = "all"
	cardFilterOpen   = "open"
	cardFilterClosed = "closed"
)

// Date filter modes for the card creation-date filter. Trello's card endpoints
// only support filtering by creation date (via the before/since cursor, which
// operate on the card id's embedded timestamp), so this mirrors the ClickUp
// pattern but applies to created dates only.
const (
	dateModeAny       = "any"
	dateModeDashboard = "dashboard"
	dateModeCustom    = "custom"
)

// Settings represents the data source instance settings. Both credentials are
// secrets kept out of this struct's JSON and read from DecryptedSecureJSONData.
type Settings struct {
	// apiKey is the Trello API key (sent as the `key` query parameter).
	apiKey string
	// apiToken is the Trello API token (sent as the `token` query parameter).
	apiToken string
}

// LoadSettings parses the data source instance settings, reading both secrets
// from secure storage.
func LoadSettings(s backend.DataSourceInstanceSettings) (Settings, error) {
	settings := Settings{}
	if s.DecryptedSecureJSONData != nil {
		settings.apiKey = strings.TrimSpace(s.DecryptedSecureJSONData["apiKey"])
		settings.apiToken = strings.TrimSpace(s.DecryptedSecureJSONData["apiToken"])
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType selects what to fetch: "cards" (default) or "count".
	QueryType string `json:"queryType"`
	// HideSystemFields drops metadata-style columns (see system_fields.go) from
	// the returned frame when true. Defaults to false.
	HideSystemFields bool `json:"hideSystemFields"`

	// --- Scope -------------------------------------------------------------

	// BoardId is the Trello board to read cards from. Required (cards are always
	// board- or list-scoped).
	BoardId string `json:"boardId"`
	// ListId optionally narrows the query to a single list on the board. When
	// set, cards are read from the list endpoint directly.
	ListId string `json:"listId"`

	// --- Card filters ------------------------------------------------------

	// CardFilter is the visibility filter passed to Trello: "all" (default),
	// "open", or "closed".
	CardFilter string `json:"cardFilter"`
	// MemberIds filters cards to those assigned to any of the given member ids
	// (applied client-side; Trello has no server-side member filter on cards).
	MemberIds []string `json:"memberIds"`
	// LabelIds filters cards to those carrying any of the given label ids
	// (applied client-side; Trello has no server-side label filter on cards).
	LabelIds []string `json:"labelIds"`

	// CreatedMode selects the card creation-date filter source: "any" (default),
	// "dashboard" (use the panel time range), or "custom" (use the bounds below).
	CreatedMode string `json:"createdMode"`
	// CreatedAfter/CreatedBefore bound the card creation date when CreatedMode is
	// "custom" (ISO-8601, Unix millis/seconds, or a card id).
	CreatedAfter  string `json:"createdAfter"`
	CreatedBefore string `json:"createdBefore"`

	// Fields restricts the returned columns (after flattening). Empty returns all
	// flattened fields.
	Fields []string `json:"fields"`
	// Limit caps the number of cards returned for "cards" queries. 0 returns all
	// (auto-paginated via the before cursor). Ignored by "count".
	Limit int `json:"limit"`

	// TimeRange is the panel/dashboard time range. It is not part of the query
	// JSON; it is populated from the backend request and used when CreatedMode is
	// "dashboard".
	TimeRange backend.TimeRange `json:"-"`
}

// LoadQuery parses the raw query JSON into a QueryModel and applies defaults.
func LoadQuery(raw json.RawMessage) (QueryModel, error) {
	q := QueryModel{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &q); err != nil {
			return q, fmt.Errorf("invalid query json: %w", err)
		}
	}
	if q.QueryType == "" {
		q.QueryType = queryTypeCards
	}
	if q.CardFilter == "" {
		q.CardFilter = cardFilterAll
	}
	if q.CreatedMode == "" {
		q.CreatedMode = dateModeAny
	}
	return q, nil
}

// BoardInfo is a lightweight board representation for the board dropdown.
type BoardInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc,omitempty"`
	ShortURL string `json:"shortUrl,omitempty"`
	Closed   bool   `json:"closed"`
}

// ListInfo is a lightweight list representation for the list dropdown.
type ListInfo struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Pos    float64 `json:"pos,omitempty"`
	Closed bool    `json:"closed"`
}

// MemberInfo is a lightweight member representation for the member multi-select.
type MemberInfo struct {
	ID        string `json:"id"`
	FullName  string `json:"fullName,omitempty"`
	Username  string `json:"username,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// LabelInfo is a lightweight label representation for the label multi-select.
type LabelInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}
