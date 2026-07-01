package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	// maxCardsPerPage is Trello's hard cap on results returned by the card
	// endpoints. There is no offset/page parameter; larger result sets are walked
	// with the `before` cursor (see iterateCards).
	maxCardsPerPage = 1000
	// maxRecords bounds how many cards we will ever accumulate for a single query
	// (a safety valve when no Limit is set).
	maxRecords = 100000
)

// fullCardFields is the set of card fields requested for "cards" queries. Note
// that customFieldItems is NOT a `fields` value — it is enabled via its own
// boolean query parameter (see cardFetchOpts.customFieldItems).
const fullCardFields = "id,name,desc,due,dueComplete,start,dateLastActivity,idList,idBoard,idMembers,labels,idChecklists,closed,pos,shortUrl,url,badges"

// Client is a thin wrapper around Trello's REST API v1. Trello is hosted SaaS
// and authenticates via `key` + `token` query parameters on every request.
type Client struct {
	apiKey     string
	apiToken   string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Trello API client. The provided httpClient is normally the
// SDK-managed client so that proxy, TLS and timeout settings are respected. Both
// the API key and token are required.
func NewClient(settings Settings, httpClient *http.Client) (*Client, error) {
	base := strings.TrimRight(trelloAPIBase, "/")
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", base, err)
	}
	if settings.apiKey == "" {
		return nil, fmt.Errorf("trello API key is not configured")
	}
	if settings.apiToken == "" {
		return nil, fmt.Errorf("trello API token is not configured")
	}
	return &Client{
		apiKey:     settings.apiKey,
		apiToken:   settings.apiToken,
		baseURL:    base,
		httpClient: httpClient,
	}, nil
}

// errorResponse is the JSON body Trello returns on error. Trello is
// inconsistent: some endpoints return {"message":"..."} and others a bare
// string, which `do` handles separately.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (e errorResponse) message() string {
	switch {
	case e.Error != "":
		return e.Error
	case e.Message != "":
		return e.Message
	default:
		return ""
	}
}

// do issues a GET request to the given path (relative to the API root, including
// the auth query parameters) and returns the raw response body. The full URL —
// which carries the secret key/token — is never logged.
func (c *Client) do(ctx context.Context, path string) (json.RawMessage, error) {
	full := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to trello failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading trello response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e errorResponse
		if json.Unmarshal(raw, &e) == nil && e.message() != "" {
			return nil, fmt.Errorf("trello returned status %d: %s", resp.StatusCode, truncate(e.message()))
		}
		return nil, fmt.Errorf("trello returned status %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	return raw, nil
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

// authParams returns the `?key=...&token=...` query string appended to every
// request. The values are URL-escaped.
func (c *Client) authParams() string {
	return "?key=" + url.QueryEscape(c.apiKey) + "&token=" + url.QueryEscape(c.apiToken)
}

// Ping performs a minimal authenticated request (the authorized member endpoint)
// to validate connectivity and credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "/1/members/me"+c.authParams())
	return err
}

// ---------------------------------------------------------------------------
// Resource helpers: populate the QueryEditor dropdowns. All are board-scoped
// except ListBoards, which is user-scoped via /1/members/me/boards.
// ---------------------------------------------------------------------------

// ListBoards returns the open boards the authenticated member can see.
func (c *Client) ListBoards(ctx context.Context) ([]BoardInfo, error) {
	raw, err := c.do(ctx, "/1/members/me/boards"+c.authParams()+"&fields=id,name,desc,shortUrl,closed,dateLastActivity&filter=open")
	if err != nil {
		return nil, err
	}
	var boards []BoardInfo
	if err := json.Unmarshal(raw, &boards); err != nil {
		return nil, fmt.Errorf("failed parsing boards: %w", err)
	}
	return boards, nil
}

// GetBoard returns the raw board object for the given id.
func (c *Client) GetBoard(ctx context.Context, boardID string) (map[string]any, error) {
	raw, err := c.do(ctx, "/1/boards/"+url.PathEscape(boardID)+c.authParams())
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("failed parsing board: %w", err)
	}
	return result, nil
}

