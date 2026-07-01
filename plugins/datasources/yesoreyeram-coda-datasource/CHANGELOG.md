# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Coda Web API (bearer token kept server-side).
- Config editor: secure **API Token**, optional **Default Doc ID**.
- Coda SaaS base URL `https://coda.io/apis/v1`; resource paths appended to it.
- Health check validates connectivity and credentials via `/whoami`.

### Query editor
- **Query types**: Rows and Count.
- Live **Doc**, **Table** and **Columns** pickers, fetched from the Coda API
  (column data type read from each column's `format.type`).
- **Filter** (single-column equality), an **Advanced query** raw escape hatch,
  **Sort by** (createdAt / updatedAt / natural), **Visible only**, **Value
  format** (simple / simpleWithArrays / rich) and a **Limit**.

### Backend
- **Single-column filtering** compiled into Coda's `query` parameter
  (`<column>:<value>`; names quoted, ids as-is, values JSON-encoded). A raw query
  takes precedence. Coda has no general filter language — richer filtering is a
  Grafana-transformations job.
- Rows parsed from Coda's `values` map (always `useColumnNames=true`), flattened
  to records keyed by column name plus synthetic `id`, `name`, `index`,
  `createdAt`, `updatedAt`, `href` and `browserLink` columns.
- **Column projection** applied client-side (the rows endpoint has no columns
  parameter).
- **Count**: reads the table's `rowCount` for an unfiltered count (one request);
  paginates and counts the matching rows when a filter is applied.
- Automatic pagination following Coda's `pageToken` cursor.
- Data-plane-compliant frames: rows → `table` (v0.1), count → `numeric-wide`
  (v0.1). `date`/`dateTime` (incl. `createdAt`/`updatedAt`) parsed to UTC time
  fields; array/object cells serialised to JSON.
- Friendly error hints for 401 / 403 / 404 / 429 responses.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource against `coda.io`.
