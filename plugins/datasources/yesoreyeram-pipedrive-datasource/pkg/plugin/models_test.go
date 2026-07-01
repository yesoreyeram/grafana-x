package plugin

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings(t *testing.T) {
	t.Run("api token mode", func(t *testing.T) {
		s := backend.DataSourceInstanceSettings{
			JSONData:                []byte(`{"companyDomain":"testcompany"}`),
			DecryptedSecureJSONData: map[string]string{"apiToken": "tok"},
		}
		settings, err := LoadSettings(s)
		require.NoError(t, err)
		require.Equal(t, "testcompany", settings.CompanyDomain)
		require.Equal(t, authAPIToken, settings.AuthMethod)
		require.Equal(t, "tok", settings.credential())
		require.Equal(t, authAPIToken, settings.authMode())
	})

	t.Run("oauth mode", func(t *testing.T) {
		s := backend.DataSourceInstanceSettings{
			JSONData:                []byte(`{"companyDomain":"testcompany","authMethod":"oauth"}`),
			DecryptedSecureJSONData: map[string]string{"oauthToken": "bearer-tok"},
		}
		settings, err := LoadSettings(s)
		require.NoError(t, err)
		require.Equal(t, authOAuth, settings.AuthMethod)
		require.Equal(t, "bearer-tok", settings.credential())
		require.Equal(t, authOAuth, settings.authMode())
	})

	t.Run("trims domain and tokens", func(t *testing.T) {
		s := backend.DataSourceInstanceSettings{
			JSONData:                []byte(`{"companyDomain":"  acme  "}`),
			DecryptedSecureJSONData: map[string]string{"apiToken": "  tok  "},
		}
		settings, err := LoadSettings(s)
		require.NoError(t, err)
		require.Equal(t, "acme", settings.CompanyDomain)
		require.Equal(t, "tok", settings.apiToken)
	})

	t.Run("empty settings", func(t *testing.T) {
		settings, err := LoadSettings(backend.DataSourceInstanceSettings{})
		require.NoError(t, err)
		require.Empty(t, settings.CompanyDomain)
		require.Empty(t, settings.credential())
	})
}

func TestNewClient(t *testing.T) {
	t.Run("builds base URL from domain", func(t *testing.T) {
		c, err := NewClient(Settings{CompanyDomain: "acme", AuthMethod: authAPIToken, apiToken: "tok"}, nil)
		require.NoError(t, err)
		require.Equal(t, "https://acme.pipedrive.com/api/v1", c.baseURL)
		require.Equal(t, authAPIToken, c.authMethod)
		require.Equal(t, "tok", c.token)
	})

	t.Run("missing domain does not error (deferred to CheckHealth)", func(t *testing.T) {
		c, err := NewClient(Settings{AuthMethod: authAPIToken, apiToken: "tok"}, nil)
		require.NoError(t, err)
		require.Empty(t, c.baseURL)
	})

	t.Run("oauth mode resolves bearer credential", func(t *testing.T) {
		c, err := NewClient(Settings{CompanyDomain: "acme", AuthMethod: authOAuth, oauthToken: "bearer"}, nil)
		require.NoError(t, err)
		require.Equal(t, authOAuth, c.authMethod)
		require.Equal(t, "bearer", c.token)
	})
}

func TestLoadQuery(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		q, err := LoadQuery(nil)
		require.NoError(t, err)
		require.Equal(t, "deals", q.QueryType)
		require.Equal(t, "all", q.Status)
		require.Equal(t, "DESC", q.SortDir)
		require.Equal(t, "deals", q.CountEntity)
		require.True(t, q.shouldMapCustomFields())
	})

	t.Run("parses custom query", func(t *testing.T) {
		raw := json.RawMessage(`{"queryType":"persons","limit":50,"start":10,"filterId":"7","mapCustomFields":false}`)
		q, err := LoadQuery(raw)
		require.NoError(t, err)
		require.Equal(t, "persons", q.QueryType)
		require.Equal(t, 50, q.Limit)
		require.Equal(t, 10, q.Start)
		require.Equal(t, "7", q.FilterId)
		require.False(t, q.shouldMapCustomFields())
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := LoadQuery(json.RawMessage(`{bad`))
		require.Error(t, err)
	})
}
