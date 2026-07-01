package plugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// decodeBody reads and JSON-decodes the request body into a generic map.
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	if len(raw) == 0 {
		return map[string]any{}
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}

func TestNewClient_DefaultsToAttioCloud(t *testing.T) {
	c, err := NewClient(Settings{apiToken: "tok"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, attioCloudURL, c.baseURL)
}

func TestNewClient_InvalidURL(t *testing.T) {
	_, err := NewClient(Settings{BaseURL: "://nope"}, http.DefaultClient)
	require.Error(t, err)
}

func TestNewClient_StripsTrailingSlash(t *testing.T) {
	c, err := NewClient(Settings{BaseURL: "https://api.attio.com/", apiToken: "tok"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, "https://api.attio.com", c.baseURL)
}

func TestQueryRecords(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/v2/objects/people/records/query", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":{"record_id":"r1"},"created_at":"2024-01-02T03:04:05.000000000Z","values":{"name":[{"attribute_type":"personal-name","full_name":"Ada Lovelace"}]}}]}`))
	})
	defer srv.Close()

	rows, err := c.QueryRecords(context.Background(), QueryModel{ObjectID: "people"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Ada Lovelace", rows[0]["name"])
	require.Equal(t, "r1", rows[0]["_record_id"])
	require.Equal(t, "2024-01-02T03:04:05.000000000Z", rows[0]["_created_at"])
}

func TestQueryRecords_RequiresObject(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.QueryRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestQueryRecords_ForwardsBody(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		require.EqualValues(t, 10, body["limit"])
		require.EqualValues(t, 20, body["offset"])
		sorts, ok := body["sorts"].([]any)
		require.True(t, ok)
		require.Len(t, sorts, 1)
		first := sorts[0].(map[string]any)
		require.Equal(t, "name", first["attribute"])
		require.Equal(t, "desc", first["direction"])
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.QueryRecords(context.Background(), QueryModel{
		ObjectID:  "people",
		Limit:     10,
		Offset:    20,
		sortItems: []SortItem{{Field: "name", Direction: "desc"}},
	})
	require.NoError(t, err)
}

func TestQueryRecords_ForwardsFilter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		filter, ok := body["filter"].(map[string]any)
		require.True(t, ok)
		stage, ok := filter["stage"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "Won", stage["$eq"])
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	q := QueryModel{
		ObjectID: "deals",
		filter: &FilterNode{
			Kind:      "group",
			Connector: "and",
			Children:  []FilterNode{{Kind: "condition", Field: "stage", Op: "eq", Value: "Won"}},
		},
	}
	_, err := c.QueryRecords(context.Background(), q)
	require.NoError(t, err)
}

func TestQueryRecords_Paginates(t *testing.T) {
	const pageSize = defaultPageSize
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body := decodeBody(t, r)
		offset := body["offset"]
		if calls == 1 {
			require.EqualValues(t, 0, offset)
			b := `{"data":[`
			for i := 0; i < pageSize; i++ {
				if i > 0 {
					b += ","
				}
				b += `{"id":{"record_id":"r` + strconv.Itoa(i) + `"},"values":{}}`
			}
			b += `]}`
			_, _ = w.Write([]byte(b))
			return
		}
		require.EqualValues(t, pageSize, offset)
		_, _ = w.Write([]byte(`{"data":[{"id":{"record_id":"last"},"values":{}}]}`))
	}))
	defer srv.Close()

	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	rows, err := c.QueryRecords(context.Background(), QueryModel{ObjectID: "people", Limit: pageSize + 1})
	require.NoError(t, err)
	require.Len(t, rows, pageSize+1)
	require.Equal(t, 2, calls)
}

func TestQueryRecords_RespectsLimit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body := decodeBody(t, r)
		require.EqualValues(t, 2, body["limit"])
		_, _ = w.Write([]byte(`{"data":[{"id":{"record_id":"a"},"values":{}},{"id":{"record_id":"b"},"values":{}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	rows, err := c.QueryRecords(context.Background(), QueryModel{ObjectID: "people", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 1, calls)
}

func TestCountRecords(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/v2/objects/people/records/query", r.URL.Path)
		body := decodeBody(t, r)
		if calls == 1 {
			require.EqualValues(t, 0, body["offset"])
			b := `{"data":[`
			for i := 0; i < defaultPageSize; i++ {
				if i > 0 {
					b += ","
				}
				b += `{"id":{"record_id":"r` + strconv.Itoa(i) + `"},"values":{}}`
			}
			b += `]}`
			_, _ = w.Write([]byte(b))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"record_id":"x"},"values":{}},{"id":{"record_id":"y"},"values":{}}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Settings{BaseURL: srv.URL, apiToken: "tok"}, srv.Client())

	n, err := c.CountRecords(context.Background(), QueryModel{ObjectID: "people"})
	require.NoError(t, err)
	require.EqualValues(t, defaultPageSize+2, n)
	require.Equal(t, 2, calls)
}

func TestCountRecords_RequiresObject(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.CountRecords(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListObjects(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/objects", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"api_slug":"people","singular_noun":"Person","plural_noun":"People"},{"api_slug":"companies","singular_noun":"Company","plural_noun":"Companies"},{"api_slug":null}]}`))
	})
	defer srv.Close()

	objs, err := c.ListObjects(context.Background())
	require.NoError(t, err)
	require.Len(t, objs, 2) // null api_slug is filtered out
	require.Equal(t, "people", objs[0].APISlug)
	require.Equal(t, "People", objs[0].PluralNoun)
}

func TestListAttributes(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/objects/people/attributes", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"api_slug":"name","title":"Name","type":"personal-name","is_required":true},{"api_slug":"email_addresses","title":"Email","type":"email-address"}]}`))
	})
	defer srv.Close()

	attrs, err := c.ListAttributes(context.Background(), "people")
	require.NoError(t, err)
	require.Len(t, attrs, 2)
	require.Equal(t, "name", attrs[0].APISlug)
	require.Equal(t, "personal-name", attrs[0].Type)
	require.True(t, attrs[0].IsRequired)
}

func TestListAttributes_RequiresObjectID(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListAttributes(context.Background(), "")
	require.Error(t, err)
}

func TestPing(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		require.Equal(t, "/v2/self", r.URL.Path)
		_, _ = w.Write([]byte(`{"active":true,"workspace_name":"Acme"}`))
	})
	defer srv.Close()
	require.NoError(t, c.Ping(context.Background()))
}

func TestPing_Inactive(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"active":false}`))
	})
	defer srv.Close()
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "inactive")
}

func TestErrorBodyExtraction(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status_code":401,"type":"authentication_error","code":"unauthorized","message":"Invalid API key"}`))
	})
	defer srv.Close()

	_, err := c.ListObjects(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "Invalid API key")
	require.Contains(t, err.Error(), "API Token")
}
