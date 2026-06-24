package plugin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, srv *httptest.Server, apiToken string) *Client {
	t.Helper()
	c, err := NewClient(Settings{apiToken: apiToken}, http.DefaultClient)
	require.NoError(t, err)
	c.baseURL = srv.URL
	return c
}

func TestNewClient_DefaultsToCloudURL(t *testing.T) {
	c, err := NewClient(Settings{}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, shortcutCloudURL, c.baseURL)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://proxy.example.com/"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com", c.baseURL)
}

// TestClient_ShortcutTokenHeader asserts auth uses the Shortcut-Token header
// (NOT Authorization: Bearer and NOT the deprecated token query param).
func TestClient_ShortcutTokenHeader(t *testing.T) {
	var gotToken, gotAuth, gotQueryToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Shortcut-Token")
		gotAuth = r.Header.Get("Authorization")
		gotQueryToken = r.URL.Query().Get("token")
		_, _ = io.WriteString(w, `{"data":[],"total":0}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "my-token")
	_, _, err := c.ListStories(context.Background(), QueryModel{})
	require.NoError(t, err)
	require.Equal(t, "my-token", gotToken)
	require.Empty(t, gotAuth)
	require.Empty(t, gotQueryToken)
}

func TestClient_PingHitsCurrentMemberEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"id":"u1","profile":{"name":"Me"}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/api/v3/member", gotPath)
}

func TestClient_HitsSearchStoriesEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"data":[],"total":0}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	_, _, err := c.ListStories(context.Background(), QueryModel{})
	require.NoError(t, err)
	require.Equal(t, "/api/v3/search/stories", gotPath)
}

func TestClient_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"Invalid token"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "Invalid token")
}

// TestClient_ListStories_PaginatesViaNextToken verifies the search `next`
// page-path is followed (resolved against the host) until it is null.
func TestClient_ListStories_PaginatesViaNextToken(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		next := r.URL.Query().Get("next")
		switch next {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"id":1,"name":"First"},{"id":2,"name":"Second"}],"next":"/api/v3/search/stories?query=is%3Astory&next=tok2&page_size=25","total":3}`)
		case "tok2":
			_, _ = io.WriteString(w, `{"data":[{"id":3,"name":"Third"}],"next":null,"total":3}`)
		default:
			_, _ = io.WriteString(w, `{"data":[],"next":null,"total":3}`)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	stories, total, err := c.ListStories(context.Background(), QueryModel{})
	require.NoError(t, err)
	require.Len(t, stories, 3)
	require.Equal(t, "First", stories[0]["name"])
	require.Equal(t, "Third", stories[2]["name"])
	require.Equal(t, 3, total)
	require.Len(t, paths, 2)
	// The second request must follow the relative `next` path verbatim.
	require.Contains(t, paths[1], "next=tok2")
	require.True(t, strings.HasPrefix(paths[1], "/api/v3/search/stories"))
}

// TestClient_ListStories_StopsWhenNextNull verifies a single page is fetched
// when the response carries no next token.
func TestClient_ListStories_StopsWhenNextNull(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"data":[{"id":1},{"id":2}],"next":null,"total":2}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	stories, total, err := c.ListStories(context.Background(), QueryModel{})
	require.NoError(t, err)
	require.Len(t, stories, 2)
	require.Equal(t, 2, total)
	require.Equal(t, 1, calls)
}

func TestClient_ListStories_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// page_size must be clamped down to the limit on the first request.
		require.Equal(t, "2", r.URL.Query().Get("page_size"))
		_, _ = io.WriteString(w, `{"data":[{"id":1},{"id":2},{"id":3}],"next":"/api/v3/search/stories?next=more","total":9}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	stories, total, err := c.ListStories(context.Background(), QueryModel{Limit: 2})
	require.NoError(t, err)
	require.Len(t, stories, 2) // truncated to the limit
	require.Equal(t, 9, total) // total still reflects the full match count
}

// TestClient_ListStories_BuildsQueryString verifies structured filters compile
// into the Shortcut search query string and are sent in the `query` param.
func TestClient_ListStories_BuildsQueryString(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = io.WriteString(w, `{"data":[],"total":0}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	_, _, err := c.ListStories(context.Background(), QueryModel{
		StoryType:      "bug",
		WorkflowStates: []string{"In Progress"},
		Labels:         []string{"auth", "needs copy"},
		Owners:         []string{"alice"},
		Epic:           "Auth overhaul",
		Archived:       archivedExclude,
	})
	require.NoError(t, err)

	q := got.Get("query")
	require.Contains(t, q, "type:bug")
	require.Contains(t, q, `state:"In Progress"`)
	require.Contains(t, q, "label:auth")
	require.Contains(t, q, `label:"needs copy"`)
	require.Contains(t, q, "owner:alice")
	require.Contains(t, q, `epic:"Auth overhaul"`)
	require.Contains(t, q, "!is:archived")
	require.Equal(t, "25", got.Get("page_size"))
	require.Equal(t, "full", got.Get("detail"))
}

