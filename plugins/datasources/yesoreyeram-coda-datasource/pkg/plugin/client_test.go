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
	c, err := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", DocID: "docABC"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestListDocs(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/docs", r.URL.Path)
		require.Equal(t, "200", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"items":[{"id":"doc1","name":"My Doc"},{"id":"doc2","name":"Another Doc"}]}`))
	})
	defer srv.Close()

	docs, err := c.ListDocs(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 2)
	require.Equal(t, "doc1", docs[0].ID)
	require.Equal(t, "My Doc", docs[0].Title)
}

func TestListDocs_Paginates(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			require.Empty(t, r.URL.Query().Get("pageToken"))
			_, _ = w.Write([]byte(`{"items":[{"id":"doc1","name":"A"}],"nextPageToken":"tok2"}`))
			return
		}
		require.Equal(t, "tok2", r.URL.Query().Get("pageToken"))
		_, _ = w.Write([]byte(`{"items":[{"id":"doc2","name":"B"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	docs, err := c.ListDocs(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 2)
	require.Equal(t, 2, calls)
}

func TestListTables(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/docs/docABC/tables", r.URL.Path)
		_, _ = w.Write([]byte(`{"items":[{"id":"tbl1","name":"Table 1"},{"id":"tbl2","name":"Table 2"}]}`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, tables, 2)
	require.Equal(t, "tbl1", tables[0].ID)
	require.Equal(t, "Table 1", tables[0].Title)
}

func TestListTables_RequiresDoc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, _ := NewClient(Settings{apiToken: "tok"}, srv.Client())
	_, err := c.ListTables(context.Background(), "")
	require.Error(t, err)
}

func TestListColumns_ReadsDataTypeFromFormat(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/docs/docABC/tables/tbl1/columns", r.URL.Path)
		// The resource-level "type" is always "column"; the data type lives in
		// "format.type".
		_, _ = w.Write([]byte(`{"items":[
			{"id":"col1","name":"Name","type":"column","format":{"type":"text"}},
			{"id":"col2","name":"Age","type":"column","format":{"type":"number"}}
		]}`))
	})
	defer srv.Close()

	cols, err := c.ListColumns(context.Background(), "", "tbl1")
	require.NoError(t, err)
	require.Len(t, cols, 2)
	require.Equal(t, "Name", cols[0].Title)
	require.Equal(t, "text", cols[0].Type)
	require.Equal(t, "number", cols[1].Type)
}

func TestListRows_FlattensValuesMapAndMetadata(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/docs/docABC/tables/tblXYZ/rows", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("useColumnNames"))
		require.Equal(t, "simple", r.URL.Query().Get("valueFormat"))
		// Coda returns cells in a `values` map keyed by column name.
		_, _ = w.Write([]byte(`{"items":[
			{"id":"row1","name":"Row 1","index":7,"href":"https://coda.io/apis/v1/row1",
			 "browserLink":"https://coda.io/d/_dX#_rrow1","createdAt":"2024-01-02T03:04:05.000Z",
			 "updatedAt":"2024-02-03T04:05:06.000Z",
			 "values":{"Name":"Alice","Age":30}}
		]}`))
	})
	defer srv.Close()

	rows, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ", ValueFormat: "simple"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
	require.EqualValues(t, 30, rows[0]["Age"])
	require.Equal(t, "row1", rows[0]["id"])
	require.Equal(t, "Row 1", rows[0]["name"])
	require.EqualValues(t, 7, rows[0]["index"])
	require.Equal(t, "https://coda.io/apis/v1/row1", rows[0]["href"])
	require.Equal(t, "https://coda.io/d/_dX#_rrow1", rows[0]["browserLink"])
	require.Equal(t, "2024-01-02T03:04:05.000Z", rows[0]["createdAt"])
	require.Equal(t, "2024-02-03T04:05:06.000Z", rows[0]["updatedAt"])
}

