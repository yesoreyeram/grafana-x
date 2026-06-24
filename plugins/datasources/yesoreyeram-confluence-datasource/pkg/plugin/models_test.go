package plugin

import (
	"encoding/base64"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_BasicAuth(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://acme.atlassian.net/wiki/","authMode":"basic","email":"a@b.com"}`),
		DecryptedSecureJSONData: map[string]string{
			"apiToken": "tok",
		},
	})
	require.NoError(t, err)
	// Trailing slash is trimmed.
	require.Equal(t, "https://acme.atlassian.net/wiki", s.BaseURL)
	require.Equal(t, authBasic, s.AuthMode)
	require.Equal(t, "a@b.com", s.Email)
	require.True(t, s.hasCredential())

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("a@b.com:tok"))
	require.Equal(t, want, s.authHeader())
}

func TestLoadSettings_BearerAuth(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://confluence.example.com","authMode":"bearer"}`),
		DecryptedSecureJSONData: map[string]string{
			"bearerToken": "pat-123",
		},
	})
	require.NoError(t, err)
	require.Equal(t, authBearer, s.AuthMode)
	require.True(t, s.hasCredential())
	require.Equal(t, "Bearer pat-123", s.authHeader())
}

func TestLoadSettings_DefaultsToBasic(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"baseURL":"https://acme.atlassian.net/wiki"}`),
	})
	require.NoError(t, err)
	require.Equal(t, authBasic, s.AuthMode)
	// No credential configured.
	require.False(t, s.hasCredential())
	require.Equal(t, "", s.authHeader())
}

func TestLoadSettings_BasicMissingEmailHasNoCredential(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"baseURL":"https://acme.atlassian.net/wiki","authMode":"basic"}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "tok"},
	})
	require.NoError(t, err)
	require.False(t, s.hasCredential())
}

func TestLoadSettings_FallsBackToURLField(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		URL: "https://acme.atlassian.net/wiki",
	})
	require.NoError(t, err)
	require.Equal(t, "https://acme.atlassian.net/wiki", s.BaseURL)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery([]byte(`{}`))
	require.NoError(t, err)
	require.Equal(t, queryTypePages, q.QueryType)

	q, err = LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, queryTypePages, q.QueryType)
}

func TestLoadQuery_ParsesFields(t *testing.T) {
	q, err := LoadQuery([]byte(`{"queryType":"search","cql":"type=page","spaceId":"1","fields":"id,title","limit":10}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeSearch, q.QueryType)
	require.Equal(t, "type=page", q.CQL)
	require.Equal(t, "1", q.SpaceID)
	require.Equal(t, "id,title", q.Fields)
	require.Equal(t, 10, q.Limit)
}

func TestSplitFields(t *testing.T) {
	require.Equal(t, []string{"id", "title"}, splitFields(" id , title ,"))
	require.Empty(t, splitFields(""))
}

func TestListableQueryType(t *testing.T) {
	require.True(t, listableQueryType(queryTypePages))
	require.True(t, listableQueryType(queryTypeBlogposts))
	require.True(t, listableQueryType(queryTypeSearch))
	require.True(t, listableQueryType(""))
	require.False(t, listableQueryType(queryTypeCount))
	require.False(t, listableQueryType("nope"))
}

func TestBuildCQL(t *testing.T) {
	require.Equal(t, "type=page", BuildCQL("  type=page  "))
	require.Equal(t, "", BuildCQL("   "))
}

func TestEscapeCQLValue(t *testing.T) {
	require.Equal(t, `a\"b`, EscapeCQLValue(`a"b`))
	require.Equal(t, `a\\b`, EscapeCQLValue(`a\b`))
}

func TestSpaceCQL(t *testing.T) {
	require.Equal(t, `space = "ENG"`, SpaceCQL("ENG"))
	require.Equal(t, "", SpaceCQL("  "))
}

func TestRequestLimit(t *testing.T) {
	require.Equal(t, defaultPageSize, requestLimit(0))
	require.Equal(t, 5, requestLimit(5))
	// The per-request page size never exceeds defaultPageSize (which is itself
	// below the API max of maxLimit).
	require.Equal(t, defaultPageSize, requestLimit(1000))
}