func TestClient_ListStories_DefaultQueryWhenNoFilters(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("query")
		_, _ = io.WriteString(w, `{"data":[],"total":0}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	_, _, err := c.ListStories(context.Background(), QueryModel{})
	require.NoError(t, err)
	require.Equal(t, defaultSearchQuery, got)
}

// TestClient_CountStories_UsesTotalField verifies Count is a single request that
// reads the `total` field (no pagination).
func TestClient_CountStories_UsesTotalField(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/api/v3/search/stories", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("page_size"))
		_, _ = io.WriteString(w, `{"data":[{"id":1}],"next":"/api/v3/search/stories?next=more","total":1234}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	count, err := c.CountStories(context.Background(), QueryModel{})
	require.NoError(t, err)
	require.EqualValues(t, 1234, count)
	require.Equal(t, 1, calls) // single request despite the next token
}

func TestClient_ListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/projects", r.URL.Path)
		_, _ = io.WriteString(w, `[{"id":1,"name":"Backend"},{"id":2,"name":"Frontend"}]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	projects, err := c.ListProjects(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 2)
	require.Equal(t, "Backend", projects[0].Name)
}

func TestClient_ListEpics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/epics", r.URL.Path)
		_, _ = io.WriteString(w, `[{"id":5,"name":"Auth overhaul"}]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	epics, err := c.ListEpics(context.Background())
	require.NoError(t, err)
	require.Len(t, epics, 1)
	require.Equal(t, "Auth overhaul", epics[0].Name)
}

func TestClient_ListIterations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/iterations", r.URL.Path)
		_, _ = io.WriteString(w, `[{"id":3,"name":"Sprint 12"}]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	iterations, err := c.ListIterations(context.Background())
	require.NoError(t, err)
	require.Len(t, iterations, 1)
	require.Equal(t, "Sprint 12", iterations[0].Name)
}

// TestClient_ListMembers_FlattensProfile verifies name/mention_name are read
// from the nested profile object (not the top level).
func TestClient_ListMembers_FlattensProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/members", r.URL.Path)
		_, _ = io.WriteString(w, `[{"id":"uuid-1","profile":{"name":"Alice","mention_name":"alice"}}]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	members, err := c.ListMembers(context.Background())
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, "uuid-1", members[0].ID)
	require.Equal(t, "Alice", members[0].Name)
	require.Equal(t, "alice", members[0].MentionName)
}

// TestClient_ListTeams_UsesGroupsEndpoint verifies teams come from /groups.
func TestClient_ListTeams_UsesGroupsEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/groups", r.URL.Path)
		_, _ = io.WriteString(w, `[{"id":"team-1","name":"Engineering","mention_name":"eng"}]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	teams, err := c.ListTeams(context.Background())
	require.NoError(t, err)
	require.Len(t, teams, 1)
	require.Equal(t, "Engineering", teams[0].Name)
}

func TestClient_ListLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/labels", r.URL.Path)
		_, _ = io.WriteString(w, `[{"id":1,"name":"bug"},{"id":2,"name":"feature"}]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	labels, err := c.ListLabels(context.Background())
	require.NoError(t, err)
	require.Len(t, labels, 2)
	require.Equal(t, "bug", labels[0].Name)
}

func TestClient_ListWorkflows_Deduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/workflows", r.URL.Path)
		_, _ = io.WriteString(w, `[
			{"id":1,"name":"Engineering","states":[
				{"id":100,"name":"Todo","type":"unstarted"},
				{"id":101,"name":"Done","type":"completed"}
			]},
			{"id":2,"name":"Product","states":[
				{"id":100,"name":"Todo","type":"unstarted"}
			]}
		]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, "tok")
	states, err := c.ListWorkflows(context.Background())
	require.NoError(t, err)
	require.Len(t, states, 2)
}
