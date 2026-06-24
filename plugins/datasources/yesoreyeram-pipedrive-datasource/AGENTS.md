# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for
[Pipedrive](https://developers.pipedrive.com/docs/api/v1).
Plugin id: `yesoreyeram-pipedrive-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to Pipedrive's REST
API v1, follows offset-based pagination (`more_items_in_collection`/`next_start`),
remaps custom-field hash keys to names, flattens Pipedrive records, and converts
results into Grafana data frames. Pipedrive is **cloud only** (hosted SaaS); the
plugin connects via `https://{companyDomain}.pipedrive.com/api/v1`.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getPipelines/getStages/getUsers)
  types.ts                PipedriveQuery, PipedriveDataSourceOptions, PipelineInfo/StageInfo/UserInfo, enums
  components/
    ConfigEditor.tsx      Company Domain + auth method (API token | OAuth) + secret credential
    QueryEditor.tsx       Query type; count entity; deal status; pipeline/stage; user; saved filter id; map-custom-fields toggle; filter builder; sort; pagination
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (entity|count), CheckHealth (/users/me), CallResource (/pipelines /stages /users)
    client.go             REST client: get() (api_token query OR Bearer header), ListRecords (v1 pagination), CountRecords (pagination), fetchFieldMap/remapCustomFields, entityListParams, ListPipelines/ListStages/ListUsers
    queries.go            Query type + entity path + {entity}Fields path maps; standard field catalogs
    models.go             Settings (companyDomain/authMethod + secret apiToken/oauthToken), QueryModel (queryType/filters/sort/limit/start/filterId/countEntity/mapCustomFields), LoadSettings/LoadQuery
    frame.go              recordsToFrame / countToFrame + Pipedrive record flattening; type inference, time parsing
    filter.go             Client-side filter matching for arbitrary field filters
    *_test.go             client/frame/model/filter unit tests
    golden_test.go        Golden data-frame snapshots (testdata/*.jsonc)
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-pipedrive-datasource <script>`.

| Task                | Command |
| ------------------- | ------- |
| Build (front+back)  | `yarn build` (frontend + `mage buildAll`) |
| Frontend only       | `yarn build:frontend` |
| Backend only        | `yarn build:backend` (alias for `mage buildAll`) |
| Frontend watch      | `yarn dev` |
| Typecheck           | `yarn typecheck` |
| Lint                | `yarn lint` (`yarn lint:fix` to fix) |
| Frontend tests      | `yarn test` |
| Backend tests       | `go test ./pkg/...` |
| Backend build (1)   | `mage -v build:linuxARM64` (or `build:linux`, `build:darwinARM64`, …) |
| Local stack         | `docker compose up` (build `yarn build` first) |

Before declaring work done, run: `yarn typecheck && yarn lint && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Two auth modes.** `authMethod` is `apiToken` (default) or `oauth`. apiToken
  is sent as the `api_token` **query parameter**; oauth is sent as an
  `Authorization: Bearer` **header**. `Settings.authMode()` falls back to
  whichever credential is actually configured. Both secrets (`apiToken`,
  `oauthToken`) live in `secureJsonData`; never log them or send them to the
  browser.
- **Base URL is derived from companyDomain.** The plugin constructs
  `https://{companyDomain}.pipedrive.com/api/v1`. `NewClient` does NOT hard-fail
  on a missing domain/credential — validation is deferred to `CheckHealth` so the
  instance is always creatable and reports a friendly message.
- **Pagination is offset-based and MUST follow the metadata.** Each list request
  sends `start` + `limit` (max 500). The response carries
  `additional_data.pagination.{start, limit, more_items_in_collection, next_start}`.
  `listAll` loops while `more_items_in_collection` is true, advancing to
  `next_start` (falling back to `start += pageLimit`), until the limit/safety cap.
  **Never** blindly increment `start` without checking `more_items_in_collection`
  — that was the original bug.
- **Count paginates.** Pipedrive has no list count endpoint, so `CountRecords`
  paginates the chosen entity (same `more_items_in_collection` loop) and sums the
  page sizes. Works for any entity (`countEntity`, default deals). `/deals/summary`
  exists as a faster deals-only alternative but is intentionally not used so one
  implementation covers all entities.
- **Custom field hash → name remapping.** Pipedrive custom fields are keyed by a
  40-char hex hash. When `mapCustomFields` is on (default), `fetchFieldMap` reads
  `{entity}Fields` (`dealFields`/`personFields`/`organizationFields`/`productFields`)
  to build `key → name`, and `remapCustomFields` renames columns (including
  subfields like `{hash}_currency` → `{name}_currency`). A fetch failure is
  non-fatal — records are returned with raw hashes. Never clobber an existing
  column when renaming.
- **No CRM Search API.** Unlike HubSpot, Pipedrive list endpoints filter only by
  query params server-side (`status`, `pipeline_id`, `stage_id`, `user_id`,
  `filter_id`). `filter_id` takes precedence over the others (API behaviour).
  Arbitrary field filtering is applied client-side, post-fetch.
- **Record flattening.** Records are mostly flat. Relation objects (`person_id`,
  `org_id`, `user_id`) flatten to their `name`; `email`/`phone` arrays flatten to
  the `value` (NOT the `label`). `flattenObject` key preference puts `value`
  before `label` to make this work — do not reorder it.
- **Health check** is `GET /api/v1/users/me`.
- **Time fields** parse `2006-01-02 15:04:05` and `2006-01-02`; a column is only
  time-typed when its name is a known date key or ends in `_time`/`_date`.
- **Frame types.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Time columns first; row order preserved.
- **Rate limiting.** Pipedrive uses a token-budget rate limit (~10 req/s for
  token auth). The plugin relies on the SDK HTTP client for retry/backoff.
- **API v1 vs v2.** v1 is used for breadth/offset-pagination simplicity. v2 list
  endpoints use cursor pagination (`cursor`/`next_cursor`); switching an entity to
  v2 means changing its pagination loop.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots checked via the SDK golden
checker. Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (target Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`RadioButtonGroup`/`Input`/`SecretInput`/
  `InlineSwitch`, not `Combobox`.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- Match existing code style; do not introduce new frameworks or build tooling.
