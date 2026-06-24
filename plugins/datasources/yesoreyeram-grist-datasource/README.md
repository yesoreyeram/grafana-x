# Grist Datasource

A Grafana data source plugin (Go backend + React frontend) for
[Grist](https://www.getgrist.com/), the open-source relational spreadsheet
platform.

## Features

- **Records** query — list rows from a Grist table with filters, multi-column
  sort, field selection and a limit.
- **Count** query — number of matching rows (via SQL `COUNT(*)`).
- **SQL** query — run a raw read-only Grist SQL `SELECT`.
- Type-aware visual filter editor with AND/OR groups.
- Resource discovery for documents, tables and columns.
- Correct Date/DateTime handling (epoch seconds → time fields).

## Configuration

| Field       | Description                                                                 | Required |
| ----------- | --------------------------------------------------------------------------- | -------- |
| Grist URL   | Server URL. Cloud: `https://{team}.getgrist.com`; self-hosted: instance URL | Yes      |
| API Key     | Grist API key (Profile Settings → API), sent as `Authorization: Bearer`     | Yes      |
| Default Doc | Optional default Grist document id                                           | No       |

### Base URL and the `/api` suffix

Grist serves its REST API under the `/api` path prefix. You may enter the URL
**with or without** a trailing `/api` — the backend normalises it and always
appends `/api` itself. Examples that all work:

- `https://docs.getgrist.com` (Grist Cloud personal)
- `https://acme.getgrist.com` (Grist Cloud team site)
- `https://acme.getgrist.com/api`
- `http://localhost:8484` (self-hosted)

## Query types

### Records

Lists rows from a table. The backend chooses the most efficient transport:

- **Records endpoint** (`GET /api/docs/{doc}/tables/{table}/records`) for simple
  equality/membership filters (`filter={"Col":["a","b"]}`), `sort`
  (`Name,-Age`) and `limit`.
- **SQL endpoint** (`POST /api/docs/{doc}/sql`) when the filter uses richer
  operators (`!=`, `>`, `>=`, `<`, `<=`, `contains`, OR logic, nested groups) or
  when specific fields are selected (the records endpoint has no projection
  param). Filters compile to a **parameterized** `WHERE` clause — user values
  are bound as `?` arguments, never inlined.

> **No offset pagination.** The Grist records endpoint does **not** support
> offset/cursor pagination. `limit` caps the number of rows; `limit = 0` returns
> **all** rows in a single response.

### Count

Returns the number of matching rows. Grist has no count endpoint, so the count
is derived from a parameterized `SELECT COUNT(*) ... WHERE ...` on the SQL
endpoint.

### SQL

Runs a raw read-only Grist SQL `SELECT` (single statement, no trailing
semicolon) against the document's SQLite database. Useful for aggregations and
joins. Date/DateTime columns selected via raw SQL surface as epoch-seconds
numbers (column metadata is not available for arbitrary SQL).

## Dates

Grist stores `Date` and `DateTime` columns as Unix **epoch seconds** (numbers),
not ISO strings. The backend reads the table's column metadata
(`GET /api/docs/{doc}/tables/{table}/columns` → `fields.type`), classifies
`Date` / `DateTime:*` columns, and converts their epoch-seconds values to UTC
time fields. Detection is metadata-driven, not based on sniffing string formats.

## Development

### Prerequisites

- Go 1.24+
- Node.js 20+
- Mage (build tool)

### Build & test

```bash
# from the monorepo root: yarn install (installs all workspaces)

# backend
gofmt -w pkg/ && go build ./... && go vet ./... && go test ./pkg/...

# frontend
yarn typecheck && yarn test
yarn build
```

### Golden frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the produced data frames.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

## License

AGPL-3.0
