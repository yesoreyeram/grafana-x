package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_Defaults(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{})
	require.NoError(t, err)
	require.Equal(t, mondayCloudURL, s.BaseURL)
	require.Equal(t, authAPIKey, s.AuthMethod)
}

func TestLoadSettings_ReadsSecrets(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"authMethod":"oauth","apiVersion":"2024-10"}`),
		DecryptedSecureJSONData: map[string]string{
			"apiToken":   "tok_x",
			"oauthToken": "oauth_tok",
		},
	})
	require.NoError(t, err)
	require.Equal(t, authOAuth, s.AuthMethod)
	require.Equal(t, "2024-10", s.APIVersion)

	token, bearer := s.credential()
	require.Equal(t, "oauth_tok", token)
	require.True(t, bearer)
}

func TestSettings_Credential_APIKey(t *testing.T) {
	s := Settings{AuthMethod: authAPIKey, apiToken: "tok_x"}
	token, bearer := s.credential()
	require.Equal(t, "tok_x", token)
	require.False(t, bearer)
}

func TestLoadSettings_URLFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{URL: "https://proxy.example/v2"})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example/v2", s.BaseURL)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, queryTypeItems, q.QueryType)
	require.Equal(t, stateActive, q.State)
	require.True(t, q.includeColumns())

	q, err = LoadQuery(json.RawMessage(`{"queryType":"boards","limit":5}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeBoards, q.QueryType)
	require.Equal(t, 5, q.Limit)
	require.Equal(t, stateActive, q.State)
}

func TestLoadQuery_GroupingDefaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{"queryType":"items","groupBy":"status"}`))
	require.NoError(t, err)
	require.True(t, q.isGrouped())
	require.Equal(t, aggCount, q.Aggregation) // default aggregation

	q, err = LoadQuery(json.RawMessage(`{"queryType":"items","groupBy":"status","aggregation":"sum","aggregationColumn":"points"}`))
	require.NoError(t, err)
	require.Equal(t, aggSum, q.Aggregation)
	require.Equal(t, "points", q.AggregationColumn)

	q, err = LoadQuery(json.RawMessage(`{"queryType":"items"}`))
	require.NoError(t, err)
	require.False(t, q.isGrouped())
	require.Empty(t, q.Aggregation)
}

func TestQueryModel_IncludeColumns(t *testing.T) {
	on := true
	off := false
	require.True(t, QueryModel{}.includeColumns())
	require.True(t, QueryModel{IncludeColumnValues: &on}.includeColumns())
	require.False(t, QueryModel{IncludeColumnValues: &off}.includeColumns())
}

func TestValidState(t *testing.T) {
	require.True(t, validState(stateActive))
	require.True(t, validState(stateAll))
	require.True(t, validState(stateArchived))
	require.True(t, validState(stateDeleted))
	require.False(t, validState("nonsense"))
}
