# Grafana Pipedrive Data Source

A Grafana data source plugin for [Pipedrive](https://developers.pipedrive.com/docs/api/v1).
Query deals, persons, organizations, products, and record counts from Pipedrive
CRM directly in Grafana. The Go backend talks to the Pipedrive REST API v1,
follows offset-based pagination correctly, remaps custom-field hash keys to
readable names, and converts results into Grafana data frames.

## Features

- **Multiple entity types** — Deals, Persons, Organizations, Products, and Count
- **Two authentication modes** — API token (query parameter) or OAuth2 (Bearer)
- **Correct v1 pagination** — automatically follows
  `more_items_in_collection` / `next_start` across pages (500 records/request)
- **Custom field mapping** — translates 40-character custom-field hash keys into
  their human-readable names (e.g. `abc…a` → `Lead Source`)
- **Server-side filters** — status, pipeline, stage, user, and saved filter id
  are pushed to the Pipedrive API as query parameters
- **Client-side filters** — refine fetched records with EQ, NEQ, GT, GTE, LT,
  LTE, LIKE, NOT_LIKE on any field (including mapped custom-field names)
- **Sort** — sort by any field, ascending or descending (v1 `sort` syntax)
- **Count queries** — count any entity matching the filters
- **Nested value flattening** — relation objects (`person_id`, `org_id`) flatten
  to their name; `email`/`phone` arrays flatten to the primary value
- **Server-side credentials** — token stored securely in Grafana's backend

## Requirements

- Grafana >= 10.4.0
- A Pipedrive **API token** (Settings > Personal preferences > API) **or** an
  **OAuth2 access token**

## Configuration

| Field | Description |
|---|---|
| **Company Domain** | Your Pipedrive company subdomain. For `mycompany.pipedrive.com`, enter `mycompany`. The base URL is built as `https://{domain}.pipedrive.com/api/v1`. |
| **Authentication** | `API token` (default) or `OAuth token`. |
| **API Token** | (API token auth) Generate at **Settings > Personal preferences > API**. Sent as the `api_token` query parameter. Stored securely. |
| **OAuth Token** | (OAuth auth) A Pipedrive OAuth2 access token. Sent as `Authorization: Bearer`. Stored securely. |

Both credentials are kept in `secureJsonData` and never exposed to the browser.
The health check calls `GET /api/v1/users/me` to validate the token.

## Query Editor

| Section | Description |
|---|---|
| **Query Type** | Deals, Persons, Organizations, Products, or Count. |
| **Count entity** | (Count only) Which entity to count. |
| **Deal Status** | (Deals) all, open, won, lost, deleted. |
| **Pipeline / Stage** | (Deals) Filter by pipeline and/or stage. |
| **User** | (Deals, Persons) Filter by Pipedrive user. |
| **Saved filter ID** | Optional Pipedrive saved-filter id. When set it takes precedence over the status/pipeline/stage/user filters (matching the API). |
| **Map custom fields** | Translate custom-field hashes to names (one extra API call per query; on by default). |
| **Filters** | Client-side field filters applied after fetch (EQ, NEQ, GT, GTE, LT, LTE, LIKE, NOT_LIKE). |
| **Sort** | Field to sort by and direction (server-side v1 `sort`). |
| **Limit / Start** | Total record cap across pages, and the initial offset. Set Limit to 0 to fetch all matching records. |

## Template variables

`applyTemplateVariables` interpolates the pipeline id, stage id, user id, saved
filter id, status, sort field, and every client-side filter field/value, so
dashboard variables work in all of them.

## How it works (backend)

- **Base URL** — `https://{companyDomain}.pipedrive.com/api/v1` (cloud only).
- **Auth** — `apiToken` mode appends `?api_token=…`; `oauth` mode sends
  `Authorization: Bearer …`. If only one credential is configured, the backend
  uses whichever is present.
- **Pagination (v1, offset-based)** — every list request sends `start` + `limit`
  (max 500). The response carries
  `additional_data.pagination.{start, limit, more_items_in_collection, next_start}`.
  The backend loops while `more_items_in_collection` is true, advancing to
  `next_start`, until the requested limit (or a safety cap) is reached. It does
  **not** blindly increment `start`.
- **Count** — Pipedrive has no count endpoint for list resources, so Count
  paginates the chosen entity with minimal parsing and sums the page sizes
  (following the same `more_items_in_collection` loop). This works uniformly for
  every entity type. Deals additionally expose `GET /deals/summary`, a more
  efficient single-request total, but pagination is used here for one correct
  implementation across entities.
- **Custom fields** — Pipedrive returns custom fields keyed by a 40-character
  hash. When mapping is enabled, the backend fetches the matching
  `{entity}Fields` endpoint (`dealFields`, `personFields`, `organizationFields`,
  `productFields`), builds a `key → name` map, and renames record columns. Hash
  subfields such as `{hash}_currency` become `{name}_currency`. If the field
  fetch fails (e.g. missing scope), records are still returned keyed by their raw
  hashes.
- **Time fields** — `add_time`, `update_time`, `close_time`, `won_time`,
  `lost_time`, `expected_close_date`, `next_activity_date`, `last_activity_date`
  (and any `*_time` / `*_date` column) are parsed from Pipedrive's
  `2006-01-02 15:04:05` and `2006-01-02` formats to UTC time fields.
- **Frames** — Records → `FrameTypeTable` (v0.1); Count → `FrameTypeNumericWide`
  (v0.1). Time columns are moved to the front; row order is preserved.

## API v1 vs v2

Pipedrive is migrating list endpoints to **v2** (cursor-based pagination via
`cursor` / `additional_data.next_cursor`). This plugin targets **v1** for
breadth and simplicity: v1 covers deals, persons, organizations, and products
with consistent offset pagination and the `{entity}Fields` custom-field schema.
v1 endpoints remain available during the migration period.

## Local Development

```bash
# Type-check, lint, frontend + backend tests
yarn typecheck
yarn lint
yarn test
go test ./pkg/...

# Build
yarn build

# Regenerate golden data-frame snapshots after an intentional change
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
```

## Limitations

- **Rate limits** — Pipedrive uses a token-budget rate limit (per-request cost;
  roughly ~10 requests/second for token auth). Large unbounded queries make many
  paginated calls; set a Limit where possible. The plugin relies on the SDK HTTP
  client for retry/backoff.
- **Client-side filters** — Pipedrive's list API only filters by
  status/pipeline/stage/user/filter_id server-side. Other field filters are
  applied after fetch, so they operate on the records within your Limit.
- **Custom date fields** — custom fields keep their values but are typed as
  strings unless their column name ends in `_date`/`_time` (standard date fields
  are always parsed).
- **Max page size** — 500 records per request (handled transparently by
  pagination).
