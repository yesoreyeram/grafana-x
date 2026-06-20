# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the NocoDB Data API (token kept server-side).
- Config editor: **Platform** (NocoDB Cloud / self-hosted), **Base URL**,
  **API Version** (v2 / v3), secure **API Token**, and optional **Default Base ID**.
- Health check validates connectivity and credentials.
- Supports both the NocoDB Data API **v2** and **v3** (v3 uses the base-scoped
  path and response shape; falls back to v2 when no base id is available).

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- Live **Table**, **View** and **Fields** pickers (fetched from NocoDB);
  system fields are hidden.
- Structured **filter builder** with type-aware operators and nested AND/OR
  groups, consistent with Grafana's inline-row UI.
- Multi-field **Sort** builder and **Limit**.
- Switching tables clears table-dependent options (view, filters, sort, fields).

### Backend
- Server-side filter compilation (`filterTree` → NocoDB `where`); `@` quoting for
  v2, plain for v3.
- Automatic pagination up to the requested limit (or a safety cap).
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); DateTime/Date parsed
  to UTC time fields for time-series panels. Row order is preserved so the query
  `sort` is honoured.
- Template variable interpolation for filter values (list operators use `csv`),
  table, view, sort and fields.

### Tooling
- Local Docker stack (NocoDB + idempotent seeder + Grafana with anonymous admin)
  with sample tables: Customers, Metrics (time series), Logs, Sales.
