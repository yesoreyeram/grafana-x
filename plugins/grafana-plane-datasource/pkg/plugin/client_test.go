package plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	require.Equal(t, planeCloudURL, c.baseURL)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://plane.example.com/"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://plane.example.com", c.baseURL)
}

func TestClient_APIKeyHeader(t *testing.T) {
	var gotKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authAPIKey, apiKey: "plane_api_secret"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "plane_api_secret", gotKey)
	require.Empty(t, gotAuth)
}

func TestClient_OAuthHeaderIsBearer(t *testing.T) {
	var gotKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authOAuth, oauthToken: "tok"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "Bearer tok", gotAuth)
	require.Empty(t, gotKey)
}

func TestClient_PingHitsCurrentUserEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/api/v1/users/me/", gotPath)
}

func TestClient_ErrorBodySurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid API key"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid API key")
	require.Contains(t, err.Error(), "401")
}

func TestClient_ErrorBody_DetailKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Not found.")
}

func TestClient_ListWorkItems_PaginatesAndFlattens(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/workspaces/my-team/projects/p1/work-items/", r.URL.Path)
		cursor := r.URL.Query().Get("cursor")
		thisCall := calls
		calls++
		if thisCall == 0 {
			require.Empty(t, cursor)
			_, _ = w.Write([]byte(`{"next_cursor":"100:1:0","next_page_results":true,"results":[{"id":"a","name":"one"}]}`))
			return
		}
		require.Equal(t, "100:1:0", cursor)
		_, _ = w.Write([]byte(`{"next_cursor":"","next_page_results":false,"results":[{"id":"b","name":"two"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "my-team", ProjectId: "p1",
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "one", records[0]["name"])
	require.Equal(t, "two", records[1]["name"])
	require.Equal(t, 2, calls)
}

func TestClient_ListWorkItems_StopsWhenNoNextPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"next_page_results":false,"results":[{"id":"a"},{"id":"b"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p",
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, 1, calls)
}

func TestClient_ListWorkItems_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"next_cursor":"100:1:0","next_page_results":true,"results":` + makeItems(100) + `}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p", Limit: 5,
	})
	require.NoError(t, err)
	require.Len(t, records, 5)
}

// TestClient_ListWorkItems_OnlySafeParamsSent verifies that the work item list
// request sends ONLY the params the Plane endpoint honours (order_by, expand,
// per_page) and does NOT send the ineffective filter params.
func TestClient_ListWorkItems_OnlySafeParamsSent(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:     queryTypeWorkItems,
		WorkspaceSlug: "w",
		ProjectId:     "p",
		Priorities:    []string{"high", "urgent"},
		States:        []string{"s1"},
		Assignees:     []string{"u1", "u2"},
		Labels:        []string{"l1"},
		Expand:        []string{"assignees", "state", "labels"},
	})
	require.NoError(t, err)
	require.Equal(t, "assignees,state,labels", got.Get("expand"))
	require.Equal(t, "-created_at", got.Get("order_by"))
	require.Equal(t, "100", got.Get("per_page"))
	// Filter params must NOT be sent — the endpoint ignores them.
	require.Empty(t, got["priority"])
	require.Empty(t, got["state"])
	require.Empty(t, got["assignees"])
	require.Empty(t, got["labels"])
}

// TestClient_ListWorkItems_FiltersClientSide verifies that priority / state /
// assignee / label filters are applied to the fetched items, since Plane's list
// endpoint does not filter server-side.
func TestClient_ListWorkItems_FiltersClientSide(t *testing.T) {
	body := `{"results":[
		{"id":"1","name":"a","priority":"high","state":"st1","assignees":["u1","u2"],"labels":["l1"]},
		{"id":"2","name":"b","priority":"low","state":"st2","assignees":["u3"],"labels":["l2"]},
		{"id":"3","name":"c","priority":"high","state":"st2","assignees":["u2"],"labels":["l1","l3"]}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})

	// Priority filter.
	recs, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p", Priorities: []string{"high"},
	})
	require.NoError(t, err)
	require.Len(t, recs, 2)
	require.Equal(t, "a", recs[0]["name"])
	require.Equal(t, "c", recs[1]["name"])

	// State filter.
	recs, err = c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p", States: []string{"st2"},
	})
	require.NoError(t, err)
	require.Len(t, recs, 2)

	// Assignee filter (array membership).
	recs, err = c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p", Assignees: []string{"u2"},
	})
	require.NoError(t, err)
	require.Len(t, recs, 2)

	// Label filter.
	recs, err = c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p", Labels: []string{"l1"},
	})
	require.NoError(t, err)
	require.Len(t, recs, 2)

	// Combined filters are AND across groups.
	recs, err = c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p",
		Priorities: []string{"high"}, States: []string{"st2"},
	})
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "c", recs[0]["name"])
}

// TestClient_ListWorkItems_FiltersExpandedRelations verifies filtering works when
// relations are expanded into objects (state/assignees/labels carry an "id").
func TestClient_ListWorkItems_FiltersExpandedRelations(t *testing.T) {
	body := `{"results":[
		{"id":"1","name":"a","state":{"id":"st1","name":"Todo"},"assignees":[{"id":"u1","display_name":"Alice"}],"labels":[{"id":"l1","name":"bug"}]},
		{"id":"2","name":"b","state":{"id":"st2","name":"Done"},"assignees":[{"id":"u2","display_name":"Bob"}],"labels":[{"id":"l2","name":"chore"}]}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})

	recs, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p",
		States: []string{"st1"}, Assignees: []string{"u1"}, Labels: []string{"l1"},
	})
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "a", recs[0]["name"])
}

// TestClient_ListWorkItems_LimitAfterFilter verifies the limit bounds matching
// rows, not scanned rows.
func TestClient_ListWorkItems_LimitAfterFilter(t *testing.T) {
	body := `{"results":[
		{"id":"1","priority":"high"},
		{"id":"2","priority":"low"},
		{"id":"3","priority":"high"},
		{"id":"4","priority":"high"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})

	recs, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeWorkItems, WorkspaceSlug: "w", ProjectId: "p",
		Priorities: []string{"high"}, Limit: 2,
	})
	require.NoError(t, err)
	require.Len(t, recs, 2) // two of the three "high" items
}

