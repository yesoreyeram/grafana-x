# Contributing

Thanks for your interest in improving the Grafana Supabase data source plugin.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local end-to-end stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Supabase project URL and service role key

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): Supabase client, query handling, filter compiler, data-frame conversion |
| `provisioning/` | Grafana provisioning (env-based datasource + example) |
| `docker-compose.yaml` | Grafana (anonymous admin) provisioned against Supabase PostgREST API |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-supabase-datasource`.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-supabase-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_supabase_<os>_<arch>
mage -v build:linuxARM64   # pick your platform
mage -v buildAll           # all platforms
```

### Run the local stack

```bash
mage -v build:linuxARM64
yarn build
SUPABASE_API_URL=https://xxx.supabase.co/rest/v1 SUPABASE_SERVICE_KEY=eyJ... docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed).

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds SupabaseQuery { queryType, tableId, select, filterTree, sort, limit, offset }
  → applyTemplateVariables() interpolates filter values, table, select
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query + filterTree + sort
      → client.ListRecords / client.CountRecords (pkg/plugin/client.go)
          → BuildParams compiles the filter tree → PostgREST query params (pkg/plugin/filter.go)
          → Supabase PostgREST API, auto-paginated via Range headers
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Dual auth headers**: every request sends both `apikey` and
  `Authorization: Bearer <key>` with the same key value. Do not change this.
- **Filters are compiled server-side** from the JSON `filterTree`.
- **Row order is preserved** in `recordsToFrame`.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`.
- **Secrets stay on the server**: the service key is `secureJsonData`;
  never log it or send it to the browser.

### Supabase API quirks

- **Auth**: dual headers (`apikey` + `Authorization: Bearer`), both receiving the
  same key value.
- Records are listed at `/rest/v1/{table}`, paged via **Range** header
  (`Range: 0-99`), response contains `Content-Range: 0-99/1000`.
- Filters use query parameters: `field=operator.value`.
- OR conditions use the `or` parameter: `or=(field.eq.value,field.gt.value)`.
- Schema is exposed as OpenAPI at `GET /rest/v1/`.

## Testing

```bash
go test ./pkg/...       # Go unit tests
```

### Golden data-frame tests

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata   # review every change before committing
```

## Coding conventions

- **TypeScript**: use only stable `@grafana/ui` components.
- **Go**: format with `gofmt`; keep functions small and testable.
- Match the existing style. Don't add new frameworks or state libraries.

## License

By contributing you agree your contributions are licensed under the project's
[Apache-2.0](./LICENSE) license.
