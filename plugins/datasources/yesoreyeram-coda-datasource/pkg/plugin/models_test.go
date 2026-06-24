package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_Token(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "tok"},
	})
	require.NoError(t, err)
	require.Equal(t, "tok", s.apiToken)
	// Base URL defaults to the Coda API base path.
	require.Equal(t, codaDefaultURL, s.BaseURL)
}

func TestLoadSettings_CustomDoc(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"docId":"docABC"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "docABC", s.DocID)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, QueryTypeRows, q.QueryType)
	require.Equal(t, defaultValueFormat, q.ValueFormat)
}

func TestLoadQuery_EmptyRaw(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, QueryTypeRows, q.QueryType)
	require.Equal(t, defaultValueFormat, q.ValueFormat)
}

func TestLoadQuery_ParsesFields(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"count",
		"docId":"docABC",
		"tableId":"tbl1",
		"filterColumn":"Plan",
		"filterValue":"pro",
		"sortBy":"natural",
		"visibleOnly":true,
		"valueFormat":"rich",
		"limit":50
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.Equal(t, QueryTypeCount, q.QueryType)
	require.Equal(t, "docABC", q.DocID)
	require.Equal(t, "tbl1", q.TableID)
	require.Equal(t, "Plan", q.FilterColumn)
	require.Equal(t, "pro", q.FilterValue)
	require.Equal(t, "natural", q.SortBy)
	require.True(t, q.VisibleOnly)
	require.Equal(t, "rich", q.ValueFormat)
	require.Equal(t, 50, q.Limit)
}

func TestLoadQuery_Invalid(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{not json`))
	require.Error(t, err)
}
