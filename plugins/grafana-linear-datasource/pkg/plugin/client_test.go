package plugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
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
	require.Equal(t, linearCloudURL, c.baseURL)
}

func TestClient_APIKeyHeaderIsRaw(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"data":{"viewer":{"id":"1","name":"me"}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authAPIKey, apiKey: "lin_api_secret"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "lin_api_secret", gotAuth)
}

func TestClient_OAuthHeaderIsBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"data":{"viewer":{"id":"1"}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authOAuth, oauthToken: "tok"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "Bearer tok", gotAuth)
}

func TestClient_GraphQLErrorsSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":"Authentication required"}]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication required")
}

func TestClient_HTTPErrorStatusSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `unauthorized`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestClient_ListRecords_IssuesPaginatesAndFlattens(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &req))
		require.Contains(t, req.Query, "issues(")

		if page == 0 {
			page++
			_, _ = io.WriteString(w, `{"data":{"issues":{
				"nodes":[
					{"identifier":"ENG-1","title":"a","state":{"name":"Todo"},"team":{"key":"ENG","name":"Engineering"}}
				],
				"pageInfo":{"hasNextPage":true,"endCursor":"c1"}
			}}}`)
			return
		}
		// Second page should send the cursor.
		require.Equal(t, "c1", req.Variables["after"])
		_, _ = io.WriteString(w, `{"data":{"issues":{
			"nodes":[
				{"identifier":"ENG-2","title":"b","state":{"name":"Done"},"team":{"key":"ENG","name":"Engineering"}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":""}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeIssues})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "ENG-1", records[0]["identifier"])
	require.Equal(t, "Todo", records[0]["state"])
	require.Equal(t, "Engineering", records[0]["team"])
	require.Equal(t, "ENG-2", records[1]["identifier"])
}

func TestClient_ListRecords_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		// The client should request only `first: 1` because limit is 1.
		require.EqualValues(t, 1, req.Variables["first"])
		_, _ = io.WriteString(w, `{"data":{"issues":{
			"nodes":[{"identifier":"ENG-1"}],
			"pageInfo":{"hasNextPage":true,"endCursor":"c1"}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeIssues, Limit: 1})
	require.NoError(t, err)
	require.Len(t, records, 1)
}

func TestClient_ListRecords_IssuesFilterSent(t *testing.T) {
	var gotFilter map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if f, ok := req.Variables["filter"].(map[string]any); ok {
			gotFilter = f
		}
		_, _ = io.WriteString(w, `{"data":{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeIssues,
		TeamId:    "team-uuid",
		States:    []string{"In Progress"},
	})
	require.NoError(t, err)
	require.NotNil(t, gotFilter)
	require.Contains(t, gotFilter, "team")
	require.Contains(t, gotFilter, "state")
}

func TestBuildFilter_MultiValueAndNewFilters(t *testing.T) {
	q := QueryModel{
		QueryType:     queryTypeIssues,
		TeamId:        "team-uuid",
		States:        []string{"Todo", "In Progress"},
		Assignees:     []string{"alice@example.com", "Bob"},
		Labels:        []string{"bug", "p1"},
		Priorities:    []int{1, 2},
		Projects:      []string{"Mobile"},
		Creator:       "carol@example.com",
		SearchQuery:   "login",
		CreatedMode:   dateModeCustom,
		CreatedAfter:  "2024-01-01",
		CreatedBefore: "2024-12-31",
		UpdatedMode:   dateModeCustom,
		UpdatedAfter:  "2024-06-01",
	}
	f := buildFilter(q)
	require.NotNil(t, f)

	// State / project / label use `in`.
	state := f["state"].(map[string]any)["name"].(map[string]any)
	require.ElementsMatch(t, []any{"Todo", "In Progress"}, state["in"])

	project := f["project"].(map[string]any)["name"].(map[string]any)
	require.ElementsMatch(t, []any{"Mobile"}, project["in"])

	labels := f["labels"].(map[string]any)["some"].(map[string]any)["name"].(map[string]any)
	require.ElementsMatch(t, []any{"bug", "p1"}, labels["in"])

	// Priority list.
	require.ElementsMatch(t, []any{1, 2}, f["priority"].(map[string]any)["in"])

	// Assignee / creator are `or` groups.
	require.Contains(t, f["assignee"].(map[string]any), "or")
	require.Contains(t, f["creator"].(map[string]any), "or")

	// Title contains.
	require.Equal(t, "login", f["title"].(map[string]any)["containsIgnoreCase"])

	// Custom date ranges.
	created := f["createdAt"].(map[string]any)
	require.Equal(t, "2024-01-01", created["gte"])
	require.Equal(t, "2024-12-31", created["lte"])
	updated := f["updatedAt"].(map[string]any)
	require.Equal(t, "2024-06-01", updated["gte"])
	require.NotContains(t, updated, "lte")
}

func TestBuildFilter_DashboardDateMode(t *testing.T) {
	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC)
	q := QueryModel{
		QueryType:   queryTypeIssues,
		CreatedMode: dateModeDashboard,
		UpdatedMode: dateModeDashboard,
		TimeRange:   backend.TimeRange{From: from, To: to},
	}
	f := buildFilter(q)
	require.NotNil(t, f)

	created := f["createdAt"].(map[string]any)
	require.Equal(t, "2024-03-01T00:00:00Z", created["gte"])
	require.Equal(t, "2024-03-31T23:59:59Z", created["lte"])
	updated := f["updatedAt"].(map[string]any)
	require.Equal(t, "2024-03-01T00:00:00Z", updated["gte"])
	require.Equal(t, "2024-03-31T23:59:59Z", updated["lte"])
}

func TestBuildFilter_DashboardModeWithoutRangeIsIgnored(t *testing.T) {
	q := QueryModel{QueryType: queryTypeIssues, CreatedMode: dateModeDashboard}
	require.Nil(t, buildFilter(q)) // zero time range -> no createdAt filter -> empty filter -> nil
}

func TestBuildFilter_AnyDateModeIgnoresBounds(t *testing.T) {
	// Even with bounds set, "any" mode (the default) emits no date filter.
	q := QueryModel{
		QueryType:    queryTypeIssues,
		CreatedAfter: "2024-01-01",
		UpdatedAfter: "2024-01-01",
	}
	require.Nil(t, buildFilter(q))
}

func TestClient_IncludeArchivedVariableSent(t *testing.T) {
	var gotArchived any
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		gotArchived, present = req.Variables["includeArchived"]
		_, _ = io.WriteString(w, `{"data":{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeIssues, IncludeArchived: true})
	require.NoError(t, err)
	require.True(t, present)
	require.Equal(t, true, gotArchived)
}

func TestBuildIssuesQuery_FieldSelection(t *testing.T) {
	// Default set when no fields requested.
	def := buildIssuesQuery(nil)
	require.Contains(t, def, "identifier")
	require.Contains(t, def, "state { name type }")

	// Custom set: only requested fields (+ always id).
	custom := buildIssuesQuery([]string{"title", "assignee"})
	require.Contains(t, custom, "id")
	require.Contains(t, custom, "title")
	require.Contains(t, custom, "assignee { name email }")
	require.NotContains(t, custom, "priorityLabel")

	// Unknown fields ignored -> falls back to default.
	fallback := buildIssuesQuery([]string{"nonsense"})
	require.Contains(t, fallback, "identifier")
}

func TestIssueFieldNames_SortedAndContainsKnown(t *testing.T) {
	names := IssueFieldNames()
	require.Contains(t, names, "identifier")
	require.Contains(t, names, "labels")
	require.True(t, sort.StringsAreSorted(names))
}

func TestClient_ListRaw_FindsNestedConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"team":{"issues":{
			"nodes":[{"identifier":"ENG-1"},{"identifier":"ENG-2"}],
			"pageInfo":{"hasNextPage":false}
		}}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeRaw,
		RawQuery:  `query { team(id:"x") { issues { nodes { identifier } } } }`,
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "ENG-1", records[0]["identifier"])
}

