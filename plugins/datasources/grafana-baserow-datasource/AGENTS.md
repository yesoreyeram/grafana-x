# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Baserow](https://baserow.io).
Plugin id: `yesoreyeram-baserow-datasource`. The frontend (TypeScript/React) renders
the config and query editors; the Go backend talks to the Baserow REST API, builds
filters, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getTables/getFields/getViews)
  types.ts                BaserowQuery, BaserowDataSourceOptions, FieldInfo, TableInfo, ViewInfo, enums
  sort.ts                 parseSort / serializeSort (Baserow `-field,field` order_by <-> structured rows)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      Platform, Base URL, Auth mode (token | email+password), credentials, Database ID
    QueryEditor.tsx       Query type, [Database picker], Table+View, Fields, Filters, Sort, Limit
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/databases /tables /fields /views)
    client.go             Baserow HTTP client: auth (token | JWT), ListRecords, CountRecords, ListTables/Fields/Views, ListDatabases
    models.go             Settings (platform/baseURL/authMode/databaseId/email/token/password), QueryModel
    filter.go             BuildFilters: structured filter tree -> Baserow `filters` JSON tree
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Baserow + Grafana (anonymous admin); no sample-data seeding
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained, not the create-plugin .config indirection)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory** (`yarn` resolves the workspace
automatically), or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-baserow-datasource <script>`. Install deps with a
single `yarn install` at the monorepo root — there is no per-plugin install.

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

- **Filters are built server-side.** The query editor persists a JSON `filterTree`
  on the query; `pkg/plugin/filter.go::BuildFilters` compiles it into the Baserow
  `filters` JSON tree (`{filter_type, filters:[{field,type,value}], groups:[...]}`)
  passed as the `filters` query parameter. Rows are requested with
  `user_field_names=true`, so `field` is the field name.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Baserow's
  returned order honours the query `order_by`. Only columns are reordered (time
  fields first). Re-sorting rows is a known past bug; never reintroduce it.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Date/DateTime strings parse to UTC `*time.Time`
  fields.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value (list operators use `csv` formatting).

## Baserow API specifics (verify before changing)

- **Base URL must hit the API, not the web app.** If Baserow returns HTML
  ("Site not found"), the request reached the web frontend. For the self-hosted
  all-in-one image, the host Grafana uses must be registered via
  `BASEROW_PUBLIC_URL`/`BASEROW_EXTRA_PUBLIC_URLS` (the compose sets
  `BASEROW_EXTRA_PUBLIC_URLS=http://baserow`). `client.go::htmlResponseHint`
  detects this and returns an actionable error.
- **Two auth modes** (`authMode` in jsonData) — and they hit DIFFERENT endpoints
  because Baserow's database token is only accepted on a subset of routes
  (verified against the OpenAPI `security` blocks):
  - `token` (default): a **database token**, `Authorization: Token <token>`.
    - Tables: `GET /api/database/tables/all-tables/` (token-aware; filtered by the
      optional Database ID client-side). The per-database
      `/tables/database/{id}/` endpoint **rejects** tokens (JWT only) — don't use
      it here. This was the cause of the original 401 on connect.
    - Health/Ping: `GET /api/database/tokens/check/` (the only token-aware,
      table-agnostic endpoint). Database ID is **not** required in token mode.
    - Views: not available (endpoint rejects tokens) → `ListViews` returns empty.
    - Rows/fields/count: `rows/table/{id}/` and `fields/table/{id}/` accept tokens.
  - `password`: email + password exchanged at `POST /api/user/token-auth/` for a
    **JWT**, `Authorization: JWT <jwt>`. The client caches the JWT and on a `401`
    re-authenticates and retries once (`doWithRetry`). Uses the per-database
    `/tables/database/{id}/` (Database ID required) and can enumerate
    workspaces/databases (`/databases` resource → `ListDatabases`), so the query
    editor shows a Database picker when no fixed Database ID is set.
- List rows: `GET /api/database/rows/table/{table_id}/?user_field_names=true`
  returns `{count, next, previous, results}`; pagination via `page` + `size`
  (max **200**).
- Field selection uses `include=Field1,Field2` (user field names).
- Sort uses `order_by=field,-field2` with user field names.
- **No dedicated count endpoint** — Count reads the `count` field from a
  `size=1` row request (respects `filters`). No group-by/aggregation endpoint;
  aggregate in Grafana via Transformations.
- Boolean filters use the `boolean` filter type with value `true`/`false`.

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

## Verifying against live Baserow

`docker compose up` runs an **empty** self-hosted Baserow at
http://localhost:8980 (no sample-data seeding) plus Grafana. Create a database,
table and a database token (or use email/password) in the Baserow UI, then export
credentials to auto-provision the datasource:

```bash
BASEROW_API_TOKEN=... BASEROW_DATABASE_ID=1 docker compose up
# or: BASEROW_AUTH_MODE=password BASEROW_EMAIL=... BASEROW_PASSWORD=... docker compose up
```

Grafana runs with anonymous admin, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{databases,tables,fields,views}` without
auth to verify behavior end-to-end. Provisioning is
`provisioning/datasources/baserow.yaml` (env-interpolated).

To test against **Baserow Cloud** instead, configure a datasource with
Platform = cloud (forces `https://api.baserow.io`) — the backend ignores the
configured Base URL for cloud (`Settings.Platform == "cloud"` in models.go).
