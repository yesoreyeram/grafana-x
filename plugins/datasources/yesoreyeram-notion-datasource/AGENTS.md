# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Notion](https://www.notion.so).
Plugin id: `yesoreyeram-notion-datasource`. The frontend (TypeScript/React) renders
the config and query editors; the Go backend talks to the Notion REST API, builds
filters, paginates, flattens typed page properties, and converts results into
Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getDatabases/getProperties)
  types.ts                NotionQuery, NotionDataSourceOptions, DatabaseInfo, PropertyInfo, enums
  sort.ts                 parseSort / serializeSort (`-field,field` <-> structured rows)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      API URL, Notion-Version, Integration Token, Default Database ID
    QueryEditor.tsx       Query type, Database, Properties, Filters, Sort, Limit
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/databases /properties)
    client.go             Notion HTTP client: ListRecords (cursor pagination), CountRecords, ListDatabases/ListProperties
    models.go             Settings (baseURL/notionVersion/token), QueryModel, LoadSettings/LoadQuery
    filter.go             BuildFilter: structured filter tree -> Notion JSON filter object
    frame.go              recordsToFrame / countToFrame + Notion page flattening; type inference, time parsing
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained, not the create-plugin .config indirection)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory** (`yarn` resolves the workspace
automatically), or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-notion-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `NOTION_API_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Filters are built server-side.** The query editor persists a JSON `filterTree`
  on the query; `pkg/plugin/filter.go::BuildFilter` compiles it into the Notion
  **JSON filter object** (`{and:[...]}` / `{or:[...]}`, property-type-specific
  conditions). Notion filters are JSON in the POST body, NOT a query string.
- **Pages are flattened.** Notion returns deeply-typed property objects;
  `frame.go::flattenPages`/`flattenProperty` reduces each property to a scalar
  before the shared frame builder runs. Add new property types there.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Notion's
  returned order honours the query `sorts`. Only columns are reordered (time
  fields first). Re-sorting rows would be a bug; never introduce it.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Date strings parse to UTC `*time.Time` fields.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value (list operators use `csv` formatting).
- **Notion API specifics learned the hard way** (verify before changing):
  - Every request needs the `Notion-Version` header and `Authorization: Bearer`.
  - There is **no count endpoint** — count is derived by paginating the query.
  - There is **no "view" concept** in the API — only databases and properties.
  - Pagination is **cursor-only** (`start_cursor`/`next_cursor`/`has_more`); there
    is no offset. `page_size` is capped at 100.
  - There is **no native list/`in` operator** — list operators expand into an
    or/and group of `equals`/`does_not_equal` conditions server-side.
  - Operators are property-type specific (e.g. `rich_text` uses `contains`,
    `number` uses `greater_than`, `date` uses `before`/`after`).

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (the plugin
  targets Grafana >= 10.4 / runs on 11.x). `Combobox` is NOT available on older
  Grafana — use `Select`/`MultiSelect`/`RadioButtonGroup`.
- Pure logic (sort, filter serialization, type inference, page flattening) lives
  in standalone, unit-tested modules — add tests there rather than only in
  components.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- Match existing code style; do not introduce new frameworks or build tooling.
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps are pinned to exact
  versions (no `^`/`~`). Keep them exact when adding/upgrading deps.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

A golden diff means frame behavior changed — confirm it is intended.

## Verifying against live Notion

There is no public Notion server image (it is hosted SaaS). To verify
end-to-end, create an internal integration at
https://www.notion.so/my-integrations, share a database with it, then run
`NOTION_API_TOKEN=secret_... docker compose up` (build `dist/` first). Grafana
runs with anonymous admin at http://localhost:3000, so you can hit `/api/ds/query`
and `/api/datasources/uid/<uid>/resources/{databases,properties}` without auth.
