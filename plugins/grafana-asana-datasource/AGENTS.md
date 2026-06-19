# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Asana](https://asana.com).
Plugin id: `yesoreyeram-asana-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to Asana's **REST API
v1.0**, follows the cursor paginator, flattens nested objects into scalars, and
converts results into Grafana data frames. Asana is **cloud only** (hosted SaaS);
there is no local/self-hosted mode.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getWorkspaces/getTeams/getProjects/getSections/getUsers/getTags/getTaskFields)
  types.ts                AsanaQuery, AsanaDataSourceOptions, AsanaResource, enums
  components/
    ConfigEditor.tsx      API URL + Personal Access Token (secret)
    QueryEditor.tsx       Query type; cascading workspace/team/project/section pickers; assignee picker; incomplete-only; modified date mode; fields selector; archived (projects); limit; raw REST path + response key
  plugin.json             Plugin manifest (executable gpx_asana)
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData, CheckHealth (/users/me), CallResource (/workspaces /teams /projects /sections /users /tags /taskfields)
    client.go             REST client: do(), cursor pagination (limit/offset + next_page.offset), task scope selection, query-param assembly, resource queries, raw path flattening
    queries.go            Query type constants, task field catalog, opt_fields mapping, nonEmpty helper
    models.go             Settings (baseURL + secret apiKey), QueryModel, LoadSettings/LoadQuery
    frame.go              recordsToFrame / countToFrame + entity flattening; type inference, ISO-8601 time parsing, field selection
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-asana-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `ASANA_API_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **It is REST.** Each query is a GET against a path under the API root
  (`https://app.asana.com/api/1.0`, paths like `/tasks`, `/projects`). Asana
  errors arrive as `{"errors":[{"message":"...","help":"..."}]}` —
  `client.go::do` surfaces the first message as a Go error on non-2xx.
- **Auth is always Bearer.** Personal access tokens and OAuth tokens are both
  sent as `Authorization: Bearer <token>`. See `Settings.credential()`. The
  secret is `apiKey` in `secureJsonData`.
- **Responses wrap results under `data`.** List endpoints return
  `{"data":[...],"next_page":{...}}`; single resources return `{"data":{...}}`.
- **Pagination is cursor-based.** `limit` (max 100) + `offset` token; the loop
  follows `next_page.offset` until it is null/empty or the requested limit is
  reached (`listPaginated`).
- **Task scope is exclusive.** Asana's `/tasks` needs exactly one of: a
  `section`, a `project`, or `assignee` + `workspace`. `listTasks` picks the most
  specific in that order; otherwise it errors.
- **Field selection is server-side via `opt_fields`.** Asana returns compact
  records (gid, name, resource_type) unless `opt_fields` is set. `taskOptFields`
  maps friendly names to opt_fields paths (e.g. `assignee` → `assignee.name`,
  `projects` → `projects.name`) and defaults to the full catalog. The frame
  builder additionally honours `QueryModel.Fields` via `selectColumns`.
- **Date filter has a mode** (`modifiedMode`): `any` (no filter), `dashboard`
  (uses `QueryModel.TimeRange.From`, populated from `query.TimeRange` in
  `datasource.go`), or `custom`. `addModifiedSince` emits an ISO-8601
  `modified_since`; custom bounds are normalised by `toISO`.
- **Dates are ISO-8601 strings.** `frame.go::toColumnTime` parses string values
  with the layouts in `timeLayouts` (RFC3339, date-only, …). Numeric ids/counts
  stay numeric/string — only strings are considered for time.
- **Entities are flattened.** `frame.go::flattenItem` maps known relations
  (`assignee`/`parent`/`workspace`/`owner`/`team` → name,
  `current_status` → text, `projects`/`tags`/`followers`/`members` → joined
  names) and falls back to `flattenValue`/`flattenObject` (picks
  name/title/text/username/id) for everything else. Extend there.
- **Custom fields are expanded into columns.** `flattenItem` defers
  `custom_fields` and `addCustomFields` turns each into its own column keyed by
  the field name, with the typed value (number/text/enum/multi_enum/date/people)
  and a `display_value` fallback; collisions are suffixed `(2)`. They are
  requested via `taskCustomFieldOptFields` (part of the default task
  `opt_fields`), so they appear out of the box. Because custom-field column names
  are dynamic, `datasource.go` builds the task frame with `recordsToFrame(..., nil)`
  — Asana `opt_fields` is the authoritative column selector, not client-side
  `selectColumns`.
- **Raw queries** (`queryType == "raw"`) GET the user's path; if `RawRoot` is set
  that key's value is flattened, else `findArray` locates the first array of
  objects anywhere in the response (typically `data`); otherwise the single
  object under `data` (or the whole body) becomes one row.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows. Only
  columns are reordered (time fields first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  the scalar scope inputs, the modified bound, and the raw path/root.
- **Secrets stay on the server**: `apiKey` is `secureJsonData`; never log it or
  send it to the browser.

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

## Verifying against live Asana

There is no Asana server image (it is hosted SaaS). To verify end-to-end, create
a personal access token at **My Settings → Apps → Developer apps**, then run
`ASANA_API_TOKEN=1/123:abc... docker compose up` (build `dist/` first). Grafana
runs with anonymous admin at http://localhost:3000, so you can hit `/api/ds/query`
and `/api/datasources/uid/<uid>/resources/{workspaces,teams,projects,sections,users,tags}`
without auth.
