package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_DefaultsToAttioCloud(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "tok"},
	})
	require.NoError(t, err)
	require.Equal(t, attioCloudURL, s.BaseURL)
	require.Equal(t, "tok", s.apiToken)
}

func TestLoadSettings_CustomURL(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://proxy.example.com","defaultObjectId":"people"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com", s.BaseURL)
	require.Equal(t, "people", s.DefaultObjectID)
}

func TestLoadSettings_URLFieldFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{}`),
		URL:      "https://from-url-field",
	})
	require.NoError(t, err)
	require.Equal(t, "https://from-url-field", s.BaseURL)
}

func TestLoadSettings_InvalidJSON(t *testing.T) {
	_, err := LoadSettings(backend.DataSourceInstanceSettings{JSONData: []byte(`not-json`)})
	require.Error(t, err)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, QueryTypeRecords, q.QueryType)
}

func TestLoadQuery_Empty(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, "", q.QueryType)
}

func TestLoadQuery_ParsesFilterTreeAndSort(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"records",
		"objectId":"deals",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"stage\",\"op\":\"eq\",\"value\":\"Won\"}]}",
		"sort":"[{\"field\":\"name\",\"direction\":\"desc\"}]"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, "group", q.filter.Kind)
	require.Len(t, q.filter.Children, 1)
	require.Equal(t, "stage", q.filter.Children[0].Field)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "name", q.sortItems[0].Field)
	require.Equal(t, "desc", q.sortItems[0].Direction)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	raw := json.RawMessage(`{"objectId":"people","filterTree":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	raw := json.RawMessage(`{"objectId":"people","sort":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}
