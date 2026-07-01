package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testUUID  = "base-uuid-1"
	testToken = "exchanged-access-token"
)

// tokenPath is the seahub endpoint that exchanges the Base API Token for a
// short-lived Base-Token (access token) plus the base uuid.
const tokenPath = "/api/v2.1/dtable/app-access-token/"

// newTestServer builds an httptest server whose handler also serves the token
// exchange. The exchange records how many times it was called via *exchanges and
// points dtable_server back at the same test server so data calls land here too.
func newTestServer(t *testing.T, exchanges *int, dataHandler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			require.Equal(t, "Token base-api-token", r.Header.Get("Authorization"))
			if exchanges != nil {
				*exchanges++
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"app_name":      "test",
				"access_token":  testToken,
				"dtable_uuid":   testUUID,
				"dtable_server": srv.URL + "/api-gateway/",
				"dtable_socket": srv.URL + "/",
			})
			return
		}
		dataHandler(w, r)
	}))
	c, err := NewClient(Settings{ServerURL: srv.URL, apiToken: "base-api-token"}, srv.Client())
	require.NoError(t, err)
	return c, srv
}

func TestPing_ExchangesToken(t *testing.T) {
	exchanges := 0
	c, srv := newTestServer(t, &exchanges, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	defer srv.Close()

	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, 1, exchanges)
	require.Equal(t, testUUID, c.dtableUUID)
	require.Equal(t, testToken, c.accessToken)
}

func TestListRecords_RowsEndpoint(t *testing.T) {
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api-gateway/api/v2/dtables/"+testUUID+"/rows/", r.URL.Path)
		require.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
		q := r.URL.Query()
		require.Equal(t, "Table1", q.Get("table_name"))
		require.Equal(t, "true", q.Get("convert_keys"))
		require.Equal(t, "1000", q.Get("limit"))
		require.Equal(t, "0", q.Get("start"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rows": []map[string]any{
				{"_id": "r1", "_ctime": "2024-01-01T00:00:00.000+00:00", "_mtime": "2024-01-02T00:00:00.000+00:00", "_creator": "u@auth.local", "Name": "Alice", "Age": float64(30)},
			},
		})
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableName: "Table1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Alice", rows[0]["Name"])
	require.Equal(t, "r1", rows[0]["_id"])
	require.Contains(t, rows[0], "_ctime")
	require.Contains(t, rows[0], "_mtime")
	// Internal metadata other than _id/_ctime/_mtime is stripped.
	require.NotContains(t, rows[0], "_creator")
}

func TestListRecords_ForwardsView(t *testing.T) {
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Default View", r.URL.Query().Get("view_name"))
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": []map[string]any{}})
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableName: "Table1", ViewName: "Default View"})
	require.NoError(t, err)
}

func TestListRecords_ViaSQLWhenFiltered(t *testing.T) {
	var body map[string]any
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api-gateway/api/v2/dtables/"+testUUID+"/sql/", r.URL.Path)
		require.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"Name": "Alice"}},
			"success": true,
		})
	})
	defer srv.Close()

	q := QueryModel{TableName: "Table1", filter: &FilterNode{
		Kind:      "group",
		Connector: "and",
		Children:  []FilterNode{{Kind: "condition", Field: "Plan", Op: "eq", Value: "pro"}},
	}}
	rows, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	sql, _ := body["sql"].(string)
	require.Contains(t, sql, "SELECT * FROM `Table1`")
	require.Contains(t, sql, "WHERE `Plan` = ?")
	require.Contains(t, sql, "LIMIT")
	require.Equal(t, true, body["convert_keys"])
	require.Equal(t, []any{"pro"}, body["parameters"])
}

func TestListRecords_ViaSQLWithFieldsAndSort(t *testing.T) {
	var body map[string]any
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{}})
	})
	defer srv.Close()

	q := QueryModel{
		TableName: "Table1",
		Fields:    "Name, Age",
		sortItems: []SortItem{{Field: "Age", Direction: "desc"}},
	}
	_, err := c.ListRecords(context.Background(), q)
	require.NoError(t, err)
	sql := body["sql"].(string)
	require.Contains(t, sql, "SELECT `Name`, `Age` FROM `Table1`")
	require.Contains(t, sql, "ORDER BY `Age` DESC")
}

