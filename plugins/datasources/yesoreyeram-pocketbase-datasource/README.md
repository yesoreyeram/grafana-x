# PocketBase data source for Grafana

A Grafana **data source plugin with a Go backend** for
[PocketBase](https://pocketbase.io). Query your PocketBase **collections** —
records and fields — directly from Grafana, with a type-aware filter builder,
sorting, field selection and counts.

> Plugin id: `yesoreyeram-pocketbase-datasource`

## Features

- **Query types**
  - **Records** — list records (rows) from a collection.
  - **Count** — the number of records matching the filters.
- **Pickers** for collections and fields, populated live from the PocketBase API.
- **Type-aware filters** — a structured filter builder with nested AND/OR groups.
  Operators adapt to each field's type (text/number/bool/date) and are compiled
  server-side into a single
  [PocketBase filter expression](https://pocketbase.io/docs/api-records/#listsearch-records).
- **Raw filter** — an advanced escape hatch to paste a PocketBase filter
  expression directly.
- **Sorting** by one or more fields, **field selection** (`fields`), and a
  **limit** with transparent page-based pagination.
- **Three auth modes** — superuser, user (regular auth collection) or a
  pre-issued token.
- **Data-plane-compliant frames** with automatic type inference and time parsing
  (great for time series and tables).

## Data model mapping

| PocketBase | This plugin            |
| ---------- | ---------------------- |
| Collection | Collection picker      |
| Record     | Row in the data frame  |
| Field      | Column / filter field  |
| filter     | Filters / sort / limit |

The PocketBase system fields `id`, `created` and `updated` are promoted to the
front of every result frame.

## Authentication

PocketBase has no static API keys — access is via an auth token. This plugin
supports three modes:

| Mode | How it authenticates | Notes |
| ---- | -------------------- | ----- |
| **Superuser** (default) | `POST /api/collections/_superusers/auth-with-password` with the superuser email + password | Required to list collections/fields and to read records regardless of a collection's API rules. |
| **User** | `auth-with-password` against a regular auth collection (default `users`) | Access is constrained by each collection's `listRule`/`viewRule`. Collection/field pickers may be unavailable (superuser-only API). |
| **Token** | Sends a pre-issued token verbatim in the `Authorization` header | Useful for impersonate / long-lived tokens. |

The backend mints and caches the token, and transparently re-authenticates once
on a `401` (password modes).

## Configuration

| Field            | Description |
| ---------------- | ----------- |
| **URL**          | PocketBase base URL (no trailing `/api`), e.g. `http://127.0.0.1:8090`. |
| **Auth mode**    | `superuser` (default), `user`, or `token`. |
| **Identity**     | Superuser/user email (superuser/user modes). |
| **Auth collection** | The auth collection for user mode (defaults to `users`). |
| **Password**     | Account password (stored as a secret; superuser/user modes). |
| **Auth token**   | Pre-issued token (stored as a secret; token mode). |

## Local development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo. The local stack
runs a **real, self-hosted PocketBase** (so you can verify end to end), seeds it
with sample `demo` and `metrics` collections, and auto-provisions the datasource.

```bash
# from the monorepo root
yarn install

# build frontend + backend
yarn workspace @yesoreyeram/grafana-pocketbase-datasource build

# run Grafana + PocketBase + seed
docker compose -f plugins/datasources/yesoreyeram-pocketbase-datasource/docker-compose.yaml up
```

- Grafana runs with anonymous admin at <http://localhost:3000>.
- PocketBase runs at <http://localhost:8090> (admin UI at `/_/`).
- The seeded superuser is `admin@example.com` / `Password123!`.

## License

Apache-2.0. See [LICENSE](./LICENSE).
