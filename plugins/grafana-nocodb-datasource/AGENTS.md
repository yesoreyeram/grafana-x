# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [NocoDB](https://nocodb.com).
Plugin id: `yesoreyeram-nocodb-datasource`. The frontend (TypeScript/React) renders
the config and query editors; the Go backend talks to the NocoDB REST API, builds
filters, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getTables/getFields/getViews)
  types.ts                NocoDBQuery, NocoDBDataSourceOptions, FieldInfo, TableInfo, ViewInfo, enums
  sort.ts                 parseSort / serializeSort (NocoDB `-field,field` <-> structured rows)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      Platform, Base URL, API Version, API Token, Default Base ID
    QueryEditor.tsx       Query type, Table+View, Fields, Filters, Sort, Limit
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/tables /fields /views)
    client.go             NocoDB HTTP client: ListRecords (v2/v3 + pagination), CountRecords, ListTables/Fields/Views
    models.go             Settings (platform/baseURL/apiVersion/token), QueryModel, LoadSettings/LoadQuery
    filter.go             BuildWhere: structured filter tree -> NocoDB where clause (v2 `@`-quoted / v3 plain)
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
scripts/seed.mjs          Idempotent NocoDB seeder for the local Docker stack
provisioning/             Grafana provisioning (datasource example + generated)
docker-compose.yaml       NocoDB + seed + Grafana (anonymous admin)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained, not the create-plugin .config indirection)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory** (`yarn` resolves the workspace
automatically), or from the monorepo root with
`yarn workspace yesoreyeram-nocodb-datasource <script>`. Install deps with a
single `yarn install` at the monorepo root — there is no per-plugin install.

| Task                | Command |
| ------------------- | ------- |
| Install deps        | `yarn install` (run at monorepo root) |
| Frontend build      | `yarn build` |
| Frontend watch      | `yarn dev` |
| Typecheck           | `yarn typecheck` |
| Lint                | `yarn lint` (`yarn lint:fix` to fix) |
| Frontend tests      | `yarn test` |
| Backend tests       | `go test ./pkg/...` |
| Backend build (1)   | `mage -v build:linuxARM64` (or `build:linux`, `build:darwinARM64`, …) |
| Backend build (all) | `mage -v buildAll` |
| Local full stack    | `docker compose up` (build `dist/` first) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Filters are built server-side.** The query editor persists a JSON `filterTree`
  on the query; `pkg/plugin/filter.go::BuildWhere` compiles it into the NocoDB
  `where` clause. The `@` quote prefix is used for **v2 only** (v3 rejects it).
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — NocoDB's
  returned order honours the query `sort`. Only columns are reordered (time
  fields first). Re-sorting rows is a known past bug; never reintroduce it.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). DateTime/Date strings parse to UTC `*time.Time`
  fields. Checkbox `1/0` stays numeric (don't coerce to bool).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value (list operators use `csv` formatting).
- **System fields are hidden** in the Fields/Filter pickers (`ListFields` filters
  out `system` columns and ID/CreatedTime/etc. uidt types).
- **NocoDB API specifics learned the hard way** (verify before changing):
  - `in`/`anyof`/etc. take **unquoted** comma tokens: `(Status,in,open,closed)`.
  - `btw`/`nbtw` are **not supported for Number** columns — use `ge`/`le`.
  - Checkbox filters use `1`/`0`, not `true`/`false`.
  - No group-by / general aggregation endpoint exists; only a filter-aware `count`.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (the plugin
  targets Grafana >= 10.4 / runs on 11.x). `Combobox` is NOT available on older
  Grafana — use `Select`/`MultiSelect`/`RadioButtonGroup`. Past regression.
- Pure logic (sort, filter serialization, type inference) lives in standalone,
  unit-tested modules — add tests there rather than only in components.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- Match existing code style; do not introduce new frameworks or build tooling.
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps are pinned to exact
  versions (no `^`/`~`). Keep them exact when adding/upgrading deps. The
  monorepo uses Yarn 4 with `defaultSemverRangePrefix: ""`, so `yarn add`
  already records exact versions.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

A golden diff means frame behavior changed — confirm it is intended.

## Verifying against live NocoDB

`docker compose up` seeds a `Sample` base (Customers, Metrics, Logs, Sales) and
auto-provisions the datasource with a working token. Grafana runs with anonymous
admin at http://localhost:3000, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{tables,fields,views}` without auth to
verify behavior end-to-end. NocoDB API is at http://localhost:8080.
