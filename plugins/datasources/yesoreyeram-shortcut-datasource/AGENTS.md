# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[README.md](./README.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for
[Shortcut](https://shortcut.com) (formerly Clubhouse). Plugin id:
`yesoreyeram-shortcut-datasource`. The frontend (TypeScript/React) renders the
config and query editors; the Go backend talks to Shortcut's **REST API v3**,
runs story searches, follows the search `next` token, flattens stories into
scalars, and converts results into Grafana data frames. The API host is
configurable (default `https://api.app.shortcut.com`) so the plugin can run
against a proxy; Shortcut itself is SaaS-only.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getProjects/getEpics/getIterations/getMembers/getTeams/getLabels/getWorkflows/getStoryFields/getStoryTypes)
  types.ts                ShortcutQuery, ShortcutDataSourceOptions, *Info DTOs, enums + option lists
  components/
    ConfigEditor.tsx      API URL (optional override) + API token (secret, Shortcut-Token)
    QueryEditor.tsx       Query type; free-text search query; type/project/state/epic/iteration/label/owner/team filters; archived; date mode + date field + custom bounds; detail; fields; limit
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (stories|count), CheckHealth (/member), CallResource (/projects /epics /iterations /members /teams /labels /workflows /storyfields /storytypes)
    client.go             REST client: do()/request() (Shortcut-Token auth, error extraction), ListStories (search + next-token pagination), CountStories (search total), resource lists
    queries.go            buildSearchQuery (structured filters -> Shortcut query language), date range helpers, story field catalog, story types, effectiveFields
    models.go             Settings (baseURL + secret apiToken), QueryModel, LoadSettings/LoadQuery, constants
    frame.go              recordsToFrame / countToFrame + story flattening; type inference, date-key time parsing, field selection
provisioning/             Grafana provisioning (datasource example)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run from this
plugin directory, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-shortcut-datasource <script>`. Install deps
with a single `yarn install` at the monorepo root.

| Task               | Command |
| ------------------ | ------- |
| Install deps       | `yarn install` (run at monorepo root) |
| Build (front+back) | `yarn build` |
| Frontend only      | `yarn build:frontend` |
| Backend only       | `yarn build:backend` (`mage -v buildAll`) |
| Typecheck          | `yarn typecheck` |
| Lint               | `yarn lint` (`yarn lint:fix` to fix) |
| Frontend tests     | `yarn test` |
| Backend tests      | `go test ./pkg/...` |

Before declaring work done, run: `gofmt -w pkg/ && go build ./... && go vet ./... && go test ./pkg/...`
and (when `node_modules` is present) `npx tsc --noEmit -p tsconfig.json`,
`yarn lint`, `yarn test`.

## Key architecture facts (do not regress these)

- **Auth is the `Shortcut-Token` header.** NOT `Authorization: Bearer`, and NOT
  the deprecated `token` query parameter. Set in `client.go::request`.
- **There is NO list-all stories endpoint.** Stories come from
  `GET /api/v3/search/stories?query=<shortcut-query>&page_size=&detail=`. Do not
  reintroduce `GET /stories` or `project_id[]`/`workflow_state_id[]` query params
  — those endpoints/params do not exist for listing.
- **The base URL is the host only** (`https://api.app.shortcut.com`); the client
  appends `apiPrefix` (`/api/v3`). This keeps `next`-token resolution trivial.
- **Pagination follows the search `next` token.** The response is
  `{data, next, total}`; `next` is a relative path + query string
  (`/api/v3/search/stories?...&next=<token>`) and is resolved against the host
  (`client.go::followNext`). Follow until `next` is null/empty or the limit is hit.
- **Count uses the `total` field** — a single `page_size=1` search request, no
  pagination (`client.go::CountStories`).
- **Filters compile to the Shortcut search query language** in
  `queries.go::buildSearchQuery`. Search matches by **name** (mention name for
  owners), not ids, and is **AND-only** (no OR). Multi-word values are quoted.
  Deadline maps to `due:`; dates are `YYYY-MM-DD..YYYY-MM-DD` (open sides `*`).
  Archived maps to `is:archived` / `!is:archived`. Empty query falls back to
  `is:story`.
- **Resource DTOs expose names** (and mention names for members/teams) because
  the query language needs names. Members nest `name`/`mention_name` under
  `profile` — read them from there, not the top level. Teams come from `/groups`.
- **Health/Ping hits `/member`** (current member), which validates the token.
- **Stories are flattened** in `frame.go::flattenStory`: scalars pass through;
  nested arrays/objects (owner_ids, label_ids, labels, …) are preserved as
  compact JSON strings; empty `[]`/`{}` become null.
- **Time columns are restricted to known date keys** (`dateKeys` in frame.go) via
  `toColumnTime`; do not treat arbitrary date-looking strings as time.
- **Field selection** — when the query selects no fields, a curated catalog
  (`storyFieldNames`) is used as the default columns (`effectiveFields`), so the
  frame does not dump every nested collection. `/storyfields` serves the catalog.
- **Frames are data-plane compliant.** Stories → `FrameTypeTable` (v0.1);
  count → `FrameTypeNumericWide` (v0.1). Row order is preserved (search ranking);
  only columns are reordered (time first).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  the raw query, scalar inputs, every multi-value list, and the fields.
- **Secrets stay on the server**: `apiToken` is `secureJsonData`; never log it or
  send it to the browser.

## API limits to respect

- Search returns at most **1000 results total**; `page_size` is 1–250 (default 25).
- The API is rate-limited to **200 requests/minute**.

## Conventions

- TypeScript: use **stable** `@grafana/ui` components only (target Grafana
  >= 10.4 / runs on 11.x): `Select`/`MultiSelect`/`RadioButtonGroup`/`Input`/
  `SecretInput`, not `Combobox`.
- Pure logic (query building, flattening, type inference) lives in standalone,
  unit-tested Go modules — add tests there.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- Toolchain is pinned: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; JS deps are exact versions.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```
