# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [PocketBase](https://pocketbase.io).
Plugin id: `yesoreyeram-pocketbase-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend authenticates to PocketBase
(superuser/user/token), talks to the REST Records API, compiles filters into a
filter expression, paginates, and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getCollections/getFields)
  types.ts                PocketBaseQuery, PocketBaseDataSourceOptions, AuthMode, CollectionInfo, FieldInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      URL, Auth mode, Identity, Auth collection, Password (secret), Auth token (secret)
    QueryEditor.tsx       Query type, Collection, Fields, Filters, Raw filter, Sort, Limit
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/collections /fields)
    client.go             PocketBase HTTP client: auth (superuser/user/token), token cache + 401 retry, ListRecords, CountRecords, ListCollections, CollectionFields
    models.go             Settings (url/authMode/identity/authCollection + secret password/authToken), QueryModel
    filter.go             BuildFilter: structured filter tree -> PocketBase filter expression; sortParam; fieldsParam; SortItem
    frame.go              recordsToFrame / countToFrame: type inference, time parsing, data-plane contracts
scripts/seed.mjs          Idempotent PocketBase seeder for the local Docker stack
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       PocketBase + seed + Grafana (anonymous admin)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-pocketbase-datasource <script>`. Install deps with a
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
  a single PocketBase **filter expression** string (e.g.
  `status = 'active' && (total > 10 || urgent = true)`) passed as the `filter`
  query parameter. A raw `rawFilter` block on the query takes precedence when set
  (`client.go::baseFilter`).
- **PocketBase filter wire format.** Conditions render as `field OP value`; string
  values are single-quoted (with `'`/`\` escaped), numbers/bools/`null` are raw
  literals. `~`/`!~` (contains) always quote the value. Nested groups are wrapped
  in parentheses; the connector is `&&` / `||`.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — PocketBase's
  returned order honours the `sort` parameter (`-field,field`) or the default
  order. Only columns are reordered (time fields first). Re-sorting rows is a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). datetime strings parse to UTC `*time.Time` fields.
  Array/object cell values (multi-relations, multi-selects, file lists, json,
  expanded relations) are JSON-serialised to strings.
- **Identity columns.** Each record keeps the system fields `id`, `created`,
  `updated`; `orderedColumns` keeps these first (when present), then the rest
  alphabetically. `fields` projection always appends `id,created,updated` unless
  `hideSystemFields` is set.
- **Count uses `totalItems`.** PocketBase list responses include a filter-aware
  `totalItems`, so `CountRecords` issues one `perPage=1` request (NOT skipping the
  total) and reads `totalItems`.
- **Auth token is cached.** `client.go` mints a token via `auth-with-password`
  (superuser/user) or uses the static token (token mode), caches it, and
  transparently re-authenticates once on a `401` for password modes.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus collectionId/rawFilter/fields.

## PocketBase API specifics (verify before changing)

- **Auth**: there are no static API keys. Modes:
  - `superuser` — `POST /api/collections/_superusers/auth-with-password`
    `{identity,password}` → `{token}`. Superusers can list collections and read
    all records regardless of API rules.
  - `user` — same against a regular auth collection (default `users`); access is
    constrained by each collection's API rules.
  - `token` — a pre-issued token used verbatim.
  The token is sent **raw** in the `Authorization` header (no `Bearer` scheme),
  matching the official SDKs.
- **List records**: `GET /api/collections/{idOrName}/records?page=&perPage=&filter=&sort=&fields=&skipTotal=1`
  returns `{page,perPage,totalItems,totalPages,items:[{id,collectionId,collectionName,created,updated,...fields}]}`.
  Pagination is **offset** (`page`/`perPage`, page size ≤ **500**; this client uses
  200 and sets `skipTotal=1` while listing).
- **Count**: no count endpoint — read `totalItems` from a `perPage=1` list
  response (respects the `filter`). Do NOT set `skipTotal` for a count.
- **filter syntax**: `field OPERATOR value`, operators `= != > >= < <= ~ !~`
  (and `?`-prefixed any-of variants, not exposed here). Combine with `&&`, `||`
  and parentheses. `~`/`!~` auto-wrap the operand with `%` wildcards. `null`,
  `true`, `false` are literals.
- **Schema**: `GET /api/collections` lists collections (superuser only; paginated)
  and each item carries its `fields` array (PocketBase >= 0.23) or `schema`
  (older). `GET /api/collections/{idOrName}` returns one collection. System
  collections (`system:true`, e.g. `_superusers`) are filtered out; `hidden` and
  `password` fields are dropped from the field picker.
- **System fields**: every record has `id`, `collectionId`, `collectionName`;
  base collections also have `created`/`updated` autodates **only when defined**
  (PocketBase >= 0.23 does not auto-add them). datetimes render with a **space**
  separator in UTC, e.g. `2022-06-25 11:03:50.052Z` — see `frame.go::timeLayouts`.
- **Health/Ping**: `GET /api/health` is public (connectivity); credentials are
  validated by authenticating (password modes).
- **Field types**: `text`, `editor`, `email`, `url`, `number`, `bool`, `date`,
  `autodate`, `select`, `relation`, `file`, `json`, `geoPoint`, `password`
  (hidden). See `filter.ts::TYPE_CATEGORY` for the operator mapping.

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

## Verifying against live PocketBase

`docker compose up` runs a real PocketBase, creates a superuser
(`admin@example.com` / `Password123!`), seeds `demo` and `metrics` collections,
and auto-provisions the datasource (superuser auth). Grafana runs with anonymous
admin at http://localhost:3000, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{collections,fields}` without auth to
verify behavior end-to-end. PocketBase is at http://localhost:8090 (admin UI at
`/_/`).
