package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// pocketbaseDefaultURL is the default PocketBase base URL (the local dev
// server). Self-hosted instances are configured by overriding this in the data
// source settings.
const pocketbaseDefaultURL = "http://127.0.0.1:8090"

// AuthMode selects how the backend authenticates to PocketBase.
type AuthMode string

const (
	// AuthModeSuperuser authenticates against the built-in `_superusers`
	// collection. Superusers can list collections and read every record
	// regardless of a collection's API rules.
	AuthModeSuperuser AuthMode = "superuser"
	// AuthModeUser authenticates against a regular auth collection (default
	// `users`). Access is then constrained by each collection's API rules.
	AuthModeUser AuthMode = "user"
	// AuthModeToken uses a pre-issued auth token verbatim (no password
	// exchange). Useful for impersonate / long-lived tokens.
	AuthModeToken AuthMode = "token"
)

// superusersCollection is the fixed PocketBase auth collection for superusers.
const superusersCollection = "_superusers"

// defaultUserCollection is the default auth collection name for user-mode auth.
const defaultUserCollection = "users"

// Settings represents the data source instance settings configured in the
// ConfigEditor. The secret fields (password, token) are kept out of the JSON
// struct and read from DecryptedSecureJSONData.
type Settings struct {
	// URL is the root URL of the PocketBase instance, e.g. http://127.0.0.1:8090
	// (no trailing /api).
	URL string `json:"url"`
	// AuthMode selects the authentication strategy (superuser|user|token).
	AuthMode AuthMode `json:"authMode"`
	// Identity is the superuser/user email (or username) used for password auth.
	// Unused for token auth.
	Identity string `json:"identity"`
	// AuthCollection is the auth collection used for user-mode auth (defaults to
	// `users`). Ignored for superuser mode (always `_superusers`) and token mode.
	AuthCollection string `json:"authCollection"`

	// password is the auth password (secret), used by superuser/user auth.
	password string
	// authToken is a pre-issued auth token (secret), used by token auth.
	authToken string
}

// effectiveAuthCollection returns the auth collection to authenticate against
// for the configured mode.
func (s Settings) effectiveAuthCollection() string {
	switch s.AuthMode {
	case AuthModeSuperuser:
		return superusersCollection
	case AuthModeUser:
		if c := strings.TrimSpace(s.AuthCollection); c != "" {
			return c
		}
		return defaultUserCollection
	default:
		return ""
	}
}

// LoadSettings parses the data source instance settings.
func LoadSettings(s backend.DataSourceInstanceSettings) (Settings, error) {
	settings := Settings{}
	if len(s.JSONData) > 0 {
		if err := json.Unmarshal(s.JSONData, &settings); err != nil {
			return settings, fmt.Errorf("invalid settings json: %w", err)
		}
	}

	// The URL may also be supplied through the standard data source URL field.
	if settings.URL == "" {
		settings.URL = s.URL
	}
	if settings.URL == "" {
		settings.URL = pocketbaseDefaultURL
	}

	if settings.AuthMode == "" {
		settings.AuthMode = AuthModeSuperuser
	}

	if s.DecryptedSecureJSONData != nil {
		settings.password = s.DecryptedSecureJSONData["password"]
		settings.authToken = s.DecryptedSecureJSONData["authToken"]
	}
	return settings, nil
}

// QueryModel represents the per-query payload sent from the QueryEditor.
type QueryModel struct {
	// QueryType: "records" (default) lists records from a collection;
	// "count" returns the number of matching records.
	QueryType string `json:"queryType"`
	// CollectionID is the PocketBase collection id or name.
	CollectionID string `json:"collectionId"`
	// FilterTree is the structured filter, serialized as a JSON string by the
	// query editor. When present, the backend compiles it into a PocketBase
	// filter expression.
	FilterTree string `json:"filterTree"`
	// filter is the parsed FilterTree (nil when absent/invalid).
	filter *FilterNode
	// RawFilter is an optional raw PocketBase filter expression (for example
	// `status = "active" && total > 10`). When set it takes precedence over the
	// structured FilterTree (advanced/escape hatch).
	RawFilter string `json:"rawFilter"`
	// Sort is the structured sort, serialized as a JSON string by the editor and
	// compiled into the PocketBase `sort` parameter.
	Sort string `json:"sort"`
	// sortItems is the parsed Sort (nil when absent/invalid).
	sortItems []SortItem
	// Fields is an optional comma separated list of field names to return
	// (compiled into the PocketBase `fields` parameter).
	Fields string `json:"fields"`
	// HideSystemFields, when true, drops the PocketBase system columns (id,
	// collectionId, collectionName, created, updated) from the result frame.
	HideSystemFields bool `json:"hideSystemFields"`
	// Limit caps the number of records returned. 0 means return all
	// (auto-paginated).
	Limit int `json:"limit"`
}

// LoadQuery parses the raw query JSON into a QueryModel and parses the structured
// filter and sort (the PocketBase filter/sort parameters are built later by the
// client).
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
