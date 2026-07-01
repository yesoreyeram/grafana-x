# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the **Todoist unified API v1**
  (`https://api.todoist.com/api/v1`); credential kept server-side.
- Config editor: an **API Token** secure field, sent as `Authorization: Bearer`.
- Health check validates connectivity and credentials via `/projects?limit=1`.

### Query editor
- **Query types**: Tasks and Count.
- Cascading **Project → Section** pickers fetched live from the account
  (`/projects`, `/sections`); a **Label** picker (by name, `/labels`); and an
  optional **Parent task ID** scope.
- **Filter** field using Todoist's filter query language, with an optional
  **Filter language**. A non-empty filter routes to `/tasks/filter` and overrides
  the project/section/label/parent scope.
- **Limit** (Max task scan for Count).

### Backend
- v1 cursor pagination (`limit` up to 200 + opaque `cursor`, following
  `next_cursor`) up to the requested limit or a safety cap — across tasks and the
  project/section/label resource lists.
- Task scope routing: `/tasks` (project_id/section_id/label/parent_id) vs the
  dedicated `/tasks/filter` (query/lang) endpoint.
- Count derived by paginating and counting matching tasks (Todoist has no native
  count endpoint), capped by the optional limit.
- Task flattening: the nested `due` object → `dueDate` (time) / `dueString` /
  `dueIsRecurring` (bool) / `dueTimezone`; `deadline` → `deadlineDate` (time);
  `duration` → `durationAmount` / `durationUnit`; `labels` → JSON string array.
- v1 field names surfaced as-is (`checked`, `added_at`, `added_by_uid`,
  `responsible_uid`, `assigned_by_uid`, `child_order`, `note_count`); the `url`
  field was removed in v1.
- Data-plane-compliant frames: records → `table` (v0.1), count → `numeric-wide`
  (v0.1). ISO-8601 date fields parse to UTC time fields; row order preserved.

### Notes
- Returns **active (incomplete) tasks only**; completed tasks live behind
  separate v1 endpoints and are not surfaced.
- API `priority` is inverted vs the UI (`4` = highest / UI p1, `1` = none).

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `TODOIST_API_TOKEN`).
