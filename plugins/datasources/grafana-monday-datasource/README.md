# Grafana monday.com Data Source

A Grafana **data source plugin with a Go backend** for [monday.com](https://monday.com).
Query your monday.com account as Grafana data frames: list board items (with their
column values flattened into table columns), boards, groups, users, workspaces and
tags, filter them, order them, and run custom GraphQL when you need something
specific.

- Plugin id: `yesoreyeram-monday-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (monday.com GraphQL API client, cursor/page paginator, item & node flattener, frame builder)
- Cloud only — monday.com is hosted SaaS; there is no local/self-hosted mode.

## Features

- **Backend data source** — the API token / OAuth token is stored server-side and
  never sent to the browser.
- **Predefined query types**: Items, Boards, Groups, Users, Workspaces, Tags.
- **Items** are fetched per board with cursor-based pagination (`items_page` /
  `next_items_page`) and each item's **column values are flattened into table
  columns** keyed by the column title (using monday's human-readable `text`).
  **Checkbox** columns are returned as **boolean** fields.
- **Item filters**: restrict to specific **boards** and **groups**, filter by
  **name contains**, **choose which columns to return** (selected columns are
  requested via `column_values(ids: …)`), **order** by a column (asc/desc),
  toggle **include column values**, **hide system columns**, and set a **limit**.
- **Group by / aggregation**: aggregate items by a board column — e.g.
  **tasks by status**, **tasks by owner** — using monday.com's **server-side
  `aggregate` API** (no raw items fetched). Supports **count**, **count
  distinct**, **sum**, **average**, **min** or **max** (numeric aggregations take
  a value column). One row per distinct group value, sorted by the result.
- **Boards** can be filtered by **workspace** and **state** (active / all /
  archived / deleted).
- **Raw GraphQL** mode for any custom query; the first (deepest) array of objects
  in the response is flattened into a table.
- Live **Board**, **Group**, **Column** and **Workspace** pickers fetched from your
  account.
- monday's nested GraphQL objects are flattened to scalar columns (e.g.
  `group → title`, `board → name`, `workspace → name`, owners → `"Alice, Bob"`).
- Data-plane-compliant frames; timestamp fields (`created_at`, `updated_at`)
  become time fields for time-series panels, and row order honours the query
  ordering.

## Setup

### Personal API token (simplest)

1. In monday.com, open your **avatar menu → Developers → My Access Tokens** (admins
   can also find it under **Admin → API**) and copy your personal API token.
2. In Grafana, add a **monday.com** data source, choose **Personal API token**, and
   paste the token.

### OAuth token

1. Obtain a monday.com OAuth2 access token for your application.
2. In Grafana, add a **monday.com** data source, choose **OAuth token**, and paste
   the access token (sent as `Authorization: Bearer`).

The GraphQL URL defaults to `https://api.monday.com/v2`. Optionally set an
**API version** (e.g. `2024-10`) which is sent as the `API-Version` header.

## Configuration

| Field          | Description |
| -------------- | ----------- |
| GraphQL URL    | monday.com GraphQL endpoint. Defaults to `https://api.monday.com/v2`. |
| API version    | Optional `API-Version` header value (e.g. `2024-10`). |
| Authentication | `Personal API token` (sent raw) or `OAuth token` (sent as `Authorization: Bearer`). |
| API Token      | monday.com personal API token (when using API token auth). |
| OAuth Token    | monday.com OAuth2 access token (when using OAuth auth). |

## Querying

1. Choose a **Query type** (Items, Boards, Groups, Users, Workspaces, Tags, or Raw GraphQL).
2. For **Items**: select one or more **Boards** (required), optionally filter by
   **Groups**, restrict the **Columns** returned, filter by **Name contains**,
   toggle **Include column values** and **Hide system columns**, and **Order by
   column** (asc/desc). To aggregate, set **Group by** to a board column and pick
   an **Aggregation** (and a **Value column** for sum/avg/min/max/count distinct)
   — e.g. group by `Status` with `count` for "tasks by status". This runs
   monday's server-side `aggregate` query instead of fetching items.
3. For **Boards**: optionally filter by **Workspaces** and **State**.
4. For **Groups**: select the **Boards** whose groups to list.
5. Set a **Limit** (0 = all, auto-paginated).
6. For **Raw GraphQL**: write a query and optional JSON variables.

### Notes & limitations

- monday.com is a **single GraphQL endpoint** — there is no REST/table model and
  no self-hosted mode.
- Items pagination is **cursor-based** (`items_page` / `next_items_page`); boards,
  users and workspaces use **page/limit** pagination. The backend follows pages
  automatically up to the requested limit.
- Column values are exposed via their human-readable `text` (checkbox columns
  become booleans). For raw `value` JSON, use **Raw GraphQL**.
- **Group by / aggregation** uses monday.com's server-side `aggregate` query, so
  it groups by a **board column id** (not core fields like state/name) and runs
  one call per selected board. Group values are returned raw (e.g. status colour
  hex, person id); resolve them with Grafana value mappings / transformations.
- The `aggregate` query is only available on **recent API versions**. The plugin
  defaults to `2026-01`; if you pin an older **API version** that lacks it,
  grouping returns a clear error asking you to upgrade the version (or remove the
  Group by). Non-grouped queries work on older versions.
- Built-in aggregations are count / count distinct / sum / avg / min / max via
  **Group by**. For anything more, use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_monday_* (mage buildAll)
yarn dev             # frontend watch
yarn typecheck
yarn lint
yarn test            # frontend tests
go test ./pkg/...    # backend tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH.

### Local stack

```bash
yarn build   # produce dist/ (frontend + backend)
MONDAY_API_TOKEN=... docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin and the monday.com
data source auto-provisioned from `MONDAY_API_TOKEN`.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
