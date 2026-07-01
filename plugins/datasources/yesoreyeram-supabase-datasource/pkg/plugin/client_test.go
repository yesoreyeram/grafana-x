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
	c, err := NewClient(Settings{APIURL: srv.URL, apiKey: "test-key"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestNewClient_RequiresURL(t *testing.T) {
	_, err := NewClient(Settings{}, http.DefaultClient)
	require.Error(t, err)
	require.Contains(t, err.Error(), "API URL is required")
}

func TestNewClient_InvalidURL(t *testing.T) {
	_, err := NewClient(Settings{APIURL: "://bad"}, http.DefaultClient)
	require.Error(t, err)
}

// TestListRecords_SetsDualAuthHeaders is the most important auth test: Supabase
// requires BOTH the apikey header AND the Authorization: Bearer header, set to
// the same key value, on every request.
func TestListRecords_SetsDualAuthHeaders(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("apikey"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "/tblXYZ", r.URL.Path)
		require.Equal(t, "10", r.URL.Query().Get("limit"))
		// ListRecords must NOT force an expensive exact count per page.
		require.Empty(t, r.Header.Get("Prefer"))
		w.Header().Set("Content-Range", "0-1/2")
		_, _ = w.Write([]byte(`[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 10})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "Alice", rows[0]["name"])
	require.EqualValues(t, 1, rows[0]["id"])
}

func TestListRecords_RequiresTable(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_ForwardsParams(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "id,name", q.Get("select"))
		require.Equal(t, "name.desc", q.Get("order"))
		require.Equal(t, "gt.18", q.Get("age"))
		w.Header().Set("Content-Range", "0-0/1")
		_, _ = w.Write([]byte(`[{"id":1}]`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		TableID: "tblXYZ",
		Select:  "id,name",
		sortItems: []SortItem{
			{Field: "name", Direction: "desc"},
		},
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "age", Op: "gt", Value: "18"}},
		},
		Limit: 10,
	})
	require.NoError(t, err)
}

func TestListRecords_Handles206PartialContent(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// PostgREST returns 206 for ranged reads; it must be treated as success.
		w.Header().Set("Content-Range", "0-0/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(`[{"id":1,"name":"Alice"}]`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["name"])
}

func TestListRecords_Paginates(t *testing.T) {
	calls := 0
	// Want 1500 rows. defaultPageSize = 1000, so 2 calls: limit=1000 then
	// limit=500 with offset=1000.
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			require.Equal(t, "1000", r.URL.Query().Get("limit"))
			require.Empty(t, r.URL.Query().Get("offset"))
			w.Header().Set("Content-Range", "0-999/1500")
			w.WriteHeader(http.StatusPartialContent)
			rows := make([]map[string]any, 1000)
			for i := 0; i < 1000; i++ {
				rows[i] = map[string]any{"id": i + 1}
			}
			b, _ := json.Marshal(rows)
			_, _ = w.Write(b)
			return
		}
		require.Equal(t, "500", r.URL.Query().Get("limit"))
		require.Equal(t, "1000", r.URL.Query().Get("offset"))
		w.Header().Set("Content-Range", "1000-1499/1500")
		w.WriteHeader(http.StatusPartialContent)
		rows := make([]map[string]any, 500)
		for i := 0; i < 500; i++ {
			rows[i] = map[string]any{"id": i + 1001}
		}
		b, _ := json.Marshal(rows)
		_, _ = w.Write(b)
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 1500})
	require.NoError(t, err)
	require.Len(t, rows, 1500)
	require.Equal(t, 2, calls)
}

func TestListRecords_StopsOnShortPage(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Return fewer rows than the page size on the first call.
		_, _ = w.Write([]byte(`[{"id":1},{"id":2}]`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 1, calls) // short page => no second request
}

func TestListRecords_RespectsOffset(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "10", r.URL.Query().Get("limit"))
		require.Equal(t, "5", r.URL.Query().Get("offset"))
		w.Header().Set("Content-Range", "5-9/20")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(`[{"id":6},{"id":7},{"id":8},{"id":9},{"id":10}]`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 10, Offset: 5})
	require.NoError(t, err)
	require.Len(t, rows, 5)
}

// TestCountRecords_ParsesContentRange asserts the count comes from the
// Content-Range header (format "start-end/total") of a HEAD request that sets
// Prefer: count=exact.
func TestCountRecords_ParsesContentRange(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		require.Equal(t, "/tblXYZ", r.URL.Path)
		require.Equal(t, "test-key", r.Header.Get("apikey"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "count=exact", r.Header.Get("Prefer"))
		w.Header().Set("Content-Range", "0-0/42")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.EqualValues(t, 42, n)
}

func TestCountRecords_ForwardsFilter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "eq.active", r.URL.Query().Get("status"))
		w.Header().Set("Content-Range", "0-0/7")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{
		TableID: "tblXYZ",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "status", Op: "eq", Value: "active"}},
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, 7, n)
}

func TestCountRecords_UnknownTotalReturnsZero(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// No Content-Range, or total "*", means the count is unknown.
		w.Header().Set("Content-Range", "*/*")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{TableID: "tblXYZ"})
	require.NoError(t, err)
	require.EqualValues(t, 0, n)
}

func TestListTables_ParsesDefinitionsAndPaths(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/", r.URL.Path)
		_, _ = w.Write([]byte(`{
			"definitions": {
				"users": {"type": "object"},
				"orders": {"type": "object"}
			},
			"paths": {
				"/": {},
				"/users": {},
				"/audit_log": {},
				"/rpc/my_func": {}
			}
		}`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background())
	require.NoError(t, err)
	// users + orders (definitions) + audit_log (paths), rpc excluded, root excluded.
	require.Len(t, tables, 3)
	require.Equal(t, "audit_log", tables[0].ID)
	require.Equal(t, "orders", tables[1].ID)
	require.Equal(t, "users", tables[2].ID)
}

func TestListTables_PathsOnly(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"paths": {
				"/": {},
				"/people": {},
				"/rpc/search": {}
			}
		}`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background())
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "people", tables[0].ID)
}

func TestAcceptProfileHeader_SetWhenSchemaConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "analytics", r.Header.Get("Accept-Profile"))
		w.Header().Set("Content-Range", "0-0/0")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c, err := NewClient(Settings{APIURL: srv.URL, apiKey: "test-key", Schema: "analytics"}, srv.Client())
	require.NoError(t, err)

	_, err = c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 1})
	require.NoError(t, err)
}

