package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

// newTestClient returns a Client pointed at the given test server URL.
func newTestClient(t *testing.T, url string, settings Settings) *Client {
	t.Helper()
	settings.BaseURL = url
	c, err := NewClient(settings, http.DefaultClient)
	require.NoError(t, err)
	return c
}

func TestNewClient_DefaultsToCloudURL(t *testing.T) {
	c, err := NewClient(Settings{}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, asanaCloudURL, c.baseURL)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://example.com/api/1.0/"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/api/1.0", c.baseURL)
}

func TestClient_AuthHeaderIsBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":{"gid":"1","name":"me"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "pat_secret"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "Bearer pat_secret", gotAuth)
}

func TestClient_PingHitsUserEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{"gid":"1"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/users/me", gotPath)
}

func TestClient_ErrorBodySurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Not Authorized","help":"see docs"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Not Authorized")
	require.Contains(t, err.Error(), "401")
}

func TestClient_ListTasks_FromProjectPaginatesAndFlattens(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tasks", r.URL.Path)
		require.Equal(t, "p1", r.URL.Query().Get("project"))
		require.Equal(t, "100", r.URL.Query().Get("limit"))
		if calls == 0 {
			calls++
			require.Empty(t, r.URL.Query().Get("offset"))
			_, _ = w.Write([]byte(`{"data":` + makeTasks(100) + `,"next_page":{"offset":"abc"}}`))
			return
		}
		require.Equal(t, "abc", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"data":[{"gid":"z","name":"last"}],"next_page":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, Project: "p1"})
	require.NoError(t, err)
	require.Len(t, records, 101)
	require.Equal(t, "last", records[100]["name"])
}

func TestClient_ListTasks_StopsWhenNoNextPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"data":[{"gid":"a","name":"one"},{"gid":"b","name":"two"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, Project: "p1"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, 1, calls) // no next_page -> only one request
}

func TestClient_ListTasks_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":` + makeTasks(100) + `,"next_page":{"offset":"more"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, Project: "p1", Limit: 5})
	require.NoError(t, err)
	require.Len(t, records, 5)
}

func TestClient_ListTasks_SectionTakesPrecedence(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeTasks,
		Section:   "sec1",
		Project:   "p1",
	})
	require.NoError(t, err)
	require.Equal(t, "sec1", got.Get("section"))
	require.Empty(t, got.Get("project"))
	require.Contains(t, got.Get("opt_fields"), "assignee.name")
}

func TestClient_ListTasks_AssigneeRequiresWorkspace(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeTasks,
		Assignee:  "me",
		Workspace: "w1",
	})
	require.NoError(t, err)
	require.Equal(t, "me", got.Get("assignee"))
	require.Equal(t, "w1", got.Get("workspace"))
}

func TestClient_ListTasks_RequiresScope(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Project")

	// Assignee without workspace is not a valid scope.
	_, err = c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, Assignee: "me"})
	require.Error(t, err)
}

func TestClient_ListTasks_IncompleteOnly(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, Project: "p1", IncompleteOnly: true})
	require.NoError(t, err)
	require.Equal(t, "now", got.Get("completed_since"))
}

func TestClient_ModifiedSince_Dashboard(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:    queryTypeTasks,
		Project:      "p1",
		ModifiedMode: dateModeDashboard,
		TimeRange:    backend.TimeRange{From: from, To: to},
	})
	require.NoError(t, err)
	require.Equal(t, from.UTC().Format(time.RFC3339), got.Get("modified_since"))
}

func TestClient_ModifiedSince_CustomParsesISO(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:     queryTypeTasks,
		Project:       "p1",
		ModifiedMode:  dateModeCustom,
		ModifiedSince: "2024-01-01",
	})
	require.NoError(t, err)
	expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	require.Equal(t, expected, got.Get("modified_since"))
}

