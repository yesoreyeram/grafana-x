# Contributing

Thanks for your interest in improving the Grafana Plane data source plugin.
This guide covers local setup, the architecture, how to test, and the PR process.

## Prerequisites

- **Node.js >= 24.16** and **Yarn 4** (Corepack; managed by the monorepo)
- **Go >= 1.23**
- **Mage** (`go install github.com/magefile/mage@latest`) for backend builds
- **Docker** (optional) for the local stack
- A Grafana instance >= 10.4 if you want to load the plugin manually
- A Plane API key (or OAuth token) and a workspace slug

## Project layout

| Path | What |
| --- | --- |
| `src/` | Frontend (TypeScript / React): config & query editors, data source class |
| `pkg/` | Backend (Go): Plane REST client, query handling, entity flattener, data-frame conversion |
| `provisioning/` | Grafana provisioning (datasource + example) |
| `docker-compose.yaml` | Grafana with the plugin (credential + slug from env) |
| `Magefile.go` | Backend build targets (via grafana-plugin-sdk-go) |
| `webpack.config.ts` | Self-contained frontend build config |

There's a fuller architecture map for tooling in [AGENTS.md](./AGENTS.md).

## Getting started

This plugin lives in the [`grafana-x`](https://github.com/yesoreyeram/grafana-x)
Yarn 4 monorepo under `plugins/grafana-plane-datasource`. Install dependencies
once from the monorepo root; all plugin commands below run from this directory.

```bash
git clone https://github.com/yesoreyeram/grafana-x
cd grafana-x
yarn install
cd plugins/grafana-plane-datasource
```

### Build

```bash
# Frontend → dist/module.js (+ plugin.json, img, etc.)
yarn build             # or: yarn dev  (watch mode)

# Backend → dist/gpx_plane_<os>_<arch>
mage -v build:linuxARM64   # pick your platform; or build:linux, build:darwinARM64, …
mage -v buildAll           # all platforms
```

Both write into `dist/`, which is the loadable plugin directory.

### Run it in Grafana (manual)

Point Grafana at the repo and allow the unsigned plugin:

```ini
# grafana.ini / env
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-plane-datasource
```

Symlink or copy `dist/` into Grafana's plugins directory as
`yesoreyeram-plane-datasource`, then restart Grafana.

### Run the local stack

```bash
mage -v build:linuxARM64   # or build:linux on amd64
yarn build
PLANE_API_KEY=plane_api_... PLANE_WORKSPACE_SLUG=my-team docker compose up
```

This starts **Grafana** at http://localhost:3000 with **anonymous admin** (no
login needed) and the Plane data source pre-configured from `PLANE_API_KEY` /
`PLANE_WORKSPACE_SLUG`. Generate an API key at **Profile Settings → Personal
Access Tokens** first.

## Architecture overview

### Request flow

```
QueryEditor (React)
  → builds PlaneQuery { queryType, workspaceSlug/projectId, priorities, states,
    assignees, labels, date filters, expand, fields, orderBy, limit, rawPath }
  → applyTemplateVariables() interpolates the scalar inputs + multi-value lists
  → backend QueryData (pkg/plugin/datasource.go)
      → LoadQuery parses the query
      → client.ListRecords (pkg/plugin/client.go)
          → work items: GET /api/v1/workspaces/{slug}/projects/{id}/work-items/,
            assemble query params, follow cursor pagination (listPaged)
          → projects/states/labels/cycles/modules/members: the matching
            workspace- or project-scoped endpoint
          → raw: GET the path, flatten the response key (or results / first array)
          → flattenEntity reduces nested relations to scalars (pkg/plugin/frame.go)
      → recordsToFrame / countToFrame (pkg/plugin/frame.go) → Grafana data.Frame
```

### Things that must not regress

- **REST, not GraphQL**: each query is a GET against a versioned path under the
  API root; Plane errors arrive as `{"error"/"detail"/"message":"..."}` and are
  surfaced by `client.go::do`.
- **Auth header depends on method**: API keys use `X-API-Key`; OAuth tokens use
  `Authorization: Bearer`. See `Settings.credential()`.
- **Endpoint selection**: per query type in `ListRecords`. Work items / states /
  labels / cycles / modules are project-scoped; projects and members are
  workspace-scoped.
- **Cursor pagination**: `listPaged` sends `per_page=100` and follows
  `next_cursor` while `next_page_results` is true, stopping at the limit, an empty
  page, or a missing cursor.
- **Dates are ISO-8601 strings**: known Plane date columns (`created_at`,
  `updated_at`, `completed_at`, `archived_at`, `start_date`, `target_date`,
  `deleted_at`) are parsed from RFC3339 / `YYYY-MM-DD` strings to UTC time fields.
  Date filters are sent as `*__gte` / `*__lte` ISO-8601 bounds.
- **Entities are flattened**: nested objects become scalar columns; relation
  arrays are joined. Relations may be expanded objects or bare UUIDs — both must
  work. Add new shapes in `flattenEntity` / `flattenObject`.
- **Row order is preserved** in `recordsToFrame` — Plane's returned order already
  honours `order_by`. Do not re-sort rows.
- **Data-plane compliance**: records → `FrameTypeTable`; count →
  `FrameTypeNumericWide`.
- **Secrets stay on the server**: the credential is `secureJsonData`; never log it
  or send it to the browser.

### Plane API quirks

- Base URL is `https://api.plane.so` (configurable for self-hosted); paths are
  versioned (`/api/v1/...`).
- Hierarchy: Workspace (slug) → Project (UUID) → Work item.
- List endpoints paginate with a cursor (`value:offset:is_prev`); `per_page`
  caps at 100, the response carries `next_cursor` and `next_page_results`.
- API keys are sent in the `X-API-Key` header; OAuth tokens use `Bearer`.
- `expand` includes related objects inline; `fields` selects fields; `order_by`
  accepts a field name with an optional `-` prefix for descending.
- Plane rate-limits to **60 requests/minute** per key.
- `/api/v1/users/me/` is the cheapest authenticated call — used for the health
  check.

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
  Grafana; use `Select` / `MultiSelect` / `RadioButtonGroup` / `Input`.
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
