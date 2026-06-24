# Changelog

## Unreleased

### Fixed
- **Count is now filter-aware.** Switched from `meta=total_count` (which ignored
  the filter and reported the whole-collection size) to the Directus aggregate
  API `?aggregate[count]=*`, which respects the same `filter`/`search` as the
  records query. The count value is parsed robustly whether the driver returns a
  number or a numeric string.
- **Health check now validates the token.** `CheckHealth`/`Ping` calls
  `GET /users/me` (requires auth) instead of `GET /server/ping` (no auth), so a
  missing/invalid token correctly fails *Save & test*.
- **System collections are hidden.** `ListCollections` filters out Directus
  internal `directus_*` collections.
- Removed redundant double-handling of the `offset` parameter and made
  pagination advance by the actual page length.
- Only send `Content-Type: application/json` when a request has a body.

### Added
- Richer filter operators compiled server-side: `_starts_with`, `_ends_with`,
  `_between`/`_nbetween`, `_in`/`_nin`, `_null`/`_nnull`, `_empty`/`_nempty`, and
  case-insensitive `_icontains`/`_nicontains`. List/range operators use `csv`
  template-variable formatting.
- Cleaner Directus error messages (extracted from the `errors[].message` body).
- Backend tests for aggregate count parsing (number + string), `directus_*`
  collection filtering, offset pagination start, Bearer-only auth, and the
  expanded operator set.

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Directus REST API (API token kept
  server-side, sent as `Authorization: Bearer <token>`).
- Config editor: secure **API Token**, required **API URL**
  (Directus is self-hosted), optional **Default Collection**.

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- Live **Collection** and **Fields** pickers, fetched from the Directus
  schema API.
- **Search** input (Directus full-text search parameter).
- Structured **filter builder** with type-aware operators (text / number / date /
  boolean) and nested AND/OR groups, consistent with Grafana's inline-row UI.
- Multi-field **Sort** builder, **Limit**, and **Offset** (offset-based pagination).
- Switching collection clears collection-dependent options (filters, sort, fields).

### Backend
- Server-side filter compilation (`filterTree` → Directus JSON filter object).
- Offset-based pagination (100 records per request) up to the requested limit
  (or a safety cap of 100,000).
- Count via `meta=total_count` parameter, with fallback to data array length.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); date/dateTime parsed
  to UTC time fields for time-series panels. Array/object cells serialised as
  JSON. Row order is preserved so the query sort is honoured.
- Template variable interpolation for filter values, collection, fields and search.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource
  against a self-hosted Directus instance.
