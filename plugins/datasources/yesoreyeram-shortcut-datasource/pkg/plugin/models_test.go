package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_Token(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		DecryptedSecureJSONData: map[string]string{"apiToken": "sc-api-token"},
	})
	require.NoError(t, err)
	require.Equal(t, "sc-api-token", s.apiToken)
	require.Equal(t, shortcutCloudURL, s.BaseURL)
}

func TestLoadSettings_Defaults(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{})
	require.NoError(t, err)
	require.Empty(t, s.apiToken)
	require.Equal(t, shortcutCloudURL, s.BaseURL)
}

func TestLoadSettings_BaseURLFromJSONData(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://proxy.example.com"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com", s.BaseURL)
}

func TestLoadSettings_BaseURLFromURLField(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		URL:      "https://proxy.internal",
		JSONData: []byte(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.internal", s.BaseURL)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, queryTypeStories, q.QueryType)
	require.Equal(t, dateModeAny, q.DateMode)
	require.Equal(t, dateFieldCreated, q.DateField)
	require.Equal(t, archivedAny, q.Archived)
	require.Equal(t, detailFull, q.Detail)

	q, err = LoadQuery(json.RawMessage(`{"queryType":"count","limit":10}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeCount, q.QueryType)
	require.Equal(t, 10, q.Limit)
}

func TestLoadQuery_Invalid(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{not json`))
	require.Error(t, err)
}

func TestStoryFieldNames_SortedAndContainsKnown(t *testing.T) {
	names := StoryFieldNames()
	require.Contains(t, names, "id")
	require.Contains(t, names, "name")
	require.Contains(t, names, "owner_ids")
	require.Contains(t, names, "workflow_state_id")
	require.Contains(t, names, "created_at")
	for i := 1; i < len(names); i++ {
		require.LessOrEqual(t, names[i-1], names[i])
	}
}

func TestStoryTypeOptions(t *testing.T) {
	require.Equal(t, []string{"feature", "bug", "chore"}, StoryTypeOptions())
}

func TestEffectiveFields(t *testing.T) {
	// Empty selection falls back to the full catalog.
	require.Equal(t, storyFieldNames, effectiveFields(nil))
	require.Equal(t, storyFieldNames, effectiveFields([]string{"  "}))
	// A non-empty selection is used as-is (trimmed).
	require.Equal(t, []string{"id", "name"}, effectiveFields([]string{"id", " name ", ""}))
}
