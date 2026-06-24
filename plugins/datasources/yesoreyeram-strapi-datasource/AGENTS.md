# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Strapi](https://strapi.io).
Plugin id: `yesoreyeram-strapi-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Strapi REST
API, compiles filters, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getContentTypes/getFields)
  types.ts                StrapiQuery, StrapiDataSourceOptions, FieldInfo, ContentTypeInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      API URL, API Version (v4/v5), API Token, Default Content Type
    QueryEditor.tsx       Query type, Content Type (free-text), Fields, Populate, Filters, Sort, Page, PageSize
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/content-types /fields, degrade gracefully)
    client.go             Strapi HTTP client: bearer auth, ListRecords, CountRecords, ListContentTypes, ListFields, Ping
    models.go             Settings (baseURL + apiVersion + secret apiToken), QueryModel
    filter.go             BuildFilter: structured filter tree -> Strapi URL filter params; SortItem
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against a Strapi instance
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-strapi-datasource <script>`. Install deps with a
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
  Strapi URL query parameters (e.g. `filters[field][$eq]=value`).
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Strapi's
  returned order honours the query `sort`. Only columns are reordered (time fields
  first). Re-sorting rows would be a bug.
- **Handles BOTH Strapi v4 and v5 response shapes.** v4 nests fields under
  `attributes` (`{data:[{id, attributes:{…}}]}`); v5 is flat with a `documentId`
  (`{data:[{id, documentId, …fields}]}`). `frame.go::flattenRecord` lifts v4
  `attributes` to top-level columns and passes v5 records through, keeping `id`
  (and `documentId`). Detection is **per record** (presence of an `attributes`
  object), so a misconfigured `apiVersion` still works. Do not regress this.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/dateTime strings parse to UTC `*time.Time`
  fields. Nested relations/components/media/dynamic-zone values are
  JSON-serialised to strings.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus content type, fields, and populate.

## Strapi API specifics (verify before changing)

- **Auth**: a static API token sent as `Authorization: Bearer <token>`. Created
  in the Strapi admin under Settings > API Tokens (a **read-only** token is
  enough). The API token is for the **content API** (`/api/...`) only.
- **List records**: `GET /api/{pluralApiId}` returns
  `{data:[…], meta:{pagination:{page,pageSize,pageCount,total}}}`. The `data`
  element shape differs by version (see v4/v5 note above). Pagination is
  **page-based** (`pagination[page]=1&pagination[pageSize]=25`, pageSize max 100;
  offset mode `pagination[start]/[limit]` also exists). Params used: `fields[i]`,
  `filters[…]`, `sort[i]=field:asc`, `populate` (`*` or `populate[i]`),
  `pagination[page]/[pageSize]`.
- **Count**: `GET /api/{pluralApiId}?pagination[pageSize]=1` reads
  `meta.pagination.total`.
- **Schema discovery is NOT available with an API token.** The only schema
  endpoint, `GET /content-type-builder/content-types` (note: NOT under `/api`),
  requires an **admin JWT**, so it 401s with an API token. `ListContentTypes`/
  `ListFields` therefore **degrade gracefully** — the resource handlers in
  `datasource.go` return an empty list (never an error), and the QueryEditor uses
  free-text inputs (`allowCustomValue`) for content type and fields. Don't make
  the editor depend on discovery succeeding.
- **Health/Ping** (`client.go::Ping`): no universal ping. With a Default Content
  Type configured → `GET /api/{contentType}?pagination[pageSize]=1` (2xx ⇒
  healthy). Without one → a lightweight reachability check against `/api/` that
  only fails on 401/403 or network errors (a bad token can't always be detected
  on the bare root — documented).
- **Filter format**: URL query params like `filters[field][$op]=value`. List
  operators use **indexed arrays** (`filters[f][$in][0]=a&filters[f][$in][1]=b`),
  NOT comma-joined values. Logical groups use
  `filters[$and][0][a][$eq]=1&filters[$and][1][$or][0][b][$eq]=2`. Operator set
  in `filter.go::operatorMap` must stay in sync with `src/filter.ts`.

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

## Verifying against live Strapi

Strapi is self-hosted; `docker compose up` runs Grafana with the datasource
auto-provisioned against your Strapi instance:

```bash
STRAPI_URL=https://your-strapi.example.com STRAPI_API_TOKEN=your-token docker compose up
```

Create a token in the Strapi admin panel under Settings > API Tokens.
Grafana runs with anonymous admin, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{content-types,fields}` without auth
to verify behavior end-to-end.
