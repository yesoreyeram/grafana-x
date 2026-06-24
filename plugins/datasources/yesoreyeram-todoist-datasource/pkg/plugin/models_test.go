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

	q, err = LoadQuery(json.RawMessage(`{"queryType":"count","projectId":"p1","label":"urgent","limit":5}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeCount, q.QueryType)
	require.Equal(t, "p1", q.ProjectId)
	require.Equal(t, "urgent", q.Label)
	require.Equal(t, 5, q.Limit)
}

func TestLoadQuery_Invalid(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{not json`))
	require.Error(t, err)
}

func TestLoadSettings_DefaultsAndSecret(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "abc123"},
	})
	require.NoError(t, err)
	require.Equal(t, todoistCloudURL, s.BaseURL)
	require.Equal(t, "https://api.todoist.com/api/v1", s.BaseURL)
	require.Equal(t, "abc123", s.credential())
}

func TestLoadSettings_RespectsCustomBaseURL(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://proxy.example.com/api/v1"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com/api/v1", s.BaseURL)
}

func TestSettings_EmptySecret(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{})
	require.NoError(t, err)
	require.Empty(t, s.credential())
}
