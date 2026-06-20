package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", DatabaseID: "1"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestListRecords(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Token tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/database/rows/table/1/", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("user_field_names"))
		_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":1,"Name":"Alice"}]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
}

func TestListRecords_ForwardsParams(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "-Age,Name", q.Get("order_by"))
		require.Equal(t, "Name,Age", q.Get("include"))
		require.Equal(t, "7", q.Get("view_id"))
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		TableID: "1", Sort: "-Age,Name", Fields: "Name,Age", ViewID: "7",
	})
	require.NoError(t, err)
}

func TestListRecords_ForwardsFilters(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		filters := r.URL.Query().Get("filters")
		require.Contains(t, filters, `"field":"Plan"`)
		require.Contains(t, filters, `"type":"equal"`)
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv.Close()

	q := QueryModel{TableID: "1", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "Plan", Op: "equal", Value: "pro"}},
	}}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestListRecords_Paginates(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			// First page reports a next URL, so the client requests page 2.
			_, _ = w.Write([]byte(`{"count":2,"next":"http://x/?page=2","results":[{"Name":"a"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"Name":"b"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", DatabaseID: "1"}, srv.Client())

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "1"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "a", rows[0]["Name"])
	require.Equal(t, "b", rows[1]["Name"])
}

func TestRecordsToFrame_DoesNotReorderSortedRows(t *testing.T) {
	// Rows returned in the order Baserow produced them (e.g. Age DESC) must be
	// preserved by the frame layer even when a time column is present.
	records := []map[string]any{
		{"Age": float64(40), "Name": "c", "CreatedAt": "2024-01-01T00:00:00Z"},
		{"Age": float64(30), "Name": "a", "CreatedAt": "2024-03-01T00:00:00Z"},
		{"Age": float64(20), "Name": "b", "CreatedAt": "2024-02-01T00:00:00Z"},
	}
	frame := recordsToFrame("A", records)

	ageField, _ := frame.FieldByName("Age")
	require.NotNil(t, ageField)
	require.EqualValues(t, 40, *ageField.At(0).(*float64))
	require.EqualValues(t, 30, *ageField.At(1).(*float64))
	require.EqualValues(t, 20, *ageField.At(2).(*float64))
}

func TestCountRecords(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/database/rows/table/1/", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("size"))
		require.Contains(t, r.URL.Query().Get("filters"), `"field":"Plan"`)
		_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"Name":"x"}]}`))
	})
	defer srv.Close()

	q := QueryModel{TableID: "1", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "Plan", Op: "equal", Value: "pro"}},
	}}
	n, err := c.CountRecords(context.Background(), q)
	require.NoError(t, err)
	require.EqualValues(t, 2, n)
}

// Token mode lists tables via the token-aware all-tables endpoint and filters by
// the requested database id client-side.
func TestListTables_TokenMode(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Token tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/database/tables/all-tables/", r.URL.Path)
		_, _ = w.Write([]byte(`[
			{"id":11,"name":"Users","database_id":9},
			{"id":12,"name":"Orders","database_id":9},
			{"id":13,"name":"Other","database_id":99}
		]`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "9")
	require.NoError(t, err)
	require.Len(t, tables, 2) // table from database 99 filtered out
	require.Equal(t, "11", tables[0].ID)
	require.Equal(t, "Users", tables[0].Title)
	require.Equal(t, "9", tables[0].DatabaseID)
}

func TestListTables_TokenMode_NoDatabaseIDReturnsAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/database/tables/all-tables/", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":11,"name":"Users","database_id":9},{"id":13,"name":"Other","database_id":99}]`))
	}))
	defer srv.Close()
	// No configured database id -> returns every accessible table.
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	tables, err := c.ListTables(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, tables, 2)
}

// Password (JWT) mode uses the per-database endpoint.
func TestListTables_PasswordMode(t *testing.T) {
	c, srv := newPasswordClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/token-auth/":
			_, _ = w.Write([]byte(`{"token":"jwt-1"}`))
		case "/api/database/tables/database/9/":
			require.Equal(t, "JWT jwt-1", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`[{"id":11,"name":"Users"}]`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "9")
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "9", tables[0].DatabaseID)
}

func TestListTables_PasswordMode_RequiresDatabaseID(t *testing.T) {
	c, srv := newPasswordClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/token-auth/" {
			_, _ = w.Write([]byte(`{"token":"jwt-1"}`))
			return
		}
		http.NotFound(w, r)
	})
	defer srv.Close()
	_, err := c.ListTables(context.Background(), "")
	require.Error(t, err)
}

func TestListFields(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/database/fields/table/11/", r.URL.Path)
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"Name","type":"text"},
			{"id":2,"name":"Age","type":"number"},
			{"id":3,"name":"","type":"text"}
		]`))
	})
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "11")
	require.NoError(t, err)
	require.Len(t, fields, 2) // unnamed field skipped
	require.Equal(t, "Name", fields[0].Title)
	require.Equal(t, "text", fields[0].Type)
	require.Equal(t, "Age", fields[1].Title)
	require.Equal(t, "number", fields[1].Type)
}

func TestListFields_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListFields(context.Background(), "")
	require.Error(t, err)
}

// Password (JWT) mode can list views.
func TestListViews_PasswordMode(t *testing.T) {
	c, srv := newPasswordClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/token-auth/":
			_, _ = w.Write([]byte(`{"token":"jwt-1"}`))
		case "/api/database/views/table/11/":
			require.Equal(t, "JWT jwt-1", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`[{"id":21,"name":"Grid"},{"id":22,"name":"Gallery"}]`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	views, err := c.ListViews(context.Background(), "11")
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "21", views[0].ID)
	require.Equal(t, "Grid", views[0].Title)
}

