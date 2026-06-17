# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Plane](https://plane.so).
Plugin id: `yesoreyeram-plane-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to Plane's **REST
API v1**, follows cursor pagination, flattens nested objects into scalars, and
converts results into Grafana data frames. The API URL is configurable, so the
plugin works against **Plane Cloud and self-hosted** instances.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getProjects/getStates/getLabels/getMembers/getWorkItemFields/getPriorities)
  types.ts                PlaneQuery, PlaneDataSourceOptions, ProjectInfo/StateInfo/LabelInfo/MemberInfo, enums
  components/
    ConfigEditor.tsx      API URL, workspace slug, auth method (api key | OAuth), secret credential
    QueryEditor.tsx       Query type; workspace input + project picker; multi-value priority/state/assignee/label filters; created/updated date modes; expand; fields; order by; limit; raw REST path + response key
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData, CheckHealth (/api/v1/users/me/), CallResource (/projects /states /labels /members /workitemfields /priorities)
    client.go             REST client: do(), cursor pagination (listPaged/listPagedFiltered), endpoint selection, backend work item filtering (workItemFilter), resource queries, raw path flattening
    queries.go            Query type constants, work item field catalog, priority options, order-by normalization, nonEmpty helper
    models.go             Settings (baseURL/workspaceSlug/authMethod + secret apiKey/oauthToken), QueryModel, LoadSettings/LoadQuery, resolveWorkspace
    frame.go              recordsToFrame / countToFrame + entity flattening; type inference, RFC3339 time parsing, field selection
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (key + slug from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-plane-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `PLANE_API_KEY`/`PLANE_WORKSPACE_SLUG`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **It is REST, not GraphQL.** Each query is a GET against a versioned path under
  the API root (`https://api.plane.so`, paths like
  `/api/v1/workspaces/{slug}/projects/{id}/work-items/`). Plane errors arrive as
  `{"error":...}` / `{"detail":...}` / `{"message":...}` — `client.go::do`
  surfaces any present message on non-2xx.
- **Auth header depends on method.** API keys are sent as the **`X-API-Key`**
  header. OAuth tokens are sent as `Authorization: Bearer`. See
  `Settings.credential()` (returns `(token, bearer)`).
- **Hierarchy.** Workspace (slug) → Project (UUID) → Work item. The workspace is
  a free-text slug (with a data source default fallback via
  `QueryModel.resolveWorkspace`); the project is a live picker.
- **Endpoint selection** is per query type in `client.go::ListRecords`. Work
  items / states / labels / cycles / modules are project-scoped; projects and
  members are workspace-scoped.
- **Pagination is cursor-based.** `listPaged` sends `per_page=100` and follows
  `next_cursor` while `next_page_results` is true, stopping at the requested
  limit, an empty page, or no cursor. Plane's envelope is
  `{results, next_cursor, next_page_results, ...}`.
- **Work item filters are applied in the backend, NOT via query params.** Plane's
  List Work Items endpoint (`/work-items/`) does **not** support filtering query
  parameters — it silently ignores unknown params and returns the full list. So
  `workItemParams` sends only `order_by` / `expand` (+ `per_page`), and
  `workItemFilter` (in `client.go`) filters the raw items in `listPagedFiltered`
  before flattening. Do NOT "restore" `priority`/`state`/`assignees`/`labels` as
  query params — they do nothing. Matching is OR within a group, AND across
  groups, against the raw API values (works whether relations are expanded
  objects or bare UUID strings). The `Limit` applies AFTER filtering.
- **Date filters have a mode** (`createdMode`/`updatedMode`): `any` (no filter),
  `dashboard` (uses `QueryModel.TimeRange`, populated from `query.TimeRange` in
  `datasource.go`), or `custom`. `resolveDateBounds` turns the mode into
  `[from, to]` time bounds and `withinBounds` matches each item's
  `created_at`/`updated_at` — also a backend filter, not a query param.
- **Dates are ISO-8601 strings.** The `dateKeys` set marks columns whose string
  values are RFC3339 timestamps or `YYYY-MM-DD` dates; `toColumnTime` parses them
  to UTC time fields. Plane does NOT use Unix millis (unlike some other trackers).
- **Entities are flattened.** `frame.go::flattenEntity` maps known relations
  (`state`→name (+`state_group`), people fields `created_by`/`updated_by`/etc.→
  display_name/email, `project`/`parent`/`cycle`/`module`→name,
  `assignees`/`labels`/…→joined names or ids) and falls back to
  `flattenValue`/`flattenObject` for everything else. Relations may be **expanded
  objects** or **bare UUID strings** depending on `expand`; both must work.
- **Field selection** is applied at frame build time (`selectColumns`), driven by
  `QueryModel.Fields`; the catalog is `WorkItemFieldNames()` served via
  `/workitemfields`.
- **Raw queries** (`queryType == "raw"`) GET the user's path; if `RawRoot` is set
  that key's value is flattened, else Plane's `results` array is used, else the
  first array of objects anywhere in the response; otherwise the top-level object
  becomes one row.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Plane's
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
  `Input`/`SecretInput`, not `Combobox`.
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

## Verifying against live Plane

Generate an API key at **Profile Settings → Personal Access Tokens**, find your
workspace slug in the Plane URL, then run
`PLANE_API_KEY=plane_api_... PLANE_WORKSPACE_SLUG=my-team docker compose up`
(build `dist/` first). Grafana runs with anonymous admin at
http://localhost:3000, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{projects,states,labels,members}` without
auth.