func TestListRecords_RowsPaginates(t *testing.T) {
	calls := 0
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		calls++
		start := r.URL.Query().Get("start")
		if calls == 1 {
			require.Equal(t, "0", start)
			rows := make([]map[string]any, rowsPageSize)
			for i := range rows {
				rows[i] = map[string]any{"Name": "x"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows})
			return
		}
		require.Equal(t, "1000", start)
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": []map[string]any{{"Name": "y"}}})
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableName: "Table1"})
	require.NoError(t, err)
	require.Len(t, rows, rowsPageSize+1)
	require.Equal(t, 2, calls)
}

func TestListRecords_RespectsLimit(t *testing.T) {
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "2", r.URL.Query().Get("limit"))
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": []map[string]any{{"Name": "a"}, {"Name": "b"}}})
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableName: "Table1", Limit: 2})
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestCountRecords(t *testing.T) {
	var body map[string]any
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api-gateway/api/v2/dtables/"+testUUID+"/sql/", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"COUNT(*)": float64(42)}},
		})
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{TableName: "Table1"})
	require.NoError(t, err)
	require.EqualValues(t, 42, n)
	require.Contains(t, body["sql"].(string), "SELECT COUNT(*) FROM `Table1`")
}

func TestRunSQL(t *testing.T) {
	var body map[string]any
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"city": "Paris", "n": float64(3)}},
		})
	})
	defer srv.Close()

	rows, err := c.RunSQL(context.Background(), QueryModel{SQL: "SELECT city, COUNT(*) AS n FROM Contacts GROUP BY city"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Paris", rows[0]["city"])
	require.Equal(t, "SELECT city, COUNT(*) AS n FROM Contacts GROUP BY city", body["sql"])
	require.Equal(t, true, body["convert_keys"])
	require.NotContains(t, body, "parameters")
}

func TestRunSQL_RequiresSQL(t *testing.T) {
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.RunSQL(context.Background(), QueryModel{})
	require.Error(t, err)
}

func TestListTables(t *testing.T) {
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api-gateway/api/v2/dtables/"+testUUID+"/metadata/", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]any{
				"tables": []map[string]any{
					{"_id": "0000", "name": "Users", "columns": []map[string]any{
						{"key": "0000", "name": "Name", "type": "text"},
						{"key": "0001", "name": "Age", "type": "number"},
						{"key": "0002", "name": "", "type": "text"},
					}},
					{"_id": "0001", "name": "Orders", "columns": []map[string]any{}},
				},
			},
		})
	})
	defer srv.Close()

	tables, err := c.ListTables(context.Background())
	require.NoError(t, err)
	require.Len(t, tables, 2)
	require.Equal(t, "Users", tables[0].Name)
	require.Len(t, tables[0].Columns, 2) // unnamed column skipped
	require.Equal(t, "Name", tables[0].Columns[0].Name)
	require.Equal(t, "number", tables[0].Columns[1].Type)
}

func TestTokenCachedAcrossCalls(t *testing.T) {
	exchanges := 0
	c, srv := newTestServer(t, &exchanges, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": []map[string]any{}})
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableName: "T"})
	require.NoError(t, err)
	_, err = c.ListRecords(context.Background(), QueryModel{TableName: "T"})
	require.NoError(t, err)
	require.Equal(t, 1, exchanges) // exchanged once, then cached
}

func TestReexchangesOn401(t *testing.T) {
	exchanges := 0
	dataCalls := 0
	c, srv := newTestServer(t, &exchanges, func(w http.ResponseWriter, r *http.Request) {
		dataCalls++
		if dataCalls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error_msg":"Token expired."}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": []map[string]any{{"Name": "ok"}}})
	})
	defer srv.Close()

	rows, err := c.ListRecords(context.Background(), QueryModel{TableName: "T"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 2, exchanges) // initial + after 401
	require.Equal(t, 2, dataCalls)
}

func TestStatusHintSurfacedOnError(t *testing.T) {
	c, srv := newTestServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error_msg":"table not found"}`))
	})
	defer srv.Close()

	_, err := c.ListRecords(context.Background(), QueryModel{TableName: "Nope"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
	require.Contains(t, err.Error(), "table not found")
	require.True(t, strings.Contains(err.Error(), "Server URL"))
}

func TestNewClient_DefaultsToCloud(t *testing.T) {
	c, err := NewClient(Settings{apiToken: "x"}, http.DefaultClient)
	require.NoError(t, err)
	require.Equal(t, seatableCloudURL, c.serverURL)
}
