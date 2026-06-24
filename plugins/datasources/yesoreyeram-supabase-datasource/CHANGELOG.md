# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Supabase PostgREST API.
- Config editor: **Project URL** and **Service Role Key** (stored encrypted,
  never sent to the browser).
- Dual auth headers: sends both `apikey` and `Authorization: Bearer <key>` on
  every request.
- Health check validates connectivity and credentials via the PostgREST root
  endpoint.

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- Live **Table** picker, fetched from the PostgREST OpenAPI schema.
- **Select** columns (comma-separated input).
- Structured **filter builder** with PostgREST operators (eq, neq, gt, gte, lt,
  lte, like, ilike, in, is null, is not null) and nested AND/OR groups.
- Multi-field **Sort** builder, **Limit**, and **Offset**.

### Backend
- Server-side filter compilation (`filterTree` → PostgREST query params).
- Range-based pagination following the `Content-Range` header, up to the
  requested limit or a safety cap (1000 rows/request).
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); date/dateTime parsed
  to UTC time fields. Array/object cells serialised as JSON. Row order is
  preserved.
- Template variable interpolation for table, filter values and select columns.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource
  against the Supabase PostgREST API.
