# Grafana Airtable Data Source

A Grafana data source plugin (with a Go backend) for querying records from
[Airtable](https://airtable.com) bases using the
[Airtable Web API](https://airtable.com/developers/web/api/introduction).

## Features

- **Backend data source** — queries run in the Grafana server, so the Airtable
  personal access token never reaches the browser.
- **Records & Count query types** — return matching records, or just the count of
  matching records (filter-aware, ideal for stat panels).
- **Visual query editor** — base, table, view, fields (multi-select), a structured
  **filter builder** (type-aware operators, nested AND/OR groups), multi-field
  **sort**, and limit. Bases, tables, views and fields are fetched live from
  Airtable via the metadata API.
- **Server-side filter building** — the structured filter tree is compiled into an
  Airtable [`filterByFormula`](https://airtable.com/developers/web/api/list-records)
  expression on the backend. A raw formula escape hatch is also supported.
- **Automatic pagination** — follows Airtable's `offset` cursor to fetch all
  matching records (up to a safety cap) or up to a configured limit (Airtable
  pages at 100 records/request).
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with date/dateTime columns returned as real
  Grafana time fields for time-series panels. Array/object cells (multi-selects,
  linked records, attachments, collaborators) are serialised as JSON.
- **Template variable support** in base, table, view, filter values and fields.
- **Health check** validates connectivity and credentials.

## Requirements

- Grafana >= 10.4.0
- An Airtable [personal access token](https://airtable.com/developers/web/api/authentication)
  (PAT) with at least these scopes:
  - `data.records:read` — to read records.
  - `schema.bases:read` — to list bases, tables, fields and views in the editor.

  Grant the token access to the base(s) you want to query.

## Configuration

Add a new **Airtable** data source and configure:

| Field                 | Required | Description                                                                                     |
| --------------------- | -------- | ----------------------------------------------------------------------------------------------- |
| Personal Access Token | yes      | Airtable PAT, sent as `Authorization: Bearer <token>`. Stored encrypted; never sent to browser. |
| Default Base ID       | no       | Optional base id (`app...`). When set, the query editor lists this base's tables directly.       |
| API URL               | no       | Airtable API base URL. Defaults to `https://api.airtable.com`; only change for a proxy.          |

Create a token at [airtable.com/create/tokens](https://airtable.com/create/tokens).

## Querying

In the query editor, choose a **Query type**, a **Base** (unless one is
configured on the data source) and a **Table**:

- **Records** — returns matching records. Supports a **View**, a structured
  **Filters** builder (type-aware operators, nested filter groups), **Sort**
  (multi-field), **Fields** (multi-select), and a **Limit** (`0` returns all,
  auto-paginated).
- **Count** — returns the number of matching records (respects the filters). Handy
  for stat / single-value panels.

Each record frame includes synthetic `_id` and `_createdTime` columns (the
Airtable record id and creation time) alongside the record's fields.

Filters are compiled into an Airtable `filterByFormula` expression
**server-side**, and filter values support Grafana template variables. The
operators offered adapt to each field's type (text, number, date, checkbox).

date/dateTime columns are returned as proper Grafana time fields, so date/time
columns work directly in time-series panels.

### Group by / aggregation

Airtable's API does **not** expose a general group-by or per-column aggregation
endpoint (only a filter-aware record count, surfaced as the Count query type). To
group or aggregate records in Grafana, return the records and use Grafana's
**Transformations** (e.g. *Group by*, *Reduce*, *Partition by values*).

See the [List records API docs](https://airtable.com/developers/web/api/list-records)
for the full set of parameters and the
[formula field reference](https://support.airtable.com/docs/formula-field-reference)
for the filter formula language.

## Finding IDs

Airtable ids are visible in the API and (for the base) in the app URL:

- **Base id** starts with `app...` (e.g. `appXXXXXXXXXXXXXX`).
- **Table id** starts with `tbl...`; you can also use the table **name**.
- **View id** starts with `viw...`; you can also use the view **name**.

The query editor fetches these for you when the token has the `schema.bases:read`
scope.

## Troubleshooting

**`401` / unauthorized.** Re-enter the **Personal Access Token** and click
*Save & test*. Grafana stores secrets write-only, so saving the config with the
token field left blank blanks the token.

**`403` / forbidden.** The token is valid but lacks the required scopes
(`data.records:read`, `schema.bases:read`) or access to the base. Edit the token
at [airtable.com/create/tokens](https://airtable.com/create/tokens) and add the
base and scopes.

**`404` / not found.** Verify the **Base ID** (`app...`) and **Table** id/name are
correct and the token can access them.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup, architecture and
workflow. Quick reference:

Build (frontend + backend):

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn build:frontend         # frontend only -> dist/module.js
yarn build:backend          # backend only -> dist/gpx_airtable_* (mage buildAll)
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
yarn lint                   # lint
yarn typecheck              # type check
go test ./...               # backend unit tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH. Build
artifacts are written to `dist/`. Point Grafana's `plugins` path at this repo
(or symlink `dist/`) and set
`allow_loading_unsigned_plugins = yesoreyeram-airtable-datasource` for local
testing.

### Local stack (Docker Compose)

Airtable is SaaS-only, so the stack is just **Grafana** with the plugin mounted
and the datasource auto-provisioned against `https://api.airtable.com`. Build the
plugin first, then start it with your token:

```bash
yarn build   # produce dist/ (frontend + backend)
AIRTABLE_API_TOKEN=pat... AIRTABLE_BASE_ID=appXXXX docker compose up
```

This brings up **Grafana** at http://localhost:3000 with **anonymous admin**
enabled, so no login is required. Omit the variables to skip auto-provisioning
and add the datasource manually in the Grafana UI. The provisioning file is
`provisioning/datasources/airtable.yaml`;
`provisioning/datasources/airtable.yaml.example` shows a manual variant.

## License

Apache-2.0
