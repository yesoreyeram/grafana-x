# Grafana Plane Data Source

A Grafana **data source plugin with a Go backend** for [Plane](https://plane.so).
Query your Plane workspace as Grafana data frames: list work items, projects,
states, labels, cycles, modules and members, filter them, order them, and run a
custom REST request when you need something specific.

- Plugin id: `yesoreyeram-plane-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (Plane REST API client, cursor paginator, entity flattener, frame builder)
- Works with Plane Cloud and self-hosted Plane (the API URL is configurable).

## Features

- **Backend data source** — the API key / OAuth token is stored server-side and
  never sent to the browser.
- **Predefined query types**: Work items, Projects, States, Labels, Cycles,
  Modules, Members.
- **Raw REST** mode for any custom GET path; Plane's `results` envelope (or the
  first array of objects found, or an explicit response key) is flattened into a
  table.
- Plane's hierarchy is modelled simply: a **Workspace** slug and a **Project**
  picker fetched live from your account. Changing the project scopes the
  project-dependent filters.
- Rich, multi-value work item filters: **priorities**, **states**, **assignees**
  (by user id), and **labels** — plus **created/updated date filters**. Plane's
  List Work Items endpoint does not support filtering query parameters, so these
  filters are applied in the backend to the fetched items (matching the raw API
  values, whether relations are expanded or not).
- **Created** and **Updated** filters support three modes: _Any time_, _Dashboard
  range_ (follows the panel's time picker automatically), and _Custom_ (explicit
  after/before bounds). Dashboard range is resolved from the panel time range and
  applied in the backend.
- **Expand** selector to inline related objects (`assignees`, `state`, `labels`,
  …) so columns show readable names instead of UUIDs.
- **Fields selector** for work items — choose exactly which columns to return;
  leave empty for all flattened fields.
- Live **Project**, **State**, **Label** and **Member** pickers fetched from your
  account.
- Plane's nested JSON objects are flattened to scalar columns (e.g.
  `state → name` (+ `state_group`), `assignees → "Alice, Bob"`,
  `project → name`, `created_by → display_name`).
- Data-plane-compliant frames; Plane's ISO-8601 date fields (`created_at`,
  `updated_at`, `target_date`, …) become time fields for time-series panels, and
  row order honours the query ordering.

## Setup

### API key (simplest)

1. In Plane, open **Profile Settings → Personal Access Tokens** and click
   **Add personal access token** (the key starts with `plane_api_`).
2. In Grafana, add a **Plane** data source, choose **API key**, paste the key,
   and set the **Workspace slug** (the unique identifier from your Plane URL,
   e.g. the `my-team` in `https://app.plane.so/my-team/projects/`).

### OAuth token

1. Build a Plane app and complete the
   [OAuth flow](https://developers.plane.so/dev-tools/build-plane-app/overview)
   to obtain an access token.
2. In Grafana, add a **Plane** data source, choose **OAuth token**, and paste the
   access token (sent as `Authorization: Bearer`).

The API URL defaults to `https://api.plane.so`. For a self-hosted instance, set
it to your instance's API base URL.

## Configuration

| Field          | Description |
| -------------- | ----------- |
| API URL        | Plane API root. Defaults to `https://api.plane.so`. |
| Workspace slug | Default workspace slug; a query can override it. |
| Authentication | `API key` (sent as `X-API-Key`) or `OAuth token` (sent as `Authorization: Bearer`). |
| API key        | Plane personal API key, `plane_api_...` (when using key auth). |
| OAuth Token    | Plane OAuth2 access token (when using OAuth auth). |

## Querying

1. Choose a **Query type** (Work items, Projects, States, Labels, Cycles,
   Modules, Members, or Raw).
2. Set the **Workspace** (or leave empty to use the data source default), then
   pick a **Project** for project-scoped types.
3. For **Work items**: optionally filter by **Priorities**, **States**,
   **Assignees**, **Labels** (all multi-select), and **Created**/**Updated**
   (Any time / Dashboard range / Custom). Use **Expand** to inline related
   objects, **Fields** to pick which columns to return, **Order by** (e.g.
   `-created_at`, `priority`, `sequence_id`), and a **Limit** (0 = all,
   auto-paginated).
4. For **Raw**: enter a REST GET **path** (e.g.
   `/api/v1/workspaces/my-team/projects/`) and, optionally, the **response key**
   that holds the array to flatten (defaults to `results`).

Multiple values within a filter match **any** (OR). Date bounds accept ISO-8601
values (e.g. `2024-01-01` or `2024-01-01T00:00:00Z`).

### Notes & limitations

- Pagination is **cursor-based** (Plane's `cursor` / `next_cursor`, 100 items per
  page); the backend follows cursors automatically up to the requested limit.
- Plane rate-limits to **60 requests/minute** per API key; large unpaginated
  queries can hit this.
- Predefined queries flatten the entity's fields to scalar columns. For more
  fields or nested relations, use **Expand** or **Raw**.
- The **assignees**/**states**/**labels** filters use Plane **UUIDs** (the
  pickers provide them).
- Aggregations beyond row count are not provided — use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_plane_* (mage buildAll)
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
PLANE_API_KEY=plane_api_... PLANE_WORKSPACE_SLUG=my-team docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin and the Plane data
source auto-provisioned from `PLANE_API_KEY` / `PLANE_WORKSPACE_SLUG`.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
