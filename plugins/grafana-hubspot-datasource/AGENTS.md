# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [HubSpot](https://developers.hubspot.com).
Plugin id: `yesoreyeram-hubspot-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to HubSpot's **REST
API** (CRM Search API, Properties API, Pipelines API, Owners API), follows
cursor pagination, flattens HubSpot's nested `properties` envelope into scalars,
and converts results into Grafana data frames. HubSpot is **cloud only** (hosted
SaaS); the plugin works with both US (`api.hubapi.com`) and EU
(`api.hubapi.eu`) data residency.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getProperties/getPipelines/getOwners/getSearchOperators)
  types.ts                HubSpotQuery, HubSpotDataSourceOptions, PropertyInfo/PipelineInfo/PipelineStage/OwnerInfo, enums
  components/
    ConfigEditor.tsx      API URL, auth method (private app token | OAuth), secret credential
    QueryEditor.tsx       Query type dropdown; property filter builder (add/remove rows); sort by; property selection; pipeline/stage (deals/tickets); created/updated date modes; limit; raw REST method/path/body/root
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData, CheckHealth (/crm/v3/owners), CallResource (/properties /pipelines /owners /search_operators /object_types)
    client.go             REST client: doGET/doPOST, CRM Search API (searchRecords with pagination), buildSearchRequest, listPipelines, listOwners, listProperties, listRaw, flattenListResponse, resource DTO methods
    queries.go            Query type constants, objectTypeToAPIPath mapping, search operators, nonEmpty helper
    models.go             Settings (baseURL/authMethod + secret privateAppToken/oauthToken), QueryModel (filterGroups/sort/properties/pipeline/date modes/raw), LoadSettings/LoadQuery
    frame.go              recordsToFrame / countToFrame + HubSpot record flattening (flattenHubSpotRecord lifts properties.* to top level alongside id/createdAt/updatedAt/archived); type inference, time parsing
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace yesoreyeram-hubspot-datasource <script>`. Install deps with a
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
| Local stack         | `docker compose up` (run `yarn build` first; set `HUBSPOT_ACCESS_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && go test ./pkg/...`.

## Required OAuth scopes per query type

Every HubSpot API call requires the private app or OAuth app to have the correct
**read scope** granted. Missing scopes produce `HTTP 403 MISSING_SCOPES`.

| Query type / endpoint            | HubSpot scope                   |
|----------------------------------|---------------------------------|
| Contacts (`/crm/v3/objects/contacts/search`) | `crm.objects.contacts.read`     |
| Companies                        | `crm.objects.companies.read`    |
| Deals                            | `crm.objects.deals.read`        |
| Tickets                          | `crm.objects.tickets.read`      |
| Products                         | `crm.objects.products.read`     |
| Line items                       | `crm.objects.line_items.read`   |
| Engagements (meetings/calls etc) | `crm.objects.engagements.read`  |
| Pipelines                        | `crm.objects.deals.read` or `crm.objects.tickets.read` |
| Owners (`GET /crm/v3/owners`)    | `crm.objects.owners.read`       |
| Properties                       | `crm.schemas.custom.read`       |

The **CheckHealth** handler calls `GET /crm/v3/owners`, so `crm.objects.owners.read`
is required for the connection test to pass.

## Key architecture facts (do not regress these)

- **It uses the HubSpot CRM Search API (POST) for object queries.** Search
  endpoints follow `/crm/v3/objects/{object}/search` and accept a JSON body with
  `filterGroups`, `sorts`, `properties`, `limit`, and `after`. The Search API is
  used for all CRM object types (contacts/companies/deals/tickets/products/
  line_items/meetings/calls/tasks/notes/emails).
- **HubSpot returns records inside a `properties` envelope.** Each search result
  is `{ id, properties: { email, firstname, ... }, createdAt, updatedAt, archived }`.
  `flattenHubSpotRecord` lifts `properties.*` to top-level alongside `id`,
  `createdAt`, `updatedAt`, and `archived`. There is NO recursive relation
  flattening (unlike Plane) because HubSpot's Search API returns flat property
  maps.
- **Pagination is cursor-based via `after`.** The Search API response includes
  `paging.next.after` (a numeric offset string). `searchRecords` converts this
  to an int and passes it as the `after` field in the POST body until the API
  returns no paging info or the user's limit is reached.
- **Filter groups mirror HubSpot's Search API directly.** `QueryModel.FilterGroups`
  is a `[{filters: [{propertyName, operator, value}]}]` structure that maps
  directly to the HubSpot API body. Multiple filters in one group are AND'd;
  multiple groups are OR'd. The QueryEditor builds these dynamically.
- **Date filters are translated to property filters server-side.** When
  `createdMode` is `dashboard`, `buildSearchRequest` adds `createdate` GTE/LTE
  filters from `TimeRange.From/To`. When `custom`, it uses `CreatedAfter`/
  `CreatedBefore`. Same pattern for `hs_lastmodifieddate` (updated).
- **Pipeline/stage filtering is applied via filter groups.** For deals and tickets,
  the `hs_pipeline` and `hs_pipeline_stage` properties are filtered server-side
  by the Search API — no backend-side filtering needed.
- **Property definitions are fetched dynamically.** The `/properties` resource
  handler calls `GET /crm/v3/properties/{object}` to populate the property
  dropdowns in the QueryEditor. Properties are NOT hardcoded.
- **Auth is always Bearer-token-based.** Both private app tokens and OAuth tokens
  are sent as `Authorization: Bearer`. The `authMethod` distinction in Settings
  only affects the ConfigEditor UX (different labels/placeholders) — the backend
  treats both identically.
- **Pipelines, Owners, and Properties use dedicated GET endpoints.** These are not
  Search API queries: pipelines use `/crm/v3/pipelines/{object}`, owners use
  `/crm/v3/owners`, properties use `/crm/v3/properties/{object}`.
- **Raw queries support both GET and POST.** `rawMethod` selects the HTTP method;
  POST sends `rawBody` as the request body. Response is flattened the same way
  as other HubSpot responses (lifting `properties.*`, falling back to `results`
  array, then `findArray`, then single object).
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows. Only
  columns are reordered (time fields first).
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  all string and string[] fields including filter values.
- **Secrets stay on the server**: `privateAppToken`/`oauthToken` are
  `secureJsonData`; never log them or send them to the browser.
- **Rate limiting.** HubSpot limits private apps and OAuth apps to 100 requests
  per 10 seconds. The plugin relies on the SDK's HTTP client for retry/backoff
  behavior.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (target Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup`/
  `Input`/`SecretInput`, not `Combobox`.
- Pure logic (flattening, search request assembly, type inference) lives in
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

## Verifying against live HubSpot

Generate a private app access token at **Settings → Integrations → Private Apps**,
then run `HUBSPOT_ACCESS_TOKEN=pat-xxx docker compose up` (build `dist/` first).
Grafana runs with anonymous admin at http://localhost:3000, so you can hit
`/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{properties,pipelines,owners}` without
auth.