func TestClient_ListRaw_RequiresQuery(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rawQuery is required")
}

func TestClient_CountRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"projects":{
			"nodes":[{"name":"a"},{"name":"b"},{"name":"c"}],
			"pageInfo":{"hasNextPage":false}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	count, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeProjects})
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
}

func TestClient_ListTeams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"teams":{
			"nodes":[{"id":"1","key":"ENG","name":"Engineering"},{"id":"2","key":"OPS","name":"Operations"}],
			"pageInfo":{"hasNextPage":false}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	teams, err := c.ListTeams(context.Background())
	require.NoError(t, err)
	require.Len(t, teams, 2)
	require.Equal(t, "ENG", teams[0].Key)
}

func TestClient_ListStates_Deduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"workflowStates":{
			"nodes":[
				{"name":"Todo","type":"unstarted","team":{"key":"ENG"}},
				{"name":"Todo","type":"unstarted","team":{"key":"OPS"}},
				{"name":"Done","type":"completed","team":{"key":"ENG"}}
			],
			"pageInfo":{"hasNextPage":false}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	states, err := c.ListStates(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, states, 2) // "Todo" deduplicated
}

func TestClient_ListLabels_Deduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"issueLabels":{
			"nodes":[{"name":"bug"},{"name":"bug"},{"name":"feature"}],
			"pageInfo":{"hasNextPage":false}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	labels, err := c.ListLabels(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, labels, 2)
}

func TestClient_ListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"projects":{
			"nodes":[{"id":"p1","name":"Mobile"},{"id":"p2","name":"API"}],
			"pageInfo":{"hasNextPage":false}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	projects, err := c.ListProjects(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 2)
	require.Equal(t, "Mobile", projects[0].Name)
}

func TestClient_ListUsers_SkipsNameless(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"users":{
			"nodes":[{"name":"Alice","email":"a@b.com","active":true},{"name":"","email":"x@y.com"}],
			"pageInfo":{"hasNextPage":false}
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	users, err := c.ListUsers(context.Background())
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "Alice", users[0].Name)
	require.Equal(t, "a@b.com", users[0].Email)
}

func TestBuildFilter_NonFilterableTypes(t *testing.T) {
	require.Nil(t, buildFilter(QueryModel{QueryType: queryTypeUsers}))
	require.Nil(t, buildFilter(QueryModel{QueryType: queryTypeTeams}))
	require.Nil(t, buildFilter(QueryModel{QueryType: queryTypeIssues})) // no inputs -> nil
}

func TestNormalizeOrderBy(t *testing.T) {
	require.Equal(t, "createdAt", normalizeOrderBy(""))
	require.Equal(t, "createdAt", normalizeOrderBy("createdAt"))
	require.Equal(t, "updatedAt", normalizeOrderBy("updatedAt"))
	require.Equal(t, "createdAt", normalizeOrderBy("nonsense"))
}

func TestLoadQuery_Defaults(t *testing.T) {
	q, err := LoadQuery(nil)
	require.NoError(t, err)
	require.Equal(t, queryTypeIssues, q.QueryType)

	q, err = LoadQuery(json.RawMessage(`{"queryType":"projects","limit":5}`))
	require.NoError(t, err)
	require.Equal(t, queryTypeProjects, q.QueryType)
	require.Equal(t, 5, q.Limit)
}