func TestClient_ListWorkItems_RequiresWorkspaceAndProject(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})

	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeWorkItems})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Workspace")

	_, err = c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeWorkItems, WorkspaceSlug: "w"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Project")
}

func TestClient_WorkspaceFallsBackToDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/workspaces/default-ws/projects/p/work-items/", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x", WorkspaceSlug: "default-ws"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeWorkItems, ProjectId: "p"})
	require.NoError(t, err)
}

func TestClient_DateMode_Dashboard(t *testing.T) {
	body := `{"results":[
		{"id":"1","name":"old","created_at":"2024-02-15T00:00:00Z"},
		{"id":"2","name":"in","created_at":"2024-03-15T00:00:00Z"},
		{"id":"3","name":"late","created_at":"2024-04-15T00:00:00Z"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 31, 12, 0, 0, 0, time.UTC)
	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	recs, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:     queryTypeWorkItems,
		WorkspaceSlug: "w",
		ProjectId:     "p",
		CreatedMode:   dateModeDashboard,
		TimeRange:     backend.TimeRange{From: from, To: to},
	})
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "in", recs[0]["name"])
}

func TestClient_DateMode_Custom(t *testing.T) {
	body := `{"results":[
		{"id":"1","name":"before","updated_at":"2023-12-31T00:00:00Z"},
		{"id":"2","name":"in","updated_at":"2024-06-15T00:00:00Z"},
		{"id":"3","name":"after","updated_at":"2025-06-15T00:00:00Z"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	recs, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:     queryTypeWorkItems,
		WorkspaceSlug: "w",
		ProjectId:     "p",
		UpdatedMode:   dateModeCustom,
		UpdatedAfter:  "2024-01-01",
		UpdatedBefore: "2024-12-31T23:59:59Z",
	})
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "in", recs[0]["name"])
}

func TestClient_DateMode_AnyKeepsAll(t *testing.T) {
	body := `{"results":[
		{"id":"1","created_at":"2020-01-01T00:00:00Z"},
		{"id":"2","created_at":"2024-01-01T00:00:00Z"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	recs, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:     queryTypeWorkItems,
		WorkspaceSlug: "w",
		ProjectId:     "p",
		CreatedAfter:  "2024-01-01", // ignored because mode defaults to "any"
	})
	require.NoError(t, err)
	require.Len(t, recs, 2)
}

func TestClient_ListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/workspaces/w/projects/", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Apollo","identifier":"APO"},{"id":"p2","name":"Beacon"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeProjects, WorkspaceSlug: "w"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Apollo", records[0]["name"])
}

func TestClient_ListProjectScoped_States(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/workspaces/w/projects/p/states/", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"s1","name":"Todo","group":"unstarted"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeStates, WorkspaceSlug: "w", ProjectId: "p"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "Todo", records[0]["name"])
}

func TestClient_ListMembers_BareArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/workspaces/w/members/", r.URL.Path)
		_, _ = w.Write([]byte(`[{"member_id":"u1","display_name":"Alice","email":"a@b.com"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeMembers, WorkspaceSlug: "w"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "u1", records[0]["member_id"])
}

func TestClient_ListRaw_DefaultsToResultsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/workspaces/w/projects/", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Apollo"},{"id":"p2","name":"Beacon"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw, RawPath: "/api/v1/workspaces/w/projects/"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Apollo", records[0]["name"])
}

func TestClient_ListRaw_ExplicitRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"x"}],"states":[{"id":"s1","name":"Done"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw, RawPath: "/anything", RawRoot: "states"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "Done", records[0]["name"])
}

func TestClient_ListRaw_RequiresPath(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rawPath is required")
}

func TestClient_CountRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"1"},{"id":"2"},{"id":"3"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	count, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeProjects, WorkspaceSlug: "w"})
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
}

func TestClient_ListProjects_Resource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Apollo","identifier":"APO"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	projects, err := c.ListProjects(context.Background(), "w")
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, "Apollo", projects[0].Name)
	require.Equal(t, "APO", projects[0].Identifier)
}

func TestClient_ListMembers_Resource_NestedAndDeduped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[
			{"id":"m1","member":{"id":"u1","display_name":"Alice","email":"a@b.com"}},
			{"id":"m2","member":{"id":"u1","display_name":"Alice","email":"a@b.com"}},
			{"id":"m3","member_id":"u2","display_name":"Bob"}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	members, err := c.ListMembers(context.Background(), "w")
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.Equal(t, "u1", members[0].ID)
	require.Equal(t, "Alice", members[0].DisplayName)
	require.Equal(t, "u2", members[1].ID)
}

func TestNormalizeOrderBy(t *testing.T) {
	require.Equal(t, "-created_at", normalizeOrderBy(""))
	require.Equal(t, "-created_at", normalizeOrderBy("   "))
	require.Equal(t, "priority", normalizeOrderBy("priority"))
	require.Equal(t, "-updated_at", normalizeOrderBy("-updated_at"))
}

// makeItems returns a JSON array string of n minimal work item objects.
func makeItems(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf(`{"id":"i%d","name":"item %d"}`, i, i)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
