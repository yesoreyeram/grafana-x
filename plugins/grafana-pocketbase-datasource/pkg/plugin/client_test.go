package plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testToken = "test-auth-token"

// superuserSettings returns default superuser-mode settings for tests.
func superuserSettings() Settings {
	return Settings{
		AuthMode:       AuthModeSuperuser,
		Identity:       "admin@example.com",
		password:       "secret123",
		AuthCollection: "",
	}
}

// withAuth wraps a handler so the auth-with-password endpoint automatically
// returns a token; all other requests are delegated to next.
func withAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth-with-password") {
			_, _ = w.Write([]byte(fmt.Sprintf(`{"token":%q}`, token)))
			return
		}
		next(w, r)
	}
}

func newTestClient(t *testing.T, settings Settings, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	settings.URL = srv.URL
	c, err := NewClient(settings, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestNewClient_AuthCollectionForSuperuser(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	require.Equal(t, superusersCollection, c.authCollection)
}

func TestAuthenticate_PostsToAuthCollection(t *testing.T) {
	var authPath string
	c, srv := newTestClient(t, superuserSettings(), func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth-with-password") {
			authPath = r.URL.Path
			_, _ = w.Write([]byte(`{"token":"abc"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "posts"})
	require.NoError(t, err)
	require.Equal(t, "/api/collections/_superusers/auth-with-password", authPath)
}

func TestListRecords(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, testToken, r.Header.Get("Authorization"))
		require.Equal(t, "/api/collections/posts/records", r.URL.Path)
		require.Equal(t, "200", r.URL.Query().Get("perPage"))
		require.Equal(t, "1", r.URL.Query().Get("page"))
		require.Equal(t, "1", r.URL.Query().Get("skipTotal"))
		_, _ = w.Write([]byte(`{"page":1,"perPage":200,"items":[{"id":"r1","title":"Alice"}]}`))
	}))
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "posts"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["title"])
	require.Equal(t, "r1", rows[0]["id"])
}

func TestListRecords_RequiresCollection(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_ForwardsFilterSortFields(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, `status = 'active'`, q.Get("filter"))
		require.Equal(t, "-age,name", q.Get("sort"))
		require.Equal(t, "name,age,id,created,updated", q.Get("fields"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		CollectionID: "posts",
		Fields:       "name,age",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Attribute: "status", Op: "equal", Value: "active"}},
		},
		sortItems: []SortItem{
			{Attribute: "age", Direction: "desc"},
			{Attribute: "name", Direction: "asc"},
		},
	})
	require.NoError(t, err)
}

func TestListRecords_RawFilterTakesPrecedence(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, `total > 10`, r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		CollectionID: "posts",
		RawFilter:    "total > 10",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Attribute: "status", Op: "equal", Value: "active"}},
		},
	})
	require.NoError(t, err)
}

func TestListRecords_Paginates(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			require.Equal(t, "1", r.URL.Query().Get("page"))
			_, _ = w.Write([]byte(makeRecordsPage(defaultPageSize, "pageLast")))
			return
		}
		require.Equal(t, "2", r.URL.Query().Get("page"))
		_, _ = w.Write([]byte(`{"items":[{"id":"last","title":"z"}]}`))
	}))
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "posts"})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Len(t, rows, defaultPageSize+1)
	require.Equal(t, "last", rows[defaultPageSize]["id"])
}

func TestListRecords_RespectsLimit(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "2", r.URL.Query().Get("perPage"))
		_, _ = w.Write([]byte(`{"items":[{"id":"a"},{"id":"b"}]}`))
	}))
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "posts", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestCountRecords(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/collections/posts/records", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("perPage"))
		// A count must not skip the total or project fields.
		require.Empty(t, r.URL.Query().Get("skipTotal"))
		require.Empty(t, r.URL.Query().Get("fields"))
		_, _ = w.Write([]byte(`{"page":1,"perPage":1,"totalItems":137,"totalPages":137,"items":[{"id":"x"}]}`))
	}))
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{CollectionID: "posts", Fields: "title"})
	require.NoError(t, err)
	require.EqualValues(t, 137, n)
}

func TestListCollections_SkipsSystem(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/collections", r.URL.Path)
		_, _ = w.Write([]byte(`{"page":1,"perPage":200,"items":[
			{"id":"c1","name":"posts","type":"base","system":false},
			{"id":"c2","name":"users","type":"auth","system":false},
			{"id":"c3","name":"_superusers","type":"auth","system":true}
		]}`))
	}))
	defer srv.Close()

	cols, err := c.ListCollections(context.Background())
	require.NoError(t, err)
	require.Len(t, cols, 2)
	require.Equal(t, "posts", cols[0].Name)
	require.Equal(t, "base", cols[0].Type)
	require.Equal(t, "users", cols[1].Name)
}

func TestCollectionFields_FiltersHiddenAndPassword(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/collections/users", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"c2","name":"users","type":"auth","fields":[
			{"name":"id","type":"text","system":true},
			{"name":"password","type":"password","system":true,"hidden":true},
			{"name":"tokenKey","type":"text","system":true,"hidden":true},
			{"name":"email","type":"email","system":true},
			{"name":"name","type":"text"},
			{"name":"created","type":"autodate"}
		]}`))
	}))
	defer srv.Close()

	fields, err := c.CollectionFields(context.Background(), "users")
	require.NoError(t, err)
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	require.Equal(t, []string{"id", "email", "name", "created"}, names)
}

