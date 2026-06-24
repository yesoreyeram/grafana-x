package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, queryTypeCards, q.QueryType)
	require.Equal(t, cardFilterAll, q.CardFilter)
	require.Equal(t, dateModeAny, q.CreatedMode)
	// No implicit limit: 0 means "all" (auto-paginated).
	require.Equal(t, 0, q.Limit)

	q, err = LoadQuery(json.RawMessage(`{"queryType":"count","boardId":"b1","limit":5,"createdMode":"custom","createdAfter":"2024-01-01"}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeCount, q.QueryType)
	require.Equal(t, "b1", q.BoardId)
	require.Equal(t, 5, q.Limit)
	require.Equal(t, dateModeCustom, q.CreatedMode)
	require.Equal(t, "2024-01-01", q.CreatedAfter)
}

func TestLoadQuery_Invalid(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{not json`))
	require.Error(t, err)
}

func TestLoadSettings_Secrets(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		DecryptedSecureJSONData: map[string]string{"apiKey": "trello-key", "apiToken": "trello-token"},
	})
	require.NoError(t, err)
	require.Equal(t, "trello-key", s.apiKey)
	require.Equal(t, "trello-token", s.apiToken)
}

func TestLoadSettings_EmptySecrets(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{})
	require.NoError(t, err)
	require.Empty(t, s.apiKey)
	require.Empty(t, s.apiToken)
}

func TestCardFieldNames_SortedAndContains(t *testing.T) {
	names := CardFieldNames()
	require.Contains(t, names, "id")
	require.Contains(t, names, "name")
	require.Contains(t, names, "due")
	require.Contains(t, names, "labels")
	for i := 1; i < len(names); i++ {
		require.LessOrEqual(t, names[i-1], names[i])
	}
}
