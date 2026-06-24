package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestClient builds a Client pointed at an httptest server. The base URL is
// the server root (no /api/v1 prefix), so handler paths are e.g. "/deals".
func newTestClient(t *testing.T, authMethod, token string, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := &Client{
		baseURL:    srv.URL,
		authMethod: authMethod,
		token:      token,
		httpClient: srv.Client(),
	}
	return c, srv
}

// ----- Auth modes --------------------------------------------------------------

func TestAuth_APITokenQueryParam(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok-123", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/users/me", r.URL.Path)
		require.Equal(t, "tok-123", r.URL.Query().Get("api_token"))
		require.Empty(t, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":1}}`))
	})
	defer srv.Close()

	require.NoError(t, c.Ping(context.Background()))
}

func TestAuth_OAuthBearerHeader(t *testing.T) {
	c, srv := newTestClient(t, authOAuth, "oauth-xyz", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/users/me", r.URL.Path)
		require.Equal(t, "Bearer oauth-xyz", r.Header.Get("Authorization"))
		require.Empty(t, r.URL.Query().Get("api_token"))
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":1}}`))
	})
	defer srv.Close()

	require.NoError(t, c.Ping(context.Background()))
}

func TestSettings_AuthModeResolution(t *testing.T) {
	// Explicit oauth with oauth token.
	s := Settings{AuthMethod: authOAuth, oauthToken: "o"}
	require.Equal(t, authOAuth, s.authMode())
	require.Equal(t, "o", s.credential())

	// apiToken default.
	s = Settings{AuthMethod: authAPIToken, apiToken: "a"}
	require.Equal(t, authAPIToken, s.authMode())
	require.Equal(t, "a", s.credential())

	// Configured for oauth but only api token present -> falls back to apiToken.
	s = Settings{AuthMethod: authOAuth, apiToken: "a"}
	require.Equal(t, authAPIToken, s.authMode())
	require.Equal(t, "a", s.credential())

	// Configured for apiToken but only oauth token present -> falls back to oauth.
	s = Settings{AuthMethod: authAPIToken, oauthToken: "o"}
	require.Equal(t, authOAuth, s.authMode())
	require.Equal(t, "o", s.credential())
}

// ----- Errors ------------------------------------------------------------------

func TestPing_Error(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"error":"you need to be authorized","error_info":"Please check the API token"}`))
	})
	defer srv.Close()

	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "you need to be authorized")
	require.Contains(t, err.Error(), "Please check the API token")
}

func TestGet_SuccessFalseSurfacesError(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		// 200 OK but success=false.
		_, _ = w.Write([]byte(`{"success":false,"error":"scope mismatch"}`))
	})
	defer srv.Close()

	_, err := c.ListUsers(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "scope mismatch")
}

// ----- Pagination --------------------------------------------------------------

func TestListAll_FollowsMoreItemsAndNextStart(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/deals", r.URL.Path)
		calls++
		switch r.URL.Query().Get("start") {
		case "", "0":
			require.Equal(t, "500", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1},{"id":2}],"additional_data":{"pagination":{"start":0,"limit":500,"more_items_in_collection":true,"next_start":2}}}`))
		case "2":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":3}],"additional_data":{"pagination":{"start":2,"limit":500,"more_items_in_collection":false}}}`))
		default:
			t.Fatalf("unexpected start=%q", r.URL.Query().Get("start"))
		}
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeDeals, MapCustomFields: boolPtr(false)})
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, 2, calls)
	require.EqualValues(t, 1, records[0]["id"])
	require.EqualValues(t, 3, records[2]["id"])
}

func TestListAll_StopsWhenMoreItemsFalseEvenWithNextStart(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		calls++
		// more_items_in_collection=false must stop the loop even though
		// next_start is present (a naive start+=limit loop would over-fetch).
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1}],"additional_data":{"pagination":{"more_items_in_collection":false,"next_start":1}}}`))
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypePersons, MapCustomFields: boolPtr(false)})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, 1, calls)
}

func TestListAll_RespectsLimitCap(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		// The first page already has more rows than the requested limit.
		require.Equal(t, "3", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1},{"id":2},{"id":3}],"additional_data":{"pagination":{"more_items_in_collection":true,"next_start":3}}}`))
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeDeals, Limit: 3, MapCustomFields: boolPtr(false)})
	require.NoError(t, err)
	require.Len(t, records, 3)
}

