# Grafana Asana Data Source

A Grafana **data source plugin with a Go backend** for [Asana](https://asana.com).
Query your Asana account as Grafana data frames: list tasks, projects, sections,
workspaces, teams, users and tags, filter them, and run a custom REST request
when you need something specific.

- Plugin id: `yesoreyeram-asana-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (Asana REST API client, cursor paginator, entity flattener, frame builder)
- Cloud only — Asana is hosted SaaS; there is no local/self-hosted mode.

## Features

- **Backend data source** — the personal access token is stored server-side and
  never sent to the browser.
- **Predefined query types**: Tasks, Projects, Sections, Workspaces, Teams,
  Users, Tags.
- **Raw REST** mode for any custom GET path; the first array of objects in the
  response (Asana wraps results under `data`) or an explicit response key is
  flattened into a table.
- Asana's hierarchy is modelled with cascading pickers:
  **Workspace → Team → Project → Section**. Selecting a level scopes the query
  and refreshes the level below it.
- Task filters compiled to Asana's task query parameters server-side:
  **Assignee** (live Users picker), **Incomplete only**
  (`completed_since=now`), and a **Modified** filter.
- The **Modified** filter supports three modes: _Any time_, _Dashboard range_
  (follows the panel's time picker automatically), and _Custom_ (explicit
  ISO-8601 bound). Dashboard range is resolved server-side from the panel time
  range.
- **Fields selector** for tasks — choose exactly which columns to return; the
  selection is applied server-side via Asana `opt_fields`.
- Asana's nested JSON objects are flattened to scalar columns (e.g.
  `assignee → name`, `projects → "Mobile, Web"`, `tags → "bug, p1"`,
  `current_status → text`).
- **Custom fields** on tasks are expanded into one column each, named after the
  field (e.g. `Priority`, `Story Points`), with the typed value preserved
  (numbers stay numeric, enums/people become names, dates become time). They are
  returned by default; deselect `custom_fields` in **Fields** to omit them.
- Data-plane-compliant frames; Asana's ISO-8601 date fields (`created_at`,
  `modified_at`, `due_on`, `due_at`, …) become time fields for time-series
  panels, and row order honours the API ordering.

## Setup

1. In Asana, open **My Settings → Apps → Developer apps → Manage developer
   apps** and create a **Personal access token**.
2. In Grafana, add an **Asana** data source and paste the token.

The API URL defaults to `https://app.asana.com/api/1.0`.

## Configuration

| Field                 | Description |
| --------------------- | ----------- |
| API URL               | Asana API root. Defaults to `https://app.asana.com/api/1.0`. |
| Personal Access Token | Asana personal access token (or OAuth access token), sent as `Authorization: Bearer`. |

## Querying

1. Choose a **Query type** (Tasks, Projects, Sections, Workspaces, Teams, Users,
   Tags, or Raw).
2. Pick the **Workspace**, then optionally a **Team**, **Project** and
   **Section** to scope the query.
3. For **Tasks**: optionally pick an **Assignee** (applies only when no Project
   or Section is selected), toggle **Incomplete only**, set the **Modified**
   filter (Any time / Dashboard range / Custom), and use **Fields** to pick which
   columns to return. Set a **Limit** (0 = all, auto-paginated).
4. For **Raw**: enter a REST GET **path** (e.g. `/workspaces`) and, optionally,
   the **response key** that holds the array to flatten.

### How task scope is resolved

Asana's `GET /tasks` endpoint requires exactly one scope. The backend picks the
most specific available:

- a **Section** (`section`), else
- a **Project** (`project`), else
- an **Assignee** together with the **Workspace** (`assignee` + `workspace`).

### Notes & limitations

- Asana is **cloud only** — there is no self-hosted mode.
- Pagination is **cursor-based** (`limit`/`offset`, 100 per page); the backend
  follows the `next_page` offset automatically up to the requested limit.
- The basic tasks endpoint does not support arbitrary sorting or created/due
  date filters; for advanced search use a **Raw** query against
  `/workspaces/{workspace_gid}/tasks/search` (premium).
- Aggregations beyond row count are not provided — use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_asana_* (mage buildAll)
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
ASANA_API_TOKEN=1/123:abc... docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin and the Asana data
source auto-provisioned from `ASANA_API_TOKEN`.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
