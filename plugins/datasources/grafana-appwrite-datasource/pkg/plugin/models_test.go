package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_DefaultsEndpoint(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"projectId":"proj"}`),
		DecryptedSecureJSONData: map[string]string{"apiKey": "key"},
	})
	require.NoError(t, err)
	require.Equal(t, appwriteDefaultURL, s.Endpoint)
	require.Equal(t, "proj", s.ProjectID)
	require.Equal(t, "key", s.apiKey)
}

func TestLoadSettings_CustomEndpointAndDatabase(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"endpoint":"https://nyc.cloud.appwrite.io/v1","projectId":"p","databaseId":"dbABC"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://nyc.cloud.appwrite.io/v1", s.Endpoint)
	require.Equal(t, "dbABC", s.DatabaseID)
}

func TestLoadSettings_URLFieldFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{}`),
		URL:      "https://from-url-field/v1",
	})
	require.NoError(t, err)
	require.Equal(t, "https://from-url-field/v1", s.Endpoint)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, "documents", q.QueryType)
}

func TestLoadQuery_ParsesFilterTreeAndSort(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"documents",
		"collectionId":"col1",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"attribute\":\"status\",\"op\":\"equal\",\"value\":\"active\"}]}",
		"sort":"[{\"attribute\":\"age\",\"direction\":\"desc\"}]"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, "group", q.filter.Kind)
	require.Len(t, q.filter.Children, 1)
	require.Equal(t, "status", q.filter.Children[0].Attribute)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "age", q.sortItems[0].Attribute)
	require.Equal(t, "desc", q.sortItems[0].Direction)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	raw := json.RawMessage(`{"collectionId":"col1","filterTree":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	raw := json.RawMessage(`{"collectionId":"col1","sort":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}
