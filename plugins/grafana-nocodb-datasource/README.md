# Grafana NocoDB Data Source

A Grafana data source plugin (with a Go backend) for querying records from
[NocoDB](https://nocodb.com) tables using the NocoDB v2 Data API.

## Features

- **Backend data source** — queries run in the Grafana server, so the NocoDB API
  token never reaches the browser.
- **Records & Count query types** — return matching rows, or just the count of
  matching rows (filter-aware, ideal for stat panels).
- **Visual query editor** — table, view, fields (multi-select), a structured
  **filter builder** (type-aware operators, nested AND/OR groups), multi-field
  **sort**, and limit. Tables, views and fields are fetched live from NocoDB.
- **Server-side filter building** — the structured filter tree is compiled into a
  NocoDB `where` clause on the backend.
- **Platform & API version aware** — NocoDB Cloud or self-hosted; Data API v2 or
  v3.
- **Automatic pagination** — fetches all matching records (up to a safety cap) or
  up to a configured limit.
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with DateTime/Date columns returned as real
  Grafana time fields for time-series panels.
- **Template variable support** in table, view, filter values, sort and fields.
- **Health check** validates connectivity and credentials.

## Requirements

- Grafana >= 10.4.0
- A NocoDB instance (cloud or self-hosted) and an API token.

## Configuration

Add a new **NocoDB** data source and configure:

| Field           | Required | Description                                                                       |
| --------------- | -------- | --------------------------------------------------------------------------------- |
| Platform        | yes      | `NocoDB Cloud` (fixes the URL to `https://app.nocodb.com`) or `Self-hosted`.       |
| Base URL        | yes\*    | Root URL of your self-hosted NocoDB instance. Hidden/forced for Cloud.             |
| API Version     | yes      | NocoDB Data API version: `v2` (default, widely available) or `v3` (needs base id). |
| API Token       | yes      | NocoDB API token. Sent as the `xc-token` header. Stored encrypted.                |
| Default Base ID | no       | NocoDB base id (prefixed `p`). Lists tables in the editor; required for the v3 API. |

## Querying

In the query editor, choose a **Query type** and a **Table**:

- **Records** — returns matching rows. Supports a **View**, a structured
  **Filters** builder (type-aware operators, nested filter groups), **Sort**
  (multi-field), **Fields** (multi-select), and a **Limit** (`0` returns all,
  auto-paginated).
- **Count** — returns the number of matching rows (respects the filters). Handy
  for stat / single-value panels.

Filters are built into a NocoDB `where` clause **server-side**, and filter
values support Grafana template variables.

DateTime/Date columns are returned as proper Grafana time fields, so the
`Metrics` sample table works directly in time-series panels.

### Group by / aggregation

NocoDB's data API does **not** expose a general group-by or per-column
aggregation endpoint (only a filter-aware row `count`, surfaced as the Count
query type). To group or aggregate records in Grafana, return the rows and use
Grafana's **Transformations** (e.g. *Group by*, *Reduce*, *Partition by values*).

See the [NocoDB REST API docs](https://docs.nocodb.com/developer-resources/rest-APIs/overview)
for where-clause operators and date sub-operators.

## Finding IDs

NocoDB ids are visible in the app URL and context menus:

- Base ID — prefixed `p` (project)
- Table ID — prefixed `m` (model)
- View ID — prefixed `v` (view)

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup, architecture and
workflow. Quick reference:

Build (frontend + backend):

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn build:frontend         # frontend only -> dist/module.js
yarn build:backend          # backend only -> dist/gpx_nocodb_* (mage buildAll)
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
yarn lint                   # lint
yarn typecheck              # type check
go test ./...               # backend unit tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH. Build
artifacts are written to `dist/`. Point Grafana's `plugins` path at this repo
(or symlink `dist/`) and set
`allow_loading_unsigned_plugins = yesoreyeram-nocodb-datasource` for local
testing.

### Local stack (Docker Compose)

A ready-to-use stack is included. Build the plugin first, then start it:

```bash
yarn build   # produce dist/ (frontend + backend)
docker compose up
```

This brings up:

- **NocoDB** at http://localhost:8080
- a one-shot **seed job** that creates a `Sample` base with several tables
  covering different data shapes, mints an API token, and generates the Grafana
  datasource provisioning (so the NocoDB datasource is pre-configured and
  connected automatically):
  - **Customers** — mixed relational rows (text, number, currency, select, date, checkbox)
  - **Metrics** — hourly time-series per service (CPU, memory, requests, latency)
  - **Logs** — log lines (level, service, message, status code, duration)
  - **Sales** — numeric/categorical data (region, product, units, revenue)
- **Grafana** at http://localhost:3000 with **anonymous admin** enabled, so no
  login is required

The seed is idempotent: existing data is reused on restart, and a fresh
`docker compose down -v && docker compose up` re-creates the sample data. The
generated datasource lives in a named volume; the static
`provisioning/datasources/nocodb.yaml.example` shows the manual equivalent.

## License

Apache-2.0
