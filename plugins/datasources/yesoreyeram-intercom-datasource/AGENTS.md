# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Intercom](https://www.intercom.com)
(customer support & messaging). Plugin id: `yesoreyeram-intercom-datasource`. The
frontend (TypeScript/React) renders the config and query editors; the Go backend
talks to the Intercom **REST API** (list + Search endpoints), follows cursor
pagination, flattens Intercom's nested objects into scalar columns, converts
Unix epoch-seconds timestamps to time fields, and builds Grafana data frames.
Intercom is **cloud only** (hosted SaaS) with three regional hosts:
`api.intercom.io` (US), `api.eu.intercom.io` (EU), `api.au.intercom.io` (AU).

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getAdmins/getTeams/getTags)
  types.ts                IntercomQuery, IntercomDataSourceOptions, AdminInfo/TeamInfo/TagInfo, option catalogs
  filter.ts               SearchFilter model, Intercom operator catalog, template interpolation
  sort.ts                 parseSort / serializeSort (`-field` <-> {field, direction})
  components/
    ConfigEditor.tsx      Region, API URL, Intercom-Version, secure Access Token
    QueryEditor.tsx       Query type; per-entity pickers (state/role/assignee/team/tag); generic filter builder; search; sort; limit; count-of
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth (GET /me), CallResource (/admins /teams /tags)
    client.go             REST client: do() (Bearer + Intercom-Version), searchEntity, listEntity, simpleList, CountRecords, cursor pagination, resource DTO methods
    queries.go            Query type constants, entityDataKey map, searchable/cursor/simple predicates
    models.go             Settings (baseURL/region/intercomVersion + secret apiToken), QueryModel, LoadSettings/LoadQuery, hasSearch()
    filter.go             BuildSearchQuery (pickers + generic rows -> Search API query), buildSort, coerceValue, matchAllQuery
    frame.go              recordsToFrame / countToFrame + flattenIntercomRecord (epoch-seconds -> time, nested objects -> JSON strings); type inference
provisioning/             Grafana provisioning (datasource + example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token/base-url/version from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-intercom-datasource <script>`. Install deps
with a single `yarn install` at the monorepo root.

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
| Local stack         | `docker compose up` (run `yarn build` first; set `INTERCOM_API_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Auth is a single Bearer token.** Every request sends
  `Authorization: Bearer <token>`, `Intercom-Version: <version>` (default
  `2.11`) and `Accept: application/json`. The token is `secureJsonData`
  (`apiToken`); never log it or send it to the browser.
- **Region/base URL is configurable.** `Settings.BaseURL` wins; otherwise it is
  derived from `Region` (us/eu/au) via `regionBaseURL`, defaulting to
  `https://api.intercom.io`.
- **List vs Search is chosen per query.** Conversations/contacts use the cheap
  list endpoints (`GET /conversations`, `GET /contacts`) when `QueryModel.hasSearch()`
  is false, and the Search API (`POST /{entity}/search`) when any picker, search
  text or generic filter is set. **Tickets are search-only** (no list endpoint) —
  with no criteria a match-all query (`created_at > 0`) is used. Articles and
  companies are list-only; admins/teams/tags are single (non-paginated) lists.
- **Search query is built server-side.** `filter.go::BuildSearchQuery` compiles
  structured pickers (state/role/admin_assignee_id/team_assignee_id/tag_ids) plus
  generic `{field, operator, value}` rows into the Intercom Search API `query`
  object: a single condition is bare, multiple are wrapped in
  `{operator:"AND", value:[…]}`. Values are coerced to numbers/booleans where
  possible (`created_at` etc. must be numbers).
- **Pagination is cursor-based.** Responses carry
  `{pages:{next:{page, starting_after}}, total_count}`. Search sends the cursor in
  the body `pagination.starting_after`; list endpoints send it as the
  `starting_after` query param (or `page` for the legacy URL form). `per_page` is
  capped at 150; the loop stops on a missing `pages.next`, an empty page, or the
  limit/safety cap (100,000).
- **The response array key varies by entity.** `entityDataKey` maps each entity
  to its key: conversations→`conversations`, tickets→`tickets`,
  admins→`admins`, teams→`teams`, and contacts/articles/companies/tags→`data`.
- **Timestamps are Unix epoch SECONDS.** `frame.go::flattenIntercomValue`
  converts timestamp fields (`isTimestampKey`: anything ending `_at`, plus
  `snoozed_until`/`waiting_since`) from epoch seconds to RFC3339 UTC strings so
  the shared time inference types them as time fields; `0`/negative → null.
- **Nested objects are JSON-encoded.** Objects/arrays (`source`, `assignee`,
  `contacts`, `tags`, `custom_attributes`, `statistics`, …) are re-marshalled to
  compact JSON strings (sorted keys → deterministic golden output). A synthetic
  `id` (`row-<index>`) is added when an object has none.
- **Count uses total_count.** `CountRecords` reads `total_count` from a one-row
  search/list response; for the simple lists (admins/teams/tags, which lack
  `total_count`) it returns the array length.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows; only
  columns are reordered (time fields first).
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); count →
  `FrameTypeNumericWide` (v0.1).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  pickers, search text, sort and each filter value (list operators `IN`/`NIN`
  use `csv` formatting).

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (target Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`RadioButtonGroup`/`Input`/`SecretInput`,
  not `Combobox`.
- Pure logic (filter compilation, sort, type inference, flattening) lives in
  standalone, unit-tested modules — add tests there.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- Match existing code style; do not introduce new frameworks or build tooling.
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps pinned to exact versions
  (no `^`/`~`).

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

A golden diff means frame behavior changed — confirm it is intended.

## Verifying against live Intercom

There is no public Intercom server image (it is hosted SaaS). Create an access
token in the Intercom Developer Hub, then run
`INTERCOM_API_TOKEN=dG9rZW46... docker compose up` (build `dist/` first; set
`INTERCOM_BASE_URL` for EU/AU). Grafana runs with anonymous admin at
http://localhost:3000, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{admins,teams,tags}` without auth.
