# Trello Data Source for Grafana

A Grafana data source plugin that queries [Trello](https://trello.com) boards,
lists, and cards. Uses the Trello REST API v1 with API **key + token**
authentication. Trello is hosted SaaS, so the API base URL is fixed to
`https://api.trello.com`.

## Features

- **Cards** query type with board/list selection, card filter, member/label
  filters, a creation-date filter, field selection, and limit
- **Count** query type for card counts (all matching cards, auto-paginated)
- Board, list, member, and label pickers in the query editor
- Go backend for secure credential handling and server-side flattening

## Requirements

- Grafana >= 10.4.0
- Trello API key and API token (both required)

## Configuration

1. Create a Power-Up and generate an API key at
   [https://trello.com/power-ups/admin](https://trello.com/power-ups/admin)
   (API Key tab). For personal use the legacy
   [https://trello.com/app-key](https://trello.com/app-key) page also shows a key.
2. Generate an API token from the same page (the "Token" link next to the key).
3. Configure the data source in Grafana with both values.

### Authentication

Trello uses **two** credentials sent as query parameters on **every** request:

- `key` — the API key (publicly identifies the app)
- `token` — the API token (grants access to the user's data; keep it secret)

Both are stored in `secureJsonData` and never sent to the browser or logged. The
full request URL (which carries the secrets) is never written to logs.

## Querying

### Cards

Returns cards from a board (or a specific list) with optional filters:

- **Board** — the Trello board to query (required)
- **List** — optionally narrow to a single list on the board
- **Card filter** — `all`, `open`, or `closed`
- **Members** — filter by one or more board members (matches any)
- **Labels** — filter by one or more labels (matches any)
- **Created** — filter by card creation date: `Any time`, `Dashboard range`
  (uses the panel's time picker), or `Custom` (explicit after/before bounds)
- **Fields** — which flattened columns to return (empty = all)
- **Limit** — maximum number of cards (0 = all, auto-paginated)

Each card is flattened to scalar columns: `id`, `name`, `desc`, `closed`, `pos`,
`shortUrl`, `url`, `idList`, `idBoard`, `idMembers` (joined ids), `labels`
(joined names), `idChecklists` (joined ids), `due`, `dueComplete`, `start`,
`dateLastActivity`, a derived `dateCreated`, the badge counts
(`badges_votes`, `badges_comments`, `badges_attachments`, `badges_checkItems`,
`badges_checkItemsChecked`), and `customFieldItems` (compact JSON). Time columns
are placed first; row order from Trello is preserved.

### Count

Returns the count of cards matching the board/list, card filter, member/label
filters, and creation-date filter. The `Limit` field is ignored for counts.

## How it works (API notes)

- **Cursor pagination, not offset.** Trello caps card results at **1000** per
  request and has **no** `offset`/`page` parameter. Larger result sets are walked
  with the `before` cursor: each page's oldest card id (derived from the card
  id's embedded creation timestamp) becomes the `before` value for the next,
  older page. A short page ends pagination.
- **Count has no endpoint.** Counts are computed by paginating with only the
  minimal fields (`id`, plus `idMembers`/`labels` when those filters are active)
  and counting the matches.
- **Creation-date filter uses `since`/`before`.** Trello's card endpoints only
  support filtering by creation date, via `since` (lower bound) and `before`
  (upper bound). These accept an ISO-8601 date or a card id; the upper bound also
  doubles as the pagination cursor.
- **Member/label filters are client-side.** Trello's card endpoints have no
  server-side member or label filter, so these are applied in the Go backend
  after fetching.
- **Custom fields.** `customFieldItems=true` is requested for card reads and the
  resulting array is serialized to JSON in the `customFieldItems` column. To map
  ids to readable names you would additionally call
  `GET /1/boards/{id}/customFields` (not done by default).
- **`dateCreated`** is derived locally from the card id (first 8 hex digits =
  Unix-second creation time); no extra request is made.

## Limitations

- **Rate limits.** Trello allows **300 requests / 10 seconds per API key** and
  **100 requests / 10 seconds per token**. Large boards paginated 1000 at a time
  can approach these limits; a 429 surfaces as a query error.
- **No offset pagination.** Only the `before`/`since` creation-date cursor is
  available (see above).
- **Board-scoped.** Cards are always read per board (or per list); there is no
  cross-board query. Boards themselves are user-scoped via
  `/1/members/me/boards`.
- **Only creation date is server-filterable.** `due` and `dateLastActivity`
  cannot be filtered server-side by the cards endpoint; fetch and filter in the
  panel if needed.
- **Same-second precision.** Because the cursor resolves to second granularity,
  cards created in the exact same second as a page boundary are an edge case.

## Development

```bash
# Build the plugin (frontend + backend)
yarn build

# Frontend checks
yarn typecheck && yarn lint && yarn test

# Backend checks
gofmt -l pkg/ && go build ./... && go vet ./... && go test ./pkg/...
```

### Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the produced data frames.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

## License

Apache-2.0
