package plugin

import (
	"context"
	"encoding/json"
	"io"
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

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var body map[string]any
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &body))
	}
	return body
}

func TestListRecords_FlattensPages(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/databases/db1/query", r.URL.Path)
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, defaultNotionVersion, r.Header.Get("Notion-Version"))
		_, _ = w.Write([]byte(`{"results":[
			{"id":"p1","properties":{"Name":{"type":"title","title":[{"plain_text":"Alice"}]}}}
		],"has_more":false,"next_cursor":null}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{DatabaseID: "db1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
}

func TestListRecords_RequiresDatabaseID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_SendsSortsAndFilter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)

		// sorts: [-Age, Name] -> descending Age, ascending Name
		sorts, ok := body["sorts"].([]any)
		require.True(t, ok)
		require.Len(t, sorts, 2)
		first := sorts[0].(map[string]any)
		require.Equal(t, "Age", first["property"])
		require.Equal(t, "descending", first["direction"])
		second := sorts[1].(map[string]any)
		require.Equal(t, "Name", second["property"])
		require.Equal(t, "ascending", second["direction"])

		// filter is a single rich_text equals condition.
		filter := body["filter"].(map[string]any)
		require.Equal(t, "Name", filter["property"])

		_, _ = w.Write([]byte(`{"results":[],"has_more":false}`))
	})
	defer srv.Close()

	q := QueryModel{
		DatabaseID: "db1",
		Sort:       "-Age,Name",
		filter: &FilterNode{Kind: "group", Connector: "and", Children: []FilterNode{
			{Kind: "condition", Field: "Name", Category: "text", Op: "equals", Value: "Alice"},
		}},
	}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestListRecords_FollowsCursorPagination(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		calls++
		if calls == 1 {
			require.Nil(t, body["start_cursor"])
			_, _ = w.Write([]byte(`{"results":[
				{"id":"p1","properties":{"Name":{"type":"title","title":[{"plain_text":"a"}]}}}
			],"has_more":true,"next_cursor":"CUR"}`))
			return
		}
		require.Equal(t, "CUR", body["start_cursor"])
		_, _ = w.Write([]byte(`{"results":[
			{"id":"p2","properties":{"Name":{"type":"title","title":[{"plain_text":"b"}]}}}
		],"has_more":false,"next_cursor":null}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{DatabaseID: "db1"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 2, calls)
	require.Equal(t, "a", rows[0]["Name"])
	require.Equal(t, "b", rows[1]["Name"])
}

func TestCountRecords_PaginatesAndCounts(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/databases/db1/query", r.URL.Path)
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"results":[{"id":"p1"},{"id":"p2"}],"has_more":true,"next_cursor":"X"}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"p3"}],"has_more":false,"next_cursor":null}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{DatabaseID: "db1"})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
}

func TestListDatabases(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/search", r.URL.Path)
		body := decodeBody(t, r)
		filter := body["filter"].(map[string]any)
		require.Equal(t, "database", filter["value"])
		require.Equal(t, "object", filter["property"])
		_, _ = w.Write([]byte(`{"results":[
			{"id":"db1","title":[{"plain_text":"Customers"}]},
			{"id":"db2","title":[]}
		],"has_more":false,"next_cursor":null}`))
	})
	defer srv.Close()

	dbs, err := c.ListDatabases(context.Background())
	require.NoError(t, err)
	require.Len(t, dbs, 2)
	require.Equal(t, "db1", dbs[0].ID)
	require.Equal(t, "Customers", dbs[0].Title)
	// Untitled database falls back to its id as the title.
	require.Equal(t, "db2", dbs[1].Title)
}

func TestListProperties(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/databases/db1", r.URL.Path)
		_, _ = w.Write([]byte(`{"title":[{"plain_text":"Customers"}],"properties":{
			"Name":{"type":"title"},
			"MRR":{"type":"number"},
			"Active":{"type":"checkbox"}
		}}`))
	})
	defer srv.Close()

	props, err := c.ListProperties(context.Background(), "db1")
	require.NoError(t, err)
	require.Len(t, props, 3)

	byName := map[string]PropertyInfo{}
	for _, p := range props {
		byName[p.Title] = p
	}
	require.Equal(t, "title", byName["Name"].Type)
	require.Equal(t, "text", byName["Name"].Category)
	require.Equal(t, "number", byName["MRR"].Category)
	require.Equal(t, "checkbox", byName["Active"].Category)
}

func TestListProperties_RequiresDatabaseID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListProperties(context.Background(), "")
	require.Error(t, err)
}

func TestDo_SurfacesNotionErrorMessage(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"object":"error","status":400,"code":"validation_error","message":"path failed validation"}`))
	})
	defer srv.Close()

	_, err := c.ListProperties(context.Background(), "db1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "path failed validation")
}

func TestParseSort(t *testing.T) {
	got := parseSort("-Created, Name ,")
	require.Equal(t, []sortItem{
		{Property: "Created", Direction: "descending"},
		{Property: "Name", Direction: "ascending"},
	}, got)
	require.Empty(t, parseSort(""))
}
