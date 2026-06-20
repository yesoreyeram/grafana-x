package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_DefaultsURLAndAuthMode(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"identity":"admin@example.com"}`),
		DecryptedSecureJSONData: map[string]string{"password": "secret"},
	})
	require.NoError(t, err)
	require.Equal(t, pocketbaseDefaultURL, s.URL)
	require.Equal(t, AuthModeSuperuser, s.AuthMode)
	require.Equal(t, "admin@example.com", s.Identity)
	require.Equal(t, "secret", s.password)
	require.Equal(t, superusersCollection, s.effectiveAuthCollection())
}

func TestLoadSettings_UserModeDefaultsCollection(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"url":"https://pb.example.com","authMode":"user","identity":"a@b.c"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://pb.example.com", s.URL)
	require.Equal(t, AuthModeUser, s.AuthMode)
	require.Equal(t, defaultUserCollection, s.effectiveAuthCollection())
}

func TestLoadSettings_UserModeCustomCollection(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"authMode":"user","authCollection":"staff","identity":"a@b.c"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "staff", s.effectiveAuthCollection())
}

func TestLoadSettings_TokenMode(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"url":"https://pb.example.com","authMode":"token"}`),
		DecryptedSecureJSONData: map[string]string{"authToken": "tok"},
	})
	require.NoError(t, err)
	require.Equal(t, AuthModeToken, s.AuthMode)
	require.Equal(t, "tok", s.authToken)
	require.Equal(t, "", s.effectiveAuthCollection())
}

func TestLoadSettings_URLFieldFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{}`),
		URL:      "https://from-url-field",
	})
	require.NoError(t, err)
	require.Equal(t, "https://from-url-field", s.URL)
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, "records", q.QueryType)
}

func TestLoadQuery_ParsesFilterTreeAndSort(t *testing.T) {
	raw := json.RawMessage(`{
		"queryType":"records",
		"collectionId":"posts",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"attribute\":\"status\",\"op\":\"equal\",\"value\":\"active\"}]}",
		"sort":"[{\"attribute\":\"created\",\"direction\":\"desc\"}]"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, "group", q.filter.Kind)
	require.Len(t, q.filter.Children, 1)
	require.Equal(t, "status", q.filter.Children[0].Attribute)
	require.Len(t, q.sortItems, 1)
	require.Equal(t, "created", q.sortItems[0].Attribute)
	require.Equal(t, "desc", q.sortItems[0].Direction)
}

func TestLoadQuery_InvalidFilterTree(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"collectionId":"posts","filterTree":"not-json"}`))
	require.Error(t, err)
}

func TestLoadQuery_InvalidSort(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{"collectionId":"posts","sort":"not-json"}`))
	require.Error(t, err)
}
