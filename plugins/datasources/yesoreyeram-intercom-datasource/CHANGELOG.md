# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Intercom REST API (access token kept server-side).
- Config editor: **Region** (US/EU/AU), optional **API URL** override,
  **Intercom-Version** header (default `2.11`), and secure **Access Token**.
- Health check validates connectivity and credentials (`GET /me`).

### Query editor
- **Query types**: conversations, contacts, tickets, articles, companies,
  admins, teams, tags, and count.
- Structured pickers: conversation **state**, contact **role**, admin
  **assignee**, **team** assignee and **tag** (admins/teams/tags fetched live
  via resource handlers).
- Generic **filter builder** (field / operator / value rows) compiled into the
  Intercom Search API `query` object, with all search operators
  (`=`, `!=`, `>`, `<`, `~`, `!~`, `^`, `$`, `IN`, `NIN`).
- Free-text **search**, **sort** (field + direction) and **limit**.

### Backend
- Per-entity endpoint selection: list endpoints for conversations/contacts when
  unfiltered, the Search API when filtered; tickets search-only (with a
  match-all fallback); simple lists for admins/teams/tags.
- Cursor pagination following `pages.next.starting_after` (search via the body
  `pagination`, lists via the `starting_after`/`page` query params) up to the
  requested limit or a safety cap (`per_page` capped at 150).
- Intercom record flattening: scalar fields preserved, nested objects/arrays
  (`source`, `assignee`, `contacts`, `tags`, `custom_attributes`, …) serialised
  to compact JSON strings, a synthetic id added when absent.
- **Unix epoch-seconds → time fields**: timestamp fields (`*_at`,
  `snoozed_until`, `waiting_since`) converted to UTC time; `0` treated as null.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1). Column type inference (number / boolean / time /
  string); row order preserved so the query sort is honoured.
- Intercom error envelope (`{"type":"error.list","errors":[…]}`) surfaced in
  error messages.
- Template variable interpolation for pickers, search text, sort and filter
  values (list operators use `csv`).

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `INTERCOM_API_TOKEN` / `INTERCOM_BASE_URL` / `INTERCOM_VERSION`).
- Golden data-frame tests for the data-plane contract.
