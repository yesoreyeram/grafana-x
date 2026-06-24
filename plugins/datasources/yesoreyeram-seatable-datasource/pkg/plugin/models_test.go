package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_DefaultsToCloud(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "tok"},
	})
	require.NoError(t, err)
	require.Equal(t, seatableCloudURL, s.ServerURL)
	require.Equal(t, "tok", s.apiToken)
}

func TestLoadSettings_ServerURL(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"serverURL":"https://seatable.example.com"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://seatable.example.com", s.ServerURL)
}

func TestLoadSettings_LegacyBaseURLAndToken(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"baseURL":"https://legacy.seatable.io"}`),
		DecryptedSecureJSONData: map[string]string{"baseToken": "legacy-tok"},
	})
	require.NoError(t, err)
	require.Equal(t, "https://legacy.seatable.io", s.ServerURL)
	require.Equal(t, "legacy-tok", s.apiToken)
}

func TestLoadSettings_URLFieldFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{}`),
		URL:      "https://from-url-field",
	})
	require.NoError(t, err)
	require.Equal(t, "https://from-url-field", s.ServerURL)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, QueryTypeRecords, q.QueryType)
	require.False(t, q.requiresSQL())
}

func TestLoadQuery_ParsesFilterTreeAndSort(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"records",
		"tableName":"Table1",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"Plan\",\"op\":\"eq\",\"value\":\"pro\"}]}",
		"sort":"[{\"field\":\"Age\",\"direction\":\"desc\"}]"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, "Plan", q.filter.Children[0].Field)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "Age", q.sortItems[0].Field)
	require.True(t, q.requiresSQL())
}

func TestLoadQuery_FieldsForceSQL(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{"tableName":"T","fields":"Name,Age"}`))
	require.NoError(t, err)
	require.True(t, q.requiresSQL())
}

func TestLoadQuery_SQLType(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{"queryType":"sql","sql":"SELECT * FROM T"}`))
	require.NoError(t, err)
	require.Equal(t, QueryTypeSQL, q.QueryType)
	require.Equal(t, "SELECT * FROM T", q.SQL)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"tableName":"T","filterTree":"not-json"}`))
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"tableName":"T","sort":"not-json"}`))
	require.Error(t, err)
}
