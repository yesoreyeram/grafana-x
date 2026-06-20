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
	require.Equal(t, queryTypeWorkItems, q.QueryType)
	require.Equal(t, dateModeAny, q.CreatedMode)
	require.Equal(t, dateModeAny, q.UpdatedMode)

	q, err = LoadQuery(json.RawMessage(`{"queryType":"projects","workspaceSlug":"w","limit":5}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeProjects, q.QueryType)
	require.Equal(t, "w", q.WorkspaceSlug)
	require.Equal(t, 5, q.Limit)
}

func TestLoadQuery_Invalid(t *testing.T) {
	_, err := LoadQuery(json.RawMessage(`{not json`))
	require.Error(t, err)
}

func TestLoadSettings_DefaultsAndSecrets(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiKey": "plane_1"},
	})
	require.NoError(t, err)
	require.Equal(t, planeCloudURL, s.BaseURL)
	require.Equal(t, authAPIKey, s.AuthMethod)

	token, bearer := s.credential()
	require.Equal(t, "plane_1", token)
	require.False(t, bearer)
}

func TestLoadSettings_WorkspaceSlug(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"workspaceSlug":"my-team"}`),
		DecryptedSecureJSONData: map[string]string{"apiKey": "x"},
	})
	require.NoError(t, err)
	require.Equal(t, "my-team", s.WorkspaceSlug)
}

func TestSettings_Credential_OAuth(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"authMethod":"oauth"}`),
		DecryptedSecureJSONData: map[string]string{"oauthToken": "tok"},
	})
	require.NoError(t, err)
	token, bearer := s.credential()
	require.Equal(t, "tok", token)
	require.True(t, bearer)
}

func TestSettings_URLFallback(t *testing.T) {
	s, err := LoadSettings(backend.DataSourceInstanceSettings{
		URL:      "https://plane.internal/api-root",
		JSONData: []byte(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, "https://plane.internal/api-root", s.BaseURL)
}

func TestResolveWorkspace(t *testing.T) {
	require.Equal(t, "q", QueryModel{WorkspaceSlug: "q"}.resolveWorkspace("d"))
	require.Equal(t, "d", QueryModel{WorkspaceSlug: ""}.resolveWorkspace("d"))
	require.Equal(t, "d", QueryModel{WorkspaceSlug: "  "}.resolveWorkspace("d"))
	require.Equal(t, "", QueryModel{}.resolveWorkspace(""))
}

func TestWorkItemFieldNames_SortedAndContainsKnown(t *testing.T) {
	names := WorkItemFieldNames()
	require.Contains(t, names, "name")
	require.Contains(t, names, "state")
	require.Contains(t, names, "assignees")
	require.Contains(t, names, "priority")
	for i := 1; i < len(names); i++ {
		require.LessOrEqual(t, names[i-1], names[i])
	}
}

func TestPriorityOptions(t *testing.T) {
	require.Equal(t, []string{"urgent", "high", "medium", "low", "none"}, PriorityOptions())
}
