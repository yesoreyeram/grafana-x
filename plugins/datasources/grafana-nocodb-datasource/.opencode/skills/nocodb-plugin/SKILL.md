---
name: nocodb-plugin
description: Develop and debug the Grafana NocoDB data source plugin in this repo. Use when changing the query/config editors, the filter builder, the Go backend (NocoDB client, where-clause compiler, data-frame conversion), the sample-data seeder, or when verifying behavior against the local Docker stack. Covers build/test commands, architecture, and NocoDB API quirks that cause subtle bugs.
license: Apache-2.0
---

# Grafana NocoDB Data Source — development skill

A Grafana **backend data source plugin** for NocoDB. Frontend (TypeScript/React)
renders config + query editors; Go backend calls the NocoDB REST API, compiles
filters, paginates, and builds Grafana data frames. Plugin id:
`yesoreyeram-nocodb-datasource`.

## Where things live

- `src/datasource.ts` — `DataSource` class; `applyTemplateVariables` (interpolates
  filter values + ids), resource calls `getTables/getFields/getViews`.
- `src/types.ts` — `NocoDBQuery`, `NocoDBDataSourceOptions`, enums.
- `src/sort.ts` / `src/filter.ts` — pure, unit-tested logic (sort string <->
  rows; filter model, type-aware operator catalog, interpolation, JSON persistence).
- `src/components/{ConfigEditor,QueryEditor,FilterEditor}.tsx` — UI.
- `pkg/plugin/datasource.go` — `QueryData` (records|count), `CheckHealth`,
  `CallResource` (`/tables`, `/fields`, `/views`).
- `pkg/plugin/client.go` — NocoDB HTTP client (v2 + v3, pagination, count).
- `pkg/plugin/filter.go` — `BuildWhere`: filter tree -> NocoDB `where`.
- `pkg/plugin/frame.go` — records/count -> data frame; type inference, time parsing.
- `pkg/plugin/models.go` — settings + query parsing.
- `scripts/seed.mjs`, `docker-compose.yaml`, `provisioning/` — local stack.

## Commands

Always finish with all four green:

```bash
npm run typecheck && npm run lint && npm test && go test ./pkg/...
```

Builds: `npm run build` (frontend → `dist/module.js`),
`mage -v build:linuxARM64` (or other target; backend → `dist/gpx_nocodb_*`).
Full stack: `docker compose up` (build `dist/` first).

## Invariants — DO NOT regress

1. **Filters compile server-side.** Editor persists JSON `filterTree`;
   `BuildWhere` makes the `where`. `@` quote prefix is **v2 only** (v3 rejects it).
2. **Preserve row order** in `recordsToFrame` — NocoDB already returns rows in the
   query `sort` order. Re-sorting rows was a real bug; never reintroduce it. Only
   columns may be reordered (time fields first).
3. **Data-plane frames**: records → `FrameTypeTable` (v0.1); count →
   `FrameTypeNumericWide` (v0.1). DateTime/Date → UTC `*time.Time`. Checkbox `1/0`
   stays numeric (don't coerce to bool).
4. **Token is secret** (`secureJsonData`/`xc-token`); never log it or ship to the
   browser.
5. **Stable `@grafana/ui` only** — no `Combobox` (absent on older Grafana). Use
   `Select`/`MultiSelect`/`RadioButtonGroup`. Past regression.

## NocoDB API quirks (verified live; cause silent wrong results)

- `in`/`anyof`/`allof`/`nanyof`/`nallof` take **unquoted** comma tokens:
  `(Status,in,open,closed)`. Quoting the whole value breaks matching.
- `btw`/`nbtw` are **not supported for Number** columns → use `ge`/`le`.
- Checkbox filters use `1`/`0`, not `true`/`false`.
- No group-by / aggregation endpoint — only filter-aware row `count`. Aggregate in
  Grafana via Transformations.
- v3 records live at `/api/v3/data/{baseId}/{tableId}/records` with a different
  response shape (`records[].fields`, `next` cursor).

## Verifying end-to-end (Docker stack, anonymous admin)

`docker compose up` seeds a `Sample` base (Customers, Metrics=time series, Logs,
Sales) and auto-provisions the datasource. Then, with no auth:

```bash
UID=$(curl -s http://localhost:3000/api/datasources | node -e 'let d="";process.stdin.on("data",c=>d+=c).on("end",()=>console.log(JSON.parse(d)[0].uid))')
curl -s "http://localhost:3000/api/datasources/uid/$UID/resources/tables"
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d "{\"queries\":[{\"refId\":\"A\",\"datasource\":{\"uid\":\"$UID\",\"type\":\"yesoreyeram-nocodb-datasource\"},\"queryType\":\"records\",\"tableId\":\"<m...>\",\"baseId\":\"<p...>\",\"sort\":\"-Age\"}],\"from\":\"now-2y\",\"to\":\"now\"}"
```

Inspect the response `results.A.frames[0].schema.fields[].type` to confirm time
fields are `time` and the row order matches the requested `sort`.

NocoDB API is at http://localhost:8080. The first signed-up user is super admin;
the seeder is idempotent.