func TestListAll_HonoursStartOffset(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "10", r.URL.Query().Get("start"))
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":11}],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeDeals, Start: 10, MapCustomFields: boolPtr(false)})
	require.NoError(t, err)
	require.Len(t, records, 1)
}

// ----- Count -------------------------------------------------------------------

func TestCountRecords_PaginatesAndCounts(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/deals", r.URL.Path)
		switch r.URL.Query().Get("start") {
		case "", "0":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1},{"id":2}],"additional_data":{"pagination":{"more_items_in_collection":true,"next_start":2}}}`))
		case "2":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":3}],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
		}
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{CountEntity: queryTypeDeals})
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
}

func TestCountRecords_DefaultsToDeals(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/deals", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1}],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
	})
	defer srv.Close()

	n, err := c.CountRecords(context.Background(), QueryModel{})
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
}

// ----- Server-side filter params -----------------------------------------------

func TestEntityListParams_DealsFilters(t *testing.T) {
	p := entityListParams(queryTypeDeals, QueryModel{
		Status: "open", PipelineId: "3", StageId: "7", UserId: "9",
		SortBy: "update_time", SortDir: "ASC",
	})
	require.Equal(t, "open", p.Get("status"))
	require.Equal(t, "3", p.Get("pipeline_id"))
	require.Equal(t, "7", p.Get("stage_id"))
	require.Equal(t, "9", p.Get("user_id"))
	require.Equal(t, "update_time ASC", p.Get("sort"))
}

func TestEntityListParams_FilterIdTakesPrecedence(t *testing.T) {
	p := entityListParams(queryTypeDeals, QueryModel{
		FilterId: "42", Status: "open", PipelineId: "3", UserId: "9",
	})
	require.Equal(t, "42", p.Get("filter_id"))
	require.Empty(t, p.Get("status"))
	require.Empty(t, p.Get("pipeline_id"))
	require.Empty(t, p.Get("user_id"))
}

func TestEntityListParams_StatusAllOmitted(t *testing.T) {
	p := entityListParams(queryTypeDeals, QueryModel{Status: "all"})
	require.Empty(t, p.Get("status"))
}

func TestEntityListParams_SortDefaultsDesc(t *testing.T) {
	p := entityListParams(queryTypeProducts, QueryModel{SortBy: "add_time"})
	require.Equal(t, "add_time DESC", p.Get("sort"))
}

// ----- Custom field hash -> name remapping -------------------------------------

const sampleHash = "abc1234567890abc1234567890abc1234567890a" // 40 hex chars

func TestListRecords_RemapsCustomFields(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deals":
			_, _ = w.Write([]byte(`{"success":true,"data":[
				{"id":1,"title":"Big","` + sampleHash + `":"VIP","` + sampleHash + `_currency":"USD"}
			],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
		case "/dealFields":
			_, _ = w.Write([]byte(`{"success":true,"data":[
				{"key":"title","name":"Title"},
				{"key":"` + sampleHash + `","name":"Tier"}
			],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeDeals})
	require.NoError(t, err)
	require.Len(t, records, 1)
	rec := records[0]
	// Bare hash -> name; suffixed hash -> name + suffix.
	require.Equal(t, "VIP", rec["Tier"])
	require.Equal(t, "USD", rec["Tier_currency"])
	// The raw hash keys are gone.
	_, hasHash := rec[sampleHash]
	require.False(t, hasHash)
	// Standard fields untouched.
	require.Equal(t, "Big", rec["title"])
}

func TestListRecords_MappingDisabledKeepsHashes(t *testing.T) {
	dealFieldsCalled := false
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deals":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"` + sampleHash + `":"VIP"}],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
		case "/dealFields":
			dealFieldsCalled = true
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
		}
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeDeals, MapCustomFields: boolPtr(false)})
	require.NoError(t, err)
	require.Equal(t, "VIP", records[0][sampleHash])
	require.False(t, dealFieldsCalled, "dealFields must not be fetched when mapping is disabled")
}

func TestListRecords_MappingFailureIsNonFatal(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deals":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"` + sampleHash + `":"VIP"}],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
		case "/dealFields":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"success":false,"error":"missing scope"}`))
		}
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypeDeals})
	require.NoError(t, err)
	// Records still returned, keyed by the raw hash.
	require.Equal(t, "VIP", records[0][sampleHash])
}

