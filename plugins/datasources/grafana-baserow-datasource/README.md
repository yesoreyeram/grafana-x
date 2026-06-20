# Grafana Baserow Data Source

A Grafana data source plugin (with a Go backend) for querying rows from
[Baserow](https://baserow.io) tables using the Baserow REST API.

## Features

- **Backend data source** — queries run in the Grafana server, so the Baserow
  database token never reaches the browser.
- **Records & Count query types** — return matching rows, or just the count of
  matching rows (filter-aware, ideal for stat panels).
- **Visual query editor** — table, view, fields (multi-select), a structured
  **filter builder** (type-aware operators, nested AND/OR groups), multi-field
  **sort**, and limit. Tables, views and fields are fetched live from Baserow.
- **Server-side filter building** — the structured filter tree is compiled into a
  Baserow `filters` JSON tree on the backend.
- **Platform aware** — Baserow Cloud (`api.baserow.io`) or self-hosted.
- **Automatic pagination** — fetches all matching rows (up to a safety cap) or up
  to a configured limit (Baserow pages at 200 rows/request).
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with Date/DateTime columns returned as real
  Grafana time fields for time-series panels.
- **Template variable support** in table, view, filter values, sort and fields.
- **Health check** validates connectivity and credentials.

## Requirements

- Grafana >= 10.4.0
- A Baserow instance (cloud or self-hosted) and one of:
  - a **database token** plus the **database id** it is scoped to, or
  - a Baserow account **email + password**.

## Configuration

Add a new **Baserow** data source, pick a **Platform** and **Base URL**, then an
**Authentication** mode:

### Database token (default)

| Field       | Required | Description                                                                          |
| ----------- | -------- | ------------------------------------------------------------------------------------ |
| API Token   | yes      | Baserow **database token**. Sent as `Authorization: Token <token>`. Stored encrypted. |
| Database ID | no       | Optional. Filters the table list to one database; leave empty to list every table the token can access. |

Create a database token in Baserow under **Settings → Database tokens**, scope it
to the workspace/database you want to query, and grant it read permissions.

Notes for token mode (Baserow's API only accepts database tokens on a subset of
endpoints, so the plugin adapts):

- Tables are listed via the token-aware `all-tables` endpoint, so a Database ID is
  optional (it only filters the list).
- The **View** picker is unavailable (Baserow's views endpoint rejects database
  tokens) — you can still type a view id manually. Use email & password auth if
  you need to browse views.

### Email & password (JWT)

| Field               | Required | Description                                                                            |
| ------------------- | -------- | -------------------------------------------------------------------------------------- |
| Email               | yes      | Baserow account email. Used to sign in for a JWT (`Authorization: JWT <jwt>`).          |
| Password            | yes      | Baserow account password. Stored encrypted; never sent to the browser.                  |
| Default Database ID | no       | Optional. Otherwise pick the database in the query editor (all accessible DBs listed). |

With email & password the plugin signs in, caches the JWT, and **auto-refreshes**
it when it expires. This mode can enumerate every workspace/database you have
access to, so you can choose the **Database** (and table) directly in the query
editor without configuring a database id.

> Database tokens are the recommended, more constrained option for production;
> email & password is convenient when you want to browse across databases.

## Querying

In the query editor, choose a **Query type** and a **Table**:

- **Records** — returns matching rows. Supports a **View**, a structured
  **Filters** builder (type-aware operators, nested filter groups), **Sort**
  (multi-field), **Fields** (multi-select), and a **Limit** (`0` returns all,
  auto-paginated).
- **Count** — returns the number of matching rows (respects the filters). Handy
  for stat / single-value panels.

Filters are built into a Baserow `filters` JSON tree **server-side**, and filter
values support Grafana template variables. Rows are requested with
`user_field_names=true` so filters, sort and fields all use the human field names.

Date/DateTime columns are returned as proper Grafana time fields, so date/time
columns work directly in time-series panels.

### Group by / aggregation

Baserow's data API does **not** expose a general group-by or per-column
aggregation endpoint (only a filter-aware row `count`, surfaced as the Count
query type). To group or aggregate rows in Grafana, return the rows and use
Grafana's **Transformations** (e.g. *Group by*, *Reduce*, *Partition by values*).

See the [Baserow REST API docs](https://baserow.io/docs/apis%2Frest-api) for the
full set of filter types and parameters.

## Finding IDs

Baserow ids are numeric and visible in the app URL, e.g.
`/database/<databaseId>/table/<tableId>/<viewId>`.

## Troubleshooting

**`401` / `Authentication credentials were not provided`.** The request reached
the API but without valid credentials:

- **Database token mode**: re-enter the **API Token** and click *Save & test*.
  Grafana stores secrets write-only, so saving the config with the token field
  left blank blanks the token. The token must have read access to the database,
  and you must set the correct **Database ID**.
- **Email & password mode**: verify the email/password are correct for this
  instance (and re-enter the password — it is also write-only).
- This can also happen if Baserow **redirects** to a different host (Go strips the
  `Authorization` header on cross-host redirects). The plugin re-attaches it
  automatically, but the cleanest fix is to point the Base URL at the canonical
  host so no redirect occurs (for the local stack, the internal host is
  registered via `BASEROW_EXTRA_PUBLIC_URLS`).

**`Site not found` / an HTML page instead of JSON (404).** The Base URL must point
at the Baserow **API**, not the web app:

- **Cloud**: use Platform = Baserow Cloud (forces `https://api.baserow.io`).
- **Self-hosted (all-in-one image)**: the embedded reverse proxy only routes
  `/api/*` to the backend for hosts it recognises as Baserow URLs. The host
  **Grafana** uses to reach Baserow must be registered via `BASEROW_PUBLIC_URL`
  or `BASEROW_EXTRA_PUBLIC_URLS`; otherwise that host is treated as a published
  application-builder domain and returns a "Site not found" page. In the bundled
  stack Grafana reaches Baserow at `http://baserow`, so the compose file sets
  `BASEROW_EXTRA_PUBLIC_URLS=http://baserow`.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup, architecture and
workflow. Quick reference:

Build (frontend + backend):

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn build:frontend         # frontend only -> dist/module.js
yarn build:backend          # backend only -> dist/gpx_baserow_* (mage buildAll)
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
yarn lint                   # lint
yarn typecheck              # type check
go test ./...               # backend unit tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH. Build
artifacts are written to `dist/`. Point Grafana's `plugins` path at this repo
(or symlink `dist/`) and set
`allow_loading_unsigned_plugins = yesoreyeram-baserow-datasource` for local
testing.

### Local stack (Docker Compose)

A ready-to-use stack is included (a self-hosted **Baserow** + **Grafana**). It does
**not** seed any sample data — you create your own database and table in Baserow.
Build the plugin first, then start it:

```bash
yarn build   # produce dist/ (frontend + backend)
docker compose up
```

This brings up:

- **Baserow** at http://localhost:8980 (empty; first-boot template sync is
  disabled so it starts quickly)
- **Grafana** at http://localhost:3000 with **anonymous admin** enabled, so no
  login is required

Then:

1. Open Baserow, create an account, a database and a table.
2. Either create a **database token** (Settings → Database tokens) or use your
   account **email + password**.
3. Auto-provision the datasource by exporting credentials before starting:

   ```bash
   # database token auth
   BASEROW_API_TOKEN=... BASEROW_DATABASE_ID=1 docker compose up

   # OR email & password auth
   BASEROW_AUTH_MODE=password BASEROW_EMAIL=you@example.com BASEROW_PASSWORD=... docker compose up
   ```

   Omit the variables to skip auto-provisioning and add the datasource manually
   in the Grafana UI. The provisioning file is
   `provisioning/datasources/baserow.yaml`;
   `provisioning/datasources/baserow.yaml.example` shows manual variants.

> **Baserow Cloud**: to point at hosted Baserow instead of the local container,
> add the datasource in the UI with Platform = **Baserow Cloud** (it uses
> `https://api.baserow.io`), or set `platform: cloud` in a provisioning file.

## License

Apache-2.0
