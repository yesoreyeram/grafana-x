package plugin

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_CloudForcesBaserowURL(t *testing.T) {
	// Cloud platform must always resolve to api.baserow.io, ignoring any
	// configured base URL.
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"platform":"cloud","baseURL":"https://ignored.example.com"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.baserow.io", s.BaseURL)
	require.Equal(t, AuthToken, s.AuthMode) // defaults to token
}

func TestLoadSettings_SelfHostedKeepsBaseURL(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"platform":"selfhosted","baseURL":"https://baserow.example.com"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://baserow.example.com", s.BaseURL)
}

func TestLoadSettings_FallsBackToDatasourceURL(t *testing.T) {
	// When no baseURL is set in jsonData, the standard datasource URL is used.
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		URL:      "https://from-url.example.com",
		JSONData: []byte(`{"platform":"selfhosted"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://from-url.example.com", s.BaseURL)
}

func TestLoadSettings_PasswordModeSecrets(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"platform":"cloud","authMode":"password","email":"a@b.com"}`),
		DecryptedSecureJSONData: map[string]string{
			"password": "pw",
			"apiToken": "tok",
		},
	})
	require.NoError(t, err)
	require.Equal(t, AuthPassword, s.AuthMode)
	require.Equal(t, "a@b.com", s.Email)
	require.Equal(t, "pw", s.password)
	require.Equal(t, "tok", s.apiToken)
}
