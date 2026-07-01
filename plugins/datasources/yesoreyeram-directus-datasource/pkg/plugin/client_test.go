package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestNewClient_RequiresBaseURL(t *testing.T) {
	_, err := NewClient(Settings{}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Directus base URL is required")
}

func TestNewClient_StripsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://directus.example.com/", apiToken: "tok"}, nil)
	require.NoError(t, err)
	require.Equal(t, "https://directus.example.com", c.baseURL)
}

func TestListRecords(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/items/articles", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":1,"title":"Hello","status":"published"}]}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "articles"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Hello", rows[0]["title"])
}

func TestListRecords_RequiresCollection(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_ForwardsParams(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "id,title", q.Get("fields"))
		require.Equal(t, "-views", q.Get("sort"))
		require.Equal(t, "10", q.Get("limit"))
		require.Equal(t, "20", q.Get("offset"))
		require.Equal(t, "hello", q.Get("search"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		CollectionID: "articles",
		Fields:       "id,title",
		Limit:        10,
		Offset:       20,
		Search:       "hello",
		sortItems: []SortItem{
			{Field: "views", Direction: "desc"},
		},
	})
	require.NoError(t, err)
}

func TestListRecords_ForwardsFilterJSON(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// The `filter` param must be a URL-encoded JSON object, decoded here by
		// r.URL.Query() back into the original JSON string.
		filter := r.URL.Query().Get("filter")
		require.Equal(t, `{"status":{"_eq":"published"}}`, filter)
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	q := QueryModel{
		CollectionID: "articles",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "status", Op: "eq", Value: "published"}},
		},
	}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestListRecords_Paginates(t *testing.T) {
	const pageSize = 100
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		offset := r.URL.Query().Get("offset")
		if calls == 1 {
			require.Equal(t, "", offset)
			// Return a full page of items to trigger a second page request.
			b := `{"data":[`
			for i := 0; i < pageSize; i++ {
				if i > 0 {
					b += ","
				}
				b += `{"id":` + strconv.Itoa(i+1) + `}`
			}
			b += `]}`
			_, _ = w.Write([]byte(b))
			return
		}
		require.Equal(t, strconv.Itoa(pageSize), offset)
		_, _ = w.Write([]byte(`{"data":[{"id":101}]}`))
	}))
	defer srv.Close()

	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "articles", Limit: 101})
	require.NoError(t, err)
	require.Len(t, rows, 101)
	require.Equal(t, 2, calls)
}

func TestListRecords_PaginationStartsFromOffset(t *testing.T) {
	const pageSize = 100
	var offsets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offsets = append(offsets, r.URL.Query().Get("offset"))
		if len(offsets) == 1 {
			b := `{"data":[`
			for i := 0; i < pageSize; i++ {
				if i > 0 {
					b += ","
				}
				b += `{"id":` + strconv.Itoa(i+1) + `}`
			}
			b += `]}`
			_, _ = w.Write([]byte(b))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":1}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	_, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "articles", Offset: 50})
	require.NoError(t, err)
	// First page uses the user offset; the next page advances by the page size.
	require.Equal(t, []string{"50", "150"}, offsets)
}

func TestListRecords_RespectsLimit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		limit := r.URL.Query().Get("limit")
		require.Equal(t, "2", limit)
		_, _ = w.Write([]byte(`{"data":[{"id":1},{"id":2}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	rows, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "articles", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 1, calls)
}

func TestCountRecords_Aggregate(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/items/articles", r.URL.Path)
		// Count uses the aggregate API and respects the filter.
		require.Equal(t, "*", r.URL.Query().Get("aggregate[count]"))
		require.Equal(t, `{"status":{"_eq":"published"}}`, r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`{"data":[{"count":42}]}`))
	})
	defer srv.Close()

	q := QueryModel{CollectionID: "articles", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "status", Op: "eq", Value: "published"}},
	}}
	n, err := c.CountRecords(context.Background(), q)
	require.NoError(t, err)
	require.EqualValues(t, 42, n)
}

func TestCountRecords_AggregateStringCount(t *testing.T) {
	// Some database drivers return the aggregate count as a numeric string.
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"count":"137"}]}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{CollectionID: "articles"})
	require.NoError(t, err)
	require.EqualValues(t, 137, n)
}

func TestCountRecords_ForwardsSearch(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "*", r.URL.Query().Get("aggregate[count]"))
		require.Equal(t, "needle", r.URL.Query().Get("search"))
		_, _ = w.Write([]byte(`{"data":[{"count":3}]}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{CollectionID: "articles", Search: "needle"})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
}

func TestCountRecords_RequiresCollection(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.CountRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListCollections_ExcludesSystemCollections(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/collections", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[
			{"collection":"articles","meta":{"icon":"article"}},
			{"collection":"directus_users"},
			{"collection":"directus_files"},
			{"collection":"authors"},
			{"collection":""}
		]}`))
	})
	defer srv.Close()

	cols, err := c.ListCollections(context.Background())
	require.NoError(t, err)
	require.Len(t, cols, 2) // only the user collections survive
	require.Equal(t, "articles", cols[0].Collection)
	require.Equal(t, "authors", cols[1].Collection)
}

func TestListFields(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/fields/articles", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"field":"id","type":"integer"},{"field":"title","type":"string"},{"field":"views","type":"integer"}]}`))
	})
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "articles")
	require.NoError(t, err)
	require.Len(t, fields, 3)
	require.Equal(t, "title", fields[1].Field)
}

func TestListFields_RequiresCollectionID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListFields(context.Background(), "")
	require.Error(t, err)
}

func TestPing_UsesUsersMe(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Ping must hit an authenticated endpoint so a bad token is detected.
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/users/me", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":{"id":"abc"}}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestStatusHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Invalid user credentials."}]}`))
	})
	defer srv.Close()

	_, err := c.ListCollections(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "Invalid user credentials.")
	require.Contains(t, err.Error(), "API token")
}

func TestBuildQueryParams_Search(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "hello world", r.URL.Query().Get("search"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "articles", Search: "hello world"})
	require.NoError(t, err)
}

func TestBuildQueryParams_Offset(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "50", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "articles", Offset: 50})
	require.NoError(t, err)
}

func TestQueryParams_SortAsc(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "title", r.URL.Query().Get("sort"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		CollectionID: "articles",
		sortItems:    []SortItem{{Field: "title", Direction: "asc"}},
	})
	require.NoError(t, err)
}

func TestQueryParams_SortMultiple(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "-views,title", r.URL.Query().Get("sort"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		CollectionID: "articles",
		sortItems:    []SortItem{{Field: "views", Direction: "desc"}, {Field: "title", Direction: "asc"}},
	})
	require.NoError(t, err)
}

func TestToInt64(t *testing.T) {
	cases := []struct {
		in   any
		want int64
		ok   bool
	}{
		{float64(42), 42, true},
		{"137", 137, true},
		{"42.0", 42, true},
		{"  9 ", 9, true},
		{"", 0, false},
		{"abc", 0, false},
		{nil, 0, false},
		{[]any{1}, 0, false},
	}
	for _, tc := range cases {
		got, ok := toInt64(tc.in)
		require.Equal(t, tc.ok, ok, "in=%v", tc.in)
		if tc.ok {
			require.EqualValues(t, tc.want, got, "in=%v", tc.in)
		}
	}
}

// guard against accidental use of the access_token query param for auth.
func TestDo_UsesBearerHeaderNotQueryParam(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "", r.URL.Query().Get("access_token"))
		require.False(t, strings.Contains(r.URL.RawQuery, "access_token"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{CollectionID: "articles"})
	require.NoError(t, err)
}
