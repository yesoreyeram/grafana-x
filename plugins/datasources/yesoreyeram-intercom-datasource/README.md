# Grafana Intercom Data Source

A Grafana data source plugin for [Intercom](https://www.intercom.com) (customer
support & messaging). Query conversations, contacts, tickets, articles,
companies, admins, teams, and tags from the Intercom REST API directly in
Grafana.

## Features

- **Entities** — conversations, contacts, tickets, articles, companies, admins,
  teams, tags
- **List or search** — conversations and contacts list by default and switch to
  the Search API automatically when filters are set; tickets always use Search
- **Structured pickers** — conversation state, contact role, admin assignee,
  team assignee, tag (admins/teams/tags fetched live from your workspace)
- **Generic filter builder** — field / operator / value rows compiled into the
  Intercom Search API `query` object (AND'd), with all search operators
  (`=`, `!=`, `>`, `<`, `~`, `!~`, `^`, `$`, `IN`, `NIN`)
- **Count** — `total_count` for any countable entity
- **Sort** — by any field, ascending or descending
- **Cursor pagination** — transparently follows `pages.next.starting_after` up
  to your limit (or a safety cap of 100,000 records)
- **Unix-seconds timestamps** — Intercom epoch-seconds fields (`created_at`,
  `updated_at`, `last_seen_at`, `signed_up_at`, …) are converted to proper time
  fields for time-series panels
- **Regions** — US, EU and AU data residency (or a custom base URL / proxy)
- **Versioned** — configurable `Intercom-Version` header (default `2.11`)

## Authentication

Intercom uses a single auth mode: an **access token** sent as
`Authorization: Bearer <token>`.

Create a token in the [Intercom Developer Hub](https://app.intercom.com/a/apps/_/developer-hub)
(or your app's settings) and paste it into the data source configuration. The
token is stored as `secureJsonData` and never returned to the browser.

## Configuration

| Field | Description |
|---|---|
| **Region** | Intercom data residency region: US, EU or AU. Determines the API host unless **API URL** is set. |
| **API URL** | Optional. Overrides the region-derived host. `https://api.intercom.io` (US), `https://api.eu.intercom.io` (EU), `https://api.au.intercom.io` (AU), or a proxy. |
| **Intercom-Version** | Value of the `Intercom-Version` header. Default `2.11`. |
| **Access Token** | Intercom access token, sent as `Authorization: Bearer`. Stored securely. |

The health check calls `GET /me`.

## Query Editor

| Section | Applies to | Description |
|---|---|---|
| **Query type** | all | Entity to query (or `Count`). |
| **Count of** | count | Entity to count via `total_count`. |
| **State** | conversations | Conversation state: open / closed / snoozed. |
| **Role** | contacts | Contact role: user / lead. |
| **Assignee** | conversations, tickets | Admin assignee (`admin_assignee_id`). |
| **Team** | conversations, tickets | Team assignee (`team_assignee_id`). |
| **Tag** | conversations, contacts, tickets | Tag (`tag_ids` contains). |
| **Search** | conversations, contacts, tickets | Free-text contains match on the entity's primary field (e.g. `email` for contacts). |
| **Filters** | conversations, contacts, tickets | Generic Search API conditions (field / operator / value), AND'd. |
| **Sort by / Direction** | paginated entities | Field + direction (e.g. `created_at` descending). |
| **Limit** | paginated entities | Max records. `0` returns all (auto-paginated up to 100,000). |

### Endpoints used

| Query type | Endpoint |
|---|---|
| Conversations | `GET /conversations` (no filters) · `POST /conversations/search` (with filters) |
| Contacts | `GET /contacts` (no filters) · `POST /contacts/search` (with filters) |
| Tickets | `POST /tickets/search` (search only) |
| Articles | `GET /articles` |
| Companies | `GET /companies` |
| Admins | `GET /admins` |
| Teams | `GET /teams` |
| Tags | `GET /tags` |
| Health | `GET /me` |

## Quick Start

```bash
# Install dependencies (from the monorepo root)
yarn install

# Build the plugin
yarn build

# Run with a local Grafana instance
INTERCOM_API_TOKEN=dG9rZW46... docker compose up
# EU/AU: also set INTERCOM_BASE_URL=https://api.eu.intercom.io
```

Grafana runs at http://localhost:3000 with anonymous admin access.

## Pagination & timestamps

- **Pagination is cursor-based.** List/search responses carry
  `{pages:{next:{page, starting_after}}, total_count}`. The backend follows
  `pages.next.starting_after` (search sends it in the body's `pagination`; list
  endpoints send it as the `starting_after` query param) until `pages.next` is
  absent or your limit is reached. `per_page` is capped at 150.
- **Timestamps are Unix epoch SECONDS** (integers). The backend converts known
  timestamp fields (anything ending in `_at`, plus `snoozed_until` /
  `waiting_since`) to UTC time fields. An epoch of `0` is treated as "unset"
  (null).

## API Limits

Intercom rate-limits most apps to **10,000 calls per minute** at the app level,
with tighter per-endpoint limits. The plugin relies on the Grafana SDK's HTTP
client for retry/backoff behavior. Keep limits modest on dashboards that refresh
frequently.

## Limitations & notes

- **Nested objects are JSON-encoded.** Intercom records embed deeply nested
  objects/arrays (`source`, `assignee`, `contacts`, `tags`, `custom_attributes`,
  `statistics`, …). These are serialised to compact JSON strings so the data is
  visible in a flat table; use Grafana transformations (e.g. *Extract fields*) to
  pull out individual nested values.
- **Companies pagination.** `GET /companies` uses page/cursor pagination; for
  very large company sets Intercom recommends the **Scroll API**
  (`GET /companies/scroll`), which this plugin does not use. Expect the standard
  list to cover typical workspaces; the safety cap still applies.
- **Search vs list.** Conversations and contacts use the cheaper list endpoints
  when no filters are present and switch to the Search API when any filter,
  picker or search text is set. Tickets are search-only; with no criteria a
  benign match-all query (`created_at > 0`) is used.
- **Search field names.** Generic filter `field` values must be valid Intercom
  search fields for the entity (e.g. `state`, `role`, `created_at`,
  `admin_assignee_id`, `tag_ids`). Invalid fields produce an Intercom error.

## Local Development

```bash
yarn typecheck          # TypeScript
yarn lint               # ESLint (yarn lint:fix to autofix)
yarn test               # Jest (frontend unit tests)
go test ./pkg/...       # Go unit tests

# Rebuild golden test snapshots
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
```

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| 401 Unauthorized | Token invalid or expired — regenerate in the Developer Hub. |
| 404 / wrong region | Token belongs to a different region; set the matching Region / API URL. |
| Empty results | Filters match no records, or the entity has no data. |
| Unexpected field error | A generic filter `field` is not a valid Intercom search field for the entity. |
| Rate limited (429) | Reduce dashboard refresh frequency or query limit. |