func TestListRows_Paginates(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			require.Empty(t, r.URL.Query().Get("pageToken"))
			_, _ = w.Write([]byte(`{"items":[{"id":"r1","values":{}}],"nextPageToken":"p2"}`))
			return
		}
		require.Equal(t, "p2", r.URL.Query().Get("pageToken"))
		_, _ = w.Write([]byte(`{"items":[{"id":"r2","values":{}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", DocID: "docABC"}, srv.Client())

	rows, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 2, calls)
}

func TestListRows_RespectsLimit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "2", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"items":[{"id":"r1","values":{}},{"id":"r2","values":{}}],"nextPageToken":"more"}`))
	})
	defer srv.Close()

	rows, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestListRows_ForwardsRawQuery(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, `c-abc:"Apple"`, r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ", Query: `c-abc:"Apple"`})
	require.NoError(t, err)
}

func TestListRows_BuildsSingleColumnQuery(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// A column name (not a c- id) is quoted; a string value is JSON-quoted.
		require.Equal(t, `"Plan":"pro"`, r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ", FilterColumn: "Plan", FilterValue: "pro"})
	require.NoError(t, err)
}

func TestListRows_ForwardsSortByAndVisibleOnly(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "natural", r.URL.Query().Get("sortBy"))
		require.Equal(t, "true", r.URL.Query().Get("visibleOnly"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ", SortBy: "natural", VisibleOnly: true})
	require.NoError(t, err)
}

func TestListRows_IgnoresInvalidSortBy(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.URL.Query().Get("sortBy"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ", SortBy: "SomeColumn"})
	require.NoError(t, err)
}

func TestListRows_NeverSendsColumnIds(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Coda's rows endpoint has no column-projection parameter; projection is
		// applied client-side, so columnIds/columns must not be sent.
		require.Empty(t, r.URL.Query()["columnIds"])
		require.Empty(t, r.URL.Query().Get("columns"))
		_, _ = w.Write([]byte(`{"items":[
			{"id":"r1","values":{"Name":"Alice","Age":30,"Secret":"x"}}
		]}`))
	})
	defer srv.Close()

	rows, err := c.ListRows(context.Background(), QueryModel{TableID: "tblXYZ", Columns: "Name,Age"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	// Projection is applied client-side: Secret is dropped, metadata kept.
	require.Equal(t, "Alice", rows[0]["Name"])
	require.EqualValues(t, 30, rows[0]["Age"])
	_, hasSecret := rows[0]["Secret"]
	require.False(t, hasSecret)
	require.Equal(t, "r1", rows[0]["id"])
}

func TestCountRows_UnfilteredUsesTableRowCount(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Unfiltered count must hit GET /docs/{doc}/tables/{table}, not /rows.
		require.Equal(t, "/docs/docABC/tables/tblXYZ", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"tblXYZ","name":"T","rowCount":130}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", DocID: "docABC"}, srv.Client())

	n, err := c.CountRows(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.EqualValues(t, 130, n)
	require.Equal(t, 1, calls)
}

func TestCountRows_FilteredPaginatesAndCounts(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Filtered count paginates the rows endpoint applying the query filter.
		require.Equal(t, "/docs/docABC/tables/tblXYZ/rows", r.URL.Path)
		require.Equal(t, `"Plan":"pro"`, r.URL.Query().Get("query"))
		if calls == 1 {
			_, _ = w.Write([]byte(`{"items":[{"id":"r1"},{"id":"r2"}],"nextPageToken":"p2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"r3"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", DocID: "docABC"}, srv.Client())

	n, err := c.CountRows(context.Background(), QueryModel{TableID: "tblXYZ", FilterColumn: "Plan", FilterValue: "pro"})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
	require.Equal(t, 2, calls)
}

func TestPing(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/whoami", r.URL.Path)
		_, _ = w.Write([]byte(`{"name":"John Doe","loginId":"user@example.com"}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestListRows_RequiresDocAndTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, _ := NewClient(Settings{apiToken: "tok"}, srv.Client())
	_, err := c.ListRows(context.Background(), QueryModel{TableID: "tbl1"})
	require.Error(t, err)

	c2, _ := NewClient(Settings{apiToken: "tok", DocID: "docA"}, srv.Client())
	_, err = c2.ListRows(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestDo_ExtractsCodaMessageAndHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"statusCode":401,"statusMessage":"Unauthorized","message":"Invalid token"}`))
	})
	defer srv.Close()

	_, err := c.ListDocs(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "Invalid token")
	require.Contains(t, err.Error(), "API token")
}
