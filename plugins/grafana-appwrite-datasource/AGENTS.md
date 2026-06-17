# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Appwrite](https://appwrite.io).
Plugin id: `yesoreyeram-appwrite-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Appwrite REST
Databases API, compiles filters into query strings, paginates, and converts
results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getDatabases/getCollections/getAttributes)
  types.ts                AppwriteQuery, AppwriteDataSourceOptions, DatabaseInfo, CollectionInfo, AttributeInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      Endpoint, Project ID, API Key (secret), Default Database ID
    QueryEditor.tsx       Query type, [Database picker], Collection, Attributes, Filters, Raw queries, Sort, Limit
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (documents|count), CheckHealth, CallResource (/databases /collections /attributes)
    client.go             Appwrite HTTP client: project+key auth, ListDocuments, CountDocuments, ListDatabases/Collections/Attributes
    models.go             Settings (endpoint/projectId/databaseId + secret apiKey), QueryModel
    filter.go             BuildFilterQueries: structured filter tree -> Appwrite query strings; orderQueries; selectQuery; SortItem
    frame.go              documentsToFrame / countToFrame: type inference, time parsing, data-plane contracts
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against cloud.appwrite.io (no local backing service)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-appwrite-datasource <script>`. Install deps with a
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
  `filterTree` on the query; `pkg/plugin/filter.go::BuildFilterQueries` compiles
  it into a slice of Appwrite query strings passed as repeated `queries[]`
  parameters. A raw `rawQueries` block on the query takes precedence when set
  (`client.go::baseQueries`).
- **Appwrite query wire format.** Each query is a JSON string
  `{"method":..., "attribute":..., "values":[...]}` (exactly what the Appwrite
  SDK `Query` class emits). Top-level AND conditions are emitted as separate
  query strings (Appwrite AND-s them implicitly); OR / nested groups are wrapped
  in a single `{"method":"or"|"and","values":[<nested query strings>]}`.
- **Row order is preserved.** `documentsToFrame` must NOT re-sort rows —
  Appwrite's returned order honours the query sort (`orderAsc`/`orderDesc`) or
  the default order. Only columns are reordered (time fields first). Re-sorting
  rows would be a bug.
- **Frames are data-plane compliant.** Documents → `FrameTypeTable` (v0.1);
  Count → `FrameTypeNumericWide` (v0.1). datetime / ISO 8601 strings parse to UTC
  `*time.Time` fields. Array/object cell values (relationships, `$permissions`,
  array attributes) are JSON-serialised to strings.
- **Identity columns.** Each document row keeps the system attributes `$id`,
  `$createdAt`, `$updatedAt`; `orderedColumns` keeps these first, then the rest
  alphabetically. `select` queries always append these so they survive
  projection.
- **Count uses `total`.** Appwrite list responses include a filter-aware `total`,
  so `CountDocuments` issues one `limit(1)` request and reads `total` (no full
  scan).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus database/collection/attributes/rawQueries.

## Appwrite API specifics (verify before changing)

- **Auth**: two headers — `X-Appwrite-Project: <projectId>` (jsonData) and
  `X-Appwrite-Key: <apiKey>` (secret). `X-Appwrite-Response-Format` is also sent
  to pin the document envelope. API key scopes needed: `databases.read`,
  `collections.read`, `attributes.read`, `documents.read`.
- **Endpoint** includes the `/v1` suffix, e.g. `https://cloud.appwrite.io/v1`,
  a regional cloud endpoint, or a self-hosted `https://<host>/v1`.
- **List documents**: `GET /databases/{db}/collections/{col}/documents?queries[]=…`
  returns `{total, documents:[{$id,$createdAt,$updatedAt,...attrs}]}`. Pagination
  is a **cursor** (`cursorAfter(<last $id>)`) combined with `limit(n)`; page size
  ≤ **100**.
- **Count**: no count endpoint — read `total` from a `limit(1)` list response
  (respects the filter `queries[]`).
- **Schema**: `GET /databases` lists databases (cursor paginated by `$id`);
  `GET /databases/{db}/collections` lists collections;
  `GET /databases/{db}/collections/{col}/attributes` lists attributes (offset
  paginated). Each returns `{total, <items>}` with `$id`/`name` (databases,
  collections) or `key`/`type` (attributes).
- **TablesDB fallback (important).** Appwrite has two database APIs: the legacy
  `/databases` and the newer **TablesDB** `/tablesdb`. Databases/collections/
  attributes created via TablesDB are **not** returned by the legacy list
  endpoints — they respond `200` with an empty array. So the metadata listers
  (`client.go::ListDatabases/ListCollections/ListAttributes`) try the legacy
  endpoint first and, when it yields nothing, fall back to the TablesDB
  equivalents: `/tablesdb`, `/tablesdb/{db}/tables` (key `tables`), and
  `/tablesdb/{db}/tables/{tbl}/columns` (key `columns`). Document reads still go
  through `/databases/{db}/collections/{col}/documents`, which resolves for both
  worlds, so the query path is unchanged. This is why a dropdown could appear
  empty while a manually-typed id works.
- **Health/Ping**: `GET /databases?queries[]=limit(1)` (needs project + key).
- **Attribute types**: `string`, `integer`, `double`, `boolean`, `datetime`,
  `email`, `url`, `ip`, `enum`, `relationship`. See `filter.ts::TYPE_CATEGORY`
  for the operator mapping.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (targets Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup` (not
  `Combobox`, which is unavailable on older Grafana). The `Select` deprecation
  and `react-hooks/set-state-in-effect` warnings are accepted (see
  `eslint.config.mjs`); lint must stay at **0 errors**.
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

## Verifying against live Appwrite

Appwrite Cloud is SaaS; `docker compose up` runs Grafana with the datasource
auto-provisioned against `https://cloud.appwrite.io/v1`:

```bash
APPWRITE_PROJECT_ID=... APPWRITE_API_KEY=... APPWRITE_DATABASE_ID=... docker compose up
```

Create an API key in the Appwrite console with the `databases.read`,
`collections.read`, `attributes.read` and `documents.read` scopes. Grafana runs
with anonymous admin, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{databases,collections,attributes}`
without auth to verify behavior end-to-end.
