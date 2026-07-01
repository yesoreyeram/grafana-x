# Grist Datasource — Agent Guide

Guidance for AI coding agents. Humans should read [CONTRIBUTING.md](./CONTRIBUTING.md)
and [README.md](./README.md); this file is the fast, factual map.

## Plugin identity

- **Plugin ID:** `yesoreyeram-grist-datasource`
- **Executable:** `gpx_grist`
- **Go module:** `github.com/yesoreyeram/grafana-x/plugins/datasources/yesoreyeram-grist-datasource`
- A Grafana **data source plugin with a Go backend** for [Grist](https://www.getgrist.com).
  Follows the NocoDB / SeaTable plugins for structure and conventions.

## Key files

| File | Purpose |
|------|---------|
| `pkg/plugin/datasource.go` | QueryData (records/count/sql), CheckHealth, CallResource (/docs /tables /fields) |
| `pkg/plugin/client.go` | Grist REST client: records + SQL endpoints, orgs/workspaces/docs, tables, columns |
| `pkg/plugin/models.go` | Settings, QueryModel, LoadSettings/LoadQuery, requiresSQL |
| `pkg/plugin/queries.go` | Query type constants |
| `pkg/plugin/filter.go` | `simpleGristFilter` (records membership filter) + `BuildWhere` (parameterized SQL) + sort helpers |
| `pkg/plugin/sql.go` | `BuildSelectSQL` / `BuildCountSQL` |
| `pkg/plugin/frame.go` | recordsToFrame / countToFrame; epoch-seconds → time, type inference |
| `src/` | Frontend (datasource.ts, types.ts, filter.ts, sort.ts, components/) |

## Grist API specifics (verified — do not regress these)

- **Base URL.** The REST API lives under `/api`. Grist Cloud team sites are
  `https://{team}.getgrist.com`, self-hosted is the instance host. The config
  base URL may include or omit a trailing `/api`; `NewClient` normalises it and
  `apiURL()` always appends `/api`.
- **Auth:** `Authorization: Bearer <API key>` (Grist Profile Settings → API).
- **Docs listing.** There is **no flat `GET /api/docs` listing** (only `POST` to
  create). `ListDocs` enumerates `GET /api/orgs` → `GET /api/orgs/{orgId}/workspaces`
  and collects `workspaces[].docs[]`.
- **Tables:** `GET /api/docs/{docId}/tables` → `{tables:[{id, fields:{...}}]}`.
  The table `id` IS the table name (there is no `name` field).
- **Columns:** `GET /api/docs/{docId}/tables/{tableId}/columns` →
  `{columns:[{id, fields:{label, type}}]}`. The type is nested under `fields`,
  **not** top-level. `id` is the stable colId used by filters/SQL.
- **Records:** `GET /api/docs/{docId}/tables/{tableId}/records` →
  `{records:[{id:<rowId>, fields:{Col:val}}]}`. Query params: `filter` (JSON
  object `{"Col":[v1,v2]}` — values are arrays, matched as membership), `sort`
  (csv `Name,-Age`), `limit` (0 = no limit). **No `offset`/cursor pagination and
  NO `totalRows` in the response. No `fields` projection param.**
- **SQL:** `POST /api/docs/{docId}/sql` with `{sql, args}` (read-only single
  SELECT, no trailing `;`) → `{statement, records:[{fields:{...}}]}`. `args` is
  the positional-parameter key (NOT `parameters`). A GET `?q=` variant exists but
  is unused.
- **Dates:** `Date`/`DateTime:*` columns are returned as Unix **epoch seconds**
  (numbers), not ISO strings.
- **Health/Ping:** `GET /api/orgs` validates the key.

## Key architecture facts (do not regress these)

- **Records routing.** `client.ListRecords` uses the **records endpoint** for
  plain listings and simple equality/membership filters, and the **SQL endpoint**
  when `QueryModel.requiresSQL()` is true (a `Fields` projection, or a filter that
  is not a single AND-group of `eq`/`in` conditions with each column used once).
- **Filters.** `filter.go::simpleGristFilter` compiles the JSON `filterTree` into
  the Grist records `filter` object (`{"Col":[vals]}`) when representable, else
  `filter.go::BuildWhere` compiles it into a **parameterized** SQL `WHERE` with
  `?` placeholders + an ordered `[]any` of args. Values are NEVER inlined into
  SQL. Identifiers are escaped with double quotes (SQLite).
- **Count** uses `SELECT COUNT(*) AS count` via SQL (`sql.go::BuildCountSQL`) —
  Grist has no count endpoint and no offset pagination.
- **Dates are metadata-driven.** `ListRecords` fetches the table's columns,
  classifies `Date`/`DateTime:*` colIds (`dateColumnSet`), and passes the set
  into `recordsToFrame`, which converts epoch seconds → UTC `*time.Time`. Time is
  **not** inferred by sniffing strings.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Grist's
  returned order honours the query sort. Only columns are reordered (time first).
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Nullable pointer fields; arrays/objects → JSON
  string.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating filter values, docId, tableId, sort, fields and raw SQL.
- **Secrets stay on the server**: the API key is `secureJsonData`; never log it.

## Commands

Run from this plugin directory (Yarn 4 monorepo workspace).

```bash
gofmt -w pkg/ && go mod tidy && go build ./... && go vet ./... && go test ./pkg/...
yarn typecheck && yarn test    # frontend (deps via `yarn install` at monorepo root)
```

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots checked via the SDK golden
checker (includes `records_date_epoch` for the epoch-seconds date case).
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```