func TestCollectionFields_SupportsLegacySchemaKey(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		// Older PocketBase used `schema` instead of `fields`.
		_, _ = w.Write([]byte(`{"id":"c1","name":"posts","type":"base","schema":[
			{"name":"title","type":"text"}
		]}`))
	}))
	defer srv.Close()

	fields, err := c.CollectionFields(context.Background(), "posts")
	require.NoError(t, err)
	require.Len(t, fields, 1)
	require.Equal(t, "title", fields[0].Name)
}

func TestCollectionFields_RequiresCollection(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	_, err := c.CollectionFields(context.Background(), "")
	require.Error(t, err)
}

func TestReauthenticatesOn401(t *testing.T) {
	authCalls := 0
	getCalls := 0
	c, srv := newTestClient(t, superuserSettings(), func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth-with-password") {
			authCalls++
			_, _ = w.Write([]byte(fmt.Sprintf(`{"token":"tok-%d"}`, authCalls)))
			return
		}
		getCalls++
		if getCalls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status":401,"message":"token expired"}`))
			return
		}
		// Second attempt uses the refreshed token.
		require.Equal(t, "tok-2", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"items":[{"id":"r1"}]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "posts"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 2, authCalls)
	require.Equal(t, 2, getCalls)
}

func TestTokenMode_UsesStaticTokenWithoutAuthCall(t *testing.T) {
	settings := Settings{AuthMode: AuthModeToken, authToken: "static-token"}
	c, srv := newTestClient(t, settings, func(w http.ResponseWriter, r *http.Request) {
		require.NotContains(t, r.URL.Path, "auth-with-password", "token mode must not call auth-with-password")
		require.Equal(t, "static-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"items":[{"id":"r1"}]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "posts"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestTokenMode_RequiresToken(t *testing.T) {
	settings := Settings{AuthMode: AuthModeToken}
	c, srv := newTestClient(t, settings, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "posts"})
	require.Error(t, err)
}

func TestPing(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/health":
			_, _ = w.Write([]byte(`{"code":200,"message":"API is healthy."}`))
		case strings.HasSuffix(r.URL.Path, "/auth-with-password"):
			_, _ = w.Write([]byte(`{"token":"tok"}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestPing_AuthFailure(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/health":
			_, _ = w.Write([]byte(`{"code":200}`))
		case strings.HasSuffix(r.URL.Path, "/auth-with-password"):
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":400,"message":"Failed to authenticate."}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()
	require.Error(t, c.Ping(context.Background()))
}

func TestStatusHint_Forbidden(t *testing.T) {
	c, srv := newTestClient(t, superuserSettings(), withAuth(testToken, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":403,"message":"Only superusers can perform this action."}`))
	}))
	defer srv.Close()

	_, err := c.ListCollections(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
	require.Contains(t, err.Error(), "superuser")
}

// makeRecordsPage builds a records response with exactly n items, the last one
// having the given id, so ListRecords keeps paging.
func makeRecordsPage(n int, lastID string) string {
	items := make([]string, n)
	for i := 0; i < n-1; i++ {
		items[i] = `{"id":"r","title":"a"}`
	}
	items[n-1] = fmt.Sprintf(`{"id":%q,"title":"a"}`, lastID)
	return `{"page":1,"perPage":200,"items":[` + strings.Join(items, ",") + `]}`
}
