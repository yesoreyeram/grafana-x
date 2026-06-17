# Changelog

## Unreleased

- Fix empty database/collection/attribute dropdowns for projects that use the
  newer Appwrite **TablesDB** API. The legacy `/databases` list endpoints return
  an empty array for TablesDB-created resources, so the metadata listers now fall
  back to `/tablesdb`, `/tablesdb/{db}/tables` and
  `/tablesdb/{db}/tables/{tbl}/columns`. Manually-typed ids already worked; this
  makes the pickers populate too.

## 0.1.0

Initial release of the Grafana data source plugin for Appwrite.

- TypeScript/React frontend with a Go backend that talks to the Appwrite REST
  Databases API.
- Query types: **Documents** (list rows from a collection) and **Count**
  (number of matching documents).
- Database, collection and attribute pickers populated from the Appwrite API
  via backend resource handlers.
- Type-aware structured filter builder (with nested AND/OR groups) compiled
  server-side into Appwrite query strings, plus a raw queries escape hatch.
- Multi-attribute sort, attribute selection (`select`), and limit with
  transparent cursor-based pagination.
- API key + project id authentication, configurable endpoint (Appwrite Cloud,
  regional cloud, or self-hosted).
- Data-plane-compliant data frames with type inference and ISO 8601 time
  parsing for date/datetime attributes.
