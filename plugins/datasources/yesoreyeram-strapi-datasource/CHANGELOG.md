# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Strapi REST API (API token kept
  server-side, sent as `Authorization: Bearer <token>`).
- **Supports both Strapi v4 and v5** response shapes (v4 nests fields under
  `attributes`; v5 is flat with a `documentId`), auto-detected per record.
- Config editor: required **API URL**, **API Version** (v4/v5, default v5),
  secure **API Token**, optional **Default Content Type**.
- **Health check** issues a minimal request against the Default Content Type when
  set (full token validation); otherwise it performs a lightweight base-URL
  reachability check.

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- **Content Type** and **Fields** are free-text inputs (schema discovery via the
  content-type-builder endpoint needs an admin JWT and is unavailable with an API
  token; discovery degrades gracefully to an empty list).
- **Populate** input for relations: `*` (all first-level) or named relations.
- Structured **filter builder** with type-aware operators (text / number / date /
  boolean) and nested AND/OR groups, consistent with Grafana's inline-row UI.
- Multi-field **Sort** builder, **Page**, and **Page Size** (page-based pagination).
- Switching content type clears content-type-dependent options (filters, sort,
  fields, populate).

### Backend
- Server-side filter compilation (`filterTree` → Strapi REST filter query
  parameters), including `$and`/`$or` group nesting and indexed `$in`/`$notIn`
  arrays (`filters[f][$in][0]=a&filters[f][$in][1]=b`).
- Page-based pagination (`pagination[page]`/`pagination[pageSize]`, capped at 100
  per request); offset-mode params documented.
- Count via `meta.pagination.total` from a 1-record page response.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- v4 `attributes` are flattened up and v5 flat records pass through, keeping `id`
  (and `documentId`). Nested relations/components/media/dynamic zones serialise to
  JSON strings.
- Column type inference (number / boolean / time / string); date/dateTime parsed
  to UTC time fields for time-series panels. Row order is preserved so the query
  sort is honoured.
- Strapi error bodies (`{error:{status,name,message}}`) are surfaced in error
  messages.
- Template variable interpolation for filter values, content type, fields and
  populate.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource
  against a self-hosted Strapi instance.
