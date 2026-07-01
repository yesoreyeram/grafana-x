package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_Defaults(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{})
	require.NoError(t, err)
	require.Equal(t, intercomCloudURL, s.BaseURL)
	require.Equal(t, defaultIntercomVersion, s.IntercomVersion)
}

func TestLoadSettings_RegionDerivesBaseURL(t *testing.T) {
	cases := map[string]string{
		regionUS: baseURLUnitedStates,
		regionEU: baseURLEurope,
		regionAU: baseURLAustralia,
	}
	for region, want := range cases {
		s, err := LoadSettings(backend.DataSourceInstanceSettings{
			JSONData: json.RawMessage(`{"region":"` + region + `"}`),
		})
		require.NoError(t, err)
		require.Equal(t, want, s.BaseURL, "region %s", region)
	}
}

func TestLoadSettings_ExplicitBaseURLWinsOverRegion(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: json.RawMessage(`{"region":"eu","baseURL":"https://proxy.example.com"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com", s.BaseURL)
}

func TestLoadSettings_ReadsToken(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		DecryptedSecureJSONData: map[string]string{"apiToken": "secret"},
	})
	require.NoError(t, err)
	require.Equal(t, "secret", s.apiToken)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, queryTypeConversations, q.QueryType)
	require.Equal(t, queryTypeConversations, q.CountOf)
}

func TestQueryModel_HasSearch(t *testing.T) {
	require.False(t, QueryModel{QueryType: queryTypeConversations}.hasSearch())
	require.True(t, QueryModel{StatusFilter: "open"}.hasSearch())
	require.True(t, QueryModel{AssigneeID: "1"}.hasSearch())
	require.True(t, QueryModel{Filters: []SearchFilter{{Field: "role", Operator: "=", Value: "user"}}}.hasSearch())
	require.False(t, QueryModel{Filters: []SearchFilter{{Field: "", Value: "x"}}}.hasSearch())
}

func TestBuildSearchQuery_SingleCondition(t *testing.T) {
	q := QueryModel{StatusFilter: "open"}
	got := BuildSearchQuery(q, queryTypeConversations)
	require.Equal(t, map[string]any{"field": "state", "operator": "=", "value": "open"}, got)
}

func TestBuildSearchQuery_MultipleWrappedWithAnd(t *testing.T) {
	q := QueryModel{StatusFilter: "open", TeamID: "55"}
	got := BuildSearchQuery(q, queryTypeConversations)
	require.Equal(t, "AND", got["operator"])
	values := got["value"].([]map[string]any)
	require.Len(t, values, 2)
	// team_assignee_id value is coerced to a number.
	require.Equal(t, int64(55), values[1]["value"])
}

func TestBuildSearchQuery_NilWhenEmpty(t *testing.T) {
	require.Nil(t, BuildSearchQuery(QueryModel{}, queryTypeConversations))
}

func TestBuildSearchQuery_SearchQueryUsesEntityField(t *testing.T) {
	got := BuildSearchQuery(QueryModel{SearchQuery: "acme"}, queryTypeContacts)
	require.Equal(t, "email", got["field"])
	require.Equal(t, "~", got["operator"])
	require.Equal(t, "acme", got["value"])
}

func TestCoerceValue(t *testing.T) {
	require.Equal(t, int64(10), coerceValue("10"))
	require.Equal(t, 1.5, coerceValue("1.5"))
	require.Equal(t, true, coerceValue("true"))
	require.Equal(t, "open", coerceValue("open"))
}

func TestBuildSort(t *testing.T) {
	require.Nil(t, buildSort(""))
	require.Equal(t, map[string]any{"field": "created_at", "order": "descending"}, buildSort("-created_at"))
	require.Equal(t, map[string]any{"field": "created_at", "order": "ascending"}, buildSort("created_at"))
}
