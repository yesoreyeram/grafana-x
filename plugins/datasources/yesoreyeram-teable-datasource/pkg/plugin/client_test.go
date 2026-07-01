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
	c, err := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok", DefaultBaseID: "bse123"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestPing(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/auth/user", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"usr1","name":"Alice"}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestListTables_BareArray(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/base/bseABC/table", r.URL.Path)
		// Teable returns a bare JSON array (not {tables:[...]}).
		_, _ = w.Write([]byte(`[
			{"id":"tbl1","name":"Users","dbTableName":"users"},
			{"id":"tbl2","name":"Orders","dbTableName":"orders"}
		]`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "bseABC")
	require.NoError(t, err)
	require.Len(t, tables, 2)
	require.Equal(t, "tbl1", tables[0].ID)
	require.Equal(t, "Users", tables[0].Name)
}

func TestListTables_UsesDefaultBase(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/base/bse123/table", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"tbl1","name":"Users"}]`))
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, tables, 1)
}

func TestListTables_RequiresBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())
	_, err := c.ListTables(context.Background(), "")
	require.Error(t, err)
}

func TestListFields_BareArray(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/table/tbl1/field", r.URL.Path)
		// Bare JSON array; the unnamed field must be skipped.
		_, _ = w.Write([]byte(`[
			{"id":"fld1","name":"Name","type":"singleLineText"},
			{"id":"fld2","name":"Age","type":"number"},
			{"id":"fld3","name":"","type":"singleLineText"}
		]`))
	})
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "tbl1")
	require.NoError(t, err)
	require.Len(t, fields, 2) // unnamed field skipped
	require.Equal(t, "Name", fields[0].Name)
	require.Equal(t, "singleLineText", fields[0].Type)
	require.Equal(t, "number", fields[1].Type)
}

func TestListFields_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListFields(context.Background(), "")
	require.Error(t, err)
}

func TestListRecords_FlattensFieldsAndIdentityColumns(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/table/tbl1/record", r.URL.Path)
		q := r.URL.Query()
		require.Equal(t, "name", q.Get("fieldKeyType"))
		require.Equal(t, "1000", q.Get("take"))
		require.Empty(t, q.Get("skip")) // first page omits skip
		_, _ = w.Write([]byte(`{"records":[
			{"id":"rec1","fields":{"Name":"Alice","Age":30},"createdTime":"2024-01-01T00:00:00.000Z","lastModifiedTime":"2024-02-01T00:00:00.000Z"}
		]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tbl1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
	require.EqualValues(t, 30, rows[0]["Age"])
	require.Equal(t, "rec1", rows[0]["_id"])
	require.Equal(t, "2024-01-01T00:00:00.000Z", rows[0]["_createdTime"])
	require.Equal(t, "2024-02-01T00:00:00.000Z", rows[0]["_lastModifiedTime"])
}

func TestListRecords_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_PaginatesWithSkipTake(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		require.Equal(t, "/api/table/tbl1/record", r.URL.Path)
		require.Equal(t, "2", q.Get("take"))
		if calls == 1 {
			require.Empty(t, q.Get("skip"))
			// A full page (== take) signals there may be more.
			_, _ = w.Write([]byte(`{"records":[
				{"id":"r1","fields":{"Name":"a"}},
				{"id":"r2","fields":{"Name":"b"}}
			]}`))
			return
		}
		require.Equal(t, "2", q.Get("skip")) // skip advanced by take
		// A short page (< take) ends pagination.
		_, _ = w.Write([]byte(`{"records":[{"id":"r3","fields":{"Name":"c"}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	// Force a small page size via the limit so the test exercises two pages.
	rows, err := c.listRecordsWithPageSize(context.Background(), QueryModel{TableID: "tbl1"}, 2)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Equal(t, "a", rows[0]["Name"])
	require.Equal(t, "c", rows[2]["Name"])
	require.Equal(t, 2, calls)
}

func TestListRecords_RespectsLimit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// limit 2 -> take capped at 2.
		require.Equal(t, "2", r.URL.Query().Get("take"))
		_, _ = w.Write([]byte(`{"records":[
			{"id":"r1","fields":{}},
			{"id":"r2","fields":{}}
		]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableID: "tbl1", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2) // stops at the limit (page was full)
}

func TestListRecords_ForwardsParams(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "name", q.Get("fieldKeyType"))
		require.Equal(t, "viw1", q.Get("viewId"))
		require.Equal(t, []string{"Name", "Age"}, q["projection"])
		require.JSONEq(t, `[{"fieldId":"Age","order":"desc"},{"fieldId":"Name","order":"asc"}]`, q.Get("orderBy"))
		_, _ = w.Write([]byte(`{"records":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		TableID: "tbl1",
		ViewID:  "viw1",
		Fields:  "Name,Age",
		sortItems: []SortItem{
			{Field: "Age", Direction: "desc"},
			{Field: "Name", Direction: "asc"},
		},
	})
	require.NoError(t, err)
}

func TestListRecords_ForwardsFilterEnvelope(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("filter")
		require.JSONEq(t,
			`{"conjunction":"and","filterSet":[{"fieldId":"Name","operator":"is","value":"Alice"}]}`,
			filter)
		// The deprecated TQL parameter must NOT be used.
		require.Empty(t, r.URL.Query().Get("filterByTql"))
		_, _ = w.Write([]byte(`{"records":[{"id":"rec1","fields":{"Name":"Alice"}}]}`))
	})
	defer srv.Close()

	q := QueryModel{TableID: "tbl1", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "Name", Category: "text", Op: "is", Value: "Alice"}},
	}}
	rows, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestCountRecords_UsesRowCountAggregation(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/table/tbl1/aggregation/row-count", r.URL.Path)
		_, _ = w.Write([]byte(`{"rowCount":42}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{TableID: "tbl1"})
	require.NoError(t, err)
	require.EqualValues(t, 42, n)
}

func TestCountRecords_ForwardsFilter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/table/tbl1/aggregation/row-count", r.URL.Path)
		require.JSONEq(t,
			`{"conjunction":"and","filterSet":[{"fieldId":"Plan","operator":"is","value":"pro"}]}`,
			r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`{"rowCount":7}`))
	})
	defer srv.Close()

	q := QueryModel{TableID: "tbl1", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "Plan", Category: "text", Op: "is", Value: "pro"}},
	}}
	n, err := c.CountRecords(context.Background(), q)
	require.NoError(t, err)
	require.EqualValues(t, 7, n)
}

func TestCountRecords_RequiresTableID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.CountRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestStatusHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	})
	defer srv.Close()

	_, err := c.ListTables(context.Background(), "bseABC")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "API token")
}

func TestErrorMessageExtraction(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"invalid filter","code":"validation_error"}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableID: "tbl1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid filter")
	require.Contains(t, err.Error(), "422")
}

func TestOrderByParam(t *testing.T) {
	out, err := orderByParam([]SortItem{
		{Field: "Age", Direction: "desc"},
		{Field: "  ", Direction: "asc"}, // skipped (blank field)
		{Field: "Name", Direction: "weird"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `[{"fieldId":"Age","order":"desc"},{"fieldId":"Name","order":"asc"}]`, out)

	empty, err := orderByParam(nil)
	require.NoError(t, err)
	require.Equal(t, "", empty)
}

func TestEncodeQueryHelper(t *testing.T) {
	// JSON marshalling of a sample helper to confirm SortItem round-trips.
	b, err := json.Marshal([]SortItem{{Field: "f", Direction: "asc"}})
	require.NoError(t, err)
	require.JSONEq(t, `[{"field":"f","direction":"asc"}]`, string(b))
}