// ListLists returns the lists on a board.
func (c *Client) ListLists(ctx context.Context, boardID string) ([]ListInfo, error) {
	raw, err := c.do(ctx, "/1/boards/"+url.PathEscape(boardID)+"/lists"+c.authParams()+"&fields=id,name,pos,closed")
	if err != nil {
		return nil, err
	}
	var lists []ListInfo
	if err := json.Unmarshal(raw, &lists); err != nil {
		return nil, fmt.Errorf("failed parsing lists: %w", err)
	}
	return lists, nil
}

// ListMembers returns the members of a board.
func (c *Client) ListMembers(ctx context.Context, boardID string) ([]MemberInfo, error) {
	raw, err := c.do(ctx, "/1/boards/"+url.PathEscape(boardID)+"/members"+c.authParams()+"&fields=id,fullName,username,avatarUrl")
	if err != nil {
		return nil, err
	}
	var members []MemberInfo
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, fmt.Errorf("failed parsing members: %w", err)
	}
	return members, nil
}

// ListLabels returns the labels defined on a board.
func (c *Client) ListLabels(ctx context.Context, boardID string) ([]LabelInfo, error) {
	raw, err := c.do(ctx, "/1/boards/"+url.PathEscape(boardID)+"/labels"+c.authParams()+"&fields=id,name,color&limit=1000")
	if err != nil {
		return nil, err
	}
	var labels []LabelInfo
	if err := json.Unmarshal(raw, &labels); err != nil {
		return nil, fmt.Errorf("failed parsing labels: %w", err)
	}
	return labels, nil
}

// ---------------------------------------------------------------------------
// Cards: cursor pagination + client-side member/label filtering.
// ---------------------------------------------------------------------------

// CardsQuery captures everything needed to fetch and filter cards.
type CardsQuery struct {
	BoardId    string
	ListId     string
	CardFilter string
	MemberIds  []string
	LabelIds   []string
	Fields     []string
	Limit      int

	// Creation-date filter (mapped to Trello's since/before cursor parameters,
	// which operate on the card id's embedded creation timestamp).
	CreatedMode   string
	CreatedAfter  string
	CreatedBefore string
	TimeRange     backend.TimeRange
}

// cardFetchOpts controls which card payload a page request asks for.
type cardFetchOpts struct {
	// fields is the comma-separated `fields` query parameter value.
	fields string
	// customFieldItems enables the customFieldItems=true query parameter (only
	// meaningful for full card reads, not counts).
	customFieldItems bool
}

