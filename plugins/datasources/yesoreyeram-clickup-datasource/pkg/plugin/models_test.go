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
	require.Equal(t, dateModeAny, q.CreatedMode)
	require.Equal(t, dateModeAny, q.UpdatedMode)
	require.Equal(t, dateModeAny, q.DueMode)

	q, err = LoadQuery(json.RawMessage(`{"queryType":"spaces","teamId":"9","limit":5}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeSpaces, q.QueryType)
	require.Equal(t, "9", q.TeamId)
	require.Equal(t, 5, q.Limit)
}

func TestLoadQuery_Invalid(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{not json`))
	require.Error(t, err)
}

func TestLoadSettings_DefaultsAndSecrets(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiKey": "pk_1"},
	})
	require.NoError(t, err)
	require.Equal(t, clickUpCloudURL, s.BaseURL)
	require.Equal(t, authAPIKey, s.AuthMethod)

	token, bearer := s.credential()
	require.Equal(t, "pk_1", token)
	require.False(t, bearer)
}

func TestSettings_Credential_OAuth(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"authMethod":"oauth"}`),
		DecryptedSecureJSONData: map[string]string{"oauthToken": "tok"},
	})
	require.NoError(t, err)
	token, bearer := s.credential()
	require.Equal(t, "tok", token)
	require.True(t, bearer)
}

func TestSettings_URLFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		URL:      "https://proxy.example/api",
		JSONData: []byte(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example/api", s.BaseURL)
}

func TestTaskFieldNames_SortedAndContainsKnown(t *testing.T) {
	names := TaskFieldNames()
	require.Contains(t, names, "name")
	require.Contains(t, names, "status")
	require.Contains(t, names, "assignees")
	for i := 1; i < len(names); i++ {
		require.LessOrEqual(t, names[i-1], names[i])
	}
}
