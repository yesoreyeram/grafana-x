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
		DecryptedSecureJSONData: map[string]string{"apiKey": "key"},
	})
	require.NoError(t, err)
	require.Equal(t, gristDefaultURL, s.BaseURL)
	require.Equal(t, "key", s.apiKey)
}

func TestLoadSettings_CustomURLAndDoc(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://team.getgrist.com","docId":"docABC"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://team.getgrist.com", s.BaseURL)
	require.Equal(t, "docABC", s.DocID)
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
	require.Equal(t, QueryTypeRecords, q.QueryType)
}

func TestLoadQuery_ParsesFilterTreeAndSort(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"records",
		"tableId":"t1",
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

func TestLoadQuery_ParsesSQL(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{"queryType":"sql","sql":"SELECT * FROM Users"}`))
	require.NoError(t, err)
	require.Equal(t, QueryTypeSQL, q.QueryType)
	require.Equal(t, "SELECT * FROM Users", q.SQL)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"tableId":"t1","filterTree":"not-json"}`))
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"tableId":"t1","sort":"not-json"}`))
	require.Error(t, err)
}

func TestRequiresSQL(t *testing.T) {
	// No filter, no fields -> records endpoint.
	require.False(t, QueryModel{}.requiresSQL())

	// Fields projection -> SQL.
	require.True(t, QueryModel{Fields: "Name"}.requiresSQL())

	// Simple eq filter -> records endpoint.
	simple := QueryModel{filter: groupRef("and", cond("Plan", "eq", "pro"))}
	require.False(t, simple.requiresSQL())

	// Rich operator -> SQL.
	rich := QueryModel{filter: groupRef("and", cond("Age", "gt", "30"))}
	require.True(t, rich.requiresSQL())

	// Sort alone does NOT force SQL (records endpoint supports sort).
	sorted := QueryModel{sortItems: []SortItem{{Field: "Age", Direction: "desc"}}}
	require.False(t, sorted.requiresSQL())
}
