# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Asana REST API (credential kept server-side).
- Config editor: **API URL** and a **Personal Access Token** secure field.
- The token (personal access token or OAuth access token) is sent as
  `Authorization: Bearer`.
- Health check validates connectivity and credentials via the `/users/me`
  endpoint.

### Query editor
- **Query types**: Tasks, Projects, Sections, Workspaces, Teams, Users, Tags,
  and Raw REST.
- Cascading hierarchy pickers — **Workspace → Team → Project → Section** —
  fetched live from the account; selecting a level scopes the query and refreshes
  the level below it (backed by `/workspaces`, `/teams`, `/projects`, `/sections`
  resource endpoints).
- **Assignee** picker for tasks (live **Users** picker, `/users`).
- **Incomplete only** toggle (maps to `completed_since=now`) and a **Modified**
  filter with three modes — **Any time**, **Dashboard range** (follows the
  panel's time picker, no manual entry), and **Custom** (explicit ISO-8601
  bound). Dashboard range is resolved server-side from the panel time range.
- **Archived** toggle for projects; **Limit** for all list queries.
- **Fields** selector for tasks — choose which columns to return (backed by the
  `/taskfields` resource endpoint); empty returns the default field set.
- Raw REST mode: a GET path plus an optional response key to flatten.

### Backend
- REST client with cursor pagination (`limit`/`offset` token, 100 per page,
  following `next_page.offset`) up to the requested limit (or a safety cap).
- Task scope selection: a Section, a Project, or an Assignee together with a
  Workspace. Field selection is applied server-side via Asana `opt_fields`
  (friendly names mapped to opt_fields paths, e.g. `assignee` → `assignee.name`).
- Entity flattening: nested Asana objects reduced to scalar columns
  (`assignee → name`, `projects → joined names`, `tags → joined names`,
  `current_status → text`).
- Task **custom fields** are requested (via `opt_fields`) and expanded into one
  column per field, keyed by the field name, with the typed value preserved
  (number/text/enum/multi_enum/date/people) and a `display_value` fallback.
- Raw queries: an explicit response key, or the first array of objects anywhere
  in the response (Asana wraps results under `data`), is flattened; otherwise the
  top-level object becomes one row.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); Asana ISO-8601 date
  fields are parsed to UTC time fields for time-series panels. Row order is
  preserved so the API ordering is honoured.

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `ASANA_API_TOKEN`).
