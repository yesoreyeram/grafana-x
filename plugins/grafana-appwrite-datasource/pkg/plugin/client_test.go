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
	c, err := NewClient(Settings{Endpoint: srv.URL, ProjectID: "proj", apiKey: "key", DatabaseID: "dbMain"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestListDocuments(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "proj", r.Header.Get("X-Appwrite-Project"))
		require.Equal(t, "key", r.Header.Get("X-Appwrite-Key"))
		require.Equal(t, "/databases/dbMain/collections/colUsers/documents", r.URL.Path)
		require.Contains(t, r.URL.Query()["queries[]"], `{"method":"limit","values":[100]}`)
		_, _ = w.Write([]byte(`{"total":1,"documents":[{"$id":"d1","$createdAt":"2024-01-01T00:00:00.000+00:00","name":"Alice"}]}`))
	})
	defer srv.Close()

	rows, err := c.ListDocuments(context.Background(), QueryModel{CollectionID: "colUsers"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["name"])
	require.Equal(t, "d1", rows[0]["$id"])
}

func TestListDocuments_RequiresDatabaseAndCollection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	// No database configured and none in the query.
	c, _ := NewClient(Settings{Endpoint: srv.URL, ProjectID: "p", apiKey: "k"}, srv.Client())
	_, err := c.ListDocuments(context.Background(), QueryModel{CollectionID: "col1"})
	require.Error(t, err)

	c2, _ := NewClient(Settings{Endpoint: srv.URL, ProjectID: "p", apiKey: "k", DatabaseID: "dbA"}, srv.Client())
	_, err = c2.ListDocuments(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListDocuments_ForwardsQueries(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()["queries[]"]
		require.Contains(t, q, `{"method":"orderDesc","attribute":"age"}`)
		require.Contains(t, q, `{"method":"orderAsc","attribute":"name"}`)
		require.Contains(t, q, `{"method":"select","values":["name","age","$id","$createdAt","$updatedAt"]}`)
		require.Contains(t, q, `{"method":"equal","attribute":"status","values":["active"]}`)
		_, _ = w.Write([]byte(`{"total":0,"documents":[]}`))
	})
	defer srv.Close()

	_, err := c.ListDocuments(context.Background(), QueryModel{
		CollectionID: "colUsers",
		Attributes:   "name,age",
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

func TestListDocuments_RawQueriesTakePrecedence(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()["queries[]"]
		require.Contains(t, q, `{"method":"equal","attribute":"raw","values":[1]}`)
		// The structured filter must NOT be present.
		require.NotContains(t, q, `{"method":"equal","attribute":"status","values":["active"]}`)
		_, _ = w.Write([]byte(`{"total":0,"documents":[]}`))
	})
	defer srv.Close()

	_, err := c.ListDocuments(context.Background(), QueryModel{
		CollectionID: "colUsers",
		RawQueries:   "{\"method\":\"equal\",\"attribute\":\"raw\",\"values\":[1]}\n",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Attribute: "status", Op: "equal", Value: "active"}},
		},
	})
	require.NoError(t, err)
}

