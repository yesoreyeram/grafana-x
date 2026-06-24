# Contributing

Thanks for your interest in improving the Grafana Teable data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local end-to-end stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Teable [API token](https://help.teable.io/api-docs/authentication)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): Teable client, query handling, JSON filter compiler, data-frame conversion |
| `provisioning/` | Grafana provisioning (env-based datasource + example) |
| `docker-compose.yaml` | Grafana (anonymous admin) provisioned against app.teable.io |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-teable-datasource`. Install
dependencies once from the monorepo root; all plugin commands below run from this
directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-teable-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_teable_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-teable-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-teable-datasource`, then restart Grafana.

### Run the full local stack (recommended)

Teable is self-hosted / SaaS, so the stack is just Grafana provisioned against
the hosted Teable API:

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
TEABLE_API_TOKEN=tok_xxx TEABLE_BASE_ID=xxxx docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed). Omit the variables to add the datasource manually in the UI.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds TeableQuery { queryType, baseId, tableId, viewId, fields, sort, filterTree, limit }
  → applyTemplateVariables() interpolates filter values, base/table/view ids, fields
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query + filterTree + sort
      → client.ListRecords / client.CountRecords (pkg/plugin/client.go)
          → BuildFilter compiles the filter tree → Teable JSON filter object (pkg/plugin/filter.go)
          → Teable API, auto-paginated via skip/take (records) or row-count (count)
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Filters are compiled server-side** from the JSON `filterTree` into a Teable
  JSON `filter` object (`{conjunction, filterSet}`), passed as the `filter` query
  parameter. Filters support AND/OR groups with type-aware operators. Do **not**
  use the deprecated `filterByTql` string.
- **Pagination is offset-based** (`skip`/`take`, page size ≤ 1000); counts use the
  `aggregation/row-count` endpoint. Do not reintroduce a `nextKey` cursor.
- **fieldKeyType=name**: records, filter, orderBy and projection all reference
  fields by name.
- **Row order is preserved** in `recordsToFrame` — Teable's returned order
  already honours `orderBy`/the view. Do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. date/dateTime columns become UTC `*time.Time` fields;
  array/object cells are JSON-serialised.
- **Secrets stay on the server**: the API token is `secureJsonData`; never log it
  or send it to the browser.

### Teable API specifics

API docs: https://help.teable.ai/en/api-doc/overview. All paths are under `/api`
(no `/v1/` segment).

- **Auth**: API token, `Authorization: Bearer <token>`. Health: `GET /api/auth/user`.
- Records are listed at `/api/table/{tableId}/record`, paged via `skip`/`take`
  (not a cursor), filtered via the JSON `filter` param, sorted via `orderBy`,
  field-selected via `projection`, all with `fieldKeyType=name`.
- Counts come from `/api/table/{tableId}/aggregation/row-count` → `{rowCount}`.
- Schema: `GET /api/base/{baseId}/table` and `GET /api/table/{tableId}/field` both
  return **bare JSON arrays**.
- There is no dedicated group-by endpoint exposed by this plugin.

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
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/tables?baseId=<baseId>"
curl -s "http://localhost:3000/api/datasources/uid/<uid>/resources/fields?tableId=<tableId>"

# run a query
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d '{"queries":[{"refId":"A","datasource":{"uid":"<uid>","type":"yesoreyeram-teable-datasource"},"queryType":"records","baseId":"<baseId>","tableId":"<tableId>"}],"from":"now-2y","to":"now"}'
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
