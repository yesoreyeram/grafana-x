# Contributing

Thanks for your interest in improving the Grafana Coda data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local end-to-end stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Coda [API token](https://coda.io/account)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): Coda client, query handling, filter compiler, data-frame conversion |
| `provisioning/` | Grafana provisioning (env-based datasource + example) |
| `docker-compose.yaml` | Grafana (anonymous admin) provisioned against coda.io |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-coda-datasource`. Install dependencies
once from the monorepo root; all plugin commands below run from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-coda-datasource
```

### Build

```bash
yarn build             # frontend + backend -> dist/
yarn dev               # frontend watch mode
```

Both write into `dist/`, which is the loadable plugin directory.

### Run the full local stack

Coda is SaaS-only, so the stack is just Grafana provisioned against the hosted
Coda API:

```bash
yarn build
CODA_API_TOKEN=tok... CODA_DOC_ID=doc... docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed). Omit the variables to add the datasource manually in the UI.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds CodaQuery { queryType, docId, tableId, columns, filterColumn,
                       filterValue, query, sortBy, visibleOnly, valueFormat, limit }
  → applyTemplateVariables() interpolates doc/table ids, columns, filter, query
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses the query and applies defaults (valueFormat=simple)
      → client.ListRows / client.CountRows (pkg/plugin/client.go)
          → effectiveFilterQuery → single-column Coda `query` (pkg/plugin/filter.go)
          → Coda Web API, auto-paginated via the pageToken cursor
          → flattenRows turns each row's `values` map + metadata into a record
      → rowsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Single-column filtering only.** Coda's `query` parameter filters by one
  column (`<col>:<value>`). A raw `query` takes precedence over the structured
  `filterColumn`/`filterValue`. Do not reintroduce a full filter tree — the Coda
  API cannot express it.
- **Rows come from the `values` map** (not `cells`), keyed by column name because
  the client always sends `useColumnNames=true`.
- **Count is rowCount-first**: unfiltered count reads the table's `rowCount`;
  filtered count paginates the rows.
- **Column projection is client-side** (the rows endpoint has no columns param).
- **Row order is preserved** in `rowsToFrame`.
- **Data-plane compliance**: rows → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. date/dateTime columns (incl. createdAt/updatedAt)
  become UTC `*time.Time` fields; array/object cells are JSON-serialised.
- **Secrets stay on the server**: the API token is `secureJsonData`; never log it
  or send it to the browser.

## Testing

```bash
yarn typecheck          # TypeScript
yarn lint               # ESLint
yarn test               # Jest (frontend unit tests)
go test ./pkg/...       # Go unit tests
go vet ./pkg/...        # Go vet
```

Run all four before opening a PR.

### Golden data-frame tests

Data-frame output is locked down with golden files. Regenerate with:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```

## License

By contributing you agree your contributions are licensed under the project's
[Apache-2.0](./LICENSE) license.
