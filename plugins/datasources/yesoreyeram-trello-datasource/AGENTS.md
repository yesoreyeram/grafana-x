# AGENTS.md

Guidance for AI coding agents working in this repository.

## What this is

A Grafana **data source plugin with a Go backend** for [Trello](https://trello.com).
Plugin id: `yesoreyeram-trello-datasource`. The frontend (TypeScript/React) renders
the config and query editors; the Go backend talks to Trello's **REST API v1**,
authenticates via API key + token query params, and converts results into Grafana
data frames. Trello is SaaS-only; the API base URL is fixed to `https://api.trello.com`.

## Layout

```
src/                      Frontend (TypeScript / React)
  module.ts               Plugin entry: wires ConfigEditor + QueryEditor + DataSource
  datasource.ts           DataSource class: applyTemplateVariables, resource calls (getBoards/getLists/getMembers/getLabels)
  types.ts                TrelloQuery, TrelloDataSourceOptions, BoardInfo/ListInfo/MemberInfo/LabelInfo
  components/
    ConfigEditor.tsx      API Key + API Token (both SecretInput, both required)
    QueryEditor.tsx       Query type (cards/count); board picker + list picker; card filter; member/label multi-selects; created date mode; fields; limit
  plugin.json             Plugin manifest
pkg/                      Backend (Go)
  main.go                 datasource.Manage entry
  plugin/
    datasource.go         QueryData, CheckHealth (/1/members/me), CallResource (/boards /lists /members /labels)
    client.go             REST client: do(), auth params (?key=&token=), ListBoards/ListLists/ListMembers/ListLabels; ListCards/CountCards (before-cursor pagination), iterateCards
    queries.go            Query type constants, card field catalog (flattened output columns)
    models.go             Settings (apiKey/apiToken), QueryModel (boardId/listId/cardFilter/memberIds/labelIds/createdMode+bounds/fields/limit)
    frame.go              cardsToFrame / countToFrame; flattenCard (labels/members/checklists→joined, badges→badges_* counts, dateCreated derived, customFieldItems→JSON)
provisioning/             Grafana provisioning (datasource example)
docker-compose.yaml       Grafana with the plugin + datasource provisioning (key + token from env)
Magefile.go               Backend build via grafana-plugin-sdk-go
```

## Commands

| Task                | Command |
| ------------------- | ------- |
| Build (front+back)  | `yarn build` |
| Frontend only       | `yarn build:frontend` |
| Backend only        | `yarn build:backend` |
| Typecheck           | `yarn typecheck` |
| Lint                | `yarn lint` |
| Frontend tests      | `yarn test` |
| Backend tests       | `go test ./pkg/...` |
| Local stack         | `docker compose up` (build `dist/` first; set `TRELLO_API_KEY`/`TRELLO_API_TOKEN`) |

## Key architecture facts

- **Auth is via query params**: every request adds `?key={apiKey}&token={apiToken}`.
  TWO separate fields — API key and API token — both stored in `secureJsonData`.
- **SaaS only**: the base URL is fixed to `https://api.trello.com`, not configurable.
- **Resource endpoints**: `/boards`, `/lists`, `/members`, `/labels` — all board-scoped
  (except boards, which is user-scoped via `/1/members/me/boards`).
- **Card filtering**: member/label filters are applied client-side in Go after fetching
  (Trello's card endpoints have no server-side member/label filter). The `filter`
  param (all/open/closed) is passed to the API.
- **Pagination is cursor-based, NOT offset.** Trello caps card results at 1000 per
  request and has no `offset`/`page` param. `iterateCards` walks pages with the
  `before` cursor: each page's oldest card id (derived from the id's embedded
  creation timestamp via `earliestCardID`/`cardCreatedUnix`) becomes the next
  `before`. A short page ends pagination. Do NOT reintroduce offset/page/`sort` —
  the cards endpoint supports none of them.
- **Count has no endpoint.** `CountCards` paginates with minimal fields
  (`id` + `idMembers`/`labels` only when those filters are active) and counts
  matches. Count ignores `Limit`.
- **Creation-date filter** (`createdMode`: any/dashboard/custom) maps to Trello's
  `since` (lower bound) / `before` (upper bound), which operate on creation date.
  The upper bound seeds the pagination cursor. `due`/`dateLastActivity` are NOT
  server-filterable.
- **Frames are data-plane compliant**: Cards → `FrameTypeTable` (v0.1); Count →
  `FrameTypeNumericWide` (v0.1). Time columns first; row order preserved (never
  re-sort rows).
- **Secrets stay on the server**: `apiKey`/`apiToken` are `secureJsonData`; never log
  them or send them to the browser. The request URL (which carries key+token) is
  never logged.
- **Rate limits**: 300 req/10s per key, 100 req/10s per token; a 429 surfaces as a
  query error.

## Conventions

- TypeScript: keep `@grafana/ui` components stable (Select/MultiSelect/RadioButtonGroup/Input/SecretInput).
- Go: format with `gofmt`; table-driven tests with `testify`; HTTP tested via `httptest`.
- Golden frame snapshots live in `pkg/plugin/testdata/*.jsonc`; regenerate with
  `UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames` and review the diff.
- Match existing code style; do not introduce new frameworks or build tooling.
