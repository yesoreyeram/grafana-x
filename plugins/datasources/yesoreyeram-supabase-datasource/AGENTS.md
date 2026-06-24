# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Supabase](https://supabase.com).
Plugin id: `yesoreyeram-supabase-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Supabase
PostgREST API, compiles filters, paginates, and converts results into Grafana
data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getTables)
  types.ts                SupabaseQuery, SupabaseDataSourceOptions, TableInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      Project URL + Service Role Key (SecretInput) + Schema
    QueryEditor.tsx       Query type, Table, Select, Filters, Sort, Limit, Offset
    FilterEditor.tsx      Recursive filter/group builder
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/tables)
    client.go             Supabase HTTP client: dual auth (apikey + Bearer), Accept-Profile, ListRecords (limit/offset), CountRecords (HEAD + Content-Range), ListTables (OpenAPI)
    models.go             Settings (apiUrl + schema + secret serviceKey), QueryModel
    filter.go             BuildParams: structured filter tree -> PostgREST query params
    frame.go              recordsToFrame / countToFrame: type inference, time parsing
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against Supabase
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-supabase-datasource <script>`.

| Task                | Command |
| ------------------- | ------- |
| Install deps        | `yarn install` (run at monorepo root) |
| Build (front+back)  | `yarn build` (frontend + `mage buildAll`) |
| Frontend only       | `yarn build:frontend` |
| Backend only        | `yarn build:backend` |
| Frontend watch      | `yarn dev` |
| Typecheck           | `yarn typecheck` |
| Lint                | `yarn lint` |
| Frontend tests      | `yarn test` |
| Backend tests       | `go test ./pkg/...` |
| Local full stack    | `docker compose up` (run `yarn build` first) |

Before declaring work done, run: `go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Dual auth headers (do not remove either).** Every request includes BOTH
  `apikey: <key>` AND `Authorization: Bearer <key>` headers, set to the same key
  value. Supabase requires both. See `client.go::do`.
- **Filters are compiled server-side.** The query editor persists a JSON
  `filterTree` on the query; `pkg/plugin/filter.go::BuildParams` compiles it into
  PostgREST query parameters. **OR/`or=(...)` group elements MUST be
  field-qualified** (`status.eq.draft`, not `eq.draft`) — getting this wrong is
  the classic bug. Nested groups serialise inline as `and(...)`/`or(...)`; any
  operator can be negated with a `not.` prefix.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/dateTime strings parse to UTC `*time.Time`
  fields. Array/object cell values are JSON-serialised to strings.

## Supabase / PostgREST API specifics (verify before changing)

- **Auth**: dual-mode. `apikey` header identifies the project; `Authorization:
  Bearer <key>` carries the user's JWT, anon or service_role key. Both are set to
  the same value (the configured key). With the anon key, **RLS** applies; the
  service_role key bypasses RLS.
- **List records**: `GET /rest/v1/{table}` returns a JSON array. Params:
  `select=col1,col2`, `{column}={op}.{value}` (filters), `order=col.desc`,
  `limit`/`offset` (pagination). Treat **both 200 and 206** (ranged) as success.
- **Count**: `HEAD /rest/v1/{table}` with `Prefer: count=exact`; the total is the
  part after `/` in the `Content-Range` response header (`0-24/3573` → `3573`).
  See `client.go::CountRecords` / `parseContentRangeTotal`.
- **Filters/operators**: `eq,neq,gt,gte,lt,lte,like,ilike,match,imatch,in,is,cs,cd`
  (+ more). `like`/`ilike` accept `*` or `%`. `in.(a,b,c)` for lists; quote tokens
  with reserved chars (`,()`). `is.null|true|false|unknown`. Negate with `not.`.
  Top-level conditions are implicit AND; OR is `or=(a.op.v,b.op.v)`.
- **Schema discovery**: `GET /rest/v1/` returns the OpenAPI (Swagger 2.0)
  document; the plugin reads table/view names from `definitions` and `paths`
  (excluding `/rpc/`). Degrades gracefully (editor allows custom values).
- **Schema selection**: the optional `Accept-Profile: <schema>` header selects a
  non-default Postgres schema (config-level `schema` setting).
- **Health/Ping**: `GET /rest/v1/` validates connectivity + credentials.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (targets Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup` (not
  `Combobox`).
- Pure logic (sort, filter compilation/serialization, type inference) lives in
  standalone, unit-tested modules.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps pinned to exact versions
  (no `^`/`~`).

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```
