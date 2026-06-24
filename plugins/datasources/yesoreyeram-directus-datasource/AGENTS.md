# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Directus](https://directus.io).
Plugin id: `yesoreyeram-directus-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Directus REST
API, compiles filters, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getCollections/getFields)
  types.ts                DirectusQuery, DirectusDataSourceOptions, FieldInfo, CollectionInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      API URL, API Token, Default Collection
    QueryEditor.tsx       Query type, Collection picker, Fields, Search, Filters, Sort, Limit, Offset
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/collections /fields)
    client.go             Directus HTTP client: bearer auth, ListRecords, CountRecords, ListCollections, ListFields
    models.go             Settings (baseURL + secret apiToken), QueryModel
    filter.go             BuildFilter: structured filter tree -> Directus JSON filter object; SortItem
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against a Directus instance
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-directus-datasource <script>`. Install deps with a
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
  `filterTree` on the query; `pkg/plugin/filter.go::BuildFilter` compiles it into
  a Directus JSON filter object passed as the `filter` query parameter.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Directus's
  returned order honours the query `sort`. Only columns are reordered (time fields
  first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/dateTime strings parse to UTC `*time.Time`
  fields. Array/object cell values are JSON-serialised to strings.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus collection/fields/search.

## Directus API specifics (verify before changing)

- **Auth**: a Directus **static API token** (per-user, non-expiring; generated in
  the Directus admin settings). Directus accepts the token via
  `Authorization: Bearer <token>` (preferred — what this plugin sends) **or** the
  `?access_token=<token>` query parameter. We deliberately use the Bearer header
  only, so the token is never placed in URLs/logs. See
  https://docs.directus.io/guides/connect/authentication
- **List records**: `GET /items/{collection}` returns `{data:[...]}`. Pagination
  is **offset+limit** (not cursor). Params used: `fields`, `filter` (URL-encoded
  JSON object), `sort` (comma-separated, prefix `-` for desc), `limit`, `offset`,
  `search`. `limit=-1` means unlimited (avoided; the plugin pages in 100s and
  caps at `maxRecords`).
- **Count**: `GET /items/{collection}?aggregate[count]=*` returns
  `{data:[{count: N}]}` and **respects the `filter`/`search`** (and `N` may be a
  number or a numeric string depending on the DB driver — see
  `client.go::toInt64`). Do **not** use `meta=total_count` — it ignores the
  filter and reports the whole-collection size. (`meta=filter_count&limit=0` also
  works but aggregate is preferred.)
- **Schema (collections/fields)**: `GET /collections` lists collections —
  **internal `directus_*` collections are filtered out** (`client.go::ListCollections`).
  `GET /fields/{collection}` lists the fields for a specific collection.
- **Health/Ping**: `GET /users/me` — chosen because it requires a valid token.
  `GET /server/ping` needs **no auth** and would mask a missing/invalid token.
- **Filter format**: Directus uses a JSON filter object like
  `{"field_name": {"_eq": "value"}}`. Logical groups use `_and`/`_or` arrays.
  Operators (see `filter.go::operatorMap`): `_eq _neq _lt _lte _gt _gte _in _nin
  _null _nnull _empty _nempty _contains _ncontains _icontains _nicontains
  _starts_with _ends_with _between _nbetween`. List operators (`_in`/`_nin`) take
  a JSON array; `_between`/`_nbetween` take a `[min,max]` array; null/empty
  operators take boolean `true`.
- **Relational fields**: by default Directus returns related records as their
  **primary key (id)** or, for nested arrays, as JSON. To pull related fields,
  use dot-notation in **Fields**, e.g. `author.name` (the value is then a scalar
  column). Arrays/objects that remain are JSON-serialised to string cells.

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

## Verifying against live Directus

Directus is self-hosted; `docker compose up` runs Grafana with the datasource
auto-provisioned against your Directus instance:

```bash
DIRECTUS_URL=https://your-directus.example.com DIRECTUS_API_TOKEN=your-token docker compose up
```

Create a token in the Directus admin panel under Settings > API Tokens.
Grafana runs with anonymous admin, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{collections,fields}` without auth
to verify behavior end-to-end.
