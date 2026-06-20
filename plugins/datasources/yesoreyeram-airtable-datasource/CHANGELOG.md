# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Airtable Web API (personal access token kept
  server-side, sent as `Authorization: Bearer <token>`).
- Config editor: secure **Personal Access Token**, optional **Default Base ID**,
  and an optional **API URL** (defaults to `https://api.airtable.com`).
- Health check validates connectivity and credentials via the metadata
  `whoami` endpoint.

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- Live **Base**, **Table**, **View** and **Fields** pickers, fetched from the
  Airtable metadata API (requires the `schema.bases:read` scope). A Base picker
  is shown when no default base id is configured.
- Structured **filter builder** with type-aware operators (text / number / date /
  checkbox) and nested AND/OR groups, consistent with Grafana's inline-row UI.
- Multi-field **Sort** builder and **Limit**.
- Switching base/table clears table-dependent options (view, filters, sort,
  fields).

### Backend
- Server-side filter compilation (`filterTree` → Airtable `filterByFormula`),
  plus a raw-formula escape hatch that takes precedence when set.
- Automatic pagination following the Airtable `offset` cursor up to the requested
  limit (or a safety cap), 100 records per request.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1). Each record frame includes synthetic `_id` and
  `_createdTime` columns.
- Column type inference (number / boolean / time / string); date/dateTime parsed
  to UTC time fields for time-series panels. Array/object cells (multi-selects,
  linked records, attachments, collaborators) serialised as JSON. Row order is
  preserved so the query sort / view order is honoured.
- Template variable interpolation for filter values, base, table, view and fields.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource
  against the hosted Airtable API.
