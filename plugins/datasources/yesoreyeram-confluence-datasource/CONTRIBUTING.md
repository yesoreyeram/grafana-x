# Contributing

Thanks for your interest in improving the Grafana Confluence data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Confluence site + credentials (Cloud API token, or an OAuth2 / PAT token)

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class, pure logic modules |
| `pkg/` | Backend (Go): Confluence client, query handling, CQL helpers, content flattener, data-frame conversion |
| `provisioning/` | Grafana provisioning (datasource + example, both auth modes) |
| `docker-compose.yaml` | Grafana with the plugin (credentials from env) |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/datasources/yesoreyeram-confluence-datasource`.
Install dependencies once from the monorepo root; all plugin commands below run
from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/datasources/yesoreyeram-confluence-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_confluence_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-confluence-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-confluence-datasource`, then restart Grafana.

### Run the local stack

```bash
yarn build
CONFLUENCE_URL=https://your-site.atlassian.net/wiki \
CONFLUENCE_AUTH_MODE=basic \
CONFLUENCE_EMAIL=you@example.com \
CONFLUENCE_API_TOKEN=xxxx \
docker compose up
```

This starts **Grafana** at <http://localhost:3000> with **anonymous admin** (no
login needed) and the Confluence data source pre-configured. Create a Cloud API
token at <https://id.atlassian.com/manage-profile/security/api-tokens> first.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds ConfluenceQuery { queryType, spaceId, cql, sort, fields, limit }
  → applyTemplateVariables() interpolates spaceId, cql, sort, fields, cursor
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses query
      → client.ListRecords (pages|blogposts|search) / client.CountRecords (pkg/plugin/client.go)
          → builds the request URL (v2 content or v1 search), follows cursor pagination
          → flattenContentItems / flattenSearchItems reduce items to scalars (pkg/plugin/frame.go)
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **Both auth modes work.** `Settings.authHeader()` produces
  `Basic base64(email:apiToken)` or `Bearer <token>`; tests assert both.
- **CQL is only used on the search endpoint.** v2 listings use query parameters
  (`space-id`, `sort`, …). There is no structured JSON filter object.
- **Cursor pagination** follows the relative `_links.next` URL resolved against
  the site origin.
- **Content flattening**: typed items become scalar columns. Add new columns in
  `flattenContentItems` / `flattenSearchItems`.
- **Row order is preserved** in `recordsToFrame` — only columns are reordered
  (time fields first). Do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`. ISO-8601 columns become UTC `*time.Time` fields.
- **Secrets stay on the server**: tokens are `secureJsonData`; never log them or
  send them to the browser.

### Confluence API quirks

- Base URL is the wiki root (`.../wiki` on Cloud); v2 path `/api/v2/...` and CQL
  search path `/rest/api/search` are appended.
- The pages/blog posts space filter param is `space-id` (hyphen).
- `_links.next` is relative to the site origin, not the wiki base.
- Page/blog post bodies are not expanded by default (metadata + latest version
  only).
- CQL search is the **v1** endpoint; results wrap content in a `content` object
  and include `@@@hl@@@` highlight markers (stripped by the backend).

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
  `testify`. `client_test.go` exercises **both** auth modes.
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
  Grafana; use `Select` / `RadioButtonGroup` / `Input` / `SecretInput` /
  `TextArea`.
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
