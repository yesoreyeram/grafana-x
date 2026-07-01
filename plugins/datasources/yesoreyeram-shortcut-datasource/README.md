# Shortcut Data Source for Grafana

Query stories from [Shortcut](https://shortcut.com) (formerly Clubhouse) directly within Grafana.

This is a Grafana **data source plugin with a Go backend**. The frontend (TypeScript/React)
renders the config and query editors; the Go backend talks to Shortcut's **REST API v3**,
runs story searches, follows pagination, flattens stories into scalars, and converts the
results into Grafana data frames.

## Configuration

1. Go to **Shortcut Settings → API Tokens** at
   [https://app.shortcut.com/settings/account/api-tokens](https://app.shortcut.com/settings/account/api-tokens).
2. Create a new API token.
3. Paste it into the **API Token** field in the Grafana data source config.

The token is sent on every request as the **`Shortcut-Token`** header (Shortcut's required
auth scheme — it is **not** `Authorization: Bearer`, and the deprecated `token` query
parameter is never used). The token is stored as a Grafana secret and never sent to the
browser.

The data source connects to `https://api.app.shortcut.com` and appends the `/api/v3` path.
Shortcut is a hosted SaaS, so the **API URL** is normally left at its default; override it
only to point at a proxy.

## Query types

| Type      | Description                                                          |
| --------- | ------------------------------------------------------------------- |
| `Stories` | Returns matching stories as a table.                                |
| `Count`   | Returns a single numeric value — the search `total` (one request).  |

## How stories are fetched

There is **no list-all stories endpoint** in the Shortcut API. Stories are retrieved with
the **search** endpoint:

```
GET /api/v3/search/stories?query=<shortcut-query>&page_size=25&detail=full
```

The structured filters in the query editor are compiled into a **Shortcut search query
string**, optionally combined with a raw search query you type in. For example, selecting
type *bug*, state *In Progress*, owner *alice* produces:

```
type:bug state:"In Progress" owner:alice
```

### Search query language

Shortcut search matches by **name** (or **mention name** for owners), not numeric id, and
combines all operators with **AND** (OR is not supported in search). Key operators used:

| Editor filter   | Operator      | Notes                                                        |
| --------------- | ------------- | ----------------------------------------------------------- |
| Story type      | `type:`       | feature / bug / chore                                        |
| Projects        | `project:`    | by name; multi-word names are quoted                        |
| Workflow states | `state:`      | by name                                                     |
| Epic            | `epic:`       | by name                                                     |
| Iteration       | `iteration:`  | by name                                                     |
| Labels          | `label:`      | by name                                                     |
| Owners          | `owner:`      | by **mention name** (no `@`)                                |
| Teams           | `team:`       | by team name                                                |
| Archived        | `is:archived` | "Only archived" → `is:archived`; "Exclude" → `!is:archived` |
| Created         | `created:`    | `YYYY-MM-DD..YYYY-MM-DD` (open sides use `*`)                |
| Updated         | `updated:`    | as above                                                    |
| Deadline        | `due:`        | as above                                                    |

Because search is AND-only, selecting multiple **single-valued** relations (projects,
workflow states, epic, iteration) typically yields **no results** — a story has only one of
each. Multiple **labels** or **owners** are meaningful (a story can have several), matching
stories that carry **all** of the selected values.

When no filters and no free text are provided, the backend sends `query=is:story`, which
matches all stories.

### Date filtering

Dates use Shortcut's `YYYY-MM-DD` precision. There are three modes:

- **Any time** — no date filter.
- **Dashboard range** — the panel's time picker is applied to one date field you choose
  (created / updated / deadline).
- **Custom** — explicit `after`/`before` bounds for created, updated and deadline. A
  timestamp is reduced to its date part; date terms such as `today`/`yesterday` pass through.

### Pagination (next-token)

The search response is `{ "data": [...], "next": "...", "total": N }`. `next` is a
**relative path and query string** that includes a page token, e.g.
`/api/v3/search/stories?query=...&next=<token>&page_size=25`. The backend resolves it
against the API host and follows it until `next` is `null`/empty, accumulating the
`data` arrays. A `Limit` caps the number of returned rows.

### Count via total

The `Count` query type issues a **single** search request (`page_size=1`) and reads the
`total` field from the response — no pagination needed.

## Returned columns

Stories are flattened to scalars: numbers stay numeric, booleans stay boolean, the known
timestamp fields (`created_at`, `updated_at`, `started_at`, `completed_at`, `deadline`,
`moved_at`) become time columns (UTC), and nested arrays/objects (`owner_ids`, `label_ids`,
`labels`, …) are preserved losslessly as compact **JSON** strings.

By default a curated catalog of columns is returned (id, name, story_type, description,
workflow_state_id, epic_id, iteration_id, project_id, group_id, requested_by_id, owner_ids,
label_ids, estimate, archived, started, completed, blocked, blocker, num_tasks_completed,
position, created_at, updated_at, started_at, completed_at, deadline, moved_at, app_url) so
the table stays readable instead of dumping every nested collection the API returns. Use
the **Fields** multi-select to choose a different subset (custom values are allowed, e.g.
`tasks`, `comments`).

Frames conform to the Grafana data plane contract: stories → `FrameTypeTable` (v0.1);
count → `FrameTypeNumericWide` (v0.1). Row order is preserved (the search ranking); time
columns are surfaced first.

## Template variables

`applyTemplateVariables` interpolates the raw query, the scalar filter inputs (epic,
iteration, date bounds), every multi-value list (projects, workflow states, labels, owners,
teams) and the selected fields. Multi-value variables expand as CSV and are split into a
flat list for the backend.

## Limitations

- **Search only** — there is no list-all stories endpoint; everything goes through
  `GET /api/v3/search/stories`.
- **1000-result cap** — Shortcut search returns at most 1000 results total, regardless of
  paging. Narrow the query for large workspaces.
- **page_size** — defaults to 25 per page; the API accepts 1–250.
- **AND-only search** — multiple single-valued relations (project/state/epic/iteration)
  rarely match together; OR is not available in search.
- **Names, not IDs** — search matches by name (mention name for owners), so the editor
  dropdowns store names; renaming an entity in Shortcut changes what a saved query matches.
- **Archived** — `is:archived` returns only archived stories; use "Exclude archived" for
  `!is:archived`.
- **Rate limit** — Shortcut limits the API to 200 requests/minute.

## Development

```bash
# Install dependencies (run once at the monorepo root)
yarn install

# Build frontend + backend
yarn build

# Frontend only / backend only
yarn build:frontend
yarn build:backend

# Watch mode
yarn dev

# Quality gates
yarn typecheck
yarn lint
yarn test            # frontend (jest)
go test ./pkg/...    # backend (go)
```

### Golden data-frame tests

`pkg/plugin/testdata/*.jsonc` are golden snapshots of the produced data frames. Regenerate
intentionally and review the diff:

```bash
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
git diff pkg/plugin/testdata
```
