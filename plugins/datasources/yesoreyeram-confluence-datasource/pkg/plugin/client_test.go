package plugin

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newBasicClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := NewClient(Settings{
		BaseURL:  srv.URL,
		AuthMode: authBasic,
		Email:    "user@example.com",
		apiToken: "tok",
	}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func newBearerClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := NewClient(Settings{
		BaseURL:     srv.URL,
		AuthMode:    authBearer,
		bearerToken: "btok",
	}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestNewClient_RequiresBaseURL(t *testing.T) {
	_, err := NewClient(Settings{}, http.DefaultClient)
	require.Error(t, err)
}

func TestNewClient_ComputesOrigin(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://acme.atlassian.net/wiki", AuthMode: authBearer, bearerToken: "x"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://acme.atlassian.net", c.origin)
	require.Equal(t, "https://acme.atlassian.net/wiki", c.baseURL)
}

func TestDo_SendsBasicAuthHeader(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:tok"))
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, want, r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(`{"results":[],"_links":{}}`))
	})
	defer srv.Close()

	require.NoError(t, c.Ping(context.Background()))
}

func TestDo_SendsBearerAuthHeader(t *testing.T) {
	c, srv := newBearerClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer btok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"results":[],"_links":{}}`))
	})
	defer srv.Close()

	require.NoError(t, c.Ping(context.Background()))
}

func TestPing_HitsSpacesEndpoint(t *testing.T) {
	c, srv := newBearerClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/spaces", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"results":[],"_links":{}}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestListRecords_Pages_Flattens(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/pages", r.URL.Path)
		require.Equal(t, "456", r.URL.Query().Get("space-id"))
		_, _ = w.Write([]byte(`{"results":[
			{"id":"123","status":"current","title":"Release notes","spaceId":"456","authorId":"u1","createdAt":"2024-01-02T03:04:05.000Z",
			 "version":{"number":3,"message":"edit","createdAt":"2024-02-02T03:04:05.000Z","authorId":"u2"},
			 "_links":{"webui":"/spaces/ENG/pages/123/Release"}}
		],"_links":{}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypePages, SpaceID: "456"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "123", rows[0]["id"])
	require.Equal(t, "Release notes", rows[0]["title"])
	require.Equal(t, "456", rows[0]["spaceId"])
	require.Equal(t, float64(3), rows[0]["versionNumber"])
	require.Equal(t, "edit", rows[0]["versionMessage"])
	require.Equal(t, c.origin+"/spaces/ENG/pages/123/Release", rows[0]["webui"])
}

func TestListRecords_Blogposts_UsesBlogpostsEndpoint(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/blogposts", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"9","title":"Hello","spaceId":"1"}],"_links":{}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeBlogposts})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Hello", rows[0]["title"])
}

func TestListRecords_FollowsCursorPagination(t *testing.T) {
	calls := 0
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/pages", r.URL.Path)
		calls++
		if calls == 1 {
			require.Empty(t, r.URL.Query().Get("cursor"))
			_, _ = w.Write([]byte(`{"results":[{"id":"1","title":"a"}],"_links":{"next":"/api/v2/pages?limit=100&cursor=CUR"}}`))
			return
		}
		require.Equal(t, "CUR", r.URL.Query().Get("cursor"))
		_, _ = w.Write([]byte(`{"results":[{"id":"2","title":"b"}],"_links":{}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypePages})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 2, calls)
	require.Equal(t, "a", rows[0]["title"])
	require.Equal(t, "b", rows[1]["title"])
}

func TestListRecords_RespectsLimit(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		// limit is also passed as a per-request page size.
		require.Equal(t, "1", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"results":[{"id":"1","title":"a"},{"id":"2","title":"b"}],"_links":{"next":"/api/v2/pages?cursor=X"}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypePages, Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestSearch_RequiresCQL(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeSearch})
	require.Error(t, err)
}

func TestSearch_FlattensResults(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/search", r.URL.Path)
		require.Equal(t, "type=page", r.URL.Query().Get("cql"))
		_, _ = w.Write([]byte(`{"results":[
			{"content":{"id":"123","type":"page","status":"current","title":"Release notes","spaceId":"456"},
			 "title":"Release @@@hl@@@notes@@@endhl@@@","excerpt":"hello","url":"/spaces/ENG/pages/123","lastModified":"2024-03-01T10:00:00.000Z"}
		],"_links":{}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeSearch, CQL: "type=page"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "123", rows[0]["id"])
	require.Equal(t, "page", rows[0]["type"])
	require.Equal(t, "Release notes", rows[0]["title"]) // content.title preferred, highlight stripped
	require.Equal(t, "hello", rows[0]["excerpt"])
	require.Equal(t, c.origin+"/spaces/ENG/pages/123", rows[0]["url"])
	require.Equal(t, "2024-03-01T10:00:00.000Z", rows[0]["lastModified"])
}

func TestCountRecords_CountsPagesByDefault(t *testing.T) {
	calls := 0
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/pages", r.URL.Path)
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"results":[{"id":"1"},{"id":"2"}],"_links":{"next":"/api/v2/pages?cursor=X"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"3"}],"_links":{}}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeCount})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
}

func TestCountRecords_CountsSearchWhenCQLSet(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/search", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"content":{"id":"1"}},{"content":{"id":"2"}}],"_links":{}}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeCount, CQL: "text ~ hello"})
	require.NoError(t, err)
	require.EqualValues(t, 2, n)
}

func TestListSpaces(t *testing.T) {
	c, srv := newBearerClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/spaces", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[
			{"id":"1","key":"ENG","name":"Engineering","type":"global","status":"current"},
			{"id":"2","key":"HR","name":"People","type":"global","status":"current"}
		],"_links":{}}`))
	})
	defer srv.Close()

	spaces, err := c.ListSpaces(context.Background())
	require.NoError(t, err)
	require.Len(t, spaces, 2)
	require.Equal(t, "ENG", spaces[0].Key)
	require.Equal(t, "Engineering", spaces[0].Name)
	require.Equal(t, "HR", spaces[1].Key)
}

func TestDo_SurfacesV2ErrorMessage(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"status":404,"code":"not-found","title":"Space not found","detail":"no such space"}]}`))
	})
	defer srv.Close()

	_, err := c.ListSpaces(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Space not found")
	require.Contains(t, err.Error(), "no such space")
}

func TestDo_SurfacesV1ErrorMessage(t *testing.T) {
	c, srv := newBasicClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"statusCode":400,"message":"Could not parse cql"}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeSearch, CQL: "bad cql"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Could not parse cql")
}

func TestResolveNext(t *testing.T) {
	c := &Client{origin: "https://acme.atlassian.net"}
	require.Equal(t, "", c.resolveNext(""))
	require.Equal(t, "https://acme.atlassian.net/wiki/api/v2/pages?cursor=x", c.resolveNext("/wiki/api/v2/pages?cursor=x"))
	require.Equal(t, "https://other.example.com/x", c.resolveNext("https://other.example.com/x"))
}
