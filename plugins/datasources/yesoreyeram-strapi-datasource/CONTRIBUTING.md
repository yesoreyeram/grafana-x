# Contributing

Thanks for your interest in improving the Grafana Strapi data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local end-to-end stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Strapi instance with an [API token](https://docs.strapi.io/dev-docs/api-tokens)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): Strapi client, query handling, filter compiler, data-frame conversion |
| `provisioning/` | Grafana provisioning (env-based datasource + example) |
| `docker-compose.yaml` | Grafana (anonymous admin) provisioned against a Strapi instance |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-strapi-datasource`. Install dependencies
once from the monorepo root; all plugin commands below run from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-strapi-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_strapi_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-strapi-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-strapi-datasource`, then restart Grafana.

### Run the full local stack (recommended)

Strapi is self-hosted, so the stack is just Grafana provisioned against your
Strapi instance:

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
STRAPI_URL=https://your-strapi.example.com STRAPI_API_TOKEN=your-token docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed). Omit the variables to add the datasource manually in the UI.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds StrapiQuery { queryType, contentTypeId, fields, sort, filterTree, page, pageSize, populate }
  → applyTemplateVariables() interpolates filter values, content type, fields, populate
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query + filterTree + sort
      → client.ListRecords / client.CountRecords (pkg/plugin/client.go)
          → BuildFilter compiles the filter tree → Strapi URL filter params (pkg/plugin/filter.go)
          → Strapi REST API, page-based pagination
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Filters are compiled server-side** from the JSON `filterTree` into Strapi
  URL query parameters.
- **Row order is preserved** in `recordsToFrame` — Strapi's returned order
  already honours the query `sort`. Do not re-sort rows.
- **Strapi data format is flattened**: `{id, attributes:{...}}` is merged into
  a single flat record per row.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. date/dateTime columns become UTC `*time.Time` fields;
  array/object cells are JSON-serialised.
- **Secrets stay on the server**: the API token is `secureJsonData`;
  never log it or send it to the browser.

### Strapi API specifics

- **Auth**: a static API token, `Authorization: Bearer <token>`.
- Records are listed at `/api/{contentType}`, paged via **page-based pagination**
  (`pagination[page]` & `pagination[pageSize]`).
- Params: `fields[]`, `filters[][]` (URL query params), `sort[]` (`field:asc`),
  `populate[]`, `pagination[][]`.
- Schema (content types/fields) comes from `/api/content-type-builder/content-types`.
- Count uses `pagination[pageSize]=1` and reads `meta.pagination.total`.
- There is **no group-by / general aggregation** endpoint. Aggregate in Grafana
  via Transformations.

## Testing

```bash
yarn typecheck          # TypeScript
yarn lint               # ESLint (yarn lint:fix to autofix)
yarn test               # Jest (frontend unit tests)
go test ./pkg/...       # Go unit tests
```

Run all four before opening a PR. Guidelines:

- Put pure logic in standalone modules (`sort.ts`, `filter.ts`, `pkg/plugin/*.go`)
  and unit-test it there.
- Go HTTP behavior is tested with `net/http/httptest`; tests are table-driven with
  `testify`.
- For UI, prefer testing the extracted helpers; component tests use
  `@testing-library/react`.

### Golden data-frame tests

Data-frame output is locked down with golden files (the data-plane contract:
field names/types, column order, row order, frame metadata) using the Grafana SDK
golden checker. The files live in `pkg/plugin/testdata/*.jsonc`.

When you intentionally change frame output, regenerate and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata   # review every change before committing
```

Never blindly regenerate — a golden diff is the signal that frame behavior
changed.

### End-to-end verification

With the Docker stack running, you can hit Grafana's API anonymously:

```bash
# list provisioned datasources
curl -s http://localhost:3000/api/datasources

# resources
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/content-types"
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/fields?contentTypeId=<name>"

# run a query
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d '{"queries":[{"refId":"A","datasource":{"uid":"<uid>","type":"yesoreyeram-strapi-datasource"},"queryType":"records","contentTypeId":"<name>"}],"from":"now-2y","to":"now"}'
```

## Coding conventions

- **TypeScript**: use only stable `@grafana/ui` components — the plugin targets
  Grafana 10.4+ and runs on 11.x. `Combobox` is **not** available on older
  Grafana; use `Select` / `MultiSelect` / `RadioButtonGroup`.
- **Go**: format with `gofmt`; keep functions small and testable.
- Match the existing style. Don't add new frameworks, state libraries, or build
  tooling without discussion.
- Keep secrets out of logs and the client bundle.

## Pull requests

1. Create a topic branch from `main`.
2. Make focused changes with tests.
3. Run `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.
4. Update `CHANGELOG.md` and any affected docs (`README.md`, `AGENTS.md`).
5. Open a PR describing the change and how you verified it.

## License

By contributing you agree your contributions are licensed under the project's
[Apache-2.0](./LICENSE) license.
