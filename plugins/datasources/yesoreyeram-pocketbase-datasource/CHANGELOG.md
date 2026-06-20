# Changelog

## 0.1.0

Initial release of the Grafana data source plugin for PocketBase.

- TypeScript/React frontend with a Go backend that talks to the PocketBase REST
  Records API.
- Query types: **Records** (list rows from a collection) and **Count** (number
  of matching records).
- Collection and field pickers populated from the PocketBase API via backend
  resource handlers.
- Type-aware structured filter builder (with nested AND/OR groups) compiled
  server-side into a single PocketBase filter expression, plus a raw filter
  escape hatch.
- Multi-field sort, field selection (`fields`), and limit with transparent
  page-based pagination.
- Three authentication modes — **superuser**, **user** (regular auth
  collection) and **token** — with token caching and transparent
  re-authentication on `401`.
- Data-plane-compliant data frames with type inference and time parsing for
  date/autodate fields.