func TestClient_ModifiedSince_AnyAddsNothing(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:     queryTypeTasks,
		Project:       "p1",
		ModifiedSince: "2024-01-01", // ignored because mode defaults to "any"
	})
	require.NoError(t, err)
	require.Empty(t, got.Get("modified_since"))
}

func TestClient_ListProjects_ScopeAndArchived(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/projects", r.URL.Path)
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[{"gid":"p1","name":"Mobile"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	// Include archived -> omit the archived param (Asana returns both).
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeProjects, Workspace: "w1", Team: "t1", IncludeArchived: true})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "w1", got.Get("workspace"))
	require.Equal(t, "t1", got.Get("team"))
	require.False(t, got.Has("archived"))

	// Default -> only active projects (archived=false).
	_, err = c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeProjects, Workspace: "w1"})
	require.NoError(t, err)
	require.Equal(t, "false", got.Get("archived"))
}

func TestClient_ListProjects_RequiresScope(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeProjects})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Workspace or Team")
}

func TestClient_ListSections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/projects/p1/sections", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"gid":"s1","name":"To Do"},{"gid":"s2","name":"Doing"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeSections, Project: "p1"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "To Do", records[0]["name"])
}

func TestClient_ListSections_RequiresProject(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeSections})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Project")
}

func TestClient_ListWorkspaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workspaces", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"gid":"w1","name":"My Org","is_organization":true}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeWorkspaces})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "My Org", records[0]["name"])
}

func TestClient_ListTeamsRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workspaces/w1/teams", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"gid":"tm1","name":"Engineering"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTeams, Workspace: "w1"})
	require.NoError(t, err)
	require.Equal(t, "Engineering", records[0]["name"])
}

func TestClient_ListUsersAndTags_RequireWorkspace(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeUsers})
	require.Error(t, err)
	_, err = c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTags})
	require.Error(t, err)
}

func TestClient_ListRaw_AutoDetectArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workspaces", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"gid":"a","name":"x"},{"gid":"b","name":"y"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw, RawPath: "/workspaces"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "x", records[0]["name"])
}

func TestClient_ListRaw_SingleObjectUnderData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"gid":"1","name":"me","email":"me@b.com"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw, RawPath: "/users/me"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "me", records[0]["name"])
}

func TestClient_ListRaw_ExplicitRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"x":1},"data":[{"gid":"s1","name":"Eng"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw, RawPath: "/teams", RawRoot: "data"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "Eng", records[0]["name"])
}

func TestClient_ListRaw_RequiresPath(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rawPath is required")
}

func TestClient_CountRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"gid":"1","name":"a"},{"gid":"2","name":"b"},{"gid":"3","name":"c"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	count, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeWorkspaces})
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
}

func TestClient_Resource_ListWorkspacesAndProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces":
			_, _ = w.Write([]byte(`{"data":[{"gid":"w1","name":"Org"}]}`))
		case "/projects":
			require.Equal(t, "w1", r.URL.Query().Get("workspace"))
			_, _ = w.Write([]byte(`{"data":[{"gid":"p1","name":"Mobile"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	ws, err := c.ListWorkspaces(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Org", ws[0].Name)
	require.Equal(t, "w1", ws[0].Gid)

	projects, err := c.ListProjects(context.Background(), "w1", "")
	require.NoError(t, err)
	require.Equal(t, "Mobile", projects[0].Name)
}

func TestToISO(t *testing.T) {
	iso, ok := toISO("2024-01-01")
	require.True(t, ok)
	require.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), iso)

	// Passthrough for an already-RFC3339 value.
	iso, ok = toISO("2024-01-01T12:00:00Z")
	require.True(t, ok)
	require.Equal(t, "2024-01-01T12:00:00Z", iso)

	_, ok = toISO("")
	require.False(t, ok)
}

// makeTasks returns a JSON array string of n minimal task objects.
func makeTasks(n int) string {
	out := "["
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += `{"gid":"t` + strconv.Itoa(i) + `","name":"task ` + strconv.Itoa(i) + `"}`
	}
	return out + "]"
}
