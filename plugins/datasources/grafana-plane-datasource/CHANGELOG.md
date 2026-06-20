# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Plane REST API (credential kept server-side).
- Config editor: **API URL**, **Workspace slug** (default), **Authentication**
  method (API key or OAuth token), and the corresponding secure credential field.
- API keys are sent as the `X-API-Key` header; OAuth tokens are sent as
  `Authorization: Bearer`.
- Health check validates connectivity and credentials via the `/api/v1/users/me/`
  endpoint.

### Query editor
- **Query types**: Work items, Projects, States, Labels, Cycles, Modules,
  Members, and Raw REST.
- A **Workspace** input (falls back to the data source default) and a live
  **Project** picker fetched from the account (backed by the `/projects`
  resource endpoint).
- Multi-value work item filters: **priorities**, **states** (live picker,
  `/states`), **assignees** (live **Members** picker, `/members`), and **labels**
  (live picker, `/labels`).
- **Created** and **Updated** filters with three modes — **Any time**,
  **Dashboard range** (follows the panel's time picker, no manual entry), and
  **Custom** (explicit after/before bounds). Dashboard range is resolved
  server-side from the panel time range.
- **Expand** selector to inline related objects; **Order by** (any field, `-`
  prefix for descending); and **Limit**.
- **Fields** selector for work items — choose which columns to return (backed by
  the `/workitemfields` resource endpoint); empty returns all flattened fields.
- Raw REST mode: a GET path plus an optional response key to flatten (defaults to
  `results`).

### Backend
- REST client with cursor-based pagination (Plane `cursor` / `next_cursor`, 100
  items per page) up to the requested limit (or a safety cap).
- Endpoint selection per query type:
  - work items → `/api/v1/workspaces/{slug}/projects/{project_id}/work-items/`
  - projects → `/api/v1/workspaces/{slug}/projects/`
  - states / labels / cycles / modules → the matching project-scoped collection
  - members → `/api/v1/workspaces/{slug}/members/`
- Work item filters (priority / state / assignees / labels / created / updated)
  are applied in the backend to the fetched items, because Plane's List Work
  Items endpoint does not support filtering query parameters (it ignores unknown
  params and returns the full list). Filtering matches on the raw API values and
  works whether relations are expanded (objects) or not (UUIDs). Only
  `order_by` / `expand` / `per_page` are sent to the API.
- Entity flattening: nested Plane objects reduced to scalar columns
  (`state → name` + `state_group`, `project → name`,
  `created_by → display_name`, assignee/label arrays joined). Works for both
  expanded (object) and unexpanded (UUID) relations.
- Raw queries: an explicit response key, or Plane's `results` envelope, or the
  first array of objects anywhere in the response, is flattened; otherwise the
  top-level object becomes one row.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); Plane ISO-8601 date
  fields are parsed to UTC time fields for time-series panels. Row order is
  preserved so the query ordering is honoured.

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `PLANE_API_KEY` / `PLANE_WORKSPACE_SLUG`).
