package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_RequiresURL(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "tok"},
	})
	require.NoError(t, err)
	require.Equal(t, "", s.BaseURL)
	require.Equal(t, "tok", s.apiToken)
}

func TestLoadSettings_CustomURL(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://directus.example.com","defaultCollectionId":"articles"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://directus.example.com", s.BaseURL)
	require.Equal(t, "articles", s.DefaultCollectionID)
}

func TestLoadSettings_URLFieldFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{}`),
		URL:      "https://from-url-field",
	})
	require.NoError(t, err)
	require.Equal(t, "https://from-url-field", s.BaseURL)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, "records", q.QueryType)
}

func TestLoadQuery_ParsesFilterTreeAndSort(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"records",
		"collectionId":"articles",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"status\",\"op\":\"eq\",\"value\":\"published\"}]}",
		"sort":"[{\"field\":\"views\",\"direction\":\"desc\"}]"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, "group", q.filter.Kind)
	require.Len(t, q.filter.Children, 1)
	require.Equal(t, "status", q.filter.Children[0].Field)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "views", q.sortItems[0].Field)
	require.Equal(t, "desc", q.sortItems[0].Direction)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	raw := json.RawMessage(`{"collectionId":"articles","filterTree":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	raw := json.RawMessage(`{"collectionId":"articles","sort":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}
