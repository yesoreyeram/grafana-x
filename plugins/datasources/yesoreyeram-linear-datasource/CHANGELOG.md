# Changelog

## Unreleased

### Query editor
- Issue filters are now **multi-value**: states, assignees, labels, priorities
  and projects are multi-selects (values within a filter match any; filters are
  AND'd).
- New issue filters: **creator**, **created/updated date filters**, and an
  **include archived** toggle.
- The **Created** and **Updated** filters now offer three modes — **Any time**,
  **Dashboard range** (follows the panel's time picker, no manual entry), and
  **Custom** (explicit after/before bounds, shown only when Custom is selected).
  Dashboard range is resolved server-side from the panel time range.
- New **Fields** selector for issues — choose which columns to return; the
  GraphQL selection set is built dynamically (empty = default set).
- New live pickers: **Labels**, **Projects**, **Users** and **Fields** (backed by
  new `/labels`, `/projects`, `/users`, `/issuefields` resource endpoints).

### Fixes
- Each query-editor dropdown (states, assignees, labels, projects, users, fields)
  now loads **independently**. Previously they were fetched together with a single
  `Promise.all`, so one failing request blanked every list (and could prevent the
  priorities filter from being usable). Each list now also shows its own
  loading/error state, and the layout no longer collapses while a list loads.

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Linear GraphQL API (credential kept server-side).
- Config editor: **GraphQL URL**, **Authentication** method (Personal API key or
  OAuth token), and the corresponding secure credential field.
- Personal API keys are sent raw in the `Authorization` header; OAuth tokens are
  sent as `Authorization: Bearer`.
- Health check validates connectivity and credentials via the `viewer` query.

### Query editor
- **Query types**: Issues, Projects, Teams, Users, Cycles, and Raw GraphQL.
- Live **Team** and **State** pickers (fetched from the workspace).
- Simple filters for issues (team, workflow state, assignee, title contains) and
  cycles (team), plus **Order by** (created / updated) and **Limit**.
- Raw GraphQL mode with optional JSON variables.

### Backend
- GraphQL client with cursor-based pagination (`first`/`after`, `pageInfo`) up to
  the requested limit (or a safety cap). Page size capped at 250.
- Server-side filter assembly into Linear's `IssueFilter` / `CycleFilter`
  objects.
- Node flattening: nested GraphQL relations reduced to scalar columns
  (`state → name`, `assignee → name`, `team → name`, label connections joined).
- Raw queries: the first connection (object with a `nodes` array) anywhere in the
  response is found and flattened; otherwise the top-level object becomes one row.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); timestamp fields are
  parsed to UTC time fields for time-series panels. Row order is preserved so the
  query ordering is honoured.

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `LINEAR_API_KEY`).
