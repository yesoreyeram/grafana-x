# Contributing

Thanks for your interest in improving the Grafana Appwrite data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local end-to-end stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- An Appwrite project and an **API key** with the `databases.read`,
  `collections.read`, `attributes.read` and `documents.read` scopes

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): Appwrite client, query handling, query-string compiler, data-frame conversion |
| `provisioning/` | Grafana provisioning (env-based datasource + example) |
| `docker-compose.yaml` | Grafana (anonymous admin) provisioned against cloud.appwrite.io |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/grafana-appwrite-datasource`. Install dependencies
once from the monorepo root; all plugin commands below run from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/grafana-appwrite-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_appwrite_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-appwrite-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-appwrite-datasource`, then restart Grafana.

### Run the full local stack (recommended)

Appwrite Cloud is SaaS, so the stack is just Grafana provisioned against the
hosted Appwrite API (point the endpoint at a self-hosted Appwrite if you have
one):

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
APPWRITE_PROJECT_ID=... APPWRITE_API_KEY=... APPWRITE_DATABASE_ID=... docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed). Omit the variables to add the datasource manually in the UI.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds AppwriteQuery { queryType, databaseId, collectionId, attributes, sort, filterTree, rawQueries, limit }
  → applyTemplateVariables() interpolates filter values, database/collection ids, attributes, rawQueries
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query + filterTree + sort
      → client.ListDocuments / client.CountDocuments (pkg/plugin/client.go)
          → BuildFilterQueries compiles the filter tree → Appwrite query strings (pkg/plugin/filter.go)
          → Appwrite REST API, auto-paginated via the cursorAfter cursor
      → documentsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Filters are compiled server-side** from the JSON `filterTree` into Appwrite
  `queries[]` strings. A raw `rawQueries` block takes precedence when set.
- **Row order is preserved** in `documentsToFrame` — Appwrite's returned order
  already honours the query sort / default order. Do not re-sort rows.
- **Data-plane compliance**: documents → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. datetime columns become UTC `*time.Time` fields;
  array/object cells are JSON-serialised.
- **Secrets stay on the server**: the API key is `secureJsonData`; never log it or
  send it to the browser.

### Appwrite API quirks

- **Auth**: two headers — `X-Appwrite-Project` (project id) and `X-Appwrite-Key`
  (API key). Needs the `databases.read`, `collections.read`, `attributes.read`
  and `documents.read` scopes.
- Documents are listed at `/databases/{db}/collections/{col}/documents`, paged via
  the **cursorAfter cursor** (the last document's `$id`) plus `limit(n)`, page
  size ≤ 100. Each query is a JSON string
  `{"method":..., "attribute":..., "values":[...]}` passed as a repeated
  `queries[]` parameter.
- Schema (databases/collections/attributes) comes from `/databases`,
  `/databases/{db}/collections` and `/databases/{db}/collections/{col}/attributes`.
- There is **no group-by / general aggregation** endpoint. **Count** reads the
  filter-aware `total` field from a `limit(1)` list response. Aggregate in Grafana
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
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/databases"
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/collections?databaseId=<dbId>"
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/attributes?collectionId=<colId>"

# run a query
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d '{"queries":[{"refId":"A","datasource":{"uid":"<uid>","type":"yesoreyeram-appwrite-datasource"},"queryType":"documents","databaseId":"<dbId>","collectionId":"<colId>"}],"from":"now-2y","to":"now"}'
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
