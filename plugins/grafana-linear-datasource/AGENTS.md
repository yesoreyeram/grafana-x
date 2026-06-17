# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Linear](https://linear.app).
Plugin id: `yesoreyeram-linear-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to Linear's **GraphQL
API**, paginates connections, flattens nested nodes into scalars, and converts
results into Grafana data frames. Linear is **cloud only** (hosted SaaS); there
is no local/self-hosted mode.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getTeams/getStates/getLabels/getProjects/getUsers/getIssueFields)
  types.ts                LinearQuery, LinearDataSourceOptions, TeamInfo, StateInfo, LabelInfo, ProjectInfo, UserInfo, enums
  components/
    ConfigEditor.tsx      GraphQL URL, auth method (API key | OAuth), secret credential
    QueryEditor.tsx       Query type; multi-value issue filters (states/assignees/labels/priorities/projects), team/creator/title/date-range/archived; fields selector; raw GraphQL; order by; limit
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData, CheckHealth (viewer), CallResource (/teams /states /labels /projects /users /issuefields)
    client.go             GraphQL client: do(), connection pagination, raw query node discovery, multi-value filter assembly, resource queries
    queries.go            Predefined GraphQL documents; dynamic issue field selection (issueFieldSelections catalog) + static projects/teams/users/cycles
    models.go             Settings (baseURL/authMethod + secret apiKey/oauthToken), QueryModel, LoadSettings/LoadQuery
    frame.go              recordsToFrame / countToFrame + node flattening; type inference, time parsing
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (key from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-linear-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `LINEAR_API_KEY`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **It is GraphQL, not REST.** A single endpoint (`/graphql`) receives a POST with
  `{query, variables}`. GraphQL errors arrive with HTTP 200 in an `errors` array —
  `client.go::do` surfaces them as Go errors.
- **Auth header depends on method.** Personal API keys are sent **raw** in the
  `Authorization` header (no `Bearer`). OAuth tokens are sent as
  `Authorization: Bearer`. See `Settings.credential()`.
- **Predefined queries** live in `queries.go`; each exposes `$first/$after/$orderBy`
  and (for issues/cycles) `$filter` (issues also `$includeArchived`). Filters are
  assembled in `buildFilter`/`buildIssueFilter`. Multi-value inputs become `in`
  conditions; assignee/creator become `or` groups (email-or-name).
- **Date filters have a mode** (`createdMode`/`updatedMode`): `any` (no filter),
  `dashboard` (uses `QueryModel.TimeRange`, populated from `query.TimeRange` in
  `datasource.go`), or `custom` (the after/before bounds). `dateModeFilter`
  produces the `gte`/`lte` `DateComparator`; dashboard bounds are RFC3339 UTC.
- **Issue field selection is dynamic.** `buildIssuesQuery` assembles the selection
  set from the `issueFieldSelections` catalog based on the query's `fields`
  (empty = `defaultIssueFields`; `id` is always included). To add a selectable
  field, add it to the catalog — the editor's Fields list is served from
  `IssueFieldNames()` via `/issuefields`.
- **Raw queries** (`queryType == "raw"`) run the user's document and the backend
  finds the first connection (object with a `nodes` array) anywhere in the
  response via `findNodes`, then flattens it. If none, the top-level object
  becomes one row.
- **Nodes are flattened.** Linear returns nested objects; `frame.go::flattenNode`/
  `flattenValue`/`flattenObject` reduce each relation to a scalar (name → key →
  displayName → identifier → email → number) and join sub-connections
  (`{nodes:[...]}` of named objects) into a comma string. Extend there.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Linear's
  returned order honours `orderBy`. Only columns are reordered (time fields
  first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Date/time strings parse to UTC `*time.Time`
  fields.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  the scalar filter inputs and the raw query/variables.
- **Secrets stay on the server**: `apiKey`/`oauthToken` are `secureJsonData`;
  never log them or send them to the browser.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (target Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`RadioButtonGroup`/`TextArea`/`Input`,
  not `Combobox`.
- Pure logic (flattening, filter assembly, type inference) lives in standalone,
  unit-tested Go modules — add tests there.
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

## Verifying against live Linear

There is no Linear server image (it is hosted SaaS). To verify end-to-end, create
a personal API key at **Settings → Security & access → Personal API keys**, then
run `LINEAR_API_KEY=lin_api_... docker compose up` (build `dist/` first). Grafana
runs with anonymous admin at http://localhost:3000, so you can hit `/api/ds/query`
and `/api/datasources/uid/<uid>/resources/{teams,states}` without auth.
