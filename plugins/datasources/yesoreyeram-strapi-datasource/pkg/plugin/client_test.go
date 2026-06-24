package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	require.Contains(t, err.Error(), "Strapi base URL is required")
}

func TestNewClient_StripsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://strapi.example.com/", apiToken: "tok"}, nil)
	require.NoError(t, err)
	require.Equal(t, "https://strapi.example.com", c.baseURL)
}

func TestAPIURL(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "http://localhost:1337", apiToken: "tok"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:1337/api/articles", c.apiURL("/articles"))
	require.Equal(t, "http://localhost:1337/content-type-builder/content-types", c.adminURL("/content-type-builder/content-types"))
}

// --- v4 vs v5 response shapes (both must be handled) ---

func TestListRecords_V4Shape(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/articles", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":1,"attributes":{"title":"Hello","status":"published"}}],"meta":{"pagination":{"page":1,"pageSize":25,"pageCount":1,"total":1}}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{ContentTypeID: "articles"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	// The client returns the raw v4 element; flattening happens in frame.go.
	attrs, ok := rows[0]["attributes"].(map[string]any)
	require.True(t, ok, "v4 element should carry an attributes object")
	require.Equal(t, "Hello", attrs["title"])
	require.EqualValues(t, 1, rows[0]["id"])
}

func TestListRecords_V5Shape(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/articles", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":1,"documentId":"abc123","title":"Hello","status":"published"}],"meta":{"pagination":{"page":1,"pageSize":25,"pageCount":1,"total":1}}}`))
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{ContentTypeID: "articles"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	// v5 elements are flat (no attributes wrapper) and carry a documentId.
	_, hasAttrs := rows[0]["attributes"]
	require.False(t, hasAttrs, "v5 element should not have an attributes object")
	require.Equal(t, "Hello", rows[0]["title"])
	require.Equal(t, "abc123", rows[0]["documentId"])
	require.EqualValues(t, 1, rows[0]["id"])
}

func TestListRecords_RequiresContentType(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListRecords_ForwardsParams(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "title", q.Get("fields[0]"))
		require.Equal(t, "title:desc", q.Get("sort[0]"))
		require.Equal(t, "2", q.Get("pagination[page]"))
		require.Equal(t, "50", q.Get("pagination[pageSize]"))
		require.Equal(t, "author", q.Get("populate[0]"))
		_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"page":2,"pageSize":50,"pageCount":0,"total":0}}}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		ContentTypeID: "articles",
		Fields:        "title",
		Page:          2,
		PageSize:      50,
		Populate:      "author",
		sortItems:     []SortItem{{Field: "title", Direction: "desc"}},
	})
	require.NoError(t, err)
}

func TestListRecords_DefaultPagination(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "1", q.Get("pagination[page]"))
		require.Equal(t, "25", q.Get("pagination[pageSize]"))
		_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"page":1,"pageSize":25,"pageCount":0,"total":0}}}`))
	})
	defer srv.Close()

	// Page/PageSize unset -> defaults applied by the client.
	_, err := c.ListRecords(context.Background(), QueryModel{ContentTypeID: "articles"})
	require.NoError(t, err)
}

func TestListRecords_ClampsPageSize(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "100", r.URL.Query().Get("pagination[pageSize]"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{ContentTypeID: "articles", PageSize: 5000})
	require.NoError(t, err)
}

func TestListRecords_MultipleFieldsParam(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "title", q.Get("fields[0]"))
		require.Equal(t, "body", q.Get("fields[1]"))
		require.Equal(t, "views", q.Get("fields[2]"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{ContentTypeID: "articles", Fields: "title,body,views"})
	require.NoError(t, err)
}

func TestListRecords_PopulateList(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "author", q.Get("populate[0]"))
		require.Equal(t, "tags", q.Get("populate[1]"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{ContentTypeID: "articles", Populate: "author,tags"})
	require.NoError(t, err)
}

func TestListRecords_PopulateWildcard(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "*", q.Get("populate"))
		require.Equal(t, "", q.Get("populate[0]"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{ContentTypeID: "articles", Populate: "*"})
	require.NoError(t, err)
}

func TestListRecords_MultipleSortParams(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "title:asc", q.Get("sort[0]"))
		require.Equal(t, "createdAt:desc", q.Get("sort[1]"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{
		ContentTypeID: "articles",
		sortItems:     []SortItem{{Field: "title", Direction: "asc"}, {Field: "createdAt", Direction: "desc"}},
	})
	require.NoError(t, err)
}

func TestListRecords_ForwardsFilter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "published", r.URL.Query().Get("filters[status][$eq]"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	q := QueryModel{
		ContentTypeID: "articles",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "status", Op: "eq", Value: "published"}},
		},
	}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestListRecords_ForwardsInFilterAsArray(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "a", q.Get("filters[tag][$in][0]"))
		require.Equal(t, "b", q.Get("filters[tag][$in][1]"))
		require.Equal(t, "c", q.Get("filters[tag][$in][2]"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	q := QueryModel{
		ContentTypeID: "articles",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "tag", Op: "in", Value: "a,b,c"}},
		},
	}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestCountRecords(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/articles", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("pagination[pageSize]"))
		_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"page":1,"pageSize":1,"pageCount":42,"total":42}}}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{ContentTypeID: "articles"})
	require.NoError(t, err)
	require.EqualValues(t, 42, n)
}

