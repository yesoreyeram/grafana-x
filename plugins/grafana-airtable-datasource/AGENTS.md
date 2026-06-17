# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Airtable](https://airtable.com).
Plugin id: `yesoreyeram-airtable-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Airtable Web
API, compiles filters, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getBases/getTables/getFields/getViews)
  types.ts                AirtableQuery, AirtableDataSourceOptions, FieldInfo, BaseInfo, TableInfo, ViewInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      Personal Access Token, Default Base ID, API URL
    QueryEditor.tsx       Query type, [Base picker], Table+View, Fields, Filters, Sort, Limit
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/bases /tables /fields /views)
    client.go             Airtable HTTP client: bearer auth, ListRecords, CountRecords, ListBases/Tables/Fields/Views
    models.go             Settings (baseURL/baseId + secret apiToken), QueryModel
    filter.go             BuildFormula: structured filter tree -> Airtable filterByFormula expression; SortItem
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against api.airtable.com (no local backing service)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-airtable-datasource <script>`. Install deps with a
single `yarn install` at the monorepo root.

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

## Key architecture facts (do not regress these)

- **Filters are compiled server-side.** The query editor persists a JSON
  `filterTree` on the query; `pkg/plugin/filter.go::BuildFormula` compiles it into
  an Airtable `filterByFormula` expression passed as the `filterByFormula` query
  parameter. A raw `filterByFormula` on the query takes precedence when set
  (`client.go::effectiveFormula`).
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Airtable's
  returned order honours the query `sort` / view order. Only columns are
  reordered (time fields first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/dateTime strings parse to UTC `*time.Time`
  fields. Array/object cell values are JSON-serialised to strings.
- **Synthetic columns.** Each record row gets `_id` (record id) and
  `_createdTime`; `orderedColumns` keeps these first, then fields alphabetically.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus base/table/view/fields.

## Airtable API specifics (verify before changing)

- **Auth**: single mode — a personal access token (PAT) sent as
  `Authorization: Bearer <token>`. Scopes needed: `data.records:read` (records)
  and `schema.bases:read` (editor metadata pickers).
- **List records**: `GET /v0/{baseId}/{tableIdOrName}` returns
  `{records:[{id,createdTime,fields}], offset}`. Pagination is a **cursor**
  (`offset`), not a page number; `pageSize` ≤ **100**. Params used: `view`,
  `filterByFormula`, `sort[i][field]`/`sort[i][direction]`, `fields[]`,
  `pageSize`, `offset`. Table ids (`tbl...`) and names are interchangeable.
- **Count**: no count endpoint — `CountRecords` paginates with `fields[]=` (no
  user fields) and counts records (respects `filterByFormula`).
- **Schema (bases/tables/fields/views)**: the metadata API.
  `GET /v0/meta/bases` lists bases (paginated by `offset`);
  `GET /v0/meta/bases/{baseId}/tables` returns tables WITH their fields and views
  in one call (`client.go::getSchema`). A base id (`app...`) is required.
- **Health/Ping**: `GET /v0/meta/whoami` (only needs a valid token).
- **Formula language**: fields are referenced as `{Field Name}`; logical groups
  use `AND(...)`/`OR(...)`; emptiness via `= BLANK()`; substring via
  `FIND(LOWER(x), LOWER(field & "")) > 0`. See `filter.go::buildConditionFormula`.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (targets Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup` (not
  `Combobox`, which is unavailable on older Grafana).
- Pure logic (sort, filter compilation/serialization, type inference) lives in
  standalone, unit-tested modules — add tests there rather than only in
  components.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps pinned to exact versions
  (no `^`/`~`). The monorepo uses Yarn 4 with `defaultSemverRangePrefix: ""`.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

A golden diff means frame behavior changed — confirm it is intended.

## Verifying against live Airtable

Airtable is SaaS-only; `docker compose up` runs Grafana with the datasource
auto-provisioned against `https://api.airtable.com`:

```bash
AIRTABLE_API_TOKEN=pat... AIRTABLE_BASE_ID=appXXXX docker compose up
```

Create a token at https://airtable.com/create/tokens with the
`data.records:read` and `schema.bases:read` scopes and access to your base(s).
Grafana runs with anonymous admin, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{bases,tables,fields,views}` without auth
to verify behavior end-to-end.