// ListCards fetches the matching cards, transparently following the `before`
// cursor across pages, applying client-side member/label filters, flattening
// each card, and capping the result at the requested limit.
func (c *Client) ListCards(ctx context.Context, q CardsQuery) ([]map[string]any, error) {
	if strings.TrimSpace(q.BoardId) == "" && strings.TrimSpace(q.ListId) == "" {
		return nil, fmt.Errorf("a board or list is required for card queries")
	}

	hardLimit := q.Limit
	if hardLimit <= 0 {
		hardLimit = maxRecords
	}
	// When there are no client-side filters and a small limit is requested, a
	// single small page suffices. With filters we must read full pages because we
	// cannot know in advance how many cards will match.
	pageLimit := maxCardsPerPage
	if !hasClientFilters(q) && q.Limit > 0 && q.Limit < pageLimit {
		pageLimit = q.Limit
	}

	opts := cardFetchOpts{fields: fullCardFields, customFieldItems: true}
	result := make([]map[string]any, 0, pageLimit)
	err := c.iterateCards(ctx, q, opts, pageLimit, func(raw map[string]any) bool {
		flat := flattenCard(raw)
		if len(q.Fields) > 0 {
			flat = selectCardFields(flat, q.Fields)
		}
		result = append(result, flat)
		return len(result) < hardLimit
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CountCards returns the number of cards matching the query. Trello has no count
// endpoint, so cards are paginated (requesting only the minimal fields needed)
// and counted. The Limit field is intentionally ignored — a count is always of
// all matching cards.
func (c *Client) CountCards(ctx context.Context, q CardsQuery) (int64, error) {
	if strings.TrimSpace(q.BoardId) == "" && strings.TrimSpace(q.ListId) == "" {
		return 0, fmt.Errorf("a board or list is required for card queries")
	}

	opts := cardFetchOpts{fields: countCardFields(q)}
	var count int64
	err := c.iterateCards(ctx, q, opts, maxCardsPerPage, func(_ map[string]any) bool {
		count++
		return count < maxRecords
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// iterateCards walks the card pages for a query, invoking fn for each card that
// passes the client-side filters. fn returns false to stop early. Pagination is
// cursor-based: each page's earliest-created card id becomes the `before`
// parameter for the next (older) page. Trello returns at most maxCardsPerPage
// cards and has no offset/page parameter.
func (c *Client) iterateCards(ctx context.Context, q CardsQuery, opts cardFetchOpts, pageLimit int, fn func(raw map[string]any) bool) error {
	since := createdLowerBound(q)
	before := createdUpperBound(q)
	for {
		page, err := c.fetchCardsPage(ctx, q, opts, since, before, pageLimit)
		if err != nil {
			return err
		}
		for _, raw := range page {
			if !matchCardFilters(raw, q) {
				continue
			}
			if !fn(raw) {
				return nil
			}
		}
		// A short page means Trello had no more cards to return.
		if len(page) < pageLimit {
			return nil
		}
		// Advance the cursor to the oldest card in this page so the next request
		// returns strictly older cards. If we cannot derive a cursor, stop rather
		// than risk an infinite loop.
		next := earliestCardID(page)
		if next == "" || next == before {
			return nil
		}
		before = next
	}
}

// fetchCardsPage requests a single page of cards from the most specific endpoint
// available (list-scoped when ListId is set, otherwise board-scoped).
func (c *Client) fetchCardsPage(ctx context.Context, q CardsQuery, opts cardFetchOpts, since, before string, limit int) ([]map[string]any, error) {
	v := url.Values{}
	v.Set("key", c.apiKey)
	v.Set("token", c.apiToken)
	if opts.fields != "" {
		v.Set("fields", opts.fields)
	}
	if f := strings.TrimSpace(q.CardFilter); f != "" {
		v.Set("filter", f)
	}
	if since != "" {
		v.Set("since", since)
	}
	if before != "" {
		v.Set("before", before)
	}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if opts.customFieldItems {
		v.Set("customFieldItems", "true")
	}

	var path string
	if l := strings.TrimSpace(q.ListId); l != "" {
		path = "/1/lists/" + url.PathEscape(l) + "/cards"
	} else {
		path = "/1/boards/" + url.PathEscape(q.BoardId) + "/cards"
	}

	raw, err := c.do(ctx, path+"?"+v.Encode())
	if err != nil {
		return nil, err
	}
	var cards []map[string]any
	if err := json.Unmarshal(raw, &cards); err != nil {
		return nil, fmt.Errorf("failed parsing cards: %w", err)
	}
	return cards, nil
}

// countCardFields returns the minimal `fields` value for a count: just the id,
// plus whichever fields are needed to evaluate the client-side filters.
func countCardFields(q CardsQuery) string {
	fields := []string{"id"}
	if len(nonEmpty(q.MemberIds)) > 0 {
		fields = append(fields, "idMembers")
	}
	if len(nonEmpty(q.LabelIds)) > 0 {
		fields = append(fields, "labels")
	}
	return strings.Join(fields, ",")
}

// hasClientFilters reports whether any client-side card filter is active.
func hasClientFilters(q CardsQuery) bool {
	return len(nonEmpty(q.MemberIds)) > 0 || len(nonEmpty(q.LabelIds)) > 0
}

// createdLowerBound returns the `since` value (creation lower bound) for the
// query, or "" when no lower bound applies.
func createdLowerBound(q CardsQuery) string {
	switch q.CreatedMode {
	case dateModeDashboard:
		if !q.TimeRange.From.IsZero() {
			return q.TimeRange.From.UTC().Format(time.RFC3339)
		}
	case dateModeCustom:
		return toTrelloDate(q.CreatedAfter)
	}
	return ""
}

// createdUpperBound returns the initial `before` value (creation upper bound)
// for the query, or "" when no upper bound applies. During pagination this is
// superseded by each page's earliest card id.
func createdUpperBound(q CardsQuery) string {
	switch q.CreatedMode {
	case dateModeDashboard:
		if !q.TimeRange.To.IsZero() {
			return q.TimeRange.To.UTC().Format(time.RFC3339)
		}
	case dateModeCustom:
		return toTrelloDate(q.CreatedBefore)
	}
	return ""
}

// toTrelloDate normalizes a user-provided bound into a value Trello's
// since/before parameters accept. Unix millis/seconds become ISO-8601; ISO-like
// strings are reformatted to RFC3339; anything else (e.g. a card id) passes
// through unchanged.
func toTrelloDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if len(s) >= 13 {
			return time.UnixMilli(n).UTC().Format(time.RFC3339)
		}
		return time.Unix(n, 0).UTC().Format(time.RFC3339)
	}
	if t, ok := toTime(s); ok {
		return t.UTC().Format(time.RFC3339)
	}
	return s
}

// earliestCardID returns the id of the oldest-created card in a page, used as the
// next `before` cursor. Creation time is derived from the card id (a Mongo
// ObjectId whose first 8 hex digits are the Unix-second creation time).
func earliestCardID(cards []map[string]any) string {
	best := ""
	var bestTs int64
	for _, card := range cards {
		id, _ := card["id"].(string)
		if id == "" {
			continue
		}
		ts, ok := cardCreatedUnix(id)
		if !ok {
			continue
		}
		if best == "" || ts < bestTs || (ts == bestTs && id < best) {
			best = id
			bestTs = ts
		}
	}
	return best
}

// matchCardFilters reports whether a raw card passes the client-side member and
// label filters. Each filter matches when the card carries ANY of the requested
// ids. Trello's card endpoints offer no server-side member/label filter.
func matchCardFilters(card map[string]any, q CardsQuery) bool {
	if members := nonEmpty(q.MemberIds); len(members) > 0 {
		memberSet := make(map[string]bool, len(members))
		for _, m := range members {
			memberSet[m] = true
		}
		ids, _ := card["idMembers"].([]any)
		matched := false
		for _, m := range ids {
			if id, ok := m.(string); ok && memberSet[id] {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if labels := nonEmpty(q.LabelIds); len(labels) > 0 {
		labelSet := make(map[string]bool, len(labels))
		for _, l := range labels {
			labelSet[l] = true
		}
		objs, _ := card["labels"].([]any)
		matched := false
		for _, l := range objs {
			if labelObj, ok := l.(map[string]any); ok {
				if id, ok := labelObj["id"].(string); ok && labelSet[id] {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// selectCardFields restricts a flattened card record to the requested output
// columns. An empty selection returns the record unchanged.
func selectCardFields(record map[string]any, fields []string) map[string]any {
	want := make(map[string]bool, len(fields))
	for _, f := range nonEmpty(fields) {
		want[f] = true
	}
	if len(want) == 0 {
		return record
	}
	out := make(map[string]any, len(want))
	for k, v := range record {
		if want[k] {
			out[k] = v
		}
	}
	return out
}
