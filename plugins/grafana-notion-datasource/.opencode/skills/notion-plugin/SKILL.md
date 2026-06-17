---
name: notion-plugin
description: Develop and debug the Grafana Notion data source plugin in this repo. Use when changing the query/config editors, the filter builder, the Go backend (Notion client, JSON filter compiler, page flattener, data-frame conversion), or when verifying behavior against live Notion. Covers build/test commands, architecture, and Notion API quirks that cause subtle bugs.
license: Apache-2.0
---

# Grafana Notion Data Source — development skill

A Grafana **backend data source plugin** for Notion. Frontend (TypeScript/React)
renders config + query editors; Go backend calls the Notion REST API, compiles
filters to JSON, paginates by cursor, flattens typed page properties, and builds
Grafana data frames. Plugin id: `yesoreyeram-notion-datasource`.

## Where things live

- `src/datasource.ts` — `DataSource` class; `applyTemplateVariables` (interpolates
  filter values + ids), resource calls `getDatabases/getProperties`.
- `src/types.ts` — `NotionQuery`, `NotionDataSourceOptions`, enums.
- `src/sort.ts` / `src/filter.ts` — pure, unit-tested logic (sort string <->
  rows; filter model, type-aware operator catalog, interpolation, JSON persistence).
- `src/components/{ConfigEditor,QueryEditor,FilterEditor}.tsx` — UI.
- `pkg/plugin/datasource.go` — `QueryData` (records|count), `CheckHealth`,
  `CallResource` (`/databases`, `/properties`).
- `pkg/plugin/client.go` — Notion HTTP client (cursor pagination, search, retrieve).
- `pkg/plugin/filter.go` — `BuildFilter`: filter tree -> Notion JSON filter object.
- `pkg/plugin/frame.go` — records/count -> data frame; page flattening, type
  inference, time parsing.
- `pkg/plugin/models.go` — settings + query parsing.
- `docker-compose.yaml`, `provisioning/` — local stack (token from env).

## Commands

Always finish with all four green:

```bash
yarn typecheck && yarn lint && yarn test && go test ./pkg/...
```

Builds: `yarn build` (frontend → `dist/module.js`),
`mage -v build:linuxARM64` (or other target; backend → `dist/gpx_notion_*`).
Local stack: `NOTION_API_TOKEN=secret_... docker compose up` (build `dist/` first).

## Invariants — DO NOT regress

1. **Filters compile server-side.** Editor persists JSON `filterTree`;
   `BuildFilter` makes the Notion JSON filter object. List ops (`in`/`not_in`)
   expand into or/and groups (Notion has no native list operator).
2. **Pages are flattened.** `flattenProperty` reduces each typed property to a
   scalar before the frame builder. Add new property types there.
3. **Preserve row order** in `recordsToFrame` — Notion returns rows in the query
   `sorts` order. Only columns may be reordered (time fields first).
4. **Data-plane frames**: records → `FrameTypeTable` (v0.1); count →
   `FrameTypeNumericWide` (v0.1). Date strings → UTC `*time.Time`.
5. **Token is secret** (`secureJsonData`/`Authorization: Bearer`); never log it or
   ship to the browser.
6. **Stable `@grafana/ui` only** — no `Combobox` (absent on older Grafana). Use
   `Select`/`MultiSelect`/`RadioButtonGroup`.

## Notion API quirks (cause silent wrong results)

- Every request needs `Notion-Version` + `Authorization: Bearer`.
- **No count endpoint** — count paginates the query and counts results.
- **No "view" concept** — only databases (`/v1/databases/{id}/query`,
  `/v1/databases/{id}`) and the search endpoint (`/v1/search`, filter
  `{value:"database",property:"object"}`).
- **Cursor-only pagination** (`start_cursor`/`next_cursor`/`has_more`); no offset;
  `page_size` capped at 100.
- **No native list/`in` operator** — expand into or/and groups.
- Operators are property-type specific; the filter object nests the operator
  under a key named after the property type, e.g.
  `{"property":"Name","rich_text":{"equals":"x"}}`.
- Checkbox filter values are JSON booleans; numbers are JSON numbers (coerced in
  `coerceValue`).

## Verifying end-to-end (Docker stack, anonymous admin)

`NOTION_API_TOKEN=secret_... docker compose up` provisions the datasource. Then,
with no auth:

```bash
UID=$(curl -s http://localhost:3000/api/datasources | node -e 'let d="";process.stdin.on("data",c=>d+=c).on("end",()=>console.log(JSON.parse(d)[0].uid))')
curl -s "http://localhost:3000/api/datasources/uid/$UID/resources/databases"
curl -s -X POST http://localhost:3000/api/ds/query -H 'Content-Type: application/json' \
  -d "{\"queries\":[{\"refId\":\"A\",\"datasource\":{\"uid\":\"$UID\",\"type\":\"yesoreyeram-notion-datasource\"},\"queryType\":\"records\",\"databaseId\":\"<db...>\",\"sort\":\"-Created\"}],\"from\":\"now-2y\",\"to\":\"now\"}"
```

Inspect the response `results.A.frames[0].schema.fields[].type` to confirm date
fields are `time` and the row order matches the requested `sort`.