func TestMappedKey(t *testing.T) {
	m := map[string]string{sampleHash: "Tier"}

	got, ok := mappedKey(sampleHash, m)
	require.True(t, ok)
	require.Equal(t, "Tier", got)

	got, ok = mappedKey(sampleHash+"_currency", m)
	require.True(t, ok)
	require.Equal(t, "Tier_currency", got)

	_, ok = mappedKey("title", m)
	require.False(t, ok)
}

func TestIsCustomFieldHash(t *testing.T) {
	require.True(t, isCustomFieldHash(sampleHash))
	require.False(t, isCustomFieldHash("title"))
	require.False(t, isCustomFieldHash("ABC1234567890ABC1234567890ABC1234567890A")) // uppercase
	require.False(t, isCustomFieldHash(sampleHash+"x"))                             // too long
}

// ----- Resource endpoints ------------------------------------------------------

func TestListPipelines(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/pipelines", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"name":"Sales Pipeline","order_nr":0}]}`))
	})
	defer srv.Close()

	pipelines, err := c.ListPipelines(context.Background())
	require.NoError(t, err)
	require.Len(t, pipelines, 1)
	require.Equal(t, 1, pipelines[0].ID)
	require.Equal(t, "Sales Pipeline", pipelines[0].Name)
}

func TestListStages(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/stages", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("pipeline_id"))
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":5,"name":"Lead In","pipeline_id":1,"order_nr":0}]}`))
	})
	defer srv.Close()

	stages, err := c.ListStages(context.Background(), "1")
	require.NoError(t, err)
	require.Len(t, stages, 1)
	require.Equal(t, "Lead In", stages[0].Name)
	require.Equal(t, 1, stages[0].PipelineID)
}

func TestListUsers(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/users", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":7,"name":"John Doe","email":"john@example.com"}]}`))
	})
	defer srv.Close()

	users, err := c.ListUsers(context.Background())
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "John Doe", users[0].Name)
	require.Equal(t, "john@example.com", users[0].Email)
}

// ----- Entity record fetch -----------------------------------------------------

func TestListRecords_FlattensPersonEmailPhone(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/persons":
			_, _ = w.Write([]byte(`{"success":true,"data":[{
				"id":1,
				"name":"Jane Doe",
				"email":[{"label":"work","value":"jane@example.com","primary":true}],
				"phone":[{"label":"work","value":"+15551234","primary":true}],
				"org_id":{"name":"Acme","value":42}
			}],"additional_data":{"pagination":{"more_items_in_collection":false}}}`))
		case "/personFields":
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
		}
	})
	defer srv.Close()

	records, err := c.ListRecords(context.Background(), QueryModel{QueryType: queryTypePersons})
	require.NoError(t, err)
	require.Len(t, records, 1)
	rec := records[0]
	// email/phone arrays flatten to their value (not the "work" label).
	require.Equal(t, "jane@example.com", rec["email"])
	require.Equal(t, "+15551234", rec["phone"])
	// relation object flattens to its readable name.
	require.Equal(t, "Acme", rec["org_id"])
}

func TestListRecords_UnsupportedEntity(t *testing.T) {
	c, srv := newTestClient(t, authAPIToken, "tok", func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	_, err := c.ListRecords(context.Background(), QueryModel{QueryType: "widgets"})
	require.Error(t, err)
}

func TestGet_RequiresBaseURL(t *testing.T) {
	c := &Client{authMethod: authAPIToken, token: "tok", httpClient: http.DefaultClient}
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "company domain")
}

func boolPtr(b bool) *bool { return &b }