func TestAcceptProfileHeader_OmittedByDefault(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Accept-Profile"))
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableID: "tblXYZ", Limit: 1})
	require.NoError(t, err)
}

func TestPing(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("apikey"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "/", r.URL.Path)
		_, _ = w.Write([]byte(`{"swagger":"2.0"}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestDo_ExtractsPostgRESTError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"42703","message":"column users.foo does not exist","details":null,"hint":"Perhaps you meant bar"}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableID: "users", Limit: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "400")
	require.Contains(t, err.Error(), "column users.foo does not exist")
	require.Contains(t, err.Error(), "Perhaps you meant bar")
}

func TestDo_UnauthorizedHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Invalid API key"}`))
	})
	defer srv.Close()

	_, err := c.ListTables(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "Invalid API key")
	require.Contains(t, err.Error(), "Save & test")
}

func TestParseContentRangeTotal(t *testing.T) {
	mk := func(v string) http.Header {
		h := http.Header{}
		if v != "" {
			h.Set("Content-Range", v)
		}
		return h
	}
	require.EqualValues(t, 3573, parseContentRangeTotal(mk("0-24/3573")))
	require.EqualValues(t, 0, parseContentRangeTotal(mk("*/0")))
	require.EqualValues(t, -1, parseContentRangeTotal(mk("0-24/*")))
	require.EqualValues(t, -1, parseContentRangeTotal(mk("")))
	require.EqualValues(t, -1, parseContentRangeTotal(mk("garbage")))
}
