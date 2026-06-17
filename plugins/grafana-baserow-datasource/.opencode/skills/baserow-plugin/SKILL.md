---
name: baserow-plugin
description: Develop and debug the Grafana Baserow data source plugin in this repo. Use when changing the query/config editors, the filter builder, the Go backend (Baserow client, filters compiler, data-frame conversion), or when verifying behavior against the local Docker stack. Covers build/test commands, architecture, and Baserow API quirks that cause subtle bugs.
license: Apache-2.0
---

# Grafana Baserow Data Source — development skill

A Grafana **backend data source plugin** for Baserow. Frontend (TypeScript/React)
renders config + query editors; Go backend calls the Baserow REST API, compiles
filters, paginates, and builds Grafana data frames. Plugin id:
`yesoreyeram-baserow-datasource`.

## Where things live

- `src/datasource.ts` — `DataSource` class; `applyTemplateVariables` (interpolates
  filter values + ids), resource calls `getTables/getFields/getViews`.
- `src/types.ts` — `BaserowQuery`, `BaserowDataSourceOptions`, enums.
- `src/sort.ts` / `src/filter.ts` — pure, unit-tested logic (order_by string <->
  rows; filter model, type-aware operator catalog, interpolation, JSON persistence).
- `src/components/{ConfigEditor,QueryEditor,FilterEditor}.tsx` — UI.
- `pkg/plugin/datasource.go` — `QueryData` (records|count), `CheckHealth`,
  `CallResource` (`/tables`, `/fields`, `/views`).
- `pkg/plugin/client.go` — Baserow HTTP client (pagination, count via list).
- `pkg/plugin/filter.go` — `BuildFilters`: filter tree -> Baserow `filters` JSON.
- `pkg/plugin/frame.go` — records/count -> data frame; type inference, time parsing.
- `pkg/plugin/models.go` — settings + query parsing.
- `docker-compose.yaml`, `provisioning/` — local stack (empty Baserow + Grafana).

## Commands

Always finish with all four green:

```bash
yarn typecheck && yarn lint && yarn test && go test ./pkg/...
```

Builds: `yarn build:frontend` (frontend → `dist/module.js`),
`mage -v build:linuxARM64` (or other target; backend → `dist/gpx_baserow_*`).
Full stack: `docker compose up` (build `dist/` first).

## Invariants — DO NOT regress

1. **Filters compile server-side.** Editor persists JSON `filterTree`;
   `BuildFilters` makes the Baserow `filters` JSON tree
   (`{filter_type, filters:[{field,type,value}], groups:[...]}`). Rows are listed
   with `user_field_names=true`, so `field` is the field name.
2. **Preserve row order** in `recordsToFrame` — Baserow already returns rows in the
   query `order_by` order. Re-sorting rows was a real bug; never reintroduce it.
   Only columns may be reordered (time fields first).
3. **Data-plane frames**: records → `FrameTypeTable` (v0.1); count →
   `FrameTypeNumericWide` (v0.1). Date/DateTime → UTC `*time.Time`.
4. **Credentials are secret** (`secureJsonData`: `apiToken` / `password`); never
   log them or ship to the browser.
5. **Stable `@grafana/ui` only** — no `Combobox` (absent on older Grafana). Use
   `Select`/`MultiSelect`/`RadioButtonGroup`. Past regression.

## Baserow API quirks (cause silent wrong results)

- **Two auth modes hit different endpoints** — Baserow database tokens are only
  accepted on a subset of routes (check the OpenAPI `security` block before using
  a new endpoint in token mode):
  - `token`: **database token** (`Authorization: Token <token>`).
    - Tables → `GET /api/database/tables/all-tables/` (token-aware; filter by the
      optional Database ID client-side). NOT `/tables/database/{id}/` (JWT-only —
      this caused the original connect-time 401).
    - Ping/health → `GET /api/database/tokens/check/`. Database ID optional.
    - Views → unsupported (rejects tokens); `ListViews` returns empty.
    - Rows/fields/count work with the token.
  - `password`: email/password → JWT (`POST /api/user/token-auth/`,
    `Authorization: JWT <jwt>`). Client caches the JWT and retries once on 401
    (`doWithRetry`). Uses `/tables/database/{id}/` (Database ID required), lists
    databases via `/databases` → `ListDatabases`; editor shows a Database picker.
- List rows: `GET /api/database/rows/table/{table_id}/?user_field_names=true`
  returns `{count, next, previous, results}`. Page via `page`+`size` (max **200**).
- Sort = `order_by` (user field names, `-` prefix for desc); projection =
  `include`; filtering = the `filters` JSON tree (`filter_type` AND/OR).
- **No dedicated count endpoint** — Count reads `count` from a `size=1` list
  request. **No group-by/aggregation** — aggregate in Grafana via Transformations.
- Boolean filters use the `boolean` filter type with value `true`/`false`.

## Verifying end-to-end (Docker stack, anonymous admin)

`docker compose up` runs an **empty** self-hosted Baserow (no sample-data
seeding) + Grafana. Create a database/table and a database token (or use
email/password) in the Baserow UI, then auto-provision via env:

```bash
BASEROW_API_TOKEN=... BASEROW_DATABASE_ID=1 docker compose up
# or: BASEROW_AUTH_MODE=password BASEROW_EMAIL=... BASEROW_PASSWORD=... docker compose up
```

Then, with no auth:

```bash
UID=$(curl -s http://localhost:3000/api/datasources | node -e 'let d="";process.stdin.on("data",c=>d+=c).on("end",()=>console.log(JSON.parse(d)[0].uid))')
curl -s "http://localhost:3000/api/datasources/uid/$UID/resources/tables"
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d "{\"queries\":[{\"refId\":\"A\",\"datasource\":{\"uid\":\"$UID\",\"type\":\"yesoreyeram-baserow-datasource\"},\"queryType\":\"records\",\"tableId\":\"<id>\",\"sort\":\"-Age\"}],\"from\":\"now-2y\",\"to\":\"now\"}"
```

Inspect the response `results.A.frames[0].schema.fields[].type` to confirm time
fields are `time` and the row order matches the requested `sort`.

Baserow API is at http://localhost:8980. Provisioning is
`provisioning/datasources/baserow.yaml` (env-interpolated). For **Baserow Cloud**,
set Platform = cloud (forces `https://api.baserow.io`).
