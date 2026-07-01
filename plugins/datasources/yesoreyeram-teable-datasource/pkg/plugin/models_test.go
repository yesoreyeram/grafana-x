package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_DefaultsURL(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "tok"},
	})
	require.NoError(t, err)
	require.Equal(t, teableCloudURL, s.BaseURL)
	require.Equal(t, "tok", s.apiToken)
}

func TestLoadSettings_CustomURLAndBase(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://teable.example.com","defaultBaseId":"bse123"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://teable.example.com", s.BaseURL)
	require.Equal(t, "bse123", s.DefaultBaseID)
}

func TestLoadSettings_URLFieldFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{}`),
		URL:      "https://selfhosted.teable.com",
	})
	require.NoError(t, err)
	require.Equal(t, "https://selfhosted.teable.com", s.BaseURL)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, "records", q.QueryType)
}

func TestLoadQuery_ParsesFilterTreeAndSort(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"records",
		"baseId":"bse123",
		"tableId":"tbl1",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"Name\",\"category\":\"text\",\"op\":\"is\",\"value\":\"Alice\"}]}",
		"sort":"[{\"field\":\"Age\",\"direction\":\"desc\"}]"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, "group", q.filter.Kind)
	require.Len(t, q.filter.Children, 1)
	require.Equal(t, "Name", q.filter.Children[0].Field)
	require.Equal(t, "text", q.filter.Children[0].Category)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "Age", q.sortItems[0].Field)
	require.Equal(t, "desc", q.sortItems[0].Direction)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"tableId":"tbl1","filterTree":"not-json"}`))
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"tableId":"tbl1","sort":"not-json"}`))
	require.Error(t, err)
}
