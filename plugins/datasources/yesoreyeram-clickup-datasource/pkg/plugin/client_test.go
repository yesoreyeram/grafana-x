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
	require.Equal(t, clickUpCloudURL, c.baseURL)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://example.com/api/"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/api", c.baseURL)
}

func TestClient_APIKeyHeaderIsRaw(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"user":{"id":1,"username":"me"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authAPIKey, apiKey: "pk_secret"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "pk_secret", gotAuth)
}

func TestClient_OAuthHeaderIsBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authOAuth, oauthToken: "tok"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "Bearer tok", gotAuth)
}

func TestClient_PingHitsUserEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/v2/user", gotPath)
}

func TestClient_ErrorBodySurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"err":"Token invalid","ECODE":"OAUTH_025"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Token invalid")
	require.Contains(t, err.Error(), "401")
}

func TestClient_ListTasks_FromListPaginatesAndFlattens(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/list/123/task", r.URL.Path)
		gotPage := r.URL.Query().Get("page")
		if page == 0 {
			require.Equal(t, "0", gotPage)
			page++
			// Full page (100) so the client requests another page. We only send
			// two real tasks but pad last_page=false + a full count via the flag.
			tasks := makeTasks(100)
			_, _ = w.Write([]byte(`{"last_page":false,"tasks":` + tasks + `}`))
			return
		}
		require.Equal(t, "1", gotPage)
		_, _ = w.Write([]byte(`{"last_page":true,"tasks":[{"id":"z","name":"last"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, ListId: "123"})
	require.NoError(t, err)
	require.Len(t, records, 101)
	require.Equal(t, "last", records[100]["name"])
}

func TestClient_ListTasks_StopsOnShortPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"tasks":[{"id":"a","name":"one"},{"id":"b","name":"two"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, ListId: "1"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, 1, calls) // short page -> only one request
}

func TestClient_ListTasks_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"last_page":false,"tasks":` + makeTasks(100) + `}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks, ListId: "1", Limit: 5})
	require.NoError(t, err)
	require.Len(t, records, 5)
}

func TestClient_ListTasks_TeamEndpointAndScopeParams(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/team/777/task", r.URL.Path)
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeTasks,
		TeamId:    "777",
		SpaceId:   "s1",
		FolderId:  "f1",
		Statuses:  []string{"to do", "in progress"},
		Assignees: []string{"123"},
		Tags:      []string{"bug"},
	})
	require.NoError(t, err)
	require.Equal(t, "s1", got.Get("space_ids[]"))
	require.Equal(t, "f1", got.Get("project_ids[]"))
	require.ElementsMatch(t, []string{"to do", "in progress"}, got["statuses[]"])
	require.Equal(t, []string{"123"}, got["assignees[]"])
	require.Equal(t, []string{"bug"}, got["tags[]"])
	require.Equal(t, "created", got.Get("order_by"))
}

func TestClient_ListTasks_RequiresScope(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTasks})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Workspace")
}

func TestClient_DateMode_Dashboard(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer srv.Close()

	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:   queryTypeTasks,
		ListId:      "1",
		CreatedMode: dateModeDashboard,
		TimeRange:   backend.TimeRange{From: from, To: to},
	})
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(from.UnixMilli(), 10), got.Get("date_created_gt"))
	require.Equal(t, strconv.FormatInt(to.UnixMilli(), 10), got.Get("date_created_lt"))
}

func TestClient_DateMode_CustomParsesISO(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeTasks,
		ListId:    "1",
		DueMode:   dateModeCustom,
		DueAfter:  "2024-01-01",
		DueBefore: "1735689600000", // already millis
	})
	require.NoError(t, err)
	expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	require.Equal(t, strconv.FormatInt(expected, 10), got.Get("due_date_gt"))
	require.Equal(t, "1735689600000", got.Get("due_date_lt"))
}

func TestClient_DateMode_AnyAddsNothing(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:    queryTypeTasks,
		ListId:       "1",
		CreatedAfter: "2024-01-01", // ignored because mode defaults to "any"
	})
	require.NoError(t, err)
	require.Empty(t, got.Get("date_created_gt"))
}

func TestClient_ListSpaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/team/9/space", r.URL.Path)
		_, _ = w.Write([]byte(`{"spaces":[{"id":"s1","name":"Eng","private":false},{"id":"s2","name":"Ops"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeSpaces, TeamId: "9"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Eng", records[0]["name"])
}

func TestClient_ListFolders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/space/s1/folder", r.URL.Path)
		_, _ = w.Write([]byte(`{"folders":[{"id":"f1","name":"Q1","task_count":"5"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeFolders, SpaceId: "s1"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "Q1", records[0]["name"])
}

func TestClient_ListLists_FolderVsFolderless(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"lists":[{"id":"l1","name":"Backlog"}]}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})

	// Folder set -> folder endpoint.
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/folder/f1/list", r.URL.Path)
		_, _ = w.Write([]byte(`{"lists":[{"id":"l1","name":"Backlog"}]}`))
	})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeLists, FolderId: "f1", SpaceId: "s1"})
	require.NoError(t, err)
	require.Len(t, records, 1)

	// Only space set -> folderless endpoint.
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/space/s1/list", r.URL.Path)
		_, _ = w.Write([]byte(`{"lists":[{"id":"l9","name":"Sprint"}]}`))
	})
	records, err = c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeLists, SpaceId: "s1"})
	require.NoError(t, err)
	require.Equal(t, "Sprint", records[0]["name"])
}

func TestClient_ListRaw_AutoDetectArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/team/1/task", r.URL.Path)
		_, _ = w.Write([]byte(`{"tasks":[{"id":"a","name":"x"},{"id":"b","name":"y"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw, RawPath: "/v2/team/1/task"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "x", records[0]["name"])
}

func TestClient_ListRaw_ExplicitRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"shared":{"tasks":[{"id":"a"}]},"spaces":[{"id":"s1","name":"Eng"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw, RawPath: "/v2/team/1/space", RawRoot: "spaces"})
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
		_, _ = w.Write([]byte(`{"spaces":[{"id":"1","name":"a"},{"id":"2","name":"b"},{"id":"3","name":"c"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	count, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeSpaces, TeamId: "1"})
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
}

func TestClient_ListTeams_Resource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"teams":[{"id":"1","name":"My WS"},{"id":"2","name":"Other"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	teams, err := c.ListTeams(context.Background())
	require.NoError(t, err)
	require.Len(t, teams, 2)
	require.Equal(t, "My WS", teams[0].Name)
}

func TestClient_ListMembers_FiltersByTeamAndDeduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"teams":[
			{"id":"1","members":[
				{"user":{"id":10,"username":"Alice","email":"a@b.com"}},
				{"user":{"id":10,"username":"Alice","email":"a@b.com"}},
				{"user":{"id":11,"username":"Bob","email":"bob@b.com"}}
			]},
			{"id":"2","members":[{"user":{"id":99,"username":"Zed"}}]}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "x"})
	members, err := c.ListMembers(context.Background(), "1")
	require.NoError(t, err)
	require.Len(t, members, 2) // deduped Alice, and Bob; Zed excluded (team 2)
	require.Equal(t, "10", members[0].ID)
	require.Equal(t, "Alice", members[0].Username)
}

func TestNormalizeOrderBy(t *testing.T) {
	require.Equal(t, "created", normalizeOrderBy(""))
	require.Equal(t, "created", normalizeOrderBy("nonsense"))
	require.Equal(t, "updated", normalizeOrderBy("updated"))
	require.Equal(t, "due_date", normalizeOrderBy("due_date"))
	require.Equal(t, "id", normalizeOrderBy("id"))
}

func TestToUnixMillis(t *testing.T) {
	ms, ok := toUnixMillis("1735689600000")
	require.True(t, ok)
	require.EqualValues(t, 1735689600000, ms)

	ms, ok = toUnixMillis("2024-01-01")
	require.True(t, ok)
	require.EqualValues(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), ms)

	_, ok = toUnixMillis("")
	require.False(t, ok)
	_, ok = toUnixMillis("not-a-date")
	require.False(t, ok)
}

// makeTasks returns a JSON array string of n minimal task objects.
func makeTasks(n int) string {
	out := "["
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += `{"id":"t` + strconv.Itoa(i) + `","name":"task ` + strconv.Itoa(i) + `"}`
	}
	return out + "]"
}
