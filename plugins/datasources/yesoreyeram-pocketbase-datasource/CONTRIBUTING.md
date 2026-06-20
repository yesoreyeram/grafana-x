# Contributing

Thanks for your interest in improving the Grafana PocketBase data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local end-to-end stack (runs a real PocketBase)
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A PocketBase instance and credentials (a superuser, or a user/token)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): PocketBase client (auth + records), query handling, filter-expression compiler, data-frame conversion |
| `scripts/seed.mjs` | Idempotent PocketBase seeder for the local Docker stack |
| `provisioning/` | Grafana provisioning (env-based datasource + example) |
| `docker-compose.yaml` | PocketBase + seed + Grafana (anonymous admin) |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-pocketbase-datasource`. Install
dependencies once from the monorepo root; all plugin commands below run from this
directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-pocketbase-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_pocketbase_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-pocketbase-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-pocketbase-datasource`, then restart Grafana.

### Run the full local stack (recommended)

The stack runs a **real, self-hosted PocketBase**, creates a superuser, seeds
sample `demo` and `metrics` collections, and auto-provisions the datasource:

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
docker compose up
```

This starts **PocketBase** at http://localhost:8090 (admin UI at `/_/`,
`admin@example.com` / `Password123!`) and **Grafana** at http://localhost:3000
with **anonymous admin** (no login needed).

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds PocketBaseQuery { queryType, collectionId, fields, sort, filterTree, rawFilter, limit, hideSystemFields }
  → applyTemplateVariables() interpolates filter values, collectionId, fields, rawFilter
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query + filterTree + sort
      → client.ListRecords / client.CountRecords (pkg/plugin/client.go)
          → ensureToken() authenticates (superuser/user) or uses the static token
          → BuildFilter compiles the filter tree → PocketBase filter expression (pkg/plugin/filter.go)
          → PocketBase REST API, auto-paginated via page/perPage
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Filters are compiled server-side** from the JSON `filterTree` into a single
  PocketBase filter expression. A raw `rawFilter` block takes precedence when set.
- **Row order is preserved** in `recordsToFrame` — PocketBase's returned order
  already honours the `sort` / default order. Do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. datetime columns become UTC `*time.Time` fields;
  array/object cells are JSON-serialised.
- **Secrets stay on the server**: the password / auth token are `secureJsonData`;
  never log them or send them to the browser.

### PocketBase API quirks

- **Auth**: no static API keys. Authenticate via `auth-with-password`
  (superuser against `_superusers`, or a user against a regular auth collection)
  or send a pre-issued token. The token goes **raw** in the `Authorization`
  header. Listing collections/fields requires **superuser** auth.
- Records are listed at `/api/collections/{idOrName}/records`, paged via
  **`page`/`perPage`** (offset, page size ≤ 500). The `filter` is a single
  expression string; `sort` is `-field,field`; `fields` projects columns.
- **Count** reads the filter-aware `totalItems` from a `perPage=1` response.
- There is **no group-by / general aggregation** endpoint — aggregate in Grafana
  via Transformations.
- datetimes use a space separator in UTC (`2022-06-25 11:03:50.052Z`).

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
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/collections"
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/fields?collectionId=demo"

# run a query
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d '{"queries":[{"refId":"A","datasource":{"uid":"<uid>","type":"yesoreyeram-pocketbase-datasource"},"queryType":"records","collectionId":"demo"}],"from":"now-2y","to":"now"}'
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
