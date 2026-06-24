# Contributing

Thanks for your interest in improving the Grafana Intercom data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.26**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- An Intercom access token (Developer Hub or app settings)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class |
| `pkg/` | Backend (Go): Intercom REST client, query handling, record flattener, data-frame conversion |
| `provisioning/` | Grafana provisioning (datasource + example) |
| `docker-compose.yaml` | Grafana with the plugin (token/base-url/version from env) |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-intercom-datasource`.
Install dependencies once from the monorepo root; all plugin commands below run
from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-intercom-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_intercom_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-intercom-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-intercom-datasource`, then restart Grafana.

### Run the local stack

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
INTERCOM_API_TOKEN=dG9rZW46... docker compose up
# EU/AU: also set INTERCOM_BASE_URL=https://api.eu.intercom.io
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login) and the Intercom data source pre-configured from the env vars. Create an
access token in the Intercom Developer Hub first.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds IntercomQuery { queryType, countOf, statusFilter, role, assigneeId,
    teamId, tagId, searchQuery, filters[], sort, limit }
  → applyTemplateVariables() interpolates pickers, search text, sort & filter values
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses the query
      → count: client.CountRecords (reads total_count)
      → records: client.ListRecords (pkg/plugin/client.go)
          → conversations/contacts: GET list, or POST /search when hasSearch()
          → tickets: POST /tickets/search (match-all fallback)
          → articles/companies: GET list (cursor/page pagination)
          → admins/teams/tags: single GET
          → BuildSearchQuery (pkg/plugin/filter.go) compiles pickers + filters
          → flattenIntercomRecord (pkg/plugin/frame.go) flattens each object
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **REST with Bearer + version header**: every request sends
  `Authorization: Bearer`, `Intercom-Version` and `Accept: application/json`.
  Intercom errors arrive as `{"type":"error.list","errors":[{code,message}]}` and
  are surfaced by `client.go::do`.
- **List vs Search selection**: conversations/contacts list unless filtered;
  tickets are search-only; see `QueryModel.hasSearch()` and `ListRecords`.
- **Cursor pagination**: follow `pages.next.starting_after` (search via body
  `pagination`, lists via the `starting_after`/`page` query params); `per_page`
  caps at 150; stop at a missing cursor, empty page, or the limit/safety cap.
- **Unix epoch SECONDS → time**: timestamp fields are converted to UTC time;
  `0` means unset (null). See `frame.go::isTimestampKey` / `flattenIntercomValue`.
- **Nested objects → JSON strings**: `source`, `assignee`, `contacts`, `tags`,
  `custom_attributes`, `statistics`, … are serialised; add new shapes in
  `flattenIntercomValue`.
- **Row order preserved** in `recordsToFrame` — Intercom's returned order honours
  the query sort. Do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`.
- **Secrets stay on the server**: the access token is `secureJsonData`; never log
  it or send it to the browser.

### Intercom API quirks

- Base URL is region-specific: `api.intercom.io` (US), `api.eu.intercom.io` (EU),
  `api.au.intercom.io` (AU). A token only works against its own region.
- The response array key varies by entity (`conversations`/`tickets`/`admins`/
  `teams` vs `data` for contacts/articles/companies/tags) — see `entityDataKey`.
- The Search API `query` is a single `{field, operator, value}` or a
  `{operator, value:[…]}` group; operators include `=`, `!=`, `>`, `<`, `~`,
  `!~`, `^`, `$`, `IN`, `NIN`.
- Companies may require the **Scroll API** for very large sets; this plugin uses
  the standard list (documented limitation).
- `GET /me` is the cheapest authenticated call — used for the health check.
- Rate limit is ~10,000 calls/min per app (tighter per-endpoint).

## Testing

```bash
yarn typecheck          # TypeScript
yarn lint               # ESLint (yarn lint:fix to autofix)
yarn test               # Jest (frontend unit tests)
go test ./pkg/...       # Go unit tests
```

Run all four before opening a PR. Guidelines:

- Put pure logic in standalone modules (`pkg/plugin/*.go`, `src/*.ts`) and
  unit-test it there.
- Go HTTP behavior is tested with `net/http/httptest`; tests are table-driven
  with `testify`.
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
  Grafana; use `Select` / `RadioButtonGroup` / `Input`.
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
