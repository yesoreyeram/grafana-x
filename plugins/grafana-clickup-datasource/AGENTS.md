# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [ClickUp](https://clickup.com).
Plugin id: `yesoreyeram-clickup-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to ClickUp's **REST
API v2**, paginates task pages, flattens nested objects into scalars, and converts
results into Grafana data frames. ClickUp is **cloud only** (hosted SaaS); there
is no local/self-hosted mode.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getTeams/getSpaces/getFolders/getLists/getMembers/getTaskFields)
  types.ts                ClickUpQuery, ClickUpDataSourceOptions, TeamInfo, SpaceInfo, FolderInfo, ClickUpListInfo, MemberInfo, enums
  components/
    ConfigEditor.tsx      API URL, auth method (personal token | OAuth), secret credential
    QueryEditor.tsx       Query type; cascading workspace/space/folder/list pickers; multi-value task filters (statuses/assignees/tags); created/updated/due date modes; include closed/subtasks/archived; fields selector; order by; reverse; limit; raw REST path + response key
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData, CheckHealth (/v2/user), CallResource (/teams /spaces /folders /lists /members /taskfields)
    client.go             REST client: do(), task pagination, endpoint selection, query-param filter assembly, hierarchy + resource queries, raw path flattening
    queries.go            Query type constants, task field catalog, order-by normalization, nonEmpty helper
    models.go             Settings (baseURL/authMethod + secret apiKey/oauthToken), QueryModel, LoadSettings/LoadQuery
    frame.go              recordsToFrame / countToFrame + task flattening; type inference, Unix-millis time parsing, field selection
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-clickup-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `CLICKUP_API_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **It is REST, not GraphQL.** Each query is a GET against a versioned path under
  the API root (`https://api.clickup.com/api`, paths like `/v2/team/{id}/task`).
  ClickUp errors arrive as `{"err":"...","ECODE":"..."}` — `client.go::do`
  surfaces them as Go errors (on non-2xx, and on a 2xx body that carries `err`).
- **Auth header depends on method.** Personal tokens (`pk_...`) are sent **raw**
  in the `Authorization` header. OAuth tokens are sent as `Authorization: Bearer`.
  See `Settings.credential()`.
- **Hierarchy.** Workspace (team) → Space → Folder → List → Task. The editor
  pickers cascade; selecting a level scopes the query and refreshes the level
  below it.
- **Task endpoint selection.** When `ListId` is set, tasks are read from
  `/v2/list/{list_id}/task`. Otherwise from `/v2/team/{team_id}/task` (Filtered
  Team Tasks) scoped by `space_ids[]` / `project_ids[]` (folder) / `list_ids[]`.
  `listTasks` requires a Workspace or List.
- **Pagination is page-based.** `page` query param, 100 tasks per page; loop stops
  on `last_page`, a short page, an empty page, or the requested limit.
- **Date filters have a mode** (`createdMode`/`updatedMode`/`dueMode`): `any`
  (no filter), `dashboard` (uses `QueryModel.TimeRange`, populated from
  `query.TimeRange` in `datasource.go`), or `custom`. `addDateMode` emits
  `*_gt`/`*_lt` **Unix-millisecond** bounds; custom bounds accept ISO-8601 or
  raw millis (`toUnixMillis`).
- **Dates are Unix millis.** The `dateKeys` set marks columns whose string values
  are epoch milliseconds; `toColumnTime` parses them to UTC time fields. Don't
  treat arbitrary numbers as time.
- **Tasks are flattened.** `frame.go::flattenTask` maps known relations
  (`status`→name (+`status_type`), `priority`→name, `creator`→username,
  `assignees`/`watchers`/`tags`→joined names, `list`/`folder`/`space`→name) and
  falls back to `flattenValue`/`flattenObject` for everything else. Extend there.
- **Field selection** is applied at frame build time (`selectColumns`), driven by
  `QueryModel.Fields`; the catalog is `TaskFieldNames()` served via `/taskfields`.
- **Raw queries** (`queryType == "raw"`) GET the user's path; if `RawRoot` is set
  that key's value is flattened, else `findArray` locates the first array of
  objects anywhere in the response; otherwise the top-level object becomes one row.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — ClickUp's
  returned order honours `order_by`. Only columns are reordered (time fields
  first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  the scalar scope/filter inputs, the multi-value lists, and the raw path/root.
- **Secrets stay on the server**: `apiKey`/`oauthToken` are `secureJsonData`;
  never log them or send them to the browser.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (target Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup`/
  `TextArea`/`Input`/`InlineSwitch`, not `Combobox`.
- Pure logic (flattening, param assembly, type inference) lives in standalone,
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

## Verifying against live ClickUp

There is no ClickUp server image (it is hosted SaaS). To verify end-to-end,
generate a personal token at **Settings → Apps → API Token**, then run
`CLICKUP_API_TOKEN=pk_... docker compose up` (build `dist/` first). Grafana runs
with anonymous admin at http://localhost:3000, so you can hit `/api/ds/query`
and `/api/datasources/uid/<uid>/resources/{teams,spaces,folders,lists,members}`
without auth.