func TestListDocuments_Paginates(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()["queries[]"]
		if calls == 1 {
			require.NotContains(t, q, `{"method":"cursorAfter","values":["page1Last"]}`)
			// Return a full page so pagination continues. We use pageSize 100, but
			// to keep the test small we report fewer by forcing limit smaller.
			_, _ = w.Write([]byte(makeFullPage("page1Last")))
			return
		}
		require.Contains(t, q, `{"method":"cursorAfter","values":["page1Last"]}`)
		_, _ = w.Write([]byte(`{"total":101,"documents":[{"$id":"last","name":"z"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{Endpoint: srv.URL, ProjectID: "p", apiKey: "k", DatabaseID: "dbMain"}, srv.Client())

	rows, err := c.ListDocuments(context.Background(), QueryModel{CollectionID: "colUsers"})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Len(t, rows, 101)
	require.Equal(t, "last", rows[100]["$id"])
}

func TestListDocuments_RespectsLimit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// limit 2 -> page size capped at 2.
		require.Contains(t, r.URL.Query()["queries[]"], `{"method":"limit","values":[2]}`)
		_, _ = w.Write([]byte(`{"total":50,"documents":[{"$id":"a"},{"$id":"b"}]}`))
	})
	defer srv.Close()

	rows, err := c.ListDocuments(context.Background(), QueryModel{CollectionID: "colUsers", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestCountDocuments(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/databases/dbMain/collections/colUsers/documents", r.URL.Path)
		q := r.URL.Query()["queries[]"]
		require.Contains(t, q, `{"method":"limit","values":[1]}`)
		// select projection must not be applied to a count.
		for _, s := range q {
			require.NotContains(t, s, `"method":"select"`)
		}
		_, _ = w.Write([]byte(`{"total":137,"documents":[{"$id":"x"}]}`))
	})
	defer srv.Close()

	n, err := c.CountDocuments(context.Background(), QueryModel{CollectionID: "colUsers", Attributes: "name"})
	require.NoError(t, err)
	require.EqualValues(t, 137, n)
}

func TestListDatabases(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/databases", r.URL.Path)
		_, _ = w.Write([]byte(`{"total":2,"databases":[{"$id":"db1","name":"Sales"},{"$id":"db2","name":"Ops"}]}`))
	})
	defer srv.Close()

	dbs, err := c.ListDatabases(context.Background())
	require.NoError(t, err)
	require.Len(t, dbs, 2)
	require.Equal(t, "db1", dbs[0].ID)
	require.Equal(t, "Sales", dbs[0].Name)
}

func TestListCollections(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/databases/dbMain/collections", r.URL.Path)
		_, _ = w.Write([]byte(`{"total":1,"collections":[{"$id":"col1","name":"Users"}]}`))
	})
	defer srv.Close()

	cols, err := c.ListCollections(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "col1", cols[0].ID)
	require.Equal(t, "Users", cols[0].Name)
}

func TestListAttributes(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/databases/dbMain/collections/col1/attributes", r.URL.Path)
		_, _ = w.Write([]byte(`{"total":3,"attributes":[
			{"key":"name","type":"string"},
			{"key":"age","type":"integer"},
			{"key":"","type":"string"}
		]}`))
	})
	defer srv.Close()

	attrs, err := c.ListAttributes(context.Background(), "", "col1")
	require.NoError(t, err)
	require.Len(t, attrs, 2) // empty-key attribute skipped
	require.Equal(t, "name", attrs[0].Key)
	require.Equal(t, "string", attrs[0].Type)
	require.Equal(t, "age", attrs[1].Key)
}

func TestListAttributes_RequiresCollection(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListAttributes(context.Background(), "", "")
	require.Error(t, err)
}

func TestListDatabases_FallsBackToTablesDB(t *testing.T) {
	// Databases created via the newer TablesDB API are not returned by the legacy
	// /databases list endpoint (it responds empty); the client must fall back to
	// /tablesdb.
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/databases":
			_, _ = w.Write([]byte(`{"total":0,"databases":[]}`))
		case "/tablesdb":
			_, _ = w.Write([]byte(`{"total":1,"databases":[{"$id":"db1","name":"Sales"}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	dbs, err := c.ListDatabases(context.Background())
	require.NoError(t, err)
	require.Len(t, dbs, 1)
	require.Equal(t, "db1", dbs[0].ID)
	require.Equal(t, "Sales", dbs[0].Name)
}

func TestListCollections_FallsBackToTablesDBTables(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/databases/dbMain/collections":
			_, _ = w.Write([]byte(`{"total":0,"collections":[]}`))
		case "/tablesdb/dbMain/tables":
			_, _ = w.Write([]byte(`{"total":1,"tables":[{"$id":"tbl1","name":"Users"}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	cols, err := c.ListCollections(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "tbl1", cols[0].ID)
	require.Equal(t, "Users", cols[0].Name)
}

func TestListAttributes_FallsBackToTablesDBColumns(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/databases/dbMain/collections/col1/attributes":
			_, _ = w.Write([]byte(`{"total":0,"attributes":[]}`))
		case "/tablesdb/dbMain/tables/col1/columns":
			_, _ = w.Write([]byte(`{"total":2,"columns":[{"key":"name","type":"string"},{"key":"age","type":"integer"}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	attrs, err := c.ListAttributes(context.Background(), "", "col1")
	require.NoError(t, err)
	require.Len(t, attrs, 2)
	require.Equal(t, "name", attrs[0].Key)
	require.Equal(t, "string", attrs[0].Type)
	require.Equal(t, "age", attrs[1].Key)
	require.Equal(t, "integer", attrs[1].Type)
}

func TestPing(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "key", r.Header.Get("X-Appwrite-Key"))
		require.Equal(t, "/databases", r.URL.Path)
		require.Contains(t, r.URL.Query()["queries[]"], `{"method":"limit","values":[1]}`)
		_, _ = w.Write([]byte(`{"total":0,"databases":[]}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestStatusHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"app key invalid"}`))
	})
	defer srv.Close()

	_, err := c.ListDatabases(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "API key")
}

// makeFullPage builds a documents response with exactly defaultPageSize (100)
// documents, the last one having the given $id, so ListDocuments keeps paging.
func makeFullPage(lastID string) string {
	docs := make([]string, defaultPageSize)
	for i := 0; i < defaultPageSize-1; i++ {
		docs[i] = `{"$id":"d","name":"a"}`
	}
	docs[defaultPageSize-1] = `{"$id":"` + lastID + `","name":"a"}`
	out := `{"total":101,"documents":[`
	for i, d := range docs {
		if i > 0 {
			out += ","
		}
		out += d
	}
	out += `]}`
	return out
}
