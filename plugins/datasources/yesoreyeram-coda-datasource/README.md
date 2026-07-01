# Grafana Coda Data Source

A Grafana data source plugin (with a Go backend) for querying data from
[Coda](https://coda.io) docs using the
[Coda REST API v1](https://coda.io/developers/apis/v1).

## Features

- **Backend data source** — queries run server-side; the API token never reaches the browser.
- **Rows & Count query types** — return rows from a table, or just the count.
- **Visual query editor** — doc, table, columns (multi-select), a single-column filter, an advanced raw-query escape hatch, sort order, value format and limit. Docs, tables and columns are fetched live from the Coda API.
- **Automatic pagination** — follows Coda's `pageToken` cursor up to the requested limit (or a safety cap).
- **Efficient count** — uses the table's `rowCount` for an unfiltered count (one request); paginates and counts only when a filter is applied.
- **Data-plane-compliant frames** — automatic column type inference (number / boolean / time / string), with `date`/`dateTime` columns as real Grafana time fields. Array/object cells are serialised as JSON.
- **Template variable support** in doc, table, columns, filter and query.
- **Health check** validates connectivity and credentials via `/whoami`.

## Requirements

- Grafana >= 10.4.0
- A Coda [API token](https://coda.io/account) with access to the docs you want to query.

## Configuration

Add a new **Coda** data source and configure:

| Field          | Required | Description                                                                   |
| -------------- | -------- | ----------------------------------------------------------------------------- |
| API Token      | yes      | Coda API token, sent as `Authorization: Bearer <token>`. Stored encrypted.    |
| Default Doc ID | no       | Optional doc id. When set, the query editor lists this doc's tables directly.  |

Get a token at [coda.io/account](https://coda.io/account). The plugin talks to
the Coda SaaS API at `https://coda.io/apis/v1`.

## Querying

Choose a **Query type**, a **Doc** (unless one is configured on the data source) and a **Table**:

- **Rows** — returns rows. Supports **Columns** (multi-select), a single-column **Filter**, an **Advanced query**, **Sort by**, **Visible only**, **Value format** and a **Limit**.
- **Count** — returns the number of rows. When no filter is applied it reads the table's `rowCount` in a single request; when filtered it paginates and counts the matching rows. Handy for stat panels.

Each row frame includes synthetic `id`, `name`, `index`, `createdAt`,
`updatedAt`, `href` and `browserLink` columns alongside the row's cell values.
`createdAt`/`updatedAt` parse to Grafana time fields. Cell data comes from the
row's `values` map (the plugin always requests `useColumnNames=true`, so cells
are keyed by column name).

### Filtering (single column only)

Coda's rows endpoint supports filtering by **a single column** via the `query`
parameter, of the form `<columnIdOrName>:<value>` (string values are quoted,
e.g. `"Status":"open"`; column names are quoted, column ids like `c-aBc123` are
not). The plugin exposes this two ways:

- **Filter** — pick a column and enter a value (equality). The plugin compiles
  this into the Coda `query` parameter.
- **Advanced query** — type a raw Coda `query` string (e.g. `c-aBc123:"Apple"`
  or `"My Column":42`). This takes precedence over the **Filter**.

There is **no general filter language** in the Coda API. For multi-column
filters, ranges, OR logic, `contains`, etc., fetch the rows and use **Grafana
transformations** (Filter data by values, etc.).

### Sorting

`Sort by` maps to Coda's `sortBy` enum: `createdAt`, `updatedAt` or `natural`
(table-view order). `natural` only applies to visible rows, so it implies
`visibleOnly=true`. Sorting by an arbitrary column is **not** supported by the
Coda rows API; use Grafana transformations to sort by a column.

### Value format

`Value format` maps to Coda's `valueFormat`:

- **Simple** (default) — scalar values; array values (e.g. multi-selects) become
  comma-delimited strings.
- **Arrays** (`simpleWithArrays`) — array values are kept as JSON arrays.
- **Rich** — lossless encoding (text as Markdown, structured values as JSON-LD).

## Limitations

- **Single-column filtering only** — see above. Complex filtering must use Grafana transformations.
- **No column projection on the server** — the rows endpoint has no columns parameter, so the **Columns** selection is applied by the plugin after fetching all columns.
- **Coda API rate limits** apply (reading data: 100 requests / 6s; listing docs: 4 / 6s). See the [Coda API docs](https://coda.io/developers/apis/v1#section/Using-the-API/Rate-Limiting). The plugin surfaces `429` errors with a hint to slow down.
- **Eventual consistency** — Coda data read via the API can lag the live doc by a few seconds.

## Troubleshooting

**`401` / unauthorized.** Re-enter the **API Token** and click *Save & test*.

**`404` / not found.** Verify the **Doc ID** and **Table** id/name are correct and the token can access them.

**`429` / too many requests.** You hit Coda's rate limit; reduce the query/refresh frequency.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup. Quick reference:

```bash
yarn build                  # frontend + backend -> dist/
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
go test ./pkg/...           # backend unit tests
```

Coda is SaaS-only; the Docker stack runs just Grafana:

```bash
CODA_API_TOKEN=tok... CODA_DOC_ID=doc... docker compose up
```

## License

Apache-2.0
