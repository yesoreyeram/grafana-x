# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the SeaTable api-gateway. Implements SeaTable's
  **two-step token flow**: the configured **Base API Token** is exchanged
  server-side (`GET /api/v2.1/dtable/app-access-token/`) for a short-lived
  **Base-Token** (access token) plus the base's `dtable_uuid`; all data calls use
  `Authorization: Bearer <access_token>` against the api-gateway. The Base-Token
  is cached and re-fetched automatically on a `401`.
- Config editor: **Server URL** (defaults to `https://cloud.seatable.io`) and a
  secure **Base API Token**. A data source maps to a single base (the token's
  base), so no base id is configured.
- Health check validates connectivity and credentials by performing the token
  exchange.

### Query editor
- **Query types**: Records, Count (filter-aware count for stat panels), and SQL
  (raw SeaTable SQL).
- Live **Table** and **Column** pickers, fetched from the SeaTable metadata API.
- Structured **filter builder** with type-aware operators (text / number / date /
  checkbox) and nested AND/OR groups, consistent with Grafana's inline-row UI.
- Multi-column **Sort** builder, **Fields** multi-select, optional **View**, and
  **Limit**. Switching table clears table-dependent options.

### Backend
- Records are fetched via the **rows endpoint** for plain listings (optionally a
  view) and via the **SQL endpoint** when a filter, sort, or fields selection is
  present (the rows endpoint cannot filter/sort/project).
- **Parameterized** server-side filter compilation (`filterTree` → SQL `WHERE`
  with `?` placeholders + a parameters array): filter values are never inlined,
  preventing SQL injection. Identifiers are backtick-escaped.
- Count via `SELECT COUNT(*)`. Raw SQL passthrough with `convert_keys: true`.
- Automatic pagination: rows endpoint by `start`/`limit` (≤ 1000/request), SQL by
  `LIMIT`/`OFFSET` (≤ 10000/request), up to the requested limit or a safety cap.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1). Rows keep the `_id`/`_ctime`/`_mtime` identity columns
  (other internal metadata stripped); identity + time columns are ordered first.
- Column type inference (number / boolean / time / string); date/ctime/mtime
  parsed to UTC time fields. Multiple-select/collaborator/link cells serialised as
  JSON. Row order is preserved so the query sort / view order is honoured.
- Template variable interpolation for filter values, table, view, fields and SQL.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource
  against a SeaTable server.
- Go unit tests (HTTP mocked with `httptest`, including the token exchange),
  filter/SQL compiler tests, frame tests and golden data-frame snapshots.
