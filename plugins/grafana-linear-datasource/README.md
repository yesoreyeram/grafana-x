# Grafana Linear Data Source

A Grafana **data source plugin with a Go backend** for [Linear](https://linear.app).
Query your Linear workspace as Grafana data frames: list issues, projects, teams,
users and cycles, filter them, order them, and run custom GraphQL when you need
something specific.

- Plugin id: `yesoreyeram-linear-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (Linear GraphQL API client, connection paginator, node flattener, frame builder)
- Cloud only — Linear is hosted SaaS; there is no local/self-hosted mode.

## Features

- **Backend data source** — the API key / OAuth token is stored server-side and
  never sent to the browser.
- **Predefined query types**: Issues, Projects, Teams, Users, Cycles.
- **Raw GraphQL** mode for any custom query; the first connection (object with a
  `nodes` array) in the response is flattened into a table.
- Rich, multi-value issue filters compiled to Linear's GraphQL filter objects
  server-side: **states**, **assignees**, **labels**, **priorities**,
  **projects** (all multi-select), plus **team**, **creator**, **title contains**,
  **created/updated date filters**, and an **include archived** toggle. Cycles can
  be filtered by team.
- **Created** and **Updated** filters support three modes: _Any time_, _Dashboard
  range_ (follows the panel's time picker automatically), and _Custom_ (explicit
  after/before bounds). Dashboard range is resolved server-side from the panel
  time range.
- **Fields selector** for issues — choose exactly which columns to return (the
  GraphQL selection set is built dynamically); leave empty for a sensible default
  set.
- Live **Team**, **State**, **Label**, **Project**, **User** and **Field** pickers
  fetched from your workspace.
- Linear's nested GraphQL objects are flattened to scalar columns (e.g.
  `state → name`, `assignee → name`, `team → name`, `labels → "bug, p1"`).
- Data-plane-compliant frames; timestamp fields (`createdAt`, `updatedAt`,
  `dueDate`, …) become time fields for time-series panels, and row order honours
  the query ordering.

## Setup

### Personal API key (simplest)

1. In Linear, open **Settings → Security & access → Personal API keys** and
   create a key (starts with `lin_api_`).
2. In Grafana, add a **Linear** data source, choose **Personal API key**, and
   paste the key.

### OAuth token

1. Obtain a Linear OAuth2 access token for your application.
2. In Grafana, add a **Linear** data source, choose **OAuth token**, and paste
   the access token (sent as `Authorization: Bearer`).

The GraphQL URL defaults to `https://api.linear.app/graphql`.

## Configuration

| Field          | Description |
| -------------- | ----------- |
| GraphQL URL    | Linear GraphQL endpoint. Defaults to `https://api.linear.app/graphql`. |
| Authentication | `Personal API key` (sent raw) or `OAuth token` (sent as `Authorization: Bearer`). |
| API Key        | Linear personal API key (when using API key auth). |
| OAuth Token    | Linear OAuth2 access token (when using OAuth auth). |

## Querying

1. Choose a **Query type** (Issues, Projects, Teams, Users, Cycles, or Raw GraphQL).
2. For **Issues**: optionally filter by **Team**, **States**, **Assignees**,
   **Labels**, **Priorities**, **Projects** (all multi-select), **Creator**,
   **Title contains**, **Created**/**Updated** (Any time / Dashboard range /
   Custom), and **Include archived**. Use **Fields** to pick which columns to
   return.
3. For **Cycles**: optionally filter by **Team**.
4. Set **Order by** (Created / Updated) and a **Limit** (0 = all, auto-paginated).
5. For **Raw GraphQL**: write a query and optional JSON variables.

Multiple values within a filter match **any** (OR); different filters are
combined with **AND**. Date ranges accept ISO-8601 values (e.g. `2024-01-01` or
`2024-01-01T00:00:00Z`).

### Notes & limitations

- Linear is a **single GraphQL endpoint** — there is no REST/table model and no
  self-hosted mode.
- Pagination is **cursor-based** (`first`/`after`, `pageInfo`); the backend
  follows cursors automatically up to the requested limit. Page size is capped
  at 250.
- Predefined queries return a useful flat subset of each entity. For more fields
  or nested relations, use **Raw GraphQL**.
- Aggregations beyond row count are not provided — use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_linear_* (mage buildAll)
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
LINEAR_API_KEY=lin_api_... docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin and the Linear data
source auto-provisioned from `LINEAR_API_KEY`.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
