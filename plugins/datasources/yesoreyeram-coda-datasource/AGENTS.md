# AGENTS.md

Guidance for AI coding agents working in this repository. Humans should read
[CONTRIBUTING.md](./CONTRIBUTING.md); this file is the fast, factual map.

## What this is

A Grafana **data source plugin with a Go backend** for [Coda](https://coda.io).
Plugin id: `yesoreyeram-coda-datasource`. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to the Coda Web API,
builds a single-column filter, paginates, and converts results into Grafana data
frames.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getDocs/getTables/getColumns)
  types.ts                CodaQuery, CodaDataSourceOptions, DocInfo, TableInfo, ColumnInfo, enums
  components/
    ConfigEditor.tsx      API Token, Default Doc ID
    QueryEditor.tsx       Query type, [Doc picker], Table, Columns, Filter (single column), Advanced query, Sort by, Visible only, Value format, Limit
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData (rows|count), CheckHealth, CallResource (/docs /tables /columns)
    client.go             Coda HTTP client: bearer auth, ListDocs, ListTables, ListColumns, GetTable, ListRows, CountRows, flattenRows
    models.go             Settings (baseURL/apiToken/docId), QueryModel, LoadSettings/LoadQuery, DocInfo/TableInfo/ColumnInfo
    queries.go            Query type constants; valid sortBy / valueFormat sets
    filter.go             effectiveFilterQuery / buildColumnQuery: single-column `<col>:<value>` query builder
    frame.go              rowsToFrame / countToFrame: type inference, time parsing, data-plane contracts
provisioning/             Grafana provisioning (env-based datasource + example)
docker-compose.yaml       Grafana (anonymous admin) provisioned against coda.io (no local backing service)
Magefile.go               Backend build via grafana-plugin-sdk-go build targets
webpack.config.ts         Frontend build (self-contained)
```

## Commands

| Task                | Command |
| ------------------- | ------- |
| Install deps        | `yarn install` (run at monorepo root) |
| Build (front+back)  | `yarn build` |
| Frontend only       | `yarn build:frontend` |
| Backend only        | `yarn build:backend` |
| Frontend watch      | `yarn dev` |
| Typecheck           | `yarn typecheck` |
| Lint                | `yarn lint` |
| Frontend tests      | `yarn test` |
| Backend tests       | `go test ./pkg/...` |

Before declaring work done, run: `gofmt -l pkg/ && go vet ./pkg/... && go test ./pkg/...`.

## Key architecture facts (do not regress these)

- **Filtering is single-column only.** Coda's rows endpoint can only filter by one
  column. `pkg/plugin/filter.go::buildColumnQuery` compiles `FilterColumn` +
  `FilterValue` into a Coda `query` of the form `<col>:<value>` (column names are
  JSON-quoted, column ids `c-…` are not; values are JSON-encoded). A raw `Query`
  takes precedence. Do NOT reintroduce a full filter-tree — the Coda API cannot
  express it; richer filtering is documented as a Grafana-transformations job.
- **Rows come from a `values` map, not `cells`.** Coda returns
  `items:[{id,name,index,href,browserLink,createdAt,updatedAt,values:{col:val}}]`.
  The client always sends `useColumnNames=true`, so `values` is keyed by column
  name. `flattenRows` turns each row into a flat record keyed by column name plus
  synthetic metadata columns. (An older scaffold parsed `cells:[{column,value}]`
  — that shape does not exist; never go back to it.)
- **Column data type is `format.type`.** The resource-level `type` is always
  `"column"`; the real data type (text/number/checkbox/…) lives in `format.type`.
- **Count is rowCount-first.** Unfiltered count reads `GET /docs/{doc}/tables/{tbl}`
  `rowCount` (one request). Filtered count paginates the rows endpoint and counts.
- **Column projection is client-side.** The rows endpoint has NO columns/columnIds
  parameter; `flattenRows(items, keep)` drops unselected data columns after fetch.
- **Row order is preserved.** `rowsToFrame` must NOT re-sort rows. Only columns
  are reordered (time fields first).
- **Frames are data-plane compliant.** Rows → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). date/dateTime strings parse to UTC `*time.Time`
  fields. Array/object cell values are JSON-serialised to strings.
- **Synthetic columns.** Each row gets `id`, `name`, `index`, `createdAt`,
  `updatedAt`, `href`, `browserLink`; `orderedColumns` keeps these first, then
  data columns alphabetically (time columns are then pulled to the very front).
- **Template interpolation** runs in `datasource.ts::applyTemplateVariables`
  (doc, table, columns, filter column/value, raw query).

## Coda API specifics (verified against https://coda.io/developers/apis/v1)

- **Auth**: Bearer token — `Authorization: Bearer <token>`.
- **Base URL**: `https://coda.io/apis/v1` (SaaS). Resource paths are appended to
  the base, e.g. `baseURL + "/docs"`, so the base already includes `/v1`. Do NOT
  prefix paths with `/v1` again.
- **Pagination**: cursor via `pageToken`; responses carry `nextPageToken` /
  `nextPageLink`. Loop until `nextPageToken` is empty. `limit` is only a hint —
  the max page size is not guaranteed; rely on `nextPageToken`.
- **List rows**: `GET /docs/{docId}/tables/{tableIdOrName}/rows`
  params: `useColumnNames` (we send `true`), `valueFormat`
  (`simple` default / `simpleWithArrays` / `rich`), `query`, `sortBy`,
  `visibleOnly`, `limit`, `pageToken`. There is **no** `columns`/`columnIds`
  param.
- **query**: single column only — `<columnIdOrName>:<value>` (string values
  quoted; names quoted; value is JSON). NOT a free-text or multi-column search.
- **sortBy**: enum `createdAt` / `updatedAt` / `natural` only (NOT an arbitrary
  column). `natural` implies `visibleOnly=true`; sending `natural` with
  `visibleOnly=false` is a 400, so the client only ever sends `visibleOnly=true`.
- **Count**: no count endpoint. Table object (`GET /docs/{doc}/tables/{tbl}`) has
  `rowCount` (unfiltered total). For a filtered count, paginate the rows.
- **Schema**: `GET /docs` lists docs; `GET /docs/{docId}/tables` lists tables;
  `GET /docs/{docId}/tables/{tableIdOrName}/columns` lists columns (type in
  `format.type`).
- **Health**: `GET /whoami` validates the token.
- **Rate limits**: reading 100/6s, listing docs 4/6s, writes 10/6s. `429` errors
  are surfaced with a back-off hint.

## Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots. Regenerate with:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```