// Token mode cannot list views (the endpoint rejects database tokens); the
// client returns an empty list without calling the API.
func TestListViews_TokenModeReturnsEmpty(t *testing.T) {
	called := false
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	defer srv.Close()

	views, err := c.ListViews(context.Background(), "11")
	require.NoError(t, err)
	require.Empty(t, views)
	require.False(t, called, "token mode must not hit the views endpoint")
}

func TestListViews_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListViews(context.Background(), "")
	require.Error(t, err)
}

func TestHTMLResponseHint(t *testing.T) {
	require.NotEmpty(t, htmlResponseHint([]byte(`<!doctype html><html><head><title>Site not found</title></head></html>`)))
	require.NotEmpty(t, htmlResponseHint([]byte(`<html><body>nope</body></html>`)))
	require.Empty(t, htmlResponseHint([]byte(`{"count":0}`)))
	require.Empty(t, htmlResponseHint([]byte(``)))
}

func TestListTables_HTMLResponseGivesHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Site not found | Baserow</title></head></html>`))
	})
	defer srv.Close()

	_, err := c.ListTables(context.Background(), "1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTML page, not JSON")
}

func TestListTables_401GivesHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Authentication credentials were not provided."}`))
	})
	defer srv.Close()

	_, err := c.ListTables(context.Background(), "1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "database token")
}

func TestUnauthorizedHint(t *testing.T) {
	require.Contains(t, unauthorizedHint(AuthToken), "database token")
	require.Contains(t, unauthorizedHint(AuthPassword), "Email/password")
}

func TestRedirectPreservesAuthorizationHeader(t *testing.T) {
	// Simulate Baserow redirecting to a canonical host. Go strips Authorization
	// on cross-host redirects; the client's CheckRedirect must re-attach it.
	var finalAuth string
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		finalAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[{"id":1,"name":"Users"}]`))
	}))
	defer final.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	c, err := NewClient(
		Settings{BaseURL: redirector.URL, apiToken: "tok", DatabaseID: "1"},
		redirector.Client(),
	)
	require.NoError(t, err)

	_, err = c.ListTables(context.Background(), "1")
	require.NoError(t, err)
	require.Equal(t, "Token tok", finalAuth)
}

func TestPing_TokenMode(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Token tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/database/tokens/check/", r.URL.Path)
		_, _ = w.Write([]byte(`{"token":"valid"}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

// --- password (JWT) auth mode ---------------------------------------------

func newPasswordClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := NewClient(
		Settings{BaseURL: srv.URL, AuthMode: AuthPassword, Email: "a@b.com", password: "pw"},
		srv.Client(),
	)
	require.NoError(t, err)
	return c, srv
}

func TestPasswordAuth_AuthenticatesThenSendsJWT(t *testing.T) {
	var authCalls, listCalls int
	c, srv := newPasswordClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/token-auth/":
			authCalls++
			require.Equal(t, http.MethodPost, r.Method)
			body, _ := io.ReadAll(r.Body)
			require.Contains(t, string(body), `"email":"a@b.com"`)
			_, _ = w.Write([]byte(`{"token":"jwt-123"}`))
		case "/api/database/rows/table/1/":
			listCalls++
			require.Equal(t, "JWT jwt-123", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableID: "1"})
	require.NoError(t, err)
	// A second call reuses the cached JWT (no re-auth).
	_, err = c.ListRecords(context.Background(), QueryModel{TableID: "1"})
	require.NoError(t, err)
	require.Equal(t, 1, authCalls)
	require.Equal(t, 2, listCalls)
}

func TestPasswordAuth_RefreshesOn401(t *testing.T) {
	var authCalls, listCalls int
	c, srv := newPasswordClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/token-auth/":
			authCalls++
			tok := "jwt-old"
			if authCalls == 2 {
				tok = "jwt-new"
			}
			_, _ = fmt.Fprintf(w, `{"token":%q}`, tok)
		case "/api/database/rows/table/1/":
			listCalls++
			if r.Header.Get("Authorization") == "JWT jwt-old" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"expired"}`))
				return
			}
			require.Equal(t, "JWT jwt-new", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableID: "1"})
	require.NoError(t, err)
	require.Equal(t, 2, authCalls) // initial + refresh after 401
	require.Equal(t, 2, listCalls) // first 401, then success
}

func TestListDatabases(t *testing.T) {
	c, srv := newPasswordClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/token-auth/":
			_, _ = w.Write([]byte(`{"token":"jwt-1"}`))
		case "/api/workspaces/":
			_, _ = w.Write([]byte(`[{"id":7,"name":"Acme"}]`))
		case "/api/applications/workspace/7/":
			_, _ = w.Write([]byte(`[{"id":11,"name":"Sales","type":"database"},{"id":12,"name":"Site","type":"builder"}]`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	dbs, err := c.ListDatabases(context.Background())
	require.NoError(t, err)
	require.Len(t, dbs, 1) // builder app filtered out
	require.Equal(t, "11", dbs[0].ID)
	require.Equal(t, "Sales", dbs[0].Title)
	require.Equal(t, "Acme", dbs[0].WorkspaceName)
}

func TestListDatabases_RequiresPasswordAuth(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListDatabases(context.Background())
	require.Error(t, err)
}

func TestPing_PasswordMode(t *testing.T) {
	c, srv := newPasswordClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/token-auth/":
			_, _ = w.Write([]byte(`{"token":"jwt-1"}`))
		case "/api/workspaces/":
			require.Equal(t, "JWT jwt-1", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}
