package plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, url string, settings Settings) *Client {
	t.Helper()
	c, err := NewClient(settings, http.DefaultClient)
	require.NoError(t, err)
	c.baseURL = url
	return c
}

func TestNewClient_DefaultsToTrelloURL(t *testing.T) {
	_, err := NewClient(Settings{apiKey: "k", apiToken: "t"}, http.DefaultClient)
	require.NoError(t, err)
}

func TestNewClient_RequiresApiKey(t *testing.T) {
	_, err := NewClient(Settings{apiToken: "t"}, http.DefaultClient)
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key")
}

func TestNewClient_RequiresApiToken(t *testing.T) {
	_, err := NewClient(Settings{apiKey: "k"}, http.DefaultClient)
	require.Error(t, err)
	require.Contains(t, err.Error(), "API token")
}

func TestClient_AuthParams(t *testing.T) {
	c := &Client{apiKey: "mykey", apiToken: "mytoken", baseURL: "https://api.trello.com", httpClient: http.DefaultClient}
	params := c.authParams()
	require.Contains(t, params, "key=mykey")
	require.Contains(t, params, "token=mytoken")
}

func TestClient_PingHitsMembersMe(t *testing.T) {
	var gotPath, gotKey, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.URL.Query().Get("key")
		gotToken = r.URL.Query().Get("token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"me"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/1/members/me", gotPath)
	require.Equal(t, "k", gotKey)
	require.Equal(t, "t", gotToken)
}

func TestClient_ErrorBodySurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid key"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid key")
	require.Contains(t, err.Error(), "401")
}

func TestClient_ListBoards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/1/members/me/boards", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"b1","name":"My Board","desc":"A board","shortUrl":"https://trello.com/b/abc","closed":false}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	boards, err := c.ListBoards(context.Background())
	require.NoError(t, err)
	require.Len(t, boards, 1)
	require.Equal(t, "b1", boards[0].ID)
	require.Equal(t, "My Board", boards[0].Name)
}

func TestClient_ListLists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/1/boards/b1/lists", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"l1","name":"Todo","pos":1,"closed":false}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	lists, err := c.ListLists(context.Background(), "b1")
	require.NoError(t, err)
	require.Len(t, lists, 1)
	require.Equal(t, "Todo", lists[0].Name)
}

func TestClient_ListMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/1/boards/b1/members", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"u1","fullName":"Alice","username":"alice"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	members, err := c.ListMembers(context.Background(), "b1")
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, "u1", members[0].ID)
	require.Equal(t, "Alice", members[0].FullName)
}

func TestClient_ListLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/1/boards/b1/labels", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"l1","name":"bug","color":"red"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	labels, err := c.ListLabels(context.Background(), "b1")
	require.NoError(t, err)
	require.Len(t, labels, 1)
	require.Equal(t, "bug", labels[0].Name)
}

func TestClient_ListCards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/1/boards/b1/cards", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"c1","name":"Card 1","closed":false,"idMembers":[],"labels":[]},{"id":"c2","name":"Card 2","closed":true,"idMembers":[],"labels":[]}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	cards, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all"})
	require.NoError(t, err)
	require.Len(t, cards, 2)
}

func TestClient_ListCards_FilterByList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/1/lists/l1/cards", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"c1","name":"Card 1"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	cards, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", ListId: "l1", CardFilter: "all"})
	require.NoError(t, err)
	require.Len(t, cards, 1)
}

func TestClient_ListCards_FilterByMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":"c1","name":"Card 1","idMembers":["u1","u2"],"labels":[]},
			{"id":"c2","name":"Card 2","idMembers":["u3"],"labels":[]}
		]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	cards, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all", MemberIds: []string{"u1"}})
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, "Card 1", cards[0]["name"])
}

func TestClient_ListCards_FilterByLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":"c1","name":"Bug","idMembers":[],"labels":[{"id":"l1","name":"bug","color":"red"}]},
			{"id":"c2","name":"Feature","idMembers":[],"labels":[{"id":"l2","name":"enhancement","color":"green"}]}
		]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	cards, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all", LabelIds: []string{"l1"}})
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, "Bug", cards[0]["name"])
}

func TestClient_CountCards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"c1"},{"id":"c2"},{"id":"c3"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	count, err := c.CountCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all"})
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
}

func TestClient_ListCards_Limit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"c1"},{"id":"c2"},{"id":"c3"},{"id":"c4"},{"id":"c5"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	cards, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all", Limit: 3})
	require.NoError(t, err)
	require.Len(t, cards, 3)
}

func TestClient_ListCards_Fields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"c1","name":"Card 1","desc":"A card","closed":false}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	cards, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all", Fields: []string{"name", "id"}})
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, "Card 1", cards[0]["name"])
	require.Equal(t, "c1", cards[0]["id"])
	_, hasDesc := cards[0]["desc"]
	require.False(t, hasDesc)
}

func TestGetBoard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/1/boards/b1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"b1","name":"My Board","desc":"A board"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	board, err := c.GetBoard(context.Background(), "b1")
	require.NoError(t, err)
	require.Equal(t, "My Board", board["name"])
}

