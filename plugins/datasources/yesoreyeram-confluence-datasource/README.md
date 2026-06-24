# Grafana Confluence Data Source

A Grafana **data source plugin with a Go backend** for
[Confluence](https://www.atlassian.com/software/confluence) (Atlassian docs/wiki).
Query your Confluence content as Grafana data frames: list pages and blog posts,
run **CQL** searches, and count matching items.

- Plugin id: `yesoreyeram-confluence-datasource`
- Frontend: TypeScript / React (config + query editors)
- Backend: Go (Confluence REST client, cursor pagination, content flattener, frame builder)

## Features

- **Backend data source** — credentials are stored server-side and never sent to
  the browser.
- **Two authentication modes** (both fully supported):
  - **Basic** — Atlassian Cloud account **email + API token**, sent as
    `Authorization: Basic base64(email:apiToken)`.
  - **Bearer** — OAuth2 access token or a Data Center **Personal Access Token**,
    sent as `Authorization: Bearer <token>`.
- **Query types**: Pages, Blog posts, Search (CQL), and Count.
- Live **Space** picker, fetched from the spaces visible to your credentials.
- Cursor-based pagination is followed automatically up to your limit.
- Confluence content is flattened to scalar columns (id, title, spaceId, status,
  authorId, createdAt, version number/message/createdAt, webui link).
- Data-plane-compliant frames; ISO-8601 timestamps become time fields for
  time-series panels, and row order is preserved (the sort / CQL ordering is
  honoured).

## Setup

### Atlassian Cloud (Basic auth — recommended)

1. Create an API token at
   <https://id.atlassian.com/manage-profile/security/api-tokens>.
2. In Grafana, add a **Confluence** data source.
3. Set **Base URL** to `https://your-site.atlassian.net/wiki` (include `/wiki`).
4. Choose **Basic (email + API token)**, enter your Atlassian **email** and the
   **API token**.

### OAuth2 / Data Center (Bearer auth)

1. Obtain an OAuth2 access token, or create a **Personal Access Token** in a
   Confluence Data Center instance.
2. Set **Base URL** to your wiki base (e.g. `https://your-site.atlassian.net/wiki`
   for Cloud, or your Data Center URL).
3. Choose **Bearer (OAuth2 / PAT)** and paste the token.

## Configuration

| Field          | Description |
| -------------- | ----------- |
| Base URL       | Root URL of the wiki. Cloud: `https://your-site.atlassian.net/wiki` (include `/wiki`). The v2 API path (`/api/v2/...`) and the CQL search path (`/rest/api/search`) are appended to this base. |
| Authentication | `Basic` (email + API token) or `Bearer` (OAuth2 / PAT). |
| Email          | Atlassian account email (Basic auth only). |
| API Token      | Atlassian API token (Basic auth). Stored as a secret. |
| Token          | OAuth2 access token / PAT (Bearer auth). Stored as a secret. |

## Querying

1. Pick a **Query type**:
   - **Pages** — `GET /api/v2/pages` (optionally scoped to a space).
   - **Blog posts** — `GET /api/v2/blogposts` (optionally scoped to a space).
   - **Search (CQL)** — `GET /rest/api/search?cql=...`.
   - **Count** — number of matching pages (or CQL results when a CQL string is set).
2. For Pages/Blog posts, optionally choose a **Space**, a **Sort** order, the
   **Fields** (columns) to return, and a **Limit** (0 = all, auto-paginated).
3. For Search, enter a **CQL** query, e.g.
   `type = page AND space = "ENG" AND text ~ "release notes"`.

### Returned columns

**Pages / Blog posts** flatten to: `createdAt`, `versionCreatedAt` (time
fields), `id`, `title`, `spaceId`, `status`, `authorId`, `versionNumber`,
`versionMessage`, `webui` (a clickable link).

**Search** flattens to: `lastModified` (time), `id`, `type`, `status`,
`spaceId`, `title`, `excerpt`, `url`. Search highlight markers (`@@@hl@@@`) are
stripped.

## CQL search

[CQL](https://developer.atlassian.com/cloud/confluence/advanced-searching-using-cql/)
(Confluence Query Language) is a free-form query string passed straight through
to the search endpoint. Examples:

```
type = page AND space = "ENG"
text ~ "release notes"
lastmodified >= now("-7d")
creator = currentUser() ORDER BY created DESC
```

## Notes & limitations

- **CQL is only available on the search endpoint** (`/rest/api/search`, the v1
  search API). The v2 pages/blog posts endpoints filter via the `space-id`,
  `sort`, `status` etc. query parameters, not CQL.
- **Page/blog post bodies are not expanded by default.** Only metadata and the
  latest version summary are returned. Fetching rendered body content would
  require `body-format` and is intentionally omitted to keep frames tabular.
- **Pagination is cursor-based.** Confluence returns a relative `_links.next`
  URL containing a `cursor` parameter; the backend resolves it against the site
  origin and follows it until exhausted (or your limit is reached). The
  per-request page size is capped at 250 by the API.
- **Cloud vs Data Center.** This plugin targets the Confluence **Cloud** v2 REST
  API plus the v1 CQL search endpoint. Data Center exposes a different (v1) API
  surface; Bearer (PAT) auth and the CQL search endpoint work there, but the v2
  pages/blog posts listings may not. Set the Base URL accordingly.
- **Rate limits.** Atlassian applies per-tenant rate limits and may return
  `429`. The plugin relies on the SDK HTTP client; keep limits reasonable.
- **IDs are strings.** Space/page ids are numeric strings and are kept as strings
  (never coerced to numbers or timestamps).
- Aggregations beyond row count are not provided — use Grafana Transformations.

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. From this
directory:

```bash
yarn build           # frontend + backend (all platforms) -> dist/
yarn build:frontend  # frontend only -> dist/module.js
yarn build:backend   # backend only -> dist/gpx_confluence_* (mage buildAll)
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
CONFLUENCE_URL=https://your-site.atlassian.net/wiki \
CONFLUENCE_AUTH_MODE=basic \
CONFLUENCE_EMAIL=you@example.com \
CONFLUENCE_API_TOKEN=xxxx \
docker compose up
```

Grafana runs at <http://localhost:3000> with anonymous admin and the Confluence
data source auto-provisioned from the `CONFLUENCE_*` environment variables.

## License

[Apache-2.0](./LICENSE) — version %VERSION%, updated %TODAY%.
