# Grafana Notion Data Source

A Grafana **data source plugin with a Go backend** for [Notion](https://www.notion.so).
Query your Notion databases as Grafana data frames: list pages, count them,
filter with a type-aware builder, sort, and pick which properties to return.

- Plugin id: `yesoreyeram-notion-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (Notion REST API client, filter compiler, page flattener, frame builder)

## Features

- **Backend data source** — the integration token is stored server-side and
  never sent to the browser.
- **Query types**: Records (matching pages) and Count (number of matching pages).
- Live **Database** and **Properties** pickers, fetched from the databases shared
  with your integration.
- Structured **filter builder** with type-aware operators and nested AND/OR
  groups, compiled to a Notion JSON filter object server-side.
- Multi-property **Sort** builder and **Limit**.
- Notion's deeply-typed page properties are flattened to scalar columns
  (title, rich text, number, checkbox, select, status, multi-select, date,
  people, email, url, formula, rollup, …).
- Data-plane-compliant frames; date properties become time fields for
  time-series panels, and row order honours the query sort.

## Setup

1. Create an **internal integration** at https://www.notion.so/my-integrations
   and copy its token (starts with `secret_` / `ntn_`).
2. **Share** each database you want to query with the integration (open the
   database → ••• → Connections → add your integration).
3. In Grafana, add a **Notion** data source and paste the integration token.
   The API URL defaults to `https://api.notion.com` and the `Notion-Version`
   header defaults to `2022-06-28`.

## Configuration

| Field              | Description |
| ------------------ | ----------- |
| API URL            | Root URL of the Notion API. Defaults to `https://api.notion.com`. |
| Notion-Version     | Value of the `Notion-Version` header. Defaults to `2022-06-28`. |
| Integration Token  | Notion internal integration token (sent as `Authorization: Bearer`). |
| Default Database ID | Optional. Prefills the query editor. |

## Querying

1. Pick a **Database** (or type a database id).
2. Choose **Records** or **Count**.
3. Optionally add **Filters** (operators adapt to each property's type),
   **Sort** rows, select **Properties** to return, and set a **Limit**
   (0 = all, auto-paginated).

### Notes & limitations

- Notion has **no count endpoint**; Count is derived by paginating the query.
- Notion has **no "view" concept** in the API — only databases and properties.
- Pagination is **cursor-based** and `page_size` is capped at 100; the backend
  follows cursors automatically up to the requested limit.
- Aggregations beyond row count are not provided — use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_notion_* (mage buildAll)
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
NOTION_API_TOKEN=secret_... docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin and the Notion data
source auto-provisioned from `NOTION_API_TOKEN`.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
