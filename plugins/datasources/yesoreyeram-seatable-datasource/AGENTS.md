# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [SeaTable](https://seatable.io).
Plugin id: `yesoreyeram-seatable-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend performs the SeaTable
two-step token exchange, talks to the SeaTable api-gateway, compiles filters into
parameterized SQL, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource call (getTables)
  types.ts                SeaTableQuery, SeaTableDataSourceOptions, TableInfo, ColumnInfo, enums
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      Server URL, Base API Token (secret)
    QueryEditor.tsx       Query type (records|count|sql), Table, View, Fields, Filters, Sort, Limit, SQL
    FilterEditor.tsx      Recursive filter/group builder (operates on column names)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count|sql), CheckHealth, CallResource (/tables)
    client.go             SeaTable client: token exchange + caching + 401 retry, rows/SQL/metadata calls
    models.go             Settings (serverURL + secret apiToken), QueryModel, LoadSettings/LoadQuery
    filter.go             BuildWhere: structured filter tree -> parameterized SQL WHERE + params; SortItem
    sql.go                BuildSelectSQL / BuildCountSQL: SELECT/COUNT assembly, identifier escaping
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
    queries.go            Query type constants
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against a SeaTable server
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-seatable-datasource <script>`. Install deps
with a single `yarn install` at the monorepo root.

| Task                | Command |
| ------------------- | ------- |
| Install deps        | `yarn install` (run at monorepo root) |
| Build (front+back)  | `yarn build` (frontend + `mage buildAll`) |
| Frontend only       | `yarn build:frontend` |
| Backend only        | `yarn build:backend` (alias for `mage buildAll`) |
| Frontend watch      | `yarn dev` |
| Typecheck           | `yarn typecheck` |
| Lint                | `yarn lint` (`yarn lint:fix` to fix) |
| Frontend tests      | `yarn test` |
| Backend tests       | `go test ./pkg/...` |
| Backend build (1)   | `mage -v build:linuxARM64` (or `build:linux`, `build:darwinARM64`, …) |
| Local full stack    | `docker compose up` (run `yarn build` first) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## SeaTable API specifics (verified — do not regress these)

- **Auth is two-step.** The configured **Base API Token** is NOT used on data
  endpoints. `client.go` first exchanges it for a short-lived **Base-Token** +
  `dtable_uuid`:
  - `GET {server}/api/v2.1/dtable/app-access-token/` with header
    `Authorization: Token <base_api_token>` →
    `{access_token, dtable_uuid, dtable_server}`.
  - Note the path is `/api/v2.1/` (seahub), **not** `/api/v2/`.
  - All gateway calls then use `Authorization: Bearer <access_token>` against
    `{dtable_server}/api/v2/dtables/{dtable_uuid}/…`. `dtable_server` is the
    api-gateway base, e.g. `https://cloud.seatable.io/api-gateway/`.
  - The Base-Token is cached and re-fetched on a `401` (see `gatewayJSON`).
- **List Rows**: `GET …/rows/?table_name=&view_name=&start=&limit=&convert_keys=true`
  → `{rows:[…]}`. Rows are flat maps keyed by **column name** (because
  `convert_keys=true`) including `_id`, `_ctime`, `_mtime`. `limit` ≤ **1000**,
  `start` is a 0-based offset. The rows endpoint has **no filter/sort/fields**
  params.
- **SQL query**: `POST …/sql/` with `{sql, convert_keys:true, parameters:[…]}` →
  `{results:[…], metadata:[…], success}`. Supports `?` **parameterized** queries
  (used by the filter compiler). SQL returns ≤ **10000** rows/request.
- **Metadata**: `GET …/metadata/` → `{metadata:{tables:[{_id,name,columns:[{key,name,type}]}]}}`.
- **Server default**: `https://cloud.seatable.io` (configurable for self-hosted).

## Key architecture facts (do not regress these)

- **Records routing.** `client.ListRecords` uses the **rows endpoint** for plain
  listings (optionally a view) and the **SQL endpoint** when a filter, sort, or
  fields selection is present (`QueryModel.requiresSQL`). The rows endpoint cannot
  filter/sort/project; SQL has no view.
- **Filters compile to parameterized SQL.** `filter.go::BuildWhere` turns the
  JSON `filterTree` into a `WHERE` fragment with `?` placeholders and an ordered
  `[]any` of params. Values are NEVER inlined into the SQL string. Identifiers
  (table/column names) are escaped with backticks (embedded backticks stripped).
- **Count** uses `SELECT COUNT(*)` via SQL (`sql.go::BuildCountSQL`).
- **Raw SQL** (`queryType:"sql"`) is passed through unchanged with
  `convert_keys:true`; its rows are NOT normalized so the user sees exactly what
  they selected.
- **Row normalization.** Record paths keep `_id`/`_ctime`/`_mtime` and drop the
  other internal `_`-prefixed columns so the rows endpoint and SQL `SELECT *`
  produce consistent frames (`client.go::normalizeRow`).
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — SeaTable's
  returned order honours the query sort / view order. Only columns are reordered
  (identity + time fields first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/ctime/mtime parse to UTC `*time.Time`;
  array/object cells are JSON-serialised.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus table, view, fields and raw SQL.
- **Secrets stay on the server**: the Base API Token is `secureJsonData`; never
  log it or send it to the browser.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (targets Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup`/`TextArea`
  (not `Combobox`, which is unavailable on older Grafana).
- Pure logic (sort, filter compilation/serialization, SQL building, type
  inference) lives in standalone, unit-tested modules — add tests there.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest` (mock BOTH the token-exchange call and the data calls).
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps pinned to exact versions.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

A golden diff means frame behavior changed — confirm it is intended.

## Verifying against live SeaTable

`docker compose up` runs Grafana with the datasource auto-provisioned:

```bash
yarn build
SEATABLE_API_TOKEN=<base-api-token> docker compose up
# self-hosted: add SEATABLE_SERVER_URL=https://seatable.example.com
```

Create a Base API Token from a base's API Tokens panel. Grafana runs with
anonymous admin at http://localhost:3000, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/tables` without auth to verify end-to-end.
