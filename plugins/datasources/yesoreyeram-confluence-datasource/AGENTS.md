# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for
[Confluence](https://www.atlassian.com/software/confluence) (Atlassian docs/wiki).
Plugin id: `yesoreyeram-confluence-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Confluence
REST API (v2 content endpoints + v1 CQL search), follows cursor pagination,
flattens content into scalar columns, and converts results into Grafana data
frames. Both **Basic** (email + API token) and **Bearer** (OAuth2 / PAT)
authentication are supported.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, getSpaces resource call
  types.ts                ConfluenceQuery, ConfluenceDataSourceOptions, SpaceInfo, enums/options
  sort.ts                 parseSort / serializeSort (`-field`/`field` <-> structured rows)
  filter.ts               CQL string helpers (normalize/escape/spaceCQL, examples)
  components/
    ConfigEditor.tsx      Base URL; auth mode (Basic/Bearer); Basic -> email + API token; Bearer -> token
    QueryEditor.tsx       Query type; Space picker; Sort; CQL textarea; Fields; Limit
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (pages|blogposts|search|count), CheckHealth, CallResource (/spaces)
    client.go             REST client: do() + auth header, cursor pagination (eachPage/collect/count),
                          ListRecords (pages/blogposts/search dispatch), CountRecords, ListSpaces, Ping
    models.go             Settings (baseURL/authMode/email + secrets apiToken/bearerToken), authHeader,
                          QueryModel, LoadSettings/LoadQuery
    filter.go             CQL string helpers (BuildCQL/EscapeCQLValue/SpaceCQL)
    queries.go            Query type constants, listableQueryType, splitFields
    frame.go              recordsToFrame / countToFrame + content/search flattening; type inference, time parsing
provisioning/             Grafana provisioning (env-based datasource + example with both auth modes)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (CONFLUENCE_* env)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

This plugin is a workspace in the **`grafana-x` Yarn 4 monorepo**. Run the
commands below **from this plugin directory**, or from the monorepo root with
`yarn workspace @yesoreyeram/grafana-confluence-datasource <script>`. Install deps
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
| Local stack         | `docker compose up` (run `yarn build` first; set `CONFLUENCE_*`) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.

## Confluence API specifics (verify before changing)

- **Base URL** (Cloud) is the wiki root, e.g.
  `https://<site>.atlassian.net/wiki`. The v2 path `/api/v2/...` and the v1 CQL
  search path `/rest/api/search` are appended to it. Full v2 example:
  `https://<site>.atlassian.net/wiki/api/v2/pages`.
- **Auth — both modes are first-class** (`pkg/plugin/models.go::authHeader`):
  - `basic`  → `Authorization: Basic base64(email:apiToken)` (Atlassian Cloud).
  - `bearer` → `Authorization: Bearer <token>` (OAuth2 / Data Center PAT).
  The backend treats them identically once the header is built; `CheckHealth`
  errors if the mode's required fields are missing.
- **Spaces**: `GET /api/v2/spaces` → `{results:[{id,key,name,type,status}], _links:{next}}`.
- **Pages**: `GET /api/v2/pages?space-id=<id>&limit=<n>` → `{results:[{id,status,
  title,spaceId,authorId,createdAt,version:{number,message,createdAt},_links:{webui}}],
  _links:{next}}`. The param is `space-id` (hyphen), not `space_id`.
- **Blog posts**: `GET /api/v2/blogposts?space-id=<id>` — same shape as pages.
- **Search (CQL)**: `GET /rest/api/search?cql=<cql>` — this is the **v1** search
  endpoint (still used for CQL). Results wrap content in a `content` object plus
  `title`/`excerpt`/`url`/`lastModified`. Highlight markers (`@@@hl@@@`) are
  stripped in `frame.go::stripHighlight`.
- **Pagination is cursor-based.** Responses carry a relative `_links.next` (e.g.
  `/wiki/api/v2/pages?limit=5&cursor=<token>`) which is relative to the **site
  origin** (scheme://host), NOT the wiki base. `client.go::resolveNext` resolves
  it against `c.origin`. `limit` is capped at 250 by the API.
- **Health/Ping**: `GET /api/v2/spaces?limit=1`.
- **Timestamps** (`createdAt`, `version.createdAt`, `lastModified`) are ISO-8601.

## Key architecture facts (do not regress these)

- **Auth header is built once** in `Settings.authHeader()` and stored on the
  client. Tests assert both `Basic base64(email:token)` and `Bearer <token>`.
- **CQL only on the search endpoint.** Do not try to send CQL to the v2 pages /
  blog posts endpoints; they use query params. `filter.go` only normalises/escapes
  CQL strings — there is no structured JSON filter object (unlike Notion).
- **Content/search flattening** lives in `frame.go` (`flattenContentItems`,
  `flattenSearchItems`). They take the site `origin` to make `webui`/`url` links
  absolute. Add new columns there.
- **Row order is preserved.** `recordsToFrame` must NOT re-sort rows — only
  columns are reordered (time fields first). Re-sorting rows would be a bug.
- **Frames are data-plane compliant.** Records → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). ISO-8601 strings parse to UTC `*time.Time`
  fields; numeric id strings stay strings (never coerced to numbers/time).
- **Count semantics**: `CountRecords` counts CQL search results when a CQL string
  is supplied, otherwise counts pages (scoped to `space-id`).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables` over
  `spaceId`, `cql`, `sort`, `fields`, `cursor`.
- **Secrets stay on the server**: `apiToken`/`bearerToken` are `secureJsonData`;
  never log them or send them to the browser.

## Conventions

- TypeScript: keep `@grafana/ui` components **stable** ones only (targets Grafana
  >= 10.4 / runs on 11.x). Use `Select`/`RadioButtonGroup`/`Input`/`SecretInput`/
  `TextArea`, not `Combobox`.
- Pure logic (sort, CQL helpers, flattening, type inference) lives in standalone,
  unit-tested modules — add tests there.
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

## Verifying against live Confluence

There is no public Confluence server image for Cloud (hosted SaaS). To verify
end-to-end, create an API token at
<https://id.atlassian.com/manage-profile/security/api-tokens>, then run
`docker compose up` with the `CONFLUENCE_*` env vars (build `dist/` first).
Grafana runs with anonymous admin at <http://localhost:3000>, so you can hit
`/api/ds/query` and `/api/datasources/uid/<uid>/resources/spaces` without auth.
