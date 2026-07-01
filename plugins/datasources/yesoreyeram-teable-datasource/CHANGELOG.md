# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Teable API (API token kept server-side, sent
  as `Authorization: Bearer <token>`).
- Config editor: secure **API Token**, optional **Default Base ID**, and a
  **Server URL** (defaults to `https://app.teable.io`; set your own domain when
  self-hosted).
- Health check validates connectivity and credentials via the `GET /api/auth/user`
  endpoint.

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- Live **Table** and **Fields** pickers, fetched from the Teable API. Tables can be
  listed from a per-query base ID or the configured default base ID.
- Structured **filter builder** with Teable's native, type-aware operators
  (text / number / checkbox / date / single-select / multi-select / attachment)
  and nested AND/OR groups, consistent with Grafana's inline-row UI.
- Multi-field **Sort** builder and **Limit**.
- Switching base/table clears table-dependent options (filters, sort, fields).

### Backend
- Server-side filter compilation: filterTree → Teable JSON `filter` object
  (`{conjunction, filterSet:[{fieldId, operator, value}, …]}`), sent as the
  `filter` query parameter (not the deprecated `filterByTql`).
- Records fetched with `fieldKeyType=name`, so columns, filters, sort and field
  selection all use human field names.
- Offset-based pagination via `skip`/`take` (page size ≤ 1000) up to the requested
  limit (or a safety cap).
- Count via the dedicated `aggregation/row-count` endpoint (`{rowCount}`), so no
  records are paginated to count.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1). Each record frame includes synthetic `_id`,
  `_createdTime` and `_lastModifiedTime` columns.
- Column type inference (number / boolean / time / string); date/dateTime parsed
  to UTC time fields for time-series panels. Array/object cells serialised as
  JSON. Row order is preserved so the query sort order is honoured.
- Template variable interpolation for filter values, base ID, table ID, view ID
  and fields.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource
  against the hosted Teable API.
