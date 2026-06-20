# Contributing

Thanks for your interest in improving the Grafana Linear data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Linear personal API key (or OAuth token)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class |
| `pkg/` | Backend (Go): Linear GraphQL client, query handling, node flattener, data-frame conversion |
| `provisioning/` | Grafana provisioning (datasource + example) |
| `docker-compose.yaml` | Grafana with the plugin (credential from env) |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-linear-datasource`. Install dependencies
once from the monorepo root; all plugin commands below run from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-linear-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_linear_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-linear-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-linear-datasource`, then restart Grafana.

### Run the local stack

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
LINEAR_API_KEY=lin_api_... docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed) and the Linear data source pre-configured from `LINEAR_API_KEY`.
Create a personal API key at **Settings → Security & access → Personal API keys**
first.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds LinearQuery { queryType, teamId, state, assignee, searchQuery, orderBy, limit, rawQuery }
  → applyTemplateVariables() interpolates the scalar inputs + raw query/variables
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses the query
      → client.ListRecords (pkg/plugin/client.go)
          → predefined: buildConnectionQuery + buildFilter, POST /graphql, cursor-paginated
          → raw: run the document, findNodes locates the first connection
          → flattenNode reduces nested relations to scalars (pkg/plugin/frame.go)
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **GraphQL, not REST**: a single `/graphql` endpoint; GraphQL errors arrive with
  HTTP 200 in an `errors` array and are surfaced by `client.go::do`.
- **Auth header depends on method**: API keys are sent raw; OAuth tokens use
  `Bearer`. See `Settings.credential()`.
- **Nodes are flattened**: nested objects become scalar columns; sub-connections
  of named objects are joined. Add new shapes in `flattenObject`.
- **Row order is preserved** in `recordsToFrame` — Linear's returned order already
  honours `orderBy`. Do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. Date columns become UTC `*time.Time` fields.
- **Secrets stay on the server**: the credential is `secureJsonData`; never log it
  or send it to the browser.

### Linear API quirks

- Single GraphQL endpoint; queries are POSTed as `{query, variables}`.
- Connections paginate with `first`/`after` and `pageInfo { hasNextPage endCursor }`;
  page size is capped at 250.
- Personal API keys are sent raw in `Authorization` (no `Bearer`); OAuth tokens
  use `Bearer`.
- `viewer` is the cheapest authenticated query — used for the health check.

## Testing

```bash
yarn typecheck          # TypeScript
yarn lint               # ESLint (yarn lint:fix to autofix)
yarn test               # Jest (frontend unit tests)
go test ./pkg/...       # Go unit tests
```

Run all four before opening a PR. Guidelines:

- Put pure logic in standalone modules (`pkg/plugin/*.go`) and unit-test it there.
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
  Grafana; use `Select` / `RadioButtonGroup` / `TextArea` / `Input`.
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
