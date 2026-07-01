# Grafana Todoist Data Source

A Grafana **data source plugin with a Go backend** for [Todoist](https://todoist.com).
Query your active tasks as Grafana data frames, scoped by project, section, label
or a Todoist filter query.

- Plugin id: `yesoreyeram-todoist-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (Todoist API client, cursor paginator, entity flattener, frame builder)
- Cloud only — Todoist is hosted SaaS; there is no local/self-hosted mode.

## API version

This plugin targets the **Todoist unified API v1** (`https://api.todoist.com/api/v1`),
which supersedes the older REST v2 API (`https://api.todoist.com/rest/v2`). v1 is
the current, GA API and is the right choice because:

- It adds **cursor-based pagination** to the list endpoints (`limit` + `cursor`,
  responses shaped `{ "results": [...], "next_cursor": "..." }`). REST v2's
  `GET /rest/v2/tasks` returned **all** active tasks in a single un-paginated
  array and silently ignored `offset`/`limit`, so offset paging against it loops
  and duplicates rows.
- Filtering moved to a dedicated endpoint: `GET /api/v1/tasks/filter?query=...`
  (the `filter`/`lang` parameters were removed from `/api/v1/tasks`).
- Object fields were renamed to match the Todoist app (see
  [field reference](#task-fields) below).

## Features

- **Backend data source** — the API token is stored server-side and never sent
  to the browser.
- **Query types**: Tasks (task records) and Count (a single count of matching
  tasks).
- **Cascading pickers**: **Project → Section**, plus a **Label** picker (by
  name) and an optional **Parent task** scope. Selecting a project refreshes the
  section list.
- **Filter** field accepting Todoist's
  [filter query language](https://todoist.com/help/articles/introduction-to-filters-V98wIH)
  (e.g. `today | overdue`, `#Work & p1`, `@urgent & !recurring`), with an
  optional **Filter language** override. When set, the filter routes to
  `/api/v1/tasks/filter` and overrides the project/section/label/parent scope.
- **Cursor pagination** — the backend transparently follows `next_cursor` (200
  per page) up to the requested limit (or a safety cap).
- Todoist's nested `due` object is flattened into columns: `dueDate` (time),
  `dueString`, `dueIsRecurring` (bool) and `dueTimezone`. `deadline` →
  `deadlineDate` (time); `duration` → `durationAmount` / `durationUnit`;
  `labels` → a JSON string array.
- Data-plane-compliant frames; ISO-8601 date fields (`added_at`, `updated_at`,
  `completed_at`, `dueDate`, `deadlineDate`) become time fields for time-series
  panels, and row order honours the API ordering.

## Setup

1. In Todoist, open **Settings → Integrations → Developer** and copy your **API
   token**.
2. In Grafana, add a **Todoist** data source and paste the token.

The API URL defaults to `https://api.todoist.com/api/v1` (Todoist is SaaS only;
the base is overridable via provisioning only, to point at a proxy/gateway).

## Configuration

| Field     | Description |
| --------- | ----------- |
| API Token | Your Todoist API token, sent as `Authorization: Bearer`. Create one at **Todoist Settings → Integrations → Developer**. |

## Querying

1. Choose a **Query type**: Tasks (returns task records) or Count (returns a
   single count).
2. Optionally pick a **Project**, **Section** (requires a project), **Label**
   (by name) or **Parent task ID** to scope the active tasks.
3. Or enter a **Filter** query in Todoist's filter syntax (this overrides the
   scope above) and, if needed, a **Filter language**.
4. Set a **Limit** (0 = all, auto-paginated up to a safety cap of 100,000). For
   Count this is the **Max task scan** cap.

### Task fields

Tasks are returned with the Todoist v1 field names (renamed from REST v2):

| v1 field | Notes (was in REST v2) |
| --- | --- |
| `id`, `content`, `description`, `priority` | unchanged |
| `checked` (bool) | was `is_completed` |
| `added_at` (time) | was `created_at` |
| `added_by_uid` | was `creator_id` |
| `responsible_uid` | was `assignee_id` |
| `assigned_by_uid` | was `assigner_id` |
| `child_order` (number) | was `order` |
| `note_count` (number) | was `comment_count` |
| `project_id`, `section_id`, `parent_id` | unchanged |
| `labels` | JSON string array |
| `dueDate`/`dueString`/`dueIsRecurring`/`dueTimezone` | flattened from `due` |
| `deadlineDate` | flattened from `deadline` (new in v1) |
| `durationAmount`/`durationUnit` | flattened from `duration` |

> The `url` field was **removed** in v1. A task's web URL is
> `https://app.todoist.com/app/task/<id>`.

### Priority inversion

Todoist's API priority is **inverted** versus the app UI: in the API,
`priority: 4` is the **highest** (shown as **p1** in the UI) and `priority: 1`
is normal/none (no flag in the UI). Keep this in mind when filtering or coloring
on `priority`.

### Notes & limitations

- **Active tasks only.** `GET /api/v1/tasks` and `/api/v1/tasks/filter` return
  only active (incomplete) tasks. Completed tasks live behind separate endpoints
  (`/api/v1/tasks/completed/by_completion_date` and `.../by_due_date`) and are
  not surfaced by this plugin.
- **No native count endpoint.** Count queries paginate the matching tasks and
  count them server-side, capped by the **Max task scan** value.
- **Filter is exclusive.** The Todoist API cannot combine `/tasks/filter` with
  the id-based scope, so a non-empty Filter ignores project/section/label/parent.
  Express those constraints inside the filter query instead (e.g.
  `#Work & @urgent`).
- **Label filters by name.** The `label` parameter matches the label **name**,
  not its ID — the picker stores the name accordingly.
- **Rate limits.** Todoist allows ~1000 partial-sync and ~100 full-sync requests
  per 15-minute window per user, with a 15-second per-request timeout.
- Aggregations beyond row count are not provided — use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_todoist_* (mage buildAll)
yarn dev             # frontend watch
yarn typecheck
yarn lint
yarn test            # frontend tests
go test ./pkg/...    # backend tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH.

### Local stack

```bash
yarn build   # produce dist/ (frontend + backend)
TODOIST_API_TOKEN=0123... docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin and the Todoist data
source auto-provisioned from `TODOIST_API_TOKEN`.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
