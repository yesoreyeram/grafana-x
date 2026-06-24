# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Teable](https://teable.io).
Plugin id: `yesoreyeram-teable-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Teable API,
compiles filters, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getTables/getFields)
  types.ts                TeableQuery, TeableDataSourceOptions, TableInfo, FieldInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      API Token, Default Base ID, Server URL
    QueryEditor.tsx       Query type, Base ID, Table, Fields, Filters, Sort, Limit
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/tables /fields)
    client.go             Teable HTTP client: bearer auth, ListRecords (skip/take), CountRecords (row-count), ListTables/Fields
    models.go             Settings (baseURL/defaultBaseId + secret apiToken), QueryModel
    filter.go             BuildFilter: structured filter tree -> Teable JSON filter object ({conjunction,filterSet}); SortItem
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against app.teable.io (no local backing service)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-teable-datasource <script>`. Install deps with a
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

- **Filters are compiled server-side as JSON.** The query editor persists a JSON
  `filterTree` on the query; `pkg/plugin/filter.go::BuildFilter` compiles it into a
  Teable JSON `filter` object `{conjunction:"and"|"or", filterSet:[item|group,…]}`
  passed as the `filter` query parameter. Each condition is
  `{fieldId, operator, value}`. Teable filters are JSON, NOT a query string —
  do **not** use the deprecated `filterByTql`.
- **fieldKeyType=name.** Records are fetched with `fieldKeyType=name`, so
  `record.fields`, the filter `fieldId`, the `orderBy` fieldId and `projection`
  all reference fields by **name**. (Teable's field map is keyed by both id and
  name, so name references resolve correctly.)
- **Pagination is offset-based (`skip`/`take`), NOT a cursor.** `take` is capped at
  1000. `ListRecords` advances `skip` by `take` each page and stops on a short
  page. There is no `nextKey`/`offset` cursor.
- **Count uses the row-count aggregation endpoint.** `CountRecords` calls
  `GET /api/table/{tableId}/aggregation/row-count` → `{rowCount:N}` (filter-aware).
  Do NOT paginate-and-count.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Teable's
  returned order honours `orderBy`/the view. Only columns are reordered (time
  fields first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/dateTime strings parse to UTC `*time.Time`
  fields. Array/object cell values are JSON-serialised to strings.
- **Synthetic columns.** Each record row gets `_id` (record id), and when present
  `_createdTime` / `_lastModifiedTime` (from the record metadata).
  `orderedColumns` keeps these identity columns first; `orderTimeFirst` then moves
  time columns ahead. User fields follow alphabetically.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus base/table/view/fields.

## Teable API specifics (verify before changing)

API docs: https://help.teable.ai/en/api-doc/overview (cloud host `https://app.teable.io`;
self-hosted uses your own domain). All paths are under `/api` (there is **no**
`/v1/` segment).

- **Auth**: a personal access token sent as `Authorization: Bearer <token>`.
- **List records**: `GET /api/table/{tableId}/record` returns
  `{records:[{id, fields:{…}, createdTime, lastModifiedTime, …}]}`. Pagination is
  **offset-based**: `take` (page size, max 1000) + `skip` (offset). Params used:
  `fieldKeyType=name`, `take`, `skip`, `viewId`, `filter` (JSON), `orderBy`
  (`[{fieldId,order}]`), `projection` (repeated, by name).
- **Count**: `GET /api/table/{tableId}/aggregation/row-count` → `{rowCount:N}`
  (accepts the same `viewId`/`filter` params).
- **Tables listing**: `GET /api/base/{baseId}/table` → **bare array** `[{id,name,…}]`.
- **Fields listing**: `GET /api/table/{tableId}/field` → **bare array**
  `[{id,name,type,…}]`. Types: singleLineText, longText, number, rating, checkbox,
  date, createdTime, lastModifiedTime, singleSelect, multipleSelect, user, link,
  attachment, formula, rollup, autoNumber, createdBy, lastModifiedBy, button.
- **Health/Ping**: `GET /api/auth/user` (only needs a valid token).
- **Filter envelope** (`filter` param, JSON): `{conjunction, filterSet:[…]}` where
  each leaf is `{fieldId, operator, value}` and nested groups are themselves
  `{conjunction, filterSet}`. Operators by category — text: `is`, `isNot`,
  `contains`, `doesNotContain`, `isEmpty`, `isNotEmpty`; number: `is`, `isNot`,
  `isGreater`, `isGreaterEqual`, `isLess`, `isLessEqual`, `isEmpty`, `isNotEmpty`;
  checkbox: `is`; date: `is`, `isNot`, `isBefore`, `isAfter`, `isOnOrBefore`,
  `isOnOrAfter`, `isEmpty`, `isNotEmpty`; singleSelect: `is`, `isNot`, `isAnyOf`,
  `isNoneOf`, `isEmpty`, `isNotEmpty`; multipleSelect: `hasAnyOf`, `hasAllOf`,
  `hasNoneOf`, `isExactly`, `isEmpty`, `isNotEmpty`. List operators take an array
  value; unary operators (`isEmpty`/`isNotEmpty`) take `null`; date operators take
  `{mode:"exactDate", exactDate:<value>, timeZone:"UTC"}`.

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

## Verifying against live Teable

Teable is SaaS-capable; `docker compose up` runs Grafana with the datasource
auto-provisioned against `https://app.teable.io`:

```bash
TEABLE_API_TOKEN=tok_xxx TEABLE_BASE_ID=xxxx docker compose up
```

Create a token from your Teable account settings. Grafana runs with anonymous
admin, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{tables,fields}` without auth to verify
behavior end-to-end.
