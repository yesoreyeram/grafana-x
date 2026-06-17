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

// newTestClient returns a Client pointed at the given test server URL.
func newTestClient(t *testing.T, url string, settings Settings) *Client {
	t.Helper()
	settings.BaseURL = url
	c, err := NewClient(settings, http.DefaultClient)
	require.NoError(t, err)
	return c
}

func TestNewClient_DefaultsToCloudURL(t *testing.T) {
	c, err := NewClient(Settings{}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, mondayCloudURL, c.baseURL)
}

func TestClient_APIKeyHeaderIsRaw(t *testing.T) {
	var gotAuth, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("API-Version")
		_, _ = io.WriteString(w, `{"data":{"me":{"id":"1","name":"me"}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authAPIKey, APIVersion: "2024-10", apiToken: "tok_secret"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "tok_secret", gotAuth)
	require.Equal(t, "2024-10", gotVersion)
}

func TestClient_DefaultsAPIVersionWhenUnset(t *testing.T) {
	var gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("API-Version")
		_, _ = io.WriteString(w, `{"data":{"me":{"id":"1"}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authAPIKey, apiToken: "x"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, defaultAPIVersion, gotVersion)
	require.Equal(t, "2026-01", gotVersion)
}

func TestClient_OAuthHeaderIsBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"data":{"me":{"id":"1"}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{AuthMethod: authOAuth, oauthToken: "tok"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "Bearer tok", gotAuth)
}

func TestClient_GraphQLErrorsSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":"Not authenticated"}]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Not authenticated")
}

func TestClient_ErrorMessageSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"error_message":"Invalid token"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid token")
}

func TestClient_HTTPErrorStatusSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `unauthorized`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestClient_ListItems_PaginatesAndFlattens(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &req))

		if call == 0 {
			call++
			require.Contains(t, req.Query, "items_page")
			require.NotContains(t, req.Query, "next_items_page")
			_, _ = io.WriteString(w, `{"data":{"boards":[{"id":"1","name":"Tasks","items_page":{
				"cursor":"c1",
				"items":[
					{"id":"11","name":"A","state":"active","created_at":"2024-01-02T03:04:05Z",
					 "group":{"id":"g1","title":"Doing"},"board":{"id":"1","name":"Tasks"},
					 "column_values":[{"id":"status","column":{"title":"Status"},"text":"Working"}]}
				]
			}}]}}`)
			return
		}
		// Second call: next_items_page with the cursor.
		require.Contains(t, req.Query, "next_items_page")
		require.Equal(t, "c1", req.Variables["cursor"])
		_, _ = io.WriteString(w, `{"data":{"next_items_page":{
			"cursor":"",
			"items":[
				{"id":"12","name":"B","state":"active","created_at":"2024-01-03T03:04:05Z",
				 "group":{"id":"g1","title":"Doing"},"board":{"id":"1","name":"Tasks"},
				 "column_values":[{"id":"status","column":{"title":"Status"},"text":"Done"}]}
			]
		}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeItems, BoardIds: []string{"1"}})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "A", records[0]["name"])
	require.Equal(t, "Doing", records[0]["group"])
	require.Equal(t, "Tasks", records[0]["board"])
	require.Equal(t, "Working", records[0]["Status"]) // column value lifted by title
	require.Equal(t, "B", records[1]["name"])
	require.Equal(t, "Done", records[1]["Status"])
}

func TestClient_ListItems_RequiresBoard(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiToken: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeItems})
	require.Error(t, err)
	require.Contains(t, err.Error(), "board is required")
}

func TestClient_ListItems_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		require.EqualValues(t, 1, req.Variables["limit"])
		_, _ = io.WriteString(w, `{"data":{"boards":[{"id":"1","items_page":{
			"cursor":"c1","items":[{"id":"11","name":"A"}]
		}}]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeItems, BoardIds: []string{"1"}, Limit: 1})
	require.NoError(t, err)
	require.Len(t, records, 1)
}

func TestClient_ListItems_WithoutColumns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		require.NotContains(t, req.Query, "column_values")
		_, _ = io.WriteString(w, `{"data":{"boards":[{"id":"1","items_page":{
			"cursor":"","items":[{"id":"11","name":"A"}]
		}}]}}`)
	}))
	defer srv.Close()

	off := false
	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:           queryTypeItems,
		BoardIds:            []string{"1"},
		IncludeColumnValues: &off,
	})
	require.NoError(t, err)
	require.Len(t, records, 1)
}

func TestBuildItemsQueryParams(t *testing.T) {
	require.Nil(t, buildItemsQueryParams(QueryModel{QueryType: queryTypeItems}))

	p := buildItemsQueryParams(QueryModel{
		QueryType:   queryTypeItems,
		SearchQuery: "login",
		GroupIds:    []string{"g1", "g2"},
		OrderBy:     "status",
		OrderDir:    "desc",
	})
	require.NotNil(t, p)
	rules := p["rules"].([]any)
	require.Len(t, rules, 2)
	order := p["order_by"].([]any)[0].(map[string]any)
	require.Equal(t, "status", order["column_id"])
	require.Equal(t, "desc", order["direction"])
}

