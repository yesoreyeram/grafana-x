# Grafana ClickUp Data Source

A Grafana **data source plugin with a Go backend** for [ClickUp](https://clickup.com).
Query your ClickUp workspace as Grafana data frames: list tasks, lists, folders,
spaces and workspaces, filter them, order them, and run a custom REST request
when you need something specific.

- Plugin id: `yesoreyeram-clickup-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (ClickUp REST API client, paginator, task flattener, frame builder)
- Cloud only — ClickUp is hosted SaaS; there is no local/self-hosted mode.

## Features

- **Backend data source** — the personal token / OAuth token is stored
  server-side and never sent to the browser.
- **Predefined query types**: Tasks, Lists, Folders, Spaces, Workspaces.
- **Raw REST** mode for any custom GET path; the first array of objects in the
  response (or an explicit response key) is flattened into a table.
- ClickUp's hierarchy is modelled with cascading pickers:
  **Workspace → Space → Folder → List**. Selecting a level scopes the query and
  refreshes the level below it.
- Rich, multi-value task filters compiled to ClickUp's task query parameters
  server-side: **statuses**, **assignees** (by user id), **tags** (all
  multi-select), plus **created/updated/due date filters**, and **include
  closed / subtasks / archived** toggles.
- **Created**, **Updated** and **Due** filters support three modes: _Any time_,
  _Dashboard range_ (follows the panel's time picker automatically), and _Custom_
  (explicit after/before bounds). Dashboard range is resolved server-side from the
  panel time range and sent as Unix-millisecond bounds.
- **Fields selector** for tasks — choose exactly which columns to return; leave
  empty for all flattened fields.
- Live **Workspace**, **Space**, **Folder**, **List** and **Member** pickers
  fetched from your account.
- ClickUp's nested JSON objects are flattened to scalar columns (e.g.
  `status → name`, `priority → name`, `assignees → "Alice, Bob"`,
  `list → name`, `creator → username`).
- Data-plane-compliant frames; ClickUp's Unix-millisecond date fields
  (`date_created`, `date_updated`, `due_date`, …) become time fields for
  time-series panels, and row order honours the query ordering.

## Setup

### Personal token (simplest)

1. In ClickUp, open **Settings → Apps** and click **Generate** under **API
   Token** (the token starts with `pk_`).
2. In Grafana, add a **ClickUp** data source, choose **Personal token**, and
   paste the token.

### OAuth token

1. Create an OAuth app and complete the
   [OAuth flow](https://developer.clickup.com/docs/authentication) to obtain an
   access token.
2. In Grafana, add a **ClickUp** data source, choose **OAuth token**, and paste
   the access token (sent as `Authorization: Bearer`).

The API URL defaults to `https://api.clickup.com/api`.

## Configuration

| Field          | Description |
| -------------- | ----------- |
| API URL        | ClickUp API root. Defaults to `https://api.clickup.com/api`. |
| Authentication | `Personal token` (sent raw) or `OAuth token` (sent as `Authorization: Bearer`). |
| Personal token | ClickUp personal API token, `pk_...` (when using token auth). |
| OAuth Token    | ClickUp OAuth2 access token (when using OAuth auth). |

## Querying

1. Choose a **Query type** (Tasks, Lists, Folders, Spaces, Workspaces, or Raw).
2. Pick the **Workspace**, then optionally a **Space**, **Folder** and **List**
   to scope the query.
3. For **Tasks**: optionally filter by **Statuses**, **Assignees**, **Tags** (all
   multi-select), **Created**/**Updated**/**Due** (Any time / Dashboard range /
   Custom), and toggle **Include closed**, **Subtasks**, **Archived**. Use
   **Fields** to pick which columns to return. Set **Order by** (Created /
   Updated / Due date / ID), **Reverse**, and a **Limit** (0 = all,
   auto-paginated).
4. For **Raw**: enter a REST GET **path** (e.g. `/v2/team/123/task`) and,
   optionally, the **response key** that holds the array to flatten.

Multiple values within a filter match **any** (OR). Date ranges accept ISO-8601
values (e.g. `2024-01-01`) or raw Unix milliseconds.

### How task scope is resolved

- When a **List** is selected, tasks are read from that List directly
  (`GET /v2/list/{list_id}/task`).
- Otherwise tasks are read from the workspace with the **Filtered Team Tasks**
  endpoint (`GET /v2/team/{team_id}/task`), scoped by the selected Space / Folder
  / List ids when present.

### Notes & limitations

- ClickUp is **cloud only** — there is no self-hosted mode.
- Pagination is **page-based** (100 tasks per page); the backend follows pages
  automatically up to the requested limit.
- Predefined queries flatten the entity's fields to scalar columns. For more
  fields or nested relations, use **Raw**.
- The **assignees** filter uses ClickUp numeric **user ids** (the Members picker
  provides them).
- Aggregations beyond row count are not provided — use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_clickup_* (mage buildAll)
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
CLICKUP_API_TOKEN=pk_... docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin and the ClickUp data
source auto-provisioned from `CLICKUP_API_TOKEN`.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
