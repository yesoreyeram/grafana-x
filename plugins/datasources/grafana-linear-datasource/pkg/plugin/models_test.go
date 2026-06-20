package plugin

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_Defaults(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{})
	require.NoError(t, err)
	require.Equal(t, linearCloudURL, s.BaseURL)
	require.Equal(t, authAPIKey, s.AuthMethod)
}

func TestLoadSettings_ReadsSecrets(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"authMethod":"oauth"}`),
		DecryptedSecureJSONData: map[string]string{
			"apiKey":     "lin_api_x",
			"oauthToken": "tok",
		},
	})
	require.NoError(t, err)
	require.Equal(t, authOAuth, s.AuthMethod)

	token, bearer := s.credential()
	require.Equal(t, "tok", token)
	require.True(t, bearer)
}

func TestSettings_Credential_APIKey(t *testing.T) {
	s := Settings{AuthMethod: authAPIKey, apiKey: "lin_api_x"}
	token, bearer := s.credential()
	require.Equal(t, "lin_api_x", token)
	require.False(t, bearer)
}

func TestLoadSettings_URLFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{URL: "https://proxy.example/graphql"})
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example/graphql", s.BaseURL)
}
