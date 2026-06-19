package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, queryTypeTasks, q.QueryType)
	require.Equal(t, dateModeAny, q.ModifiedMode)

	q, err = LoadQuery(json.RawMessage(`{"queryType":"projects","workspace":"9","limit":5}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeProjects, q.QueryType)
	require.Equal(t, "9", q.Workspace)
	require.Equal(t, 5, q.Limit)
}

func TestLoadQuery_Invalid(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{not json`))
	require.Error(t, err)
}

func TestLoadSettings_DefaultsAndSecret(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiKey": "pat_1"},
	})
	require.NoError(t, err)
	require.Equal(t, asanaCloudURL, s.BaseURL)
	require.Equal(t, "pat_1", s.credential())
}

func TestSettings_URLFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		URL:      "https://proxy.example/api/1.0",
		JSONData: []byte(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example/api/1.0", s.BaseURL)
}

func TestTaskFieldNames_SortedAndContainsKnown(t *testing.T) {
	names := TaskFieldNames()
	require.Contains(t, names, "name")
	require.Contains(t, names, "assignee")
	require.Contains(t, names, "due_on")
	for i := 1; i < len(names); i++ {
		require.LessOrEqual(t, names[i-1], names[i])
	}
}

func TestTaskOptFields(t *testing.T) {
	// Empty selection requests the full default catalog (with nested .name paths).
	all := taskOptFields(nil)
	require.Contains(t, all, "assignee.name")
	require.Contains(t, all, "projects.name")
	require.Contains(t, all, "tags.name")
	require.Contains(t, all, "gid")
	// Custom field value sub-fields are requested by default so values come back.
	require.Contains(t, all, "custom_fields.display_value")
	require.Contains(t, all, "custom_fields.number_value")

	// Explicit selection maps friendly names to opt_fields paths.
	require.Equal(t, "name,assignee.name", taskOptFields([]string{"name", "assignee"}))

	// "custom_fields" expands to the full set of value sub-fields.
	require.Equal(t, taskCustomFieldOptFields, taskOptFields([]string{"custom_fields"}))

	// Unknown names pass through unchanged (custom paths allowed).
	require.Equal(t, "custom_fields.name", taskOptFields([]string{"custom_fields.name"}))
}
