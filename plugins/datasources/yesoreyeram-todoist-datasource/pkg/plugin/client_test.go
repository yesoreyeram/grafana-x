package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	require.Equal(t, todoistCloudURL, c.baseURL)
	require.Equal(t, "https://api.todoist.com/api/v1", c.baseURL)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://api.todoist.com/api/v1/"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://api.todoist.com/api/v1", c.baseURL)
}

func TestClient_AuthHeaderIsBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"results":[],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "token123"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "Bearer token123", gotAuth)
}

func TestClient_PingHitsProjectsEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"results":[{"id":"1","name":"Project"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/projects", gotPath)
}

func TestClient_ErrorBodySurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid token","error_code":401,"error_tag":"UNAUTHORIZED","http_code":401}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid token")
	require.Contains(t, err.Error(), "401")
}

func TestClient_ListTasks_PaginatesWithCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tasks", r.URL.Path)
		require.Equal(t, "200", r.URL.Query().Get("limit"))
		if calls == 0 {
			calls++
			require.Empty(t, r.URL.Query().Get("cursor"))
			_, _ = w.Write([]byte(`{"results":` + makeTodoistTasks(200) + `,"next_cursor":"abc"}`))
			return
		}
		require.Equal(t, "abc", r.URL.Query().Get("cursor"))
		_, _ = w.Write([]byte(`{"results":[{"id":"z","content":"last"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks})
	require.NoError(t, err)
	require.Len(t, records, 201)
	require.Equal(t, "last", records[200]["content"])
}

func TestClient_ListTasks_StopsWhenNextCursorNull(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"results":[{"id":"a","content":"one"},{"id":"b","content":"two"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, 1, calls) // null next_cursor -> only one request
}

func TestClient_ListTasks_RespectsLimit(t *testing.T) {
	var gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(`{"results":` + makeTodoistTasks(200) + `,"next_cursor":"more"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, Limit: 5})
	require.NoError(t, err)
	require.Len(t, records, 5)
	// The requested page size is clamped down to the remaining hard limit.
	require.Equal(t, "5", gotLimit)
}

func TestClient_ListTasks_SendsScopeParamsToTasksEndpoint(t *testing.T) {
	var got url.Values
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"results":[],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeTasks,
		ProjectId: "p1",
		SectionId: "s1",
		Label:     "urgent",
		ParentId:  "parent1",
	})
	require.NoError(t, err)
	require.Equal(t, "/tasks", gotPath)
	require.Equal(t, "p1", got.Get("project_id"))
	require.Equal(t, "s1", got.Get("section_id"))
	require.Equal(t, "urgent", got.Get("label"))
	require.Equal(t, "parent1", got.Get("parent_id"))
	require.Empty(t, got.Get("query"))
}

func TestClient_ListTasks_FilterRoutesToFilterEndpoint(t *testing.T) {
	var got url.Values
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"results":[],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	// Filter takes precedence over the id-based scope, which cannot be combined
	// with the dedicated /tasks/filter endpoint.
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeTasks,
		ProjectId: "p1",
		Filter:    "today | overdue",
		Lang:      "en",
	})
	require.NoError(t, err)
	require.Equal(t, "/tasks/filter", gotPath)
	require.Equal(t, "today | overdue", got.Get("query"))
	require.Equal(t, "en", got.Get("lang"))
	require.Empty(t, got.Get("project_id"))
}

func TestClient_CountRecords_PaginatesAndCounts(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tasks", r.URL.Path)
		if calls == 0 {
			calls++
			_, _ = w.Write([]byte(`{"results":` + makeTodoistTasks(200) + `,"next_cursor":"next"}`))
			return
		}
		require.Equal(t, "next", r.URL.Query().Get("cursor"))
		_, _ = w.Write([]byte(`{"results":[{"id":"x"},{"id":"y"},{"id":"z"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	count, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeCount})
	require.NoError(t, err)
	require.EqualValues(t, 203, count)
}

func TestClient_CountRecords_RespectsLimitCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":` + makeTodoistTasks(200) + `,"next_cursor":"more"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	count, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeCount, Limit: 50})
	require.NoError(t, err)
	require.EqualValues(t, 50, count)
}

func TestClient_Resource_ListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/projects", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Work"},{"id":"p2","name":"Personal"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	projects, err := c.ListProjects(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 2)
	require.Equal(t, "Work", projects[0].Name)
	require.Equal(t, "p1", projects[0].ID)
}

func TestClient_Resource_ListProjects_FollowsCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/projects", r.URL.Path)
		if calls == 0 {
			calls++
			_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Work"}],"next_cursor":"c2"}`))
			return
		}
		require.Equal(t, "c2", r.URL.Query().Get("cursor"))
		_, _ = w.Write([]byte(`{"results":[{"id":"p2","name":"Personal"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	projects, err := c.ListProjects(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 2)
	require.Equal(t, "Personal", projects[1].Name)
}

func TestClient_Resource_ListSectionsRequiresProject(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiToken: "x"})
	_, err := c.ListSections(context.Background(), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "projectId is required")
}

func TestClient_Resource_ListSections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/sections", r.URL.Path)
		require.Equal(t, "p1", r.URL.Query().Get("project_id"))
		_, _ = w.Write([]byte(`{"results":[{"id":"s1","name":"To Do"},{"id":"s2","name":"Doing"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	sections, err := c.ListSections(context.Background(), "p1")
	require.NoError(t, err)
	require.Len(t, sections, 2)
	require.Equal(t, "To Do", sections[0].Name)
}

func TestClient_Resource_ListLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/labels", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"l1","name":"urgent"}],"next_cursor":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	labels, err := c.ListLabels(context.Background())
	require.NoError(t, err)
	require.Len(t, labels, 1)
	require.Equal(t, "urgent", labels[0].Name)
}

func makeTodoistTasks(n int) string {
	out := "["
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += `{"id":"t` + strconv.Itoa(i) + `","content":"task ` + strconv.Itoa(i) + `"}`
	}
	return out + "]"
}
