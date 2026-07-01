# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Confluence REST API (credentials kept server-side).
- Config editor: **Base URL**, **Authentication mode**, and mode-specific secrets.
- **Two authentication modes**, both fully supported:
  - **Basic** — Atlassian Cloud email + API token, sent as
    `Authorization: Basic base64(email:apiToken)`.
  - **Bearer** — OAuth2 access token or Data Center Personal Access Token, sent as
    `Authorization: Bearer <token>`.
- Health check validates connectivity and credentials (`GET /api/v2/spaces?limit=1`).

### Query editor
- **Query types**: Pages, Blog posts, Search (CQL), and Count.
- Live **Space** picker (fetched via the `/spaces` resource handler).
- **Sort** order selector for pages/blog posts, **Fields** column selection, and
  **Limit**.
- **CQL** textarea for search (and to narrow a count query).

### Backend
- v2 content endpoints (`/api/v2/pages`, `/api/v2/blogposts`, `/api/v2/spaces`)
  plus the v1 CQL search endpoint (`/rest/api/search`).
- Cursor-based pagination following the relative `_links.next` URL (resolved
  against the site origin) up to the requested limit (or a safety cap). Per-request
  page size capped at the API maximum (250).
- Content flattening: pages/blog posts reduced to scalar columns (id, title,
  spaceId, status, authorId, createdAt, version number/message/createdAt, webui);
  CQL results flattened to id, type, status, spaceId, title, excerpt, url,
  lastModified (highlight markers stripped). Links are made absolute.
- Count semantics: counts CQL search results when a CQL string is set, otherwise
  counts pages (scoped to the selected space).
- Data-plane-compliant frames: records → `table` (v0.1), count → `numeric-wide`
  (v0.1). ISO-8601 timestamps parsed to UTC time fields for time-series panels;
  numeric id strings preserved as strings. Row order is preserved.
- Atlassian error messages surfaced from both the v2 (`errors[]`) and v1
  (`message`) error envelopes.
- Template variable interpolation for spaceId, cql, sort, fields and cursor.

### Tooling
- Local Docker stack (Grafana with the plugin, datasource auto-provisioned from
  `CONFLUENCE_*` env vars, supporting both auth modes).