func TestBuildItemsQuery_ColumnSelection(t *testing.T) {
	// No column ids: unfiltered column_values block, includes type for conversion.
	all := buildItemsQuery(true, nil)
	require.Contains(t, all, "column_values {")
	require.Contains(t, all, "type")
	require.NotContains(t, all, "column_values(ids:")

	// Specific column ids: restricted via the GraphQL ids argument.
	some := buildItemsQuery(true, []string{"status", "date4"})
	require.Contains(t, some, `column_values(ids: ["status", "date4"])`)

	// Without columns: no column_values block at all.
	none := buildItemsQuery(false, []string{"status"})
	require.NotContains(t, none, "column_values")
}

func TestClient_ListItems_ColumnIdsSentInQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		gotQuery = req.Query
		_, _ = io.WriteString(w, `{"data":{"boards":[{"id":"1","items_page":{"cursor":"","items":[]}}]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeItems,
		BoardIds:  []string{"1"},
		ColumnIds: []string{"status"},
	})
	require.NoError(t, err)
	require.Contains(t, gotQuery, `column_values(ids: ["status"])`)
}

func TestIdStrings(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, idStrings([]string{" a ", "", "b", "  "}))
	require.Empty(t, idStrings(nil))
}

func TestClient_ListBoards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		require.Equal(t, stateActive, req.Variables["state"])
		_, _ = io.WriteString(w, `{"data":{"boards":[
			{"id":"1","name":"Tasks","state":"active","items_count":5,"workspace":{"id":"w1","name":"Main"}},
			{"id":"2","name":"Bugs","state":"active","items_count":3,"workspace":{"id":"w1","name":"Main"}}
		]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeBoards, State: stateActive})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Tasks", records[0]["name"])
	require.Equal(t, "Main", records[0]["workspace"]) // nested object -> name
}

func TestClient_ListGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"boards":[{"id":"1","name":"Tasks","groups":[
			{"id":"g1","title":"Doing","color":"#fff","archived":false},
			{"id":"g2","title":"Done","color":"#0f0","archived":false}
		]}]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeGroups, BoardIds: []string{"1"}})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Doing", records[0]["title"])
	require.Equal(t, "Tasks", records[0]["board"])
	require.Equal(t, "1", records[0]["board_id"])
}

func TestClient_ListGroups_RequiresBoard(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiToken: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeGroups})
	require.Error(t, err)
	require.Contains(t, err.Error(), "board is required")
}

func TestClient_ListUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"users":[
			{"id":"1","name":"Alice","email":"a@b.com","enabled":true,"is_admin":true},
			{"id":"2","name":"Bob","email":"b@b.com","enabled":true,"is_admin":false}
		]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeUsers})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Alice", records[0]["name"])
}

func TestClient_ListWorkspaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		require.Equal(t, stateActive, req.Variables["state"])
		_, _ = io.WriteString(w, `{"data":{"workspaces":[
			{"id":"w1","name":"Main","kind":"open"},
			{"id":"w2","name":"Private","kind":"closed"}
		]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeWorkspaces, State: stateActive})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Main", records[0]["name"])
}

func TestClient_ListTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"tags":[
			{"id":"1","name":"urgent","color":"red"},
			{"id":"2","name":"backend","color":"blue"}
		]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeTags})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "urgent", records[0]["name"])
}

func TestClient_ListRaw_FindsCollection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"boards":[{"items_page":{
			"items":[{"id":"11","name":"A"},{"id":"12","name":"B"}]
		}}]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType: queryTypeRaw,
		RawQuery:  `query { boards { items_page { items { id name } } } }`,
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "A", records[0]["name"])
}

func TestClient_ListRaw_RequiresQuery(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiToken: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeRaw})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rawQuery is required")
}

func TestClient_CountRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"tags":[{"name":"a"},{"name":"b"},{"name":"c"}]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	count, err := c.CountRecords(context.Background(), QueryModel{QueryType: queryTypeTags})
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
}

func TestClient_ListBoards_Resource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"boards":[{"id":"1","name":"Tasks"},{"id":"2","name":"Bugs"}]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	boards, err := c.ListBoards(context.Background())
	require.NoError(t, err)
	require.Len(t, boards, 2)
	require.Equal(t, "Tasks", boards[0].Name)
}

func TestClient_ListColumns_Resource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"boards":[{"columns":[
			{"id":"name","title":"Name","type":"name"},
			{"id":"status","title":"Status","type":"status"},
			{"id":"status","title":"Status","type":"status"}
		]}]}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	columns, err := c.ListColumns(context.Background(), []string{"1"})
	require.NoError(t, err)
	require.Len(t, columns, 2) // deduplicated by id
	require.Equal(t, "Name", columns[0].Title)
}

func TestClient_ListColumns_NoBoards(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiToken: "x"})
	columns, err := c.ListColumns(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, columns)
}

func TestFindCollection(t *testing.T) {
	nodes, ok := findCollection(json.RawMessage(`{"a":{"b":{"items":[{"x":1},{"x":2}]}}}`))
	require.True(t, ok)
	require.Len(t, nodes, 2)

	// Array of scalars is skipped.
	_, ok = findCollection(json.RawMessage(`{"a":["x","y"]}`))
	require.False(t, ok)
}
