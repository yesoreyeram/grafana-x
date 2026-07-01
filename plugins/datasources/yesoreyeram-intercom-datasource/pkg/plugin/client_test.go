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
	c, err := NewClient(Settings{BaseURL: srv.URL, IntercomVersion: "2.11", apiToken: "tok"}, srv.Client())
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

func TestDo_SendsBearerAndVersionHeaders(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "2.11", r.Header.Get("Intercom-Version"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(`{"type":"admin"}`))
	})
	defer srv.Close()

	require.NoError(t, c.Ping(context.Background()))
}

func TestPing_HitsMe(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/me", r.URL.Path)
		_, _ = w.Write([]byte(`{"type":"admin","id":"1","email":"a@b.com"}`))
	})
	defer srv.Close()

	require.NoError(t, c.Ping(context.Background()))
}

func TestListConversations_UsesListWhenNoFilters(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/conversations", r.URL.Path)
		require.Equal(t, "150", r.URL.Query().Get("per_page"))
		_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[
			{"type":"conversation","id":"1","state":"open","created_at":1539897198}
		],"total_count":1,"pages":{"type":"pages","page":1,"per_page":150,"total_pages":1}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeConversations})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "1", rows[0]["id"])
	require.Equal(t, "open", rows[0]["state"])
	// created_at epoch seconds -> RFC3339 string.
	require.Equal(t, "2018-10-18T21:13:18Z", rows[0]["created_at"])
}

func TestSearchConversations_SendsQueryAndStatusFilter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/conversations/search", r.URL.Path)
		body := decodeBody(t, r)
		// Single condition is returned bare under "query".
		query := body["query"].(map[string]any)
		require.Equal(t, "state", query["field"])
		require.Equal(t, "=", query["operator"])
		require.Equal(t, "open", query["value"])
		pagination := body["pagination"].(map[string]any)
		require.EqualValues(t, 150, pagination["per_page"])
		_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[
			{"type":"conversation","id":"1","state":"open"}
		],"total_count":1,"pages":{"type":"pages"}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeConversations, StatusFilter: "open"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestSearch_FollowsCursorPagination(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		pagination := body["pagination"].(map[string]any)
		calls++
		if calls == 1 {
			require.Nil(t, pagination["starting_after"])
			_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[
				{"type":"conversation","id":"1"}
			],"total_count":2,"pages":{"type":"pages","next":{"page":2,"starting_after":"CURSOR"}}}`))
			return
		}
		// Second page must send the cursor from pages.next.starting_after.
		require.Equal(t, "CURSOR", pagination["starting_after"])
		_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[
			{"type":"conversation","id":"2"}
		],"total_count":2,"pages":{"type":"pages"}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeConversations, StatusFilter: "open"})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Len(t, rows, 2)
	require.Equal(t, "1", rows[0]["id"])
	require.Equal(t, "2", rows[1]["id"])
}

func TestListContacts_FollowsCursorViaStartingAfterParam(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/contacts", r.URL.Path)
		calls++
		if calls == 1 {
			require.Equal(t, "", r.URL.Query().Get("starting_after"))
			_, _ = w.Write([]byte(`{"type":"list","data":[
				{"type":"contact","id":"a","email":"a@b.com"}
			],"total_count":2,"pages":{"type":"pages","next":{"starting_after":"NEXT"}}}`))
			return
		}
		require.Equal(t, "NEXT", r.URL.Query().Get("starting_after"))
		_, _ = w.Write([]byte(`{"type":"list","data":[
			{"type":"contact","id":"b","email":"c@d.com"}
		],"total_count":2,"pages":{"type":"pages"}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeContacts})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Len(t, rows, 2)
}

func TestSearch_RespectsLimit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		pagination := body["pagination"].(map[string]any)
		require.EqualValues(t, 2, pagination["per_page"]) // limit clamps per_page
		_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[
			{"type":"conversation","id":"1"},{"type":"conversation","id":"2"},{"type":"conversation","id":"3"}
		],"total_count":3,"pages":{"type":"pages","next":{"starting_after":"X"}}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeConversations, StatusFilter: "open", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestTickets_AlwaysSearchWithMatchAllFallback(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tickets/search", r.URL.Path)
		body := decodeBody(t, r)
		query := body["query"].(map[string]any)
		// match-all fallback uses created_at > 0.
		require.Equal(t, "created_at", query["field"])
		require.Equal(t, ">", query["operator"])
		_, _ = w.Write([]byte(`{"type":"ticket.list","tickets":[
			{"type":"ticket","id":"t1"}
		],"total_count":1,"pages":{"type":"pages"}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTickets})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "t1", rows[0]["id"])
}

func TestSearch_MultipleFiltersWrappedWithAnd(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		query := body["query"].(map[string]any)
		require.Equal(t, "AND", query["operator"])
		values := query["value"].([]any)
		require.Len(t, values, 2)
		sort := body["sort"].(map[string]any)
		require.Equal(t, "created_at", sort["field"])
		require.Equal(t, "descending", sort["order"])
		_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[],"pages":{"type":"pages"}}`))
	})
	defer srv.Close()

	q := QueryModel{
		QueryType:    queryTypeConversations,
		StatusFilter: "open",
		AssigneeID:   "12345",
		Sort:         "-created_at",
	}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestSimpleList_Admins(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/admins", r.URL.Path)
		_, _ = w.Write([]byte(`{"type":"admin.list","admins":[
			{"type":"admin","id":"1","name":"Alice","email":"alice@acme.io"}
		]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeAdmins})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["name"])
}

func TestCount_UsesTotalCountFromSearch(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/conversations/search", r.URL.Path)
		body := decodeBody(t, r)
		pagination := body["pagination"].(map[string]any)
		require.EqualValues(t, 1, pagination["per_page"])
		_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[{"id":"1"}],"total_count":4321,"pages":{"type":"pages"}}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeCount, CountOf: queryTypeConversations})
	require.NoError(t, err)
	require.EqualValues(t, 4321, n)
}

func TestCount_SimpleListFallsBackToLength(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/teams", r.URL.Path)
		_, _ = w.Write([]byte(`{"type":"team.list","teams":[{"id":"1"},{"id":"2"},{"id":"3"}]}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeCount, CountOf: queryTypeTeams})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
}

func TestDo_SurfacesIntercomErrorMessage(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error.list","request_id":"req_1","errors":[{"code":"unauthorized","message":"Access Token Invalid"}]}`))
	})
	defer srv.Close()

	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Access Token Invalid")
	require.Contains(t, err.Error(), "401")
}

func TestListTags_ResourceHelper(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tags", r.URL.Path)
		_, _ = w.Write([]byte(`{"type":"list","data":[{"type":"tag","id":"7","name":"vip"}]}`))
	})
	defer srv.Close()

	tags, err := c.ListTags(context.Background())
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, "vip", tags[0].Name)
}

func TestNextPagination_HandlesURLForm(t *testing.T) {
	next, page := nextPagination(json.RawMessage(`{"type":"pages","next":"https://api.intercom.io/companies?per_page=50&page=3&starting_after=ZZZ"}`))
	require.Equal(t, "ZZZ", next)
	require.Equal(t, 3, page)
}
