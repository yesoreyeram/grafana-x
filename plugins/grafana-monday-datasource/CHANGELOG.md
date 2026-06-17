# Changelog

## Unreleased

### Items
- New **Group by / aggregation** for items — aggregate items by a board column
  (e.g. **tasks by status**, **tasks by owner**) using monday.com's **server-side
  `aggregate` query** (no raw items downloaded; ~150x cheaper in API complexity).
  Pick a **Group by** column and an **Aggregation**: **count** (default), **count
  distinct**, **sum**, **average**, **min** or **max**. Numeric aggregations take
  a **Value column**. Name/group filters are pushed down to the aggregate query.
  Results are one row per distinct group value, sorted by the aggregation result;
  empty groups bucket into `(empty)`. When multiple boards are selected, one
  aggregate call runs per board and a `board_id` column is added. Group values are
  returned as monday's raw values (status colour, person id, etc.).
- The plugin now pins the monday **API-Version** to a recent default (`2026-01`)
  when none is configured, since the `aggregate` query (used for grouping) only
  exists on recent versions — older versions reject it outright. The group column
  is aliased to its `column_id` (required by monday) and group values are parsed
  flexibly across API response shapes.
- Grouping now returns a **clear, actionable error** when the configured API
  version does not expose the `aggregate` query (e.g. `Cannot query field
  "aggregate"`), telling you to set a newer API version (2026-01 or later) or
  remove the Group by — instead of a raw GraphQL schema error.
- The **Columns** selector now actually restricts the returned columns: the
  selected column ids are passed to monday's `column_values(ids: [...])` so only
  those columns appear in the result (previously the selection was ignored and
  all columns were returned).
- New **Hide system columns** toggle (shown when including column values) that
  omits monday's built-in/system columns (subitems, last updated, creation log,
  formula, button, progress, item id, auto number).
- **Checkbox columns are now boolean fields** — the column `type` is read and
  checkbox values are converted to `true`/`false` instead of the raw `"v"`/empty
  text, so boolean panels and transformations work.

### Query editor
- Fixed field-label alignment: the Boards and Columns multi-selects no longer use
  `grow`, so their labels line up with the rest of the item filter rows.

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the monday.com GraphQL API (credential kept server-side).
- Config editor: **GraphQL URL**, optional **API version** (sent as the
  `API-Version` header), **Authentication** method (Personal API token or OAuth
  token), and the corresponding secure credential field.
- Personal API tokens are sent raw in the `Authorization` header; OAuth tokens are
  sent as `Authorization: Bearer`.
- Health check validates connectivity and credentials via the `me` query.

### Query editor
- **Query types**: Items, Boards, Groups, Users, Workspaces, Tags, and Raw GraphQL.
- Live **Board**, **Group**, **Column** and **Workspace** pickers (fetched from the
  account).
- **Items** filters: boards (required), groups, name contains, column selection,
  include-column-values toggle, order-by column with direction.
- **Boards** filters: workspaces and lifecycle state.
- **State** (active / all / archived / deleted) and **Limit** for the paged types.
- Raw GraphQL mode with optional JSON variables.

### Backend
- GraphQL client with cursor-based item pagination (`items_page` /
  `next_items_page`) and page/limit pagination for boards, users and workspaces,
  up to the requested limit (or a safety cap).
- Server-side `ItemsQuery` assembly (name `contains_text` rule, group `any_of`
  rule, column order_by).
- Item flattening: each item's `column_values` are lifted into top-level columns
  keyed by the column title (using the human-readable `text`); nested relations
  (`group`, `board`, `workspace`, owners) are reduced to scalar columns.
- Raw queries: the first (deepest) array of objects anywhere in the response is
  found and flattened; otherwise the top-level object becomes one row.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); timestamp fields are
  parsed to UTC time fields for time-series panels. Row order is preserved so the
  query ordering is honoured.

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `MONDAY_API_TOKEN`).
