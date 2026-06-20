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
	c, err := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestListRecords_V2(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/tables/m1/records", r.URL.Path)
		_, _ = w.Write([]byte(`{"list":[{"Id":1,"Name":"Alice"}],"pageInfo":{"isLastPage":true}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "m1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
}

func TestListRecords_ForwardsSortParam(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "-Age,Name", r.URL.Query().Get("sort"))
		_, _ = w.Write([]byte(`{"list":[],"pageInfo":{"isLastPage":true}}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableID: "m1", Sort: "-Age,Name"})
	require.NoError(t, err)
}

func TestRecordsToFrame_DoesNotReorderSortedRows(t *testing.T) {
	// Records returned in the order NocoDB produced them (e.g. Age DESC) must be
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

func TestListRecords_V3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/data/p1/m1/records", r.URL.Path)
		_, _ = w.Write([]byte(`{"records":[{"id":1,"id_fields":{"Id":1},"fields":{"Name":"Alice","Age":30}}],"next":null}`))
	}))
	defer srv.Close()
	c, err := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", APIVersion: "v3"}, srv.Client())
	require.NoError(t, err)

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "m1", BaseID: "p1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
	require.EqualValues(t, 1, rows[0]["Id"]) // merged from id_fields
}

func TestCountRecords_V2(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/tables/m1/records/count", r.URL.Path)
		require.Equal(t, `@("Plan",eq,"pro")`, r.URL.Query().Get("where"))
		_, _ = w.Write([]byte(`{"count":2}`))
	})
	defer srv.Close()

	q := QueryModel{TableID: "m1", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "Plan", Op: "eq", Value: "pro"}},
	}}
	n, err := c.CountRecords(context.Background(), q)
	require.NoError(t, err)
	require.EqualValues(t, 2, n)
}

func TestCountRecords_V3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/data/p1/m1/count", r.URL.Path)
		_, _ = w.Write([]byte(`{"count":7}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", APIVersion: "v3"}, srv.Client())

	n, err := c.CountRecords(context.Background(), QueryModel{TableID: "m1", BaseID: "p1"})
	require.NoError(t, err)
	require.EqualValues(t, 7, n)
}

func TestListRecords_V3FallsBackToV2WithoutBaseID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/tables/m1/records", r.URL.Path)
		_, _ = w.Write([]byte(`{"list":[{"Name":"Bob"}],"pageInfo":{"isLastPage":true}}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", APIVersion: "v3"}, srv.Client())

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "m1"})
	require.NoError(t, err)
	require.Equal(t, "Bob", rows[0]["Name"])
}

func TestListTables(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tok", r.Header.Get("xc-token"))
		require.Equal(t, "/api/v2/meta/bases/p_1/tables", r.URL.Path)
		_, _ = w.Write([]byte(`{"list":[{"id":"m_1","title":"Users"},{"id":"m_2","title":"Orders"}]}`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "p_1")
	require.NoError(t, err)
	require.Len(t, tables, 2)
	require.Equal(t, "m_1", tables[0].ID)
	require.Equal(t, "Users", tables[0].Title)
	require.Equal(t, "p_1", tables[0].BaseID)
}

func TestListTables_RequiresBaseID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListTables(context.Background(), "")
	require.Error(t, err)
}

func TestListFields_ExcludesSystemFields(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/meta/tables/m_1", r.URL.Path)
		_, _ = w.Write([]byte(`{"columns":[
			{"title":"Id","uidt":"ID","system":0,"pk":1},
			{"title":"CreatedAt","uidt":"CreatedTime","system":1},
			{"title":"nc_order","uidt":"Order","system":1},
			{"title":"nc_created_by","uidt":"CreatedBy","system":true},
			{"title":"Title","uidt":"SingleLineText","system":null},
			{"title":"Age","uidt":"Number"},
			{"title":""}
		]}`))
	})
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "m_1")
	require.NoError(t, err)
	require.Len(t, fields, 2) // only user fields survive
	require.Equal(t, "Title", fields[0].Title)
	require.Equal(t, "SingleLineText", fields[0].Type)
	require.Equal(t, "Age", fields[1].Title)
}

func TestListFields_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListFields(context.Background(), "")
	require.Error(t, err)
}

func TestListViews(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/meta/tables/m_1/views", r.URL.Path)
		_, _ = w.Write([]byte(`{"list":[{"id":"vw_1","title":"Grid"},{"id":"vw_2","title":"Gallery"}]}`))
	})
	defer srv.Close()

	views, err := c.ListViews(context.Background(), "m_1")
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "vw_1", views[0].ID)
	require.Equal(t, "Grid", views[0].Title)
}

func TestListViews_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListViews(context.Background(), "")
	require.Error(t, err)
}

func TestListAllTables(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/meta/bases":
			_, _ = w.Write([]byte(`{"list":[{"id":"p_1","title":"Sales"},{"id":"p_2","title":"HR"}]}`))
		case "/api/v2/meta/bases/p_1/tables":
			_, _ = w.Write([]byte(`{"list":[{"id":"m_1","title":"Leads"}]}`))
		case "/api/v2/meta/bases/p_2/tables":
			_, _ = w.Write([]byte(`{"list":[{"id":"m_2","title":"Staff"}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	tables, err := c.ListAllTables(context.Background())
	require.NoError(t, err)
	require.Len(t, tables, 2)

	byID := map[string]TableInfo{}
	for _, tbl := range tables {
		byID[tbl.ID] = tbl
	}
	require.Equal(t, "Sales", byID["m_1"].BaseTitle)
	require.Equal(t, "p_1", byID["m_1"].BaseID)
	require.Equal(t, "HR", byID["m_2"].BaseTitle)
}
