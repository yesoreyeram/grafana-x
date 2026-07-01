# Grafana Directus Data Source

A Grafana data source plugin (with a Go backend) for querying records from
[Directus](https://directus.io) collections using the Directus REST API.

## Features

- **Backend data source** — queries run in the Grafana server, so the Directus
  API token never reaches the browser.
- **Records & Count query types** — return matching records, or just the count of
  matching records (filter-aware, ideal for stat panels).
- **Visual query editor** — collection picker, fields (multi-select), a structured
  **filter builder** (type-aware operators, nested AND/OR groups), multi-field
  **sort**, limit, offset, and search. Collections and fields are fetched live
  from Directus via the schema API.
- **Server-side filter building** — the structured filter tree is compiled into a
  Directus JSON filter object on the backend.
- **Automatic pagination** — follows Directus' offset/limit pagination to fetch
  all matching records (up to a safety cap) or up to a configured limit (100
  records per request).
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with date/dateTime columns returned as real
  Grafana time fields for time-series panels. Array/object cells are serialised
  as JSON.
- **Template variable support** in collection, filter values, fields and search.
- **Health check** validates connectivity and credentials.

## Requirements

- Grafana >= 10.4.0
- A Directus instance with a configured
  [static API token](https://docs.directus.io/guides/connect/authentication)
  (a per-user, non-expiring token generated in the Directus admin panel under
  the user's profile / Settings > API Tokens).

## Configuration

Add a new **Directus** data source and configure:

| Field             | Required | Description                                                                   |
| ----------------- | -------- | ----------------------------------------------------------------------------- |
| API URL           | yes      | Directus API base URL (e.g. `https://your-directus.example.com`). No default — Directus is self-hosted. |
| API Token         | yes      | Directus static API token, sent as `Authorization: Bearer <token>`. Stored encrypted; never sent to the browser. |
| Default Collection | no      | Optional collection name. When set, the query editor selects it by default.    |

### Authentication

Directus accepts a static token either as the `Authorization: Bearer <token>`
header (**preferred**) or as a `?access_token=<token>` query parameter. This
plugin always uses the **Bearer header** so the token is never placed in URLs or
server logs (Directus itself warns against the query-parameter method). The
health check calls `GET /users/me`, which requires a valid token — so a missing
or wrong token fails *Save & test* (unlike the unauthenticated `/server/ping`).

## Querying

In the query editor, choose a **Query type** and a **Collection**:

- **Records** — returns matching records. Supports **Fields** (multi-select),
  **Search** (Directus full-text search), a structured **Filters** builder
  (type-aware operators, nested filter groups), **Sort** (multi-field),
  **Limit** (`0` returns all, auto-paginated), and **Offset** (skip records).
- **Count** — returns the number of matching records (respects the filters and
  search). Handy for stat / single-value panels. Implemented with the Directus
  aggregate API (`?aggregate[count]=*`), so the count reflects the filter — not
  the whole-collection size.

Filters are compiled into a Directus JSON filter object **server-side**, and
filter values support Grafana template variables. The operators offered adapt to
each field's type (text, number, date, boolean).

date/dateTime columns are returned as proper Grafana time fields, so date/time
columns work directly in time-series panels.

### Filters & operators

Filters are compiled into a Directus
[filter object](https://docs.directus.io/guides/connect/filter-rules) such as
`{"status": {"_eq": "published"}}`, with nested `_and`/`_or` groups. The query
editor exposes type-aware operators that map to Directus operators:

| Editor operator                        | Directus operator        | Value          |
| -------------------------------------- | ------------------------ | -------------- |
| `=`, `!=`                              | `_eq`, `_neq`            | single         |
| `>`, `>=`, `<`, `<=`                   | `_gt`, `_gte`, `_lt`, `_lte` | single     |
| contains / does not contain            | `_contains`, `_ncontains` | single        |
| starts with / ends with                | `_starts_with`, `_ends_with` | single     |
| is one of / is not one of (csv)        | `_in`, `_nin`            | JSON array     |
| is between / is not between (min,max)  | `_between`, `_nbetween`  | `[min,max]` array |
| is empty / is not empty                | `_empty`, `_nempty`      | boolean `true` |

The backend also accepts `null`/`nnull` (→ `_null`/`_nnull`) and the
case-insensitive `_icontains`/`_nicontains` variants. For `in`/`nin`/`between`,
multi-value template variables are interpolated with `csv` formatting.

### Offset-based pagination

Directus uses standard offset/limit pagination. When no limit is set, the plugin
fetches pages of 100 records until all matching records are returned (capped at
100,000 for safety). Use the **Limit** field to cap results and **Offset** to
skip records.

### Limitations

- **System collections are hidden.** Directus internal collections (those
  prefixed with `directus_`, e.g. `directus_users`, `directus_files`) are filtered
  out of the collection picker; query them only if you point the collection field
  at them explicitly.
- **Relational fields** are returned as the related record's **primary key (id)**
  (or JSON for nested arrays) unless you request related fields with dot-notation
  in **Fields** (e.g. `author.name`). Remaining array/object cells are
  JSON-serialised to strings.
- **No general group-by / aggregation** beyond the filter-aware Count — see below.

### Group by / aggregation

Directus' REST API does **not** expose a general group-by or per-column
aggregation endpoint (only a filter-aware record count, surfaced as the Count
query type). To group or aggregate records in Grafana, return the records and use
Grafana's **Transformations** (e.g. *Group by*, *Reduce*, *Partition by values*).

## Troubleshooting

**`401` / unauthorized.** Re-enter the **API Token** and click
*Save & test*. Grafana stores secrets write-only, so saving the config with the
token field left blank blanks the token.

**`404` / not found.** Verify the **API URL** and **Collection** name are correct
and the token can access them. The base URL should include the scheme and host
(e.g. `https://directus.example.com`) without a trailing path.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup, architecture and
workflow. Quick reference:

Build (frontend + backend):

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn build:frontend         # frontend only -> dist/module.js
yarn build:backend          # backend only -> dist/gpx_directus_* (mage buildAll)
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
yarn lint                   # lint
yarn typecheck              # type check
go test ./...               # backend unit tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH. Build
artifacts are written to `dist/`. Point Grafana's `plugins` path at this repo
(or symlink `dist/`) and set
`allow_loading_unsigned_plugins = yesoreyeram-directus-datasource` for local
testing.

### Local stack (Docker Compose)

Directus is self-hosted, so the stack is just **Grafana** with the plugin mounted
and the datasource auto-provisioned. Build the plugin first, then start it with
your Directus URL and token:

```bash
yarn build   # produce dist/ (frontend + backend)
DIRECTUS_URL=https://your-directus.example.com DIRECTUS_API_TOKEN=your-token docker compose up
```

This brings up **Grafana** at http://localhost:3000 with **anonymous admin**
enabled, so no login is required. Omit the variables to skip auto-provisioning
and add the datasource manually in the Grafana UI. The provisioning file is
`provisioning/datasources/directus.yaml`;
`provisioning/datasources/directus.yaml.example` shows a manual variant.

## License

Apache-2.0
