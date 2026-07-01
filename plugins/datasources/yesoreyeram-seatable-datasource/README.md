# Grafana SeaTable Data Source

A Grafana data source plugin (with a Go backend) for querying rows from
[SeaTable](https://seatable.io) bases using the
[SeaTable API gateway](https://api.seatable.com/reference/).

## Features

- **Backend data source** — queries run in the Grafana server, so the SeaTable
  Base API Token never reaches the browser.
- **Records, Count & SQL query types** — return matching rows, just the count of
  matching rows (filter-aware, ideal for stat panels), or run a raw SeaTable SQL
  statement.
- **Visual query editor** — table, view, fields (multi-select), a structured
  **filter builder** (type-aware operators, nested AND/OR groups), multi-column
  **sort**, and limit. Tables and columns are fetched live from the SeaTable
  metadata API.
- **Server-side, parameterized filtering** — the structured filter tree is
  compiled into a **parameterized SeaTable SQL WHERE clause** on the backend (`?`
  placeholders + a parameters array), so filter values can never break out of the
  query.
- **Automatic pagination** — the rows endpoint is paged by `start`/`limit` (max
  1000/request) and SQL by `LIMIT`/`OFFSET` (max 10000/request), up to a
  configured limit or a safety cap.
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with date/`_ctime`/`_mtime` columns
  returned as real Grafana time fields for time-series panels. Multiple-select,
  collaborator and link columns are serialised as JSON.
- **Template variable support** in table, view, fields, filter values and raw SQL.
- **Health check** validates connectivity and credentials by performing the
  token exchange.

## How SeaTable authentication works (two steps)

SeaTable's data API uses a **two-step** token flow, and this plugin implements it
for you:

1. You configure a **Base API Token** (created from a base's *API Tokens* panel).
   This token is scoped to a single base and is *not* used directly on data
   endpoints.
2. On first use the backend exchanges it for a short-lived **Base-Token** (access
   token) and the base's `dtable_uuid` by calling:

   ```
   GET {server}/api/v2.1/dtable/app-access-token/
   Authorization: Token <Base API Token>
   ```

   The response returns `{ access_token, dtable_uuid, dtable_server, … }`.
3. All data calls then use the api-gateway with the exchanged token:

   ```
   Authorization: Bearer <access_token>
   {dtable_server}/api/v2/dtables/{dtable_uuid}/…
   ```

The Base-Token is cached and transparently re-fetched on a `401` (it expires).
Because the Base API Token already identifies the base, **you do not configure a
base id** — one data source maps to one base.

## Requirements

- Grafana >= 10.4.0
- A SeaTable **Base API Token** (Base → *Advanced* / *API Tokens* → generate).
  Use a read-only token if you only need to read.

## Configuration

Add a new **SeaTable** data source and configure:

| Field          | Required | Description                                                                                  |
| -------------- | -------- | -------------------------------------------------------------------------------------------- |
| Server URL     | no       | SeaTable server URL. Defaults to `https://cloud.seatable.io`; set your self-hosted URL here.  |
| Base API Token | yes      | Base API Token. Exchanged server-side for a Base-Token. Stored encrypted; never sent to browser. |

## Querying

In the query editor, choose a **Query type**:

- **Records** — returns matching rows from a **Table**. Supports an optional
  **View**, a structured **Filters** builder (type-aware operators, nested filter
  groups), **Sort** (multi-column), **Fields** (multi-select), and a **Limit**
  (`0` returns all, auto-paginated).
- **Count** — returns the number of matching rows via `SELECT COUNT(*)` (respects
  the filters). Handy for stat / single-value panels.
- **SQL** — runs a raw SeaTable SQL statement. The most powerful option: full
  `SELECT` with `WHERE`, `ORDER BY`, `GROUP BY`, aggregation, etc.

Each record frame includes the synthetic `_id`, `_ctime` and `_mtime` identity
columns (row id, created time, modified time) alongside the row's columns. Other
internal columns (`_creator`, `_last_modifier`, …) are dropped for a clean frame.

### How records are fetched (rows endpoint vs SQL)

The SeaTable **List Rows** endpoint can only page a table or view — it cannot
filter, sort, or project columns. So:

- A **plain** record listing (no filters, sort or fields) uses the **rows
  endpoint** and may target a **view**.
- As soon as you add a **filter, sort, or fields selection**, the query runs via
  the **SQL endpoint** (the view is then ignored, since SQL has no view concept).

Filters are compiled into a **parameterized** SQL `WHERE` clause server-side, and
filter values support Grafana template variables. The operators offered adapt to
each column's type (text, number, date, checkbox).

date / `_ctime` / `_mtime` columns are returned as proper Grafana time fields, so
they work directly in time-series panels.

### Group by / aggregation

For grouping and aggregation, use the **SQL** query type
(`SELECT City, COUNT(*), AVG(Age) FROM Contacts GROUP BY City`) or return rows and
use Grafana's **Transformations**.

## Column type mapping

| SeaTable column type                                   | Frame type    |
| ------------------------------------------------------ | ------------- |
| number, rate, duration, percent, auto-number           | number        |
| checkbox                                               | bool          |
| date, ctime, mtime                                     | time (UTC)    |
| text, long-text, single-select, email, url, …          | string        |
| multiple-select, collaborator, link, image, file       | JSON string   |

Types are inferred from the returned values (every non-null value of a column
must fit the type, otherwise the column falls back to string).

## Limitations

- **Base-Token expiry** — handled automatically (cached + re-fetched on `401`).
- **Rows endpoint** cannot filter/sort/project — those features run via SQL,
  which has no **view** concept (a view is only honoured for plain listings).
- **Multiple-select / collaborator / link** columns are returned as JSON strings;
  filtering them via the visual builder is limited (use the SQL query type with
  `HAS ANY OF` / `HAS ALL OF` for list semantics).
- **Pagination caps**: the rows endpoint returns ≤ 1000 rows/request and SQL
  ≤ 10000 rows/request; the plugin paginates automatically up to the limit or a
  safety cap.
- A data source maps to a **single base** (the Base API Token's base).

See the [SeaTable API reference](https://api.seatable.com/reference/) and the
[SQL reference](https://developer.seatable.io/sql/select/) for details.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for full setup, architecture and
workflow. Quick reference:

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn build:frontend         # frontend only -> dist/module.js
yarn build:backend          # backend only -> dist/gpx_seatable_* (mage buildAll)
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
yarn lint                   # lint
yarn typecheck              # type check
go test ./pkg/...           # backend unit tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH. Build
artifacts are written to `dist/`. Point Grafana's `plugins` path at this repo
(or symlink `dist/`) and set
`allow_loading_unsigned_plugins = yesoreyeram-seatable-datasource` for local
testing.

### Local stack (Docker Compose)

SeaTable is hosted (or self-hosted), so the stack is just **Grafana** with the
plugin mounted and the datasource auto-provisioned. Build the plugin first, then
start it with your token:

```bash
yarn build   # produce dist/ (frontend + backend)
SEATABLE_API_TOKEN=<base-api-token> docker compose up
# self-hosted: SEATABLE_SERVER_URL=https://seatable.example.com SEATABLE_API_TOKEN=... docker compose up
```

This brings up **Grafana** at http://localhost:3000 with **anonymous admin**
enabled, so no login is required. Omit the variables to skip auto-provisioning
and add the datasource manually in the Grafana UI.

## License

Apache-2.0