// makeCardsDescending returns a JSON array of n cards whose ids encode strictly
// decreasing creation timestamps (baseTs-i), plus the id of the oldest card
// (the expected next `before` cursor). The id format mimics a Mongo ObjectId:
// 8 hex chars of Unix-second timestamp + 16 hex chars of counter.
func makeCardsDescending(n int, baseTs int64) (body string, oldestID string) {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		ts := baseTs - int64(i)
		id := fmt.Sprintf("%08x%016x", ts, i)
		parts[i] = fmt.Sprintf(`{"id":%q,"name":"card %d","idMembers":[],"labels":[]}`, id, i)
		oldestID = id // final iteration (i=n-1) has the smallest timestamp
	}
	return "[" + strings.Join(parts, ",") + "]", oldestID
}

func TestClient_ListCards_SendsKeyAndToken(t *testing.T) {
	var gotKey, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		gotToken = r.URL.Query().Get("token")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "mykey", apiToken: "mytoken"})
	_, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all"})
	require.NoError(t, err)
	require.Equal(t, "mykey", gotKey)
	require.Equal(t, "mytoken", gotToken)
}

func TestClient_ListCards_CursorPagination(t *testing.T) {
	page1, oldest1 := makeCardsDescending(maxCardsPerPage, 1600000000)
	var befores []string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "k", r.URL.Query().Get("key"))
		require.Equal(t, "t", r.URL.Query().Get("token"))
		require.Equal(t, "1000", r.URL.Query().Get("limit"))
		befores = append(befores, r.URL.Query().Get("before"))
		if calls == 0 {
			calls++
			_, _ = w.Write([]byte(page1)) // full page -> client must paginate
			return
		}
		// Older, short page -> pagination stops here.
		_, _ = w.Write([]byte(`[{"id":"5f5e0c00000000000000000a","name":"older","idMembers":[],"labels":[]}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	cards, err := c.ListCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all"})
	require.NoError(t, err)
	require.Len(t, cards, maxCardsPerPage+1)
	// Two requests: the first with no cursor, the second with before set to the
	// oldest card id from page one (NOT an offset/page number).
	require.Len(t, befores, 2)
	require.Equal(t, "", befores[0])
	require.Equal(t, oldest1, befores[1])
}

func TestClient_CountCards_PaginatesWithMinimalFields(t *testing.T) {
	page1, oldest1 := makeCardsDescending(maxCardsPerPage, 1600000000)
	var befores, fieldsParam []string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		befores = append(befores, r.URL.Query().Get("before"))
		fieldsParam = append(fieldsParam, r.URL.Query().Get("fields"))
		if calls == 0 {
			calls++
			_, _ = w.Write([]byte(page1))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"5f5e0c00000000000000000a"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	count, err := c.CountCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all"})
	require.NoError(t, err)
	require.EqualValues(t, maxCardsPerPage+1, count)
	// Count requests only the id field and uses the before cursor.
	require.Equal(t, "id", fieldsParam[0])
	require.Equal(t, "", befores[0])
	require.Equal(t, oldest1, befores[1])
}

func TestClient_CountCards_IgnoresLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"c1"},{"id":"c2"},{"id":"c3"},{"id":"c4"}]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	count, err := c.CountCards(context.Background(), CardsQuery{BoardId: "b1", CardFilter: "all", Limit: 2})
	require.NoError(t, err)
	require.EqualValues(t, 4, count) // Limit is ignored by count
}

func TestClient_ListCards_CreatedFilterCustom(t *testing.T) {
	var since, before string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since = r.URL.Query().Get("since")
		before = r.URL.Query().Get("before")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	_, err := c.ListCards(context.Background(), CardsQuery{
		BoardId:       "b1",
		CardFilter:    "all",
		CreatedMode:   dateModeCustom,
		CreatedAfter:  "2024-01-01",
		CreatedBefore: "2024-12-31",
	})
	require.NoError(t, err)
	require.Equal(t, "2024-01-01T00:00:00Z", since)
	require.Equal(t, "2024-12-31T00:00:00Z", before)
}

func TestClient_ListCards_CreatedFilterDashboard(t *testing.T) {
	var since, before string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since = r.URL.Query().Get("since")
		before = r.URL.Query().Get("before")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	_, err := c.ListCards(context.Background(), CardsQuery{
		BoardId:     "b1",
		CardFilter:  "all",
		CreatedMode: dateModeDashboard,
		TimeRange:   backend.TimeRange{From: from, To: to},
	})
	require.NoError(t, err)
	require.Equal(t, from.Format(time.RFC3339), since)
	require.Equal(t, to.Format(time.RFC3339), before)
}

func TestClient_ListCards_CreatedAnyAddsNothing(t *testing.T) {
	var since, before string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since = r.URL.Query().Get("since")
		before = r.URL.Query().Get("before")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiKey: "k", apiToken: "t"})
	_, err := c.ListCards(context.Background(), CardsQuery{
		BoardId:      "b1",
		CardFilter:   "all",
		CreatedAfter: "2024-01-01", // ignored because mode defaults to "any"
	})
	require.NoError(t, err)
	require.Empty(t, since)
	require.Empty(t, before)
}

func TestClient_ListCards_RequiresScope(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiKey: "k", apiToken: "t"})
	_, err := c.ListCards(context.Background(), CardsQuery{CardFilter: "all"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "board or list")
}
