# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the ClickUp REST API (credential kept server-side).
- Config editor: **API URL**, **Authentication** method (Personal token or OAuth
  token), and the corresponding secure credential field.
- Personal tokens are sent raw in the `Authorization` header; OAuth tokens are
  sent as `Authorization: Bearer`.
- Health check validates connectivity and credentials via the `/v2/user`
  endpoint.

### Query editor
- **Query types**: Tasks, Lists, Folders, Spaces, Workspaces, and Raw REST.
- Cascading hierarchy pickers — **Workspace → Space → Folder → List** — fetched
  live from the account; selecting a level scopes the query and refreshes the
  level below it (backed by `/teams`, `/spaces`, `/folders`, `/lists` resource
  endpoints).
- Multi-value task filters: **statuses**, **assignees** (live **Members** picker,
  `/members`), and **tags**.
- **Created**, **Updated** and **Due** filters with three modes — **Any time**,
  **Dashboard range** (follows the panel's time picker, no manual entry), and
  **Custom** (explicit after/before bounds). Dashboard range is resolved
  server-side from the panel time range.
- **Include closed**, **Subtasks** and **Archived** toggles; **Order by**
  (created / updated / due date / id), **Reverse**, and **Limit**.
- **Fields** selector for tasks — choose which columns to return (backed by the
  `/taskfields` resource endpoint); empty returns all flattened fields.
- Raw REST mode: a GET path plus an optional response key to flatten.

### Backend
- REST client with page-based pagination (100 tasks per page) up to the requested
  limit (or a safety cap).
- Endpoint selection: tasks read from a single List
  (`/v2/list/{list_id}/task`) when a List is chosen, otherwise from the workspace
  with Filtered Team Tasks (`/v2/team/{team_id}/task`), scoped by space/folder/list
  ids.
- Server-side filter assembly into ClickUp's task query parameters, with date
  modes converted to Unix-millisecond `*_gt` / `*_lt` bounds.
- Task flattening: nested ClickUp objects reduced to scalar columns
  (`status → name`, `priority → name`, `creator → username`, assignee/tag arrays
  joined).
- Raw queries: an explicit response key, or the first array of objects anywhere
  in the response, is flattened; otherwise the top-level object becomes one row.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); ClickUp
  Unix-millisecond date fields are parsed to UTC time fields for time-series
  panels. Row order is preserved so the query ordering is honoured.

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `CLICKUP_API_TOKEN`).
