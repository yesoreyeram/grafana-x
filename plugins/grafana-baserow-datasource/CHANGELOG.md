# Changelog

## Unreleased

### Authentication
- Added a second auth mode: **email & password** (JWT). The backend signs in at
  `/api/user/token-auth/`, caches the JWT, and auto-refreshes it on `401`.
- Password mode can list every accessible **database** (workspaces → databases),
  so the query editor shows a **Database** picker when no fixed database id is set
  (new `/databases` resource).
- Config editor gained an **Authentication** toggle: database token (with
  Database ID) or email & password (with optional default Database ID).

### Fixes
- **401 on connect with a database token (incl. Baserow Cloud)**: token mode used
  the `/tables/database/{id}/` and per-database health endpoints, which only accept
  a JWT — a database token is rejected there, returning
  `401 Authentication credentials were not provided`. Token mode now lists tables
  via the token-aware `all-tables` endpoint, validates via `tokens/check`, and
  treats the **Database ID as optional** (it only filters the table list). The
  **View** picker is disabled in token mode (the views endpoint rejects tokens).
- **401 / cross-host redirects**: re-attach the `Authorization` header when
  Baserow redirects to a different host (Go strips it on cross-host redirects),
  which previously surfaced as `401 Authentication credentials were not provided`.
- Clearer errors: a 401 now returns an actionable hint (auth-mode aware), an HTML
  response (wrong Base URL / web app) is explained, and whitespace-only secrets
  are reported as "not configured" instead of a confusing Baserow 401.
- Local stack: registered the internal host via `BASEROW_EXTRA_PUBLIC_URLS` so
  `/api/*` routes to the backend (no more "Site not found" HTML 404).

### Local stack
- Disabled Baserow's first-boot template sync
  (`BASEROW_TRIGGER_SYNC_TEMPLATES_AFTER_MIGRATION=false`) so the container starts
  in seconds instead of minutes.
- **Removed the sample-data seeder** (`scripts/seed.mjs` and the `baserow-seed`
  job). The local stack now runs an empty Baserow; the datasource is
  auto-provisioned from environment variables
  (`BASEROW_API_TOKEN`/`BASEROW_DATABASE_ID`, or `BASEROW_EMAIL`/`BASEROW_PASSWORD`
  with `BASEROW_AUTH_MODE=password`).
- Confirmed both **Baserow Cloud** (forces `https://api.baserow.io`) and
  **self-hosted** platforms are supported.

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Baserow REST API (database token kept server-side).
- Config editor: **Platform** (Baserow Cloud / self-hosted), **Base URL**, secure
  **API Token** (database token), and **Database ID**.
- Health check validates connectivity and credentials.

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- Live **Table**, **View** and **Fields** pickers (fetched from Baserow).
- Structured **filter builder** with type-aware operators and nested AND/OR
  groups, consistent with Grafana's inline-row UI.
- Multi-field **Sort** builder and **Limit**.
- Switching tables clears table-dependent options (view, filters, sort, fields).

### Backend
- Server-side filter compilation (`filterTree` → Baserow `filters` JSON tree).
- Rows requested with `user_field_names=true`; automatic pagination up to the
  requested limit (or a safety cap), 200 rows per request.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1).
- Column type inference (number / boolean / time / string); Date/DateTime parsed
  to UTC time fields for time-series panels. Row order is preserved so the query
  `order_by` is honoured.
- Template variable interpolation for filter values (list operators use `csv`),
  table, view, sort and fields.

### Tooling
- Local Docker stack (Baserow + Grafana with anonymous admin).
