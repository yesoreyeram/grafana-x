# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Notion REST API (integration token kept server-side).
- Config editor: **API URL**, **Notion-Version** header, secure **Integration
  Token**, and optional **Default Database ID**.
- Health check validates connectivity and credentials (`/v1/users/me`).

### Query editor
- **Query types**: Records and Count (count derived by paginating, since Notion
  has no count endpoint).
- Live **Database** and **Properties** pickers (fetched from databases shared
  with the integration via `/v1/search` and `/v1/databases/{id}`).
- Structured **filter builder** with type-aware operators and nested AND/OR
  groups, consistent with Grafana's inline-row UI.
- Multi-property **Sort** builder and **Limit**.
- Switching databases clears database-dependent options (filters, sort, fields).

### Backend
- Server-side filter compilation (`filterTree` → Notion JSON filter object).
  List operators (`in`/`not_in`) expand into or/and groups since Notion has no
  native list operator.
- Cursor-based pagination up to the requested limit (or a safety cap).
- Notion page flattening: deeply-typed property objects reduced to scalar
  columns (title, rich text, number, checkbox, select, status, multi-select,
  date, people, email, phone, url, files, formula, rollup, unique_id).
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); date properties
  parsed to UTC time fields for time-series panels. Row order is preserved so
  the query `sorts` is honoured.
- Template variable interpolation for filter values (list operators use `csv`),
  database id, sort and fields.

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `NOTION_API_TOKEN`).
