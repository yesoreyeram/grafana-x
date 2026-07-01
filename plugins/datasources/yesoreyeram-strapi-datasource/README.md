# Grafana Strapi Data Source

A Grafana data source plugin (with a Go backend) for querying data from
[Strapi](https://strapi.io) content types using the Strapi REST API. It supports
**both Strapi v4 and v5** response formats.

## Features

- **Backend data source** — queries run in the Grafana server, so the Strapi
  API token never reaches the browser.
- **Strapi v4 and v5 support** — the response shape differs between versions
  (v4 nests fields under `attributes`; v5 is flat with a `documentId`). The
  backend handles both and auto-detects the shape per record.
- **Records & Count query types** — return matching records, or just the count
  of matching records (filter-aware, ideal for stat panels).
- **Visual query editor** — content type (free-text plural API id), fields,
  **populate** for relations, a structured **filter builder** (type-aware
  operators, nested AND/OR groups), multi-field **sort**, and page-based
  pagination.
- **Server-side filter building** — the structured filter tree is compiled into
  Strapi REST filter query parameters on the backend.
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with date/dateTime columns returned as
  real Grafana time fields for time-series panels. Nested relations, components,
  media and dynamic zones are serialised as JSON strings.
- **Template variable support** in content type, filter values, fields and
  populate.

## Requirements

- Grafana >= 10.4.0
- A Strapi instance (v4 or v5) with a configured
  [API token](https://docs.strapi.io/cms/features/api-tokens) (Strapi admin
  panel → Settings → API Tokens). A **read-only** token is sufficient for
  querying.

## Configuration

Add a new **Strapi** data source and configure:

| Field                | Required | Description                                                                                                   |
| -------------------- | -------- | ------------------------------------------------------------------------------------------------------------- |
| API URL              | yes      | Strapi instance base URL (e.g. `https://your-strapi.example.com`). Do **not** include the `/api` suffix.      |
| API Version          | no       | `v5` (default) or `v4`. Selects the expected response shape; detection is automatic, so this is a hint.       |
| API Token            | yes      | Strapi API token, sent as `Authorization: Bearer <token>`. Stored encrypted; never sent to the browser.       |
| Default Content Type | no       | Optional content type plural API id (e.g. `articles`). Used to fully validate the token during Save & test.   |

### Health check

Strapi has no universal ping endpoint. When a **Default Content Type** is set,
Save & test issues a minimal request against it (`?pagination[pageSize]=1`); a
`2xx` confirms both reachability and that the token is accepted. When no content
type is set, the health check only verifies that the base URL is reachable (a
bad token cannot always be detected on the bare `/api` root), so configuring a
Default Content Type is recommended for full credential validation.

## Querying

In the query editor, choose a **Query type** and enter a **Content Type** (the
plural API id, e.g. `articles`):

- **Records** — returns matching records. Supports **Fields** (`fields[]`),
  **Populate** (relations), a structured **Filters** builder (type-aware
  operators, nested filter groups), **Sort** (multi-field), and **Page** /
  **Page Size** (page-based pagination).
- **Count** — returns the number of matching records (respects the filters),
  read from `meta.pagination.total`. Handy for stat / single-value panels.

Filters are compiled into Strapi REST filter parameters **server-side**, and
filter values support Grafana template variables. The operators offered adapt to
each field's type (text, number, date, boolean).

### Content type & field discovery (important limitation)

Strapi exposes the schema (content types and their fields) only through the
`/content-type-builder/content-types` endpoint, which requires an **admin JWT**
— a regular **API token is not accepted** there. Consequently, automatic
discovery is usually unavailable: the **Content Type** and **Fields** inputs are
**free-text** (type the plural API id and field names directly), and the editor
degrades gracefully (an empty discovery list, never an error) when discovery
isn't possible.

### v4 vs v5 response shapes

| | Strapi v4 | Strapi v5 |
| --- | --- | --- |
| List response | `{ data: [{ id, attributes: {…} }], meta: { pagination } }` | `{ data: [{ id, documentId, …fields }], meta: { pagination } }` |
| Record identity | `id` | `id` + `documentId` |
| Fields | nested under `attributes` | flat on the record |

The backend flattens v4 `attributes` up to top-level columns and passes v5 flat
records through, keeping `id` (and `documentId` for v5). Detection is per record,
so a misconfigured **API Version** still produces correct results.

### Filter operators

Compiled to Strapi operators (`filters[field][$op]=value`):

| Editor operator | Strapi | Notes |
| --- | --- | --- |
| `=` / `!=` | `$eq` / `$ne` | |
| `>` `>=` `<` `<=` | `$gt` `$gte` `$lt` `$lte` | numbers / dates |
| contains | `$contains` | |
| contains (case-insensitive) | `$containsi` | |
| does not contain | `$notContains` | |
| starts with / ends with | `$startsWith` / `$endsWith` | |
| is one of / is not one of | `$in` / `$notIn` | indexed array: `filters[f][$in][0]=a&…[1]=b` |
| is null / is not null | `$null` / `$notNull` | unary (value `true`) |

Nested groups compile to `$and` / `$or`, e.g.
`filters[$and][0][a][$eq]=1&filters[$and][1][$or][0][b][$eq]=2`.

### Pagination

Strapi uses page-based pagination: `pagination[page]` + `pagination[pageSize]`.
Use the **Page** and **Page Size** fields to control it. Page size is capped at
**100** per request (Strapi's default maximum); to retrieve more, page through or
use Grafana transformations. Count requests use `pagination[pageSize]=1` and read
`meta.pagination.total`.

### Populate (relations)

By default Strapi returns only top-level fields. Use **Populate** to include
relations, components and media:

- `*` populates all first-level relations (`populate=*`).
- a comma-separated list populates named relations (`populate[0]=author`).

Populated relations are returned as nested data and serialised to JSON strings in
the frame.

### Group by / aggregation

Strapi's REST API does **not** expose a general group-by or per-column
aggregation endpoint (only a filter-aware record count, surfaced as the Count
query type). To group or aggregate records in Grafana, return the records and use
Grafana's **Transformations** (e.g. *Group by*, *Reduce*, *Partition by values*).

## Troubleshooting

**`401` / unauthorized.** Re-enter the **API Token** and click *Save & test*.
Grafana stores secrets write-only, so saving the config with the token field left
blank blanks the token.

**`403` / forbidden.** The token lacks access to the content type. Grant the
content type's `find` permission to the token in Settings → API Tokens.

**`404` / not found.** Verify the **API URL** and the **Content Type** plural API
id (e.g. `articles`) are correct. The base URL should include the scheme and host
(e.g. `https://your-strapi.example.com`) without a trailing path or `/api` prefix.

**Content type / field dropdowns are empty.** Expected with an API token —
discovery needs an admin JWT. Type the content type plural API id and field names
directly.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup, architecture and
workflow. Quick reference:

Build (frontend + backend):

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn build:frontend         # frontend only -> dist/module.js
yarn build:backend          # backend only -> dist/gpx_strapi_* (mage buildAll)
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
yarn lint                   # lint
yarn typecheck              # type check
go test ./...               # backend unit tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH. Build
artifacts are written to `dist/`. Point Grafana's `plugins` path at this repo
(or symlink `dist/`) and set
`allow_loading_unsigned_plugins = yesoreyeram-strapi-datasource` for local
testing.

### Local stack (Docker Compose)

Strapi is self-hosted, so the stack is just **Grafana** with the plugin mounted
and the datasource auto-provisioned. Build the plugin first, then start it with
your Strapi URL and token:

```bash
yarn build   # produce dist/ (frontend + backend)
STRAPI_URL=https://your-strapi.example.com STRAPI_API_TOKEN=your-token docker compose up
```

This brings up **Grafana** at http://localhost:3000 with **anonymous admin**
enabled, so no login is required. Omit the variables to skip auto-provisioning
and add the datasource manually in the Grafana UI. The provisioning file is
`provisioning/datasources/strapi.yaml`;
`provisioning/datasources/strapi.yaml.example` shows a manual variant.

## License

Apache-2.0
