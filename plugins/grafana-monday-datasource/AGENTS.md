# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [monday.com](https://monday.com).
Plugin id: `yesoreyeram-monday-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to monday.com's
**GraphQL API**, paginates items (cursor-based) and boards/users/workspaces
(page-based), flattens nested nodes and item column values into scalars, and
converts results into Grafana data frames. monday.com is **cloud only** (hosted
SaaS); there is no local/self-hosted mode.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getBoards/getGroups/getColumns/getWorkspaces)
  types.ts                MondayQuery, MondayDataSourceOptions, BoardInfo, GroupInfo, ColumnInfo, WorkspaceInfo, enums
  components/
    ConfigEditor.tsx      GraphQL URL, API version, auth method (API token | OAuth), secret credential
    QueryEditor.tsx       Query type; items filters (boards/groups/columns/name/order/include-columns); boards filters (workspaces/state); state; limit; raw GraphQL
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData, CheckHealth (me), CallResource (/boards /groups /columns /workspaces)
    client.go             GraphQL client: do(), items_page/next_items_page pagination, page/limit pagination, ItemsQuery assembly, raw collection discovery, resource queries
    queries.go            Predefined GraphQL documents (items/boards/groups/users/workspaces/tags) + query type / state constants
    models.go             Settings (baseURL/authMethod/apiVersion + secret apiToken/oauthToken), QueryModel (incl. groupBy/aggregation), LoadSettings/LoadQuery
    aggregate.go          server-side `aggregate` query build + result parsing (group items by a column id; count/count_distinct/sum/avg/min/max)
    frame.go              recordsToFrame / countToFrame + item & node flattening; type inference, time parsing
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-monday-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `MONDAY_API_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **It is GraphQL, not REST.** A single endpoint (`/v2`) receives a POST with
  `{query, variables}`. GraphQL errors arrive with HTTP 200 in an `errors` array
  (and sometimes a top-level `error_message`) — `client.go::do` surfaces both as
  Go errors.
- **Auth header depends on method.** Personal API tokens are sent **raw** in the
  `Authorization` header (no `Bearer`). OAuth tokens are sent as
  `Authorization: Bearer`. An optional `API-Version` header is sent when
  configured. See `Settings.credential()` and `Client.do`.
- **Items live under boards.** The items query is
  `boards(ids){ items_page(limit, cursor, query_params){ cursor items {...} } }`.
  Subsequent pages use `next_items_page(cursor, limit)`. `listItems` collects the
  first page per board, then follows each board's cursor. Requires at least one
  board.
- **Item column values are flattened.** `frame.go::flattenItem` lifts each entry
  of `column_values` into a top-level column keyed by `column.title` (falling back
  to the column `id`), using the human-readable `text`. Collisions with core
  fields are suffixed ` (column)`. Toggle via `includeColumnValues` (default true).
  **Checkbox** columns (`type == "checkbox"`) are converted to booleans (`"v"` →
  true, empty → false). When `hideSystemColumns` is set, monday system columns
  (see `systemColumnTypes` / `isSystemColumn`) are skipped.
- **Column selection is server-side.** The `Columns` selector's ids are spliced
  into the query via `column_values(ids: [...])` in `queries.go::buildItemsQuery`
  / `buildNextItemsQuery` (the column `type` is always selected for conversion).
  Empty selection returns all columns.
- **Other nodes use generic flattening.** Boards/users/workspaces/groups/tags and
  raw nodes go through `flattenNode`/`flattenValue`/`flattenObject`, reducing a
  nested object to name → title → text → email → id and joining arrays of named
  objects into a comma string. Extend there.
- **ItemsQuery is assembled server-side.** `buildItemsQueryParams` builds the
  monday `query_params`: a `contains_text` rule for the name search, an `any_of`
  rule for group IDs, and `order_by` from the order column + direction.
- **Grouping/aggregation is server-side via monday's `aggregate` query.** When
  `QueryModel.GroupBy` is set (a board **column id**), `client.go::listAggregate`
  builds the `aggregate(query: { from: {type: TABLE, id}, select, group_by, query })`
  document in `aggregate.go::buildAggregateQuery` and parses it with
  `parseAggregateResults`. Functions map count→`COUNT_ITEMS`,
  count_distinct→`COUNT_DISTINCT`, sum→`SUM`, avg→`AVERAGE`, min→`MIN`,
  max→`MAX`; column-based functions require `AggregationColumn`. **The group
  column's `as` in `select` MUST equal its `column_id`** (not a custom alias) so
  monday ties the selected column to the `group_by` clause — otherwise it returns
  a single ungrouped result with a null group value. The result entry uses the
  fixed `result_value` alias; entries are matched by alias name (monday returns
  them alphabetically). `aggregate` takes ONE board, so one call runs per board
  (a `board_id` column is added when multiple boards). Group values are returned
  raw (status colour hex, person id, etc.) — no resolution. Name/group filters
  are pushed down as the aggregate `query` rules. Result column is named e.g.
  `count`, `sum(numbers)`.
- **`aggregate` is version-gated.** It only exists on recent monday API versions.
  Requests send `Client.effectiveAPIVersion()` (the configured `apiVersion`, else
  `defaultAPIVersion` = `2026-01`). When monday rejects the field/types
  (`isAggregateUnsupportedError`: "Cannot query field \"aggregate\"" / "Unknown
  type \"Aggregate…\""), `listAggregate` wraps it in an actionable error telling
  the user to set a newer API version or remove the Group by. There is **no
  client-side aggregation fallback** — grouping requires the API.
- **Pagination differs by type.** Items: cursor-based. Boards/users/workspaces:
  page/limit (page starts at 1; stop when a short page is returned, or for boards
  when explicit ids are supplied). Groups/tags: single request.
- **Raw queries** (`queryType == "raw"`) run the user's document; `findCollection`
  returns the **deepest** array of objects (so `boards[].items_page.items` wins
  over the outer `boards`), then flattens it. If none, the top-level object
  becomes one row.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — monday's
  returned order honours the query. Only columns are reordered (time fields
  first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Date/time strings parse to UTC `*time.Time`
  fields.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  the list inputs (boardIds/groupIds/workspaceIds/columnIds) and the scalar
  search/order/raw inputs.
- **Secrets stay on the server**: `apiToken`/`oauthToken` are `secureJsonData`;
  never log them or send them to the browser.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (target Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup`/
  `TextArea`/`Input`, not `Combobox`.
- Pure logic (flattening, ItemsQuery assembly, type inference) lives in
  standalone, unit-tested Go modules — add tests there.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- Match existing code style; do not introduce new frameworks or build tooling.
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps are pinned to exact
  versions (no `^`/`~`).

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

A golden diff means frame behavior changed — confirm it is intended.

## Verifying against live monday.com

There is no monday.com server image (it is hosted SaaS). To verify end-to-end,
copy your personal API token from **avatar menu → Developers → My Access Tokens**,
then run `MONDAY_API_TOKEN=... docker compose up` (build `dist/` first). Grafana
runs with anonymous admin at http://localhost:3000, so you can hit `/api/ds/query`
and `/api/datasources/uid/<uid>/resources/{boards,workspaces}` without auth.
