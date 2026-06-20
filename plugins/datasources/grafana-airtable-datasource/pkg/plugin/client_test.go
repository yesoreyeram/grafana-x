package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", BaseID: "appABC"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestListRecords(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/v0/appABC/tblXYZ", r.URL.Path)
		require.Equal(t, "100", r.URL.Query().Get("pageSize"))
		_, _ = w.Write([]byte(`{"records":[{"id":"rec1","createdTime":"2024-01-01T00:00:00.000Z","fields":{"Name":"Alice"}}]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
	require.Equal(t, "rec1", rows[0]["_id"])
	require.Equal(t, "2024-01-01T00:00:00.000Z", rows[0]["_createdTime"])
}

func TestListRecords_RequiresBaseAndTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	// No base configured and none in the query.
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())
	_, err := c.ListRecords(context.Background(), QueryModel{TableID: "tbl1"})
	require.Error(t, err)

	c2, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", BaseID: "appA"}, srv.Client())
	_, err = c2.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_ForwardsParams(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "Grid view", q.Get("view"))
		require.Equal(t, []string{"Name", "Age"}, q["fields[]"])
		require.Equal(t, "Age", q.Get("sort[0][field]"))
		require.Equal(t, "desc", q.Get("sort[0][direction]"))
		require.Equal(t, "Name", q.Get("sort[1][field]"))
		require.Equal(t, "asc", q.Get("sort[1][direction]"))
		_, _ = w.Write([]byte(`{"records":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		TableID: "tblXYZ",
		ViewID:  "Grid view",
		Fields:  "Name,Age",
		sortItems: []SortItem{
			{Field: "Age", Direction: "desc"},
			{Field: "Name", Direction: "asc"},
		},
	})
	require.NoError(t, err)
}

func TestListRecords_ForwardsFormula(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		formula := r.URL.Query().Get("filterByFormula")
		require.Equal(t, `{Plan} = "pro"`, formula)
		_, _ = w.Write([]byte(`{"records":[]}`))
	})
	defer srv.Close()

	q := QueryModel{TableID: "tblXYZ", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "Plan", Op: "eq", Value: "pro"}},
	}}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestListRecords_RawFormulaTakesPrecedence(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, `NOT({Done})`, r.URL.Query().Get("filterByFormula"))
		_, _ = w.Write([]byte(`{"records":[]}`))
	})
	defer srv.Close()

	q := QueryModel{
		TableID:         "tblXYZ",
		FilterByFormula: "NOT({Done})",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "Plan", Op: "eq", Value: "pro"}},
		},
	}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestListRecords_Paginates(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			require.Empty(t, r.URL.Query().Get("offset"))
			_, _ = w.Write([]byte(`{"records":[{"id":"r1","fields":{"Name":"a"}}],"offset":"off2"}`))
			return
		}
		require.Equal(t, "off2", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"records":[{"id":"r2","fields":{"Name":"b"}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", BaseID: "appABC"}, srv.Client())

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "a", rows[0]["Name"])
	require.Equal(t, "b", rows[1]["Name"])
	require.Equal(t, 2, calls)
}

func TestListRecords_RespectsLimit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// limit 2 -> pageSize capped at 2.
		require.Equal(t, "2", r.URL.Query().Get("pageSize"))
		_, _ = w.Write([]byte(`{"records":[{"id":"r1","fields":{}},{"id":"r2","fields":{}}],"offset":"more"}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2) // stops at limit even though offset is returned
}

func TestCountRecords(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/v0/appABC/tblXYZ", r.URL.Path)
		require.Equal(t, []string{""}, r.URL.Query()["fields[]"])
		if calls == 1 {
			_, _ = w.Write([]byte(`{"records":[{"id":"r1"},{"id":"r2"}],"offset":"o"}`))
			return
		}
		_, _ = w.Write([]byte(`{"records":[{"id":"r3"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", BaseID: "appABC"}, srv.Client())

	n, err := c.CountRecords(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
}

func TestListBases(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v0/meta/bases", r.URL.Path)
		_, _ = w.Write([]byte(`{"bases":[{"id":"app1","name":"Sales","permissionLevel":"read"},{"id":"app2","name":"Ops"}]}`))
	})
	defer srv.Close()

	bases, err := c.ListBases(context.Background())
	require.NoError(t, err)
	require.Len(t, bases, 2)
	require.Equal(t, "app1", bases[0].ID)
	require.Equal(t, "Sales", bases[0].Title)
}

func TestListBases_Paginates(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"bases":[{"id":"app1","name":"A"}],"offset":"next"}`))
			return
		}
		require.Equal(t, "next", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"bases":[{"id":"app2","name":"B"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	bases, err := c.ListBases(context.Background())
	require.NoError(t, err)
	require.Len(t, bases, 2)
}

func schemaHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v0/meta/bases/appABC/tables" {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write([]byte(`{"tables":[
		{"id":"tbl1","name":"Users","fields":[
			{"id":"fld1","name":"Name","type":"singleLineText"},
			{"id":"fld2","name":"Age","type":"number"},
			{"id":"fld3","name":"","type":"singleLineText"}
		],"views":[
			{"id":"viw1","name":"Grid","type":"grid"},
			{"id":"viw2","name":"Gallery","type":"gallery"}
		]},
		{"id":"tbl2","name":"Orders","fields":[],"views":[]}
	]}`))
}

func TestListTables(t *testing.T) {
	c, srv := newTestClient(t, schemaHandler)
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, tables, 2)
	require.Equal(t, "tbl1", tables[0].ID)
	require.Equal(t, "Users", tables[0].Title)
}

func TestListFields_ByTableID(t *testing.T) {
	c, srv := newTestClient(t, schemaHandler)
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "", "tbl1")
	require.NoError(t, err)
	require.Len(t, fields, 2) // unnamed field skipped
	require.Equal(t, "Name", fields[0].Title)
	require.Equal(t, "singleLineText", fields[0].Type)
	require.Equal(t, "Age", fields[1].Title)
}

func TestListFields_ByTableName(t *testing.T) {
	c, srv := newTestClient(t, schemaHandler)
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "", "Users")
	require.NoError(t, err)
	require.Len(t, fields, 2)
}

func TestListFields_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, schemaHandler)
	defer srv.Close()
	_, err := c.ListFields(context.Background(), "", "")
	require.Error(t, err)
}

func TestListViews(t *testing.T) {
	c, srv := newTestClient(t, schemaHandler)
	defer srv.Close()

	views, err := c.ListViews(context.Background(), "", "tbl1")
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "viw1", views[0].ID)
	require.Equal(t, "Grid", views[0].Title)
}

func TestListViews_UnknownTableReturnsEmpty(t *testing.T) {
	c, srv := newTestClient(t, schemaHandler)
	defer srv.Close()

	views, err := c.ListViews(context.Background(), "", "tblMissing")
	require.NoError(t, err)
	require.Empty(t, views)
}

func TestPing(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/v0/meta/whoami", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"usr1"}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestStatusHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"AUTHENTICATION_REQUIRED"}}`))
	})
	defer srv.Close()

	_, err := c.ListBases(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "personal access token")
}
