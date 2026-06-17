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
		DecryptedSecureJSONData: map[string]string{"apiToken": "pat"},
	})
	require.NoError(t, err)
	require.Equal(t, airtableDefaultURL, s.BaseURL)
	require.Equal(t, "pat", s.apiToken)
}

func TestLoadSettings_CustomURLAndBase(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://proxy.example.com","baseId":"appABC"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com", s.BaseURL)
	require.Equal(t, "appABC", s.BaseID)
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
		"tableId":"tbl1",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"Plan\",\"op\":\"eq\",\"value\":\"pro\"}]}",
		"sort":"[{\"field\":\"Age\",\"direction\":\"desc\"}]"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, "group", q.filter.Kind)
	require.Len(t, q.filter.Children, 1)
	require.Equal(t, "Plan", q.filter.Children[0].Field)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "Age", q.sortItems[0].Field)
	require.Equal(t, "desc", q.sortItems[0].Direction)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	raw := json.RawMessage(`{"tableId":"tbl1","filterTree":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	raw := json.RawMessage(`{"tableId":"tbl1","sort":"not-json"}`)
	_, err := LoadQuery(raw)
	require.Error(t, err)
}
