package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := NewClient(Settings{BaseURL: srv.URL, apiKey: "key", DocID: "docABC"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

// writeColumns responds to the columns metadata request used by ListRecords to
// classify Date/DateTime columns. Returns the given columns map (id -> type).
func columnsJSON(cols map[string]string) string {
	type col struct {
		ID     string `json:"id"`
		Fields struct {
			Type string `json:"type"`
		} `json:"fields"`
	}
	out := struct {
		Columns []col `json:"columns"`
	}{}
	for id, typ := range cols {
		c := col{ID: id}
		c.Fields.Type = typ
		out.Columns = append(out.Columns, c)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func TestNewClient_NormalizesAPISuffix(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://team.getgrist.com/api/", apiKey: "k"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://team.getgrist.com", c.baseURL)
	require.Equal(t, "https://team.getgrist.com/api/orgs", c.apiURL("/orgs"))
}

func TestNewClient_RequiresBaseURL(t *testing.T) {
	_, err := NewClient(Settings{apiKey: "k"}, http.DefaultClient)
	require.Error(t, err)
}

func TestListRecords_RecordsEndpoint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/docs/docABC/tables/t1/columns":
			_, _ = w.Write([]byte(columnsJSON(map[string]string{"Name": "Text"})))
		case "/api/docs/docABC/tables/t1/records":
			require.Equal(t, http.MethodGet, r.Method)
			_, _ = w.Write([]byte(`{"records":[{"id":1,"fields":{"Name":"Alice"}}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	rows, dateCols, err := c.ListRecords(context.Background(), QueryModel{TableID: "t1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
	require.EqualValues(t, 1, rows[0]["id"])
	require.Empty(t, dateCols)
}

func TestListRecords_RequiresDocAndTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiKey: "key"}, srv.Client())
	_, _, err := c.ListRecords(context.Background(), QueryModel{TableID: "t1"})
	require.Error(t, err)

	c2, _ := NewClient(Settings{BaseURL: srv.URL, apiKey: "key", DocID: "docA"}, srv.Client())
	_, _, err = c2.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_ForwardsSimpleFilterSortLimit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/docs/docABC/tables/t1/columns":
			_, _ = w.Write([]byte(columnsJSON(map[string]string{"Plan": "Text", "Age": "Int"})))
		case "/api/docs/docABC/tables/t1/records":
			q := r.URL.Query()
			require.JSONEq(t, `{"Plan":["pro"]}`, q.Get("filter"))
			require.Equal(t, "-Age", q.Get("sort"))
			require.Equal(t, "5", q.Get("limit"))
			_, _ = w.Write([]byte(`{"records":[]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	q := QueryModel{
		TableID: "t1",
		Limit:   5,
		filter: &FilterNode{Kind: "group", Connector: "and", Children: []FilterNode{
			{Kind: "condition", Field: "Plan", Op: "eq", Value: "pro"},
		}},
		sortItems: []SortItem{{Field: "Age", Direction: "desc"}},
	}
	_, _, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestListRecords_NoLimitOmitsLimitParam(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/docs/docABC/tables/t1/columns":
			_, _ = w.Write([]byte(columnsJSON(nil)))
		case "/api/docs/docABC/tables/t1/records":
			// limit must NOT be sent when 0 (Grist returns all rows).
			require.Equal(t, "", r.URL.Query().Get("limit"))
			require.Equal(t, "", r.URL.Query().Get("offset"))
			_, _ = w.Write([]byte(`{"records":[]}`))
		}
	})
	defer srv.Close()

	_, _, err := c.ListRecords(context.Background(), QueryModel{TableID: "t1"})
	require.NoError(t, err)
}

func TestListRecords_DateColumnsClassified(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/docs/docABC/tables/t1/columns":
			_, _ = w.Write([]byte(columnsJSON(map[string]string{
				"When":    "Date",
				"Started": "DateTime:America/New_York",
				"Name":    "Text",
			})))
		case "/api/docs/docABC/tables/t1/records":
			_, _ = w.Write([]byte(`{"records":[{"id":1,"fields":{"When":1705276800,"Name":"a"}}]}`))
		}
	})
	defer srv.Close()

	rows, dateCols, err := c.ListRecords(context.Background(), QueryModel{TableID: "t1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.True(t, dateCols["When"])
	require.True(t, dateCols["Started"])
	require.False(t, dateCols["Name"])
	// Epoch seconds preserved as a number for the frame builder to convert.
	require.EqualValues(t, 1705276800, rows[0]["When"])
}

func TestListRecords_ViaSQLWhenComplexFilter(t *testing.T) {
	var body map[string]any
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/docs/docABC/tables/t1/columns":
			_, _ = w.Write([]byte(columnsJSON(map[string]string{"Age": "Int"})))
		case "/api/docs/docABC/sql":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			_, _ = w.Write([]byte(`{"records":[{"fields":{"id":1,"Age":40}}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	q := QueryModel{TableID: "t1", filter: &FilterNode{Kind: "group", Connector: "and", Children: []FilterNode{
		{Kind: "condition", Field: "Age", Op: "gt", Value: "30"},
	}}}
	rows, _, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	sql, _ := body["sql"].(string)
	require.Contains(t, sql, `SELECT * FROM "t1"`)
	require.Contains(t, sql, `WHERE "Age" > ?`)
	require.Equal(t, []any{float64(30)}, body["args"])
}

func TestListRecords_ViaSQLWhenFieldsProjection(t *testing.T) {
	var body map[string]any
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/docs/docABC/tables/t1/columns":
			_, _ = w.Write([]byte(columnsJSON(map[string]string{"Name": "Text", "Age": "Int"})))
		case "/api/docs/docABC/sql":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			_, _ = w.Write([]byte(`{"records":[]}`))
		}
	})
	defer srv.Close()

	q := QueryModel{TableID: "t1", Fields: "Name, Age", sortItems: []SortItem{{Field: "Age", Direction: "desc"}}}
	_, _, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
	sql := body["sql"].(string)
	require.Contains(t, sql, `SELECT "Name", "Age" FROM "t1"`)
	require.Contains(t, sql, `ORDER BY "Age" DESC`)
	require.NotContains(t, body, "args")
}

func TestCountRecords_ViaSQL(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/docs/docABC/sql", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, _ = w.Write([]byte(`{"records":[{"fields":{"count":42}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiKey: "key", DocID: "docABC"}, srv.Client())

	n, err := c.CountRecords(context.Background(), QueryModel{TableID: "t1"})
	require.NoError(t, err)
	require.EqualValues(t, 42, n)
	require.Contains(t, body["sql"].(string), `SELECT COUNT(*) AS count FROM "t1"`)
}

func TestCountRecords_WithFilterArgs(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, _ = w.Write([]byte(`{"records":[{"fields":{"count":3}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiKey: "key", DocID: "docABC"}, srv.Client())

	q := QueryModel{TableID: "t1", filter: &FilterNode{Kind: "group", Connector: "and", Children: []FilterNode{
		{Kind: "condition", Field: "Plan", Op: "contains", Value: "pro"},
	}}}
	n, err := c.CountRecords(context.Background(), q)
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
	require.Contains(t, body["sql"].(string), `WHERE "Plan" LIKE ?`)
	require.Equal(t, []any{"%pro%"}, body["args"])
}

func TestRunSQL(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/docs/docABC/sql", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, _ = w.Write([]byte(`{"statement":"...","records":[{"fields":{"city":"Paris","n":3}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiKey: "key", DocID: "docABC"}, srv.Client())

	rows, err := c.RunSQL(context.Background(), QueryModel{SQL: "SELECT city, COUNT(*) AS n FROM Contacts GROUP BY city;"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Paris", rows[0]["city"])
	// Trailing semicolon stripped.
	require.Equal(t, "SELECT city, COUNT(*) AS n FROM Contacts GROUP BY city", body["sql"])
	require.NotContains(t, body, "args")
}

func TestRunSQL_RequiresSQL(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.RunSQL(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListDocs_EnumeratesOrgsAndWorkspaces(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/orgs":
			_, _ = w.Write([]byte(`[{"id":42,"name":"Labs","domain":"labs"}]`))
		case "/api/orgs/42/workspaces":
			_, _ = w.Write([]byte(`[{"id":1,"name":"WS","docs":[{"id":"doc1","name":"Sales"},{"id":"doc2","name":"Ops"}]}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	docs, err := c.ListDocs(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 2)
	require.Equal(t, "doc1", docs[0].ID)
	require.Equal(t, "Sales", docs[0].Title)
}

func TestListTables(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/docs/docABC/tables", r.URL.Path)
		_, _ = w.Write([]byte(`{"tables":[{"id":"Users","fields":{"tableRef":1}},{"id":"Orders","fields":{"tableRef":2}}]}`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, tables, 2)
	require.Equal(t, "Users", tables[0].ID)
	require.Equal(t, "Users", tables[0].Title)
}

func TestListFields_ParsesNestedFields(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/docs/docABC/tables/t1/columns", r.URL.Path)
		_, _ = w.Write([]byte(`{"columns":[{"id":"Name","fields":{"label":"Name","type":"Text"}},{"id":"When","fields":{"label":"When","type":"Date"}}]}`))
	})
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "", "t1")
	require.NoError(t, err)
	require.Len(t, fields, 2)
	require.Equal(t, "Name", fields[0].Title)
	require.Equal(t, "Text", fields[0].Type)
	require.Equal(t, "When", fields[1].Title)
	require.Equal(t, "Date", fields[1].Type)
}

func TestListFields_RequiresDocID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiKey: "key"}, srv.Client())
	_, err := c.ListFields(context.Background(), "", "t1")
	require.Error(t, err)
}

func TestPing_UsesOrgs(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		require.Equal(t, "/api/orgs", r.URL.Path)
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestDo_SurfacesGristError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Table not found"}`))
	})
	defer srv.Close()
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
	require.Contains(t, err.Error(), "Table not found")
}
