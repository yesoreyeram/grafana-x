package plugin

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings(t *testing.T) {
	settings, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"apiUrl":"https://test.supabase.co/rest/v1"}`),
		DecryptedSecureJSONData: map[string]string{
			"serviceKey": "test-key-123",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://test.supabase.co/rest/v1", settings.APIURL)
	require.Equal(t, "test-key-123", settings.apiKey)
	require.Empty(t, settings.Schema)
}

func TestLoadSettings_WithSchema(t *testing.T) {
	settings, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"apiUrl":"https://test.supabase.co/rest/v1","schema":"analytics"}`),
		DecryptedSecureJSONData: map[string]string{
			"serviceKey": "k",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "analytics", settings.Schema)
}

func TestLoadSettings_EmptyJSON(t *testing.T) {
	settings, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: nil,
	})
	require.NoError(t, err)
	require.Empty(t, settings.APIURL)
}

func TestLoadSettings_FallsBackToURL(t *testing.T) {
	settings, err := LoadSettings(backend.DataSourceInstanceSettings{
		URL: "https://fallback.supabase.co/rest/v1",
	})
	require.NoError(t, err)
	require.Equal(t, "https://fallback.supabase.co/rest/v1", settings.APIURL)
}

func TestLoadQuery(t *testing.T) {
	q, err := LoadQuery([]byte(`{"tableId":"users","queryType":"records","select":"id,name","limit":100,"offset":0}`))
	require.NoError(t, err)
	require.Equal(t, "users", q.TableID)
	require.Equal(t, "records", q.QueryType)
	require.Equal(t, "id,name", q.Select)
	require.Equal(t, 100, q.Limit)
	require.Equal(t, 0, q.Offset)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, "records", q.QueryType)
}

func TestLoadQuery_WithFilterTree(t *testing.T) {
	q, err := LoadQuery([]byte(`{
		"tableId":"users",
		"filterTree": "{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"age\",\"op\":\"gt\",\"value\":\"18\"}]}"
	}`))
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Len(t, q.filter.Children, 1)
	require.Equal(t, "age", q.filter.Children[0].Field)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	_, err := LoadQuery([]byte(`{"filterTree": "not-json"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid filterTree")
}

func TestLoadQuery_WithSort(t *testing.T) {
	q, err := LoadQuery([]byte(`{
		"tableId":"users",
		"sort": "[{\"field\":\"name\",\"direction\":\"desc\"}]"
	}`))
	require.NoError(t, err)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "name", q.sortItems[0].Field)
	require.Equal(t, "desc", q.sortItems[0].Direction)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	_, err := LoadQuery([]byte(`{"sort": "not-json"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid sort")
}
