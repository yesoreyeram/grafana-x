# Contributing

Thanks for your interest in improving the Grafana SeaTable data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local end-to-end stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A SeaTable **Base API Token** (Base → *API Tokens* → generate)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): SeaTable client (token exchange, rows/SQL/metadata), filter→SQL compiler, frames |
| `provisioning/` | Grafana provisioning (env-based datasource + example) |
| `docker-compose.yaml` | Grafana (anonymous admin) provisioned against a SeaTable server |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-seatable-datasource`.
Install dependencies once from the monorepo root; all plugin commands below run
from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-seatable-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_seatable_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-seatable-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-seatable-datasource`, then restart Grafana.

### Run the full local stack (recommended)

```bash
yarn build
SEATABLE_API_TOKEN=<base-api-token> docker compose up
# self-hosted: SEATABLE_SERVER_URL=https://seatable.example.com SEATABLE_API_TOKEN=... docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed). Omit the variables to add the datasource manually in the UI.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds SeaTableQuery { queryType, tableName, viewName, fields, sort, filterTree, limit, sql }
  → applyTemplateVariables() interpolates filter values, table, view, fields, sql
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query + filterTree + sort
      → client (pkg/plugin/client.go)
          → exchange Base API Token → Base-Token + dtable_uuid (cached; refreshed on 401)
          → records: rows endpoint (plain) OR SQL endpoint (filter/sort/fields)
          → count: SELECT COUNT(*); sql: raw passthrough
          → BuildWhere compiles filterTree → parameterized SQL WHERE (pkg/plugin/filter.go)
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Two-step auth.** The Base API Token is exchanged for a Base-Token via
  `GET /api/v2.1/dtable/app-access-token/`; data calls use
  `Authorization: Bearer <access_token>` against the api-gateway. The token is
  cached and re-fetched on a `401`.
- **Filters are parameterized.** `BuildWhere` emits `?` placeholders + params;
  values are never inlined. Identifiers are backtick-escaped.
- **Records routing.** Plain listings use the rows endpoint (view-aware); filter/
  sort/fields force the SQL endpoint (no view).
- **Row order is preserved** in `recordsToFrame` — do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. date/ctime/mtime become UTC `*time.Time` fields;
  array/object cells are JSON-serialised. Rows keep `_id`/`_ctime`/`_mtime`.
- **Secrets stay on the server**: the Base API Token is `secureJsonData`.

### SeaTable API quirks

- The token-exchange path is `/api/v2.1/dtable/app-access-token/` (seahub), while
  data endpoints are under `/api-gateway/api/v2/dtables/{uuid}/…`.
- The **rows** endpoint needs `convert_keys=true` to return column *names* (it
  returns column *keys* otherwise) and cannot filter/sort/project.
- The **SQL** endpoint is the powerful path: parameterized via `?` + `parameters`,
  default 100 / max 10000 rows.
- There is **no dedicated count endpoint** — Count uses `SELECT COUNT(*)`.

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
- Go HTTP behavior is tested with `net/http/httptest`; mock **both** the
  token-exchange endpoint and the data endpoints. Tests are table-driven with
  `testify`.

### Golden data-frame tests

Data-frame output is locked down with golden files (the data-plane contract:
field names/types, column order, row order, frame metadata) using the Grafana SDK
golden checker. The files live in `pkg/plugin/testdata/*.jsonc`.

When you intentionally change frame output, regenerate and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata   # review every change before committing
```

Never blindly regenerate — a golden diff is the signal that frame behavior changed.

### End-to-end verification

With the Docker stack running, you can hit Grafana's API anonymously:

```bash
# list provisioned datasources
curl -s http://localhost:3000/api/datasources

# resources
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/tables"

# run a query
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d '{"queries":[{"refId":"A","datasource":{"uid":"<uid>","type":"yesoreyeram-seatable-datasource"},"queryType":"records","tableName":"Table1"}],"from":"now-2y","to":"now"}'
```

## Coding conventions

- **TypeScript**: use only stable `@grafana/ui` components — the plugin targets
  Grafana 10.4+ and runs on 11.x. `Combobox` is **not** available on older
  Grafana; use `Select` / `MultiSelect` / `RadioButtonGroup` / `TextArea`.
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
