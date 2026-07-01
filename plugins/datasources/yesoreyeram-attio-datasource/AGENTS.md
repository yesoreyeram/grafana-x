# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Attio](https://attio.com)
(the AI-native CRM). Plugin id: `yesoreyeram-attio-datasource`. The frontend
(TypeScript/React) renders the config and query editors; the Go backend talks to
the Attio REST API, compiles filters, paginates, flattens typed attribute values,
and converts results into Grafana data frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getObjects/getAttributes)
  types.ts                AttioQuery, AttioDataSourceOptions, ObjectInfo, AttributeInfo
  sort.ts                 parseSort / serializeSort (structured sort rows <-> JSON string)
  filter.ts               Filter model, type-aware operator catalog, template interpolation, JSON persistence
  components/
    ConfigEditor.tsx      API Token (secret), API URL (optional), Default Object
    QueryEditor.tsx       Query type, Object picker, Attributes, Filters, Sort, Limit, Offset
    FilterEditor.tsx      Recursive filter/group builder (consistent with Grafana inline rows)
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (records|count), CheckHealth, CallResource (/objects /attributes)
    client.go             Attio HTTP client: bearer auth, QueryRecords (POST + offset paging), CountRecords, ListObjects, ListAttributes, Ping
    models.go             Settings (baseURL + secret apiToken), QueryModel, LoadSettings/LoadQuery
    filter.go             BuildFilter: structured filter tree -> Attio JSON filter object; SortItem
    frame.go              recordsToFrame / countToFrame + Attio value flattening; type inference, time parsing
    queries.go            Query type constants
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against the Attio API
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-attio-datasource <script>`. Install deps with a
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
| Local full stack    | `docker compose up` (run `yarn build` first; set `ATTIO_API_TOKEN`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Filters are compiled server-side.** The query editor persists a JSON
  `filterTree` on the query; `pkg/plugin/filter.go::BuildFilter` compiles it into
  an Attio JSON filter object passed in the records query POST body. Attio filters
  are JSON in the POST body, NOT a query string.
- **Attio has no negative operators.** `!=` compiles to `{"$not": {field: {"$eq": v}}}`
  and `is empty` to `{"$not": {field: {"$not_empty": true}}}`. List membership uses
  `$in`. Do not invent `$neq`/`$empty` operators.
- **Attribute values are arrays of typed objects.** Each attribute slug maps to an
  array of historical value objects discriminated by `attribute_type`.
  `frame.go::flattenRecords`/`flattenValue` reduce the **first (latest active)**
  element to a scalar. Add new attribute types there. Two synthetic columns are
  always emitted: `_record_id` and `_created_at`.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — Attio's
  returned order honours the query `sorts`. Only columns are reordered (time
  fields first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/timestamp strings parse to UTC `*time.Time`
  fields. Array/object cell values are JSON-serialised to strings.
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`,
  interpolating each filter value plus object/attributes.
- **Secrets stay on the server**: the access token is `secureJsonData`; never log
  it or send it to the browser.

## Attio API specifics (verify before changing)

- **Base URL**: `https://api.attio.com` (hosted SaaS; only overridable for a proxy).
- **Auth**: single mode — a workspace access token sent as
  `Authorization: Bearer <token>`. Generated in Attio under
  Settings > Developers > API keys.
- **List objects**: `GET /v2/objects` → `{data:[{id:{object_id,workspace_id}, api_slug, singular_noun, plural_noun}]}`.
  Standard objects: people, companies, deals, users, workspaces.
- **List attributes**: `GET /v2/objects/{object}/attributes` → `{data:[{api_slug, title, type, is_required, ...}]}`.
  Types: text, number, checkbox, currency, date, timestamp, rating, status,
  select, record-reference, actor-reference, location, domain, email-address,
  phone-number, interaction, personal-name.
- **Query records**: `POST /v2/objects/{object}/records/query` with body
  `{"filter": {...}, "sorts": [{"attribute":"slug","direction":"asc"|"desc"}], "limit": 500, "offset": 0}`
  → `{data:[{id:{record_id,object_id,workspace_id}, created_at, web_url, values:{slug:[valueObj...]}}]}`.
- **Pagination**: `limit` (max **500**) + `offset` in the POST body. Loop
  incrementing offset until a page returns fewer than `limit` rows or the hard
  limit is reached.
- **Count**: there is **no count endpoint** — count is derived by paginating the
  query and summing the page sizes.
- **Health/Ping**: `GET /v2/self` (identify) returns `{active, workspace_id, workspace_name, ...}`.
- **Filter operators**: `$eq`, `$in`, `$not_empty`, `$contains`, `$starts_with`,
  `$ends_with`, `$gt`, `$gte`, `$lt`, `$lte`; logical `$and`/`$or`/`$not`.
- **Errors**: error bodies carry `{status_code, type, code, message}`; the client
  surfaces `message`.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (targets Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`MultiSelect`/`RadioButtonGroup` (not
  `Combobox`, which is unavailable on older Grafana).
- Pure logic (sort, filter compilation/serialization, type inference, value
  flattening) lives in standalone, unit-tested modules — add tests there rather
  than only in components.
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via
  `httptest`.
- **Toolchain is pinned**: Node in `.nvmrc`/`.tool-versions`, Go in
  `go.mod`/`.go-version`/`.tool-versions`; all JS deps pinned to exact versions
  (no `^`/`~`). The monorepo uses Yarn 4 with `defaultSemverRangePrefix: ""`.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the data frames (field
names/types, column + row order, frame meta) checked via the SDK golden checker.
Regenerate intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden
git diff pkg/plugin/testdata
```

A golden diff means frame behavior changed — confirm it is intended.

## Verifying against live Attio

There is no public Attio server image (it is hosted SaaS). To verify end-to-end,
create an API key in Attio (Settings > Developers > API keys), then run
`ATTIO_API_TOKEN=... docker compose up` (build `dist/` first). Grafana runs with
anonymous admin at http://localhost:3000, so you can hit `/api/ds/query` and
`/api/datasources/uid/<uid>/resources/{objects,attributes}` without auth.