func TestCountRecords_Fallback(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":1},{"id":2},{"id":3}]}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{ContentTypeID: "articles"})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
}

func TestCountRecords_WithFilter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		require.Equal(t, "1", q.Get("pagination[pageSize]"))
		require.Equal(t, "published", q.Get("filters[status][$eq]"))
		_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"page":1,"pageSize":1,"pageCount":5,"total":5}}}`))
	})
	defer srv.Close()

	q := QueryModel{
		ContentTypeID: "articles",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "status", Op: "eq", Value: "published"}},
		},
	}
	n, err := c.CountRecords(context.Background(), q)
	require.NoError(t, err)
	require.EqualValues(t, 5, n)
}

func TestListContentTypes(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// content-type-builder lives at the admin namespace, NOT under /api.
		require.Equal(t, "/content-type-builder/content-types", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"uid":"api::article.article","schema":{"displayName":"Article","singularName":"article","pluralName":"articles","attributes":{"title":{"type":"string"},"body":{"type":"text"}}}},{"uid":"api::author.author","schema":{"displayName":"Author","singularName":"author","pluralName":"authors","attributes":{"name":{"type":"string"}}}}]}`))
	})
	defer srv.Close()

	cols, err := c.ListContentTypes(context.Background())
	require.NoError(t, err)
	require.Len(t, cols, 2)
	require.Equal(t, "api::article.article", cols[0].UID)
	require.Equal(t, "articles", cols[0].Schema.PluralName)
	require.Equal(t, "Article", cols[0].Schema.DisplayName)
}

func TestListFields(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/content-type-builder/content-types", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"uid":"api::article.article","schema":{"displayName":"Article","singularName":"article","pluralName":"articles","attributes":{"title":{"type":"string"},"views":{"type":"integer"},"publishedAt":{"type":"datetime"}}}}]}`))
	})
	defer srv.Close()

	fields, err := c.ListFields(context.Background(), "articles")
	require.NoError(t, err)
	require.Len(t, fields, 3)

	fieldMap := map[string]string{}
	for _, f := range fields {
		fieldMap[f.Field] = f.Type
	}
	require.Equal(t, "string", fieldMap["title"])
	require.Equal(t, "integer", fieldMap["views"])
	require.Equal(t, "datetime", fieldMap["publishedAt"])
}

func TestListFields_RequiresContentTypeID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListFields(context.Background(), "")
	require.Error(t, err)
}

func TestPing_WithContentType(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/api/articles", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("pagination[pageSize]"))
		_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total":0}}}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background(), "articles"))
}

func TestPing_WithContentTypeRejectsAuthError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"status":401,"name":"UnauthorizedError","message":"Missing or invalid credentials"}}`))
	})
	defer srv.Close()
	err := c.Ping(context.Background(), "articles")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestPing_NoContentTypeReachable(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/", r.URL.Path)
		// Strapi returns 404 for the bare /api root; that still proves reachability.
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`Not Found`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background(), ""))
}

func TestPing_NoContentTypeRejectsAuthError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"status":403,"name":"ForbiddenError","message":"Forbidden"}}`))
	})
	defer srv.Close()
	err := c.Ping(context.Background(), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
}

func TestStatusHint(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"status":401,"name":"UnauthorizedError","message":"Invalid credentials"}}`))
	})
	defer srv.Close()

	_, err := c.ListContentTypes(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "API token")
	require.Contains(t, err.Error(), "Invalid credentials")
}

func TestFlattenRecord_V4(t *testing.T) {
	record := map[string]any{
		"id":         float64(1),
		"attributes": map[string]any{"title": "Hello", "body": "World"},
	}
	flat := flattenRecord(record)
	require.EqualValues(t, 1, flat["id"])
	require.Equal(t, "Hello", flat["title"])
	require.Equal(t, "World", flat["body"])
	_, hasAttrs := flat["attributes"]
	require.False(t, hasAttrs)
}

func TestFlattenRecord_V5(t *testing.T) {
	record := map[string]any{
		"id":         float64(1),
		"documentId": "abc123",
		"title":      "Hello",
	}
	flat := flattenRecord(record)
	require.EqualValues(t, 1, flat["id"])
	require.Equal(t, "abc123", flat["documentId"])
	require.Equal(t, "Hello", flat["title"])
}

func TestListRecords_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.ListRecords(ctx, QueryModel{ContentTypeID: "articles"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "canceled") || strings.Contains(err.Error(), "refused"),
		"expected cancellation error, got: %v", err)
}

func TestClampPageSize(t *testing.T) {
	require.Equal(t, defaultPageSize, clampPageSize(0))
	require.Equal(t, defaultPageSize, clampPageSize(-5))
	require.Equal(t, 10, clampPageSize(10))
	require.Equal(t, maxPageSize, clampPageSize(maxPageSize))
	require.Equal(t, maxPageSize, clampPageSize(5000))
}
