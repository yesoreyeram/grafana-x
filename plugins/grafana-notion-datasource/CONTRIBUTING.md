# Contributing

Thanks for your interest in improving the Grafana Notion data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Notion internal integration token + a database shared with it

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): Notion client, query handling, filter compiler, page flattener, data-frame conversion |
| `provisioning/` | Grafana provisioning (datasource + example) |
| `docker-compose.yaml` | Grafana with the plugin (datasource token from env) |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/grafana-notion-datasource`. Install dependencies
once from the monorepo root; all plugin commands below run from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/grafana-notion-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_notion_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-notion-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-notion-datasource`, then restart Grafana.

### Run the local stack

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
NOTION_API_TOKEN=secret_... docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed) and the Notion data source pre-configured from `NOTION_API_TOKEN`.
Create the integration at https://www.notion.so/my-integrations and share a
database with it first.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds NotionQuery { queryType, databaseId, fields, sort, filterTree, limit }
  → applyTemplateVariables() interpolates filter values, sort, fields, databaseId
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query + filterTree
      → client.ListRecords / client.CountRecords (pkg/plugin/client.go)
          → BuildFilter compiles the filter tree → Notion JSON filter (pkg/plugin/filter.go)
          → POST /v1/databases/{id}/query, cursor-paginated
          → flattenPages reduces typed properties to scalars (pkg/plugin/frame.go)
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Filters are compiled server-side** from the JSON `filterTree` into a Notion
  JSON filter object. List operators (`in`/`not_in`) expand into or/and groups
  because Notion has no native list operator.
- **Pages are flattened**: typed property objects become scalar columns. Add new
  property types in `flattenProperty`.
- **Row order is preserved** in `recordsToFrame` — Notion's returned order
  already honours the query `sorts`. Do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. Date columns become UTC `*time.Time` fields.
- **Secrets stay on the server**: the integration token is `secureJsonData`;
  never log it or send it to the browser.

### Notion API quirks

- Every request needs the `Notion-Version` header and `Authorization: Bearer`.
- There is **no count endpoint** — Count paginates the query and counts results.
- There is **no "view" concept** — only databases and properties.
- Pagination is **cursor-only** (`start_cursor`/`next_cursor`/`has_more`);
  `page_size` is capped at 100.
- Operators are property-type specific (`rich_text` → `contains`, `number` →
  `greater_than`, `date` → `before`/`after`, …).

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
