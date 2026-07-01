# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Todoist](https://todoist.com).
Plugin id: `yesoreyeram-todoist-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the **Todoist
unified API v1** (`https://api.todoist.com/api/v1`), follows the cursor
paginator, flattens nested task objects into scalars, and converts results into
Grafana data frames. Todoist is **cloud only** (hosted SaaS); there is no
local/self-hosted mode.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getProjects/getSections/getLabels)
  types.ts                TodoistQuery, TodoistDataSourceOptions, ProjectInfo/SectionInfo/LabelInfo, enums
  components/
    ConfigEditor.tsx      API Token (secret)
    QueryEditor.tsx       Query type; cascading project/section pickers; label picker (by name); parent task; filter + filter language; limit
  plugin.json             Plugin manifest (executable gpx_todoist)
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (tasks|count), CheckHealth (/projects), CallResource (/projects /sections /labels)
    client.go             v1 HTTP client: do(), cursor pagination (limit/cursor + next_cursor), /tasks vs /tasks/filter routing, resource queries
    models.go             Settings (baseURL + secret apiToken), QueryModel, LoadSettings/LoadQuery, query-type constants
    frame.go              recordsToFrame / countToFrame + task flattening (due/deadline/duration/labels); type inference, ISO-8601 time parsing
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-todoist-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `TODOIST_API_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Target the v1 unified API.** Base URL is `https://api.todoist.com/api/v1`.
  Do **not** revert to REST v2 (`/rest/v2`): REST v2's `/tasks` returns ALL
  active tasks in one un-paginated array and ignores `offset`/`limit`, so offset
  paging against it loops and duplicates rows.
- **Pagination is cursor-based.** v1 list endpoints return
  `{"results":[...],"next_cursor":"..."}`; pass `limit` (1..200, default 50; we
  use 200) and `cursor`. `listPaginated` follows `next_cursor` until it is
  null/empty or the requested limit is reached. `next_cursor` is `null` when
  exhausted.
- **All list endpoints paginate** — including `/projects`, `/sections` and
  `/labels`. The resource helpers (`listResource`) must parse the `results`
  envelope and follow the cursor, NOT a plain array.
- **Filtering is a separate endpoint.** `/api/v1/tasks` takes
  `project_id`/`section_id`/`label`/`parent_id`; `/api/v1/tasks/filter` takes
  `query` (+ optional `lang`). The `filter`/`lang` params were removed from
  `/tasks`. `taskPathParams` routes to `/tasks/filter` when `Filter` is set, and
  the id-based scope is ignored (the endpoints cannot be combined).
- **`label` filters by NAME, not ID.** The label picker stores the label name.
- **Auth is Bearer.** The API token is sent as `Authorization: Bearer <token>`.
  See `Settings.credential()`. The secret is `apiToken` in `secureJsonData`.
- **Errors are JSON.** v1 returns
  `{"error":"...","error_code":...,"error_tag":"...","http_code":...}` on non-2xx;
  `client.go::do` surfaces the `error` (or `error_tag`) message.
- **Active tasks only.** Both task endpoints return only active (incomplete)
  tasks. Completed tasks need `/api/v1/tasks/completed/by_completion_date` (not
  implemented). Document this; don't pretend completed tasks are included.
- **No count endpoint.** `CountRecords` paginates matching tasks and counts the
  `results` lengths without flattening; the optional Limit caps the scan.
- **v1 field names** (renamed from REST v2): `checked` (was `is_completed`),
  `added_at` (was `created_at`), `added_by_uid` (was `creator_id`),
  `responsible_uid` (was `assignee_id`), `assigned_by_uid` (was `assigner_id`),
  `child_order` (was `order`), `note_count` (was `comment_count`). The `url`
  field was removed.
- **`due` has no `datetime` field in v1.** The due object is
  `{date, timezone, string, lang, is_recurring}`; `date` holds a full-day date
  (`YYYY-MM-DD`), a floating datetime (`...THH:MM:SS`) or a fixed-timezone
  datetime (`...THH:MM:SSZ`). `flattenDue` emits `dueDate` (time), `dueString`,
  `dueIsRecurring` (bool), `dueTimezone`. `deadline` → `deadlineDate`;
  `duration` → `durationAmount`/`durationUnit`; `labels` → JSON string array.
- **Priority is inverted vs the UI.** API `priority: 4` is highest (UI p1);
  `priority: 1` is none.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows. Only
  columns are reordered (time fields first).
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Date strings parse to UTC `*time.Time` fields.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  the scope inputs (project/section/label/parent), the filter and the filter
  language.
- **Secrets stay on the server**: `apiToken` is `secureJsonData`; never log it or
  send it to the browser. Health check uses `/projects?limit=1`.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (target Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`Input`/`InlineField`, not `Combobox`.
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

## Verifying against live Todoist

There is no Todoist server image (it is hosted SaaS). To verify end-to-end, copy
your API token from **Todoist Settings → Integrations → Developer**, then run
`TODOIST_API_TOKEN=0123... docker compose up` (build `dist/` first). Grafana runs
with anonymous admin at http://localhost:3000, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{projects,sections,labels}` without auth.
