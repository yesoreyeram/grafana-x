# Grafana Supabase Data Source

A Grafana data source plugin (with a Go backend) for querying data from
[Supabase](https://supabase.com) projects via the [PostgREST](https://postgrest.org)
API.

## Features

- **Backend data source** — the Supabase API key never reaches the browser. It
  is sent as **both** the `apikey` header **and** the `Authorization: Bearer`
  header (with the same key value) on every request, exactly as Supabase
  requires.
- **Records & Count query types** — return matching rows, or just the count of
  matching rows (filter-aware, ideal for stat panels).
- **Visual query editor** — table picker (fetched from the PostgREST OpenAPI
  schema), select columns, structured filter builder (PostgREST operators:
  `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `like`, `ilike`, `match`, `imatch`,
  `in`, `cs`, `cd`, `is null/true/false`, plus `not.` negation), multi-field
  sort, limit, and offset.
- **Server-side filter building** — the structured filter tree is compiled into
  correct PostgREST query parameters on the backend (implicit `AND`, explicit
  `or=(...)` groups with field-qualified conditions, `not.` negation, `in.(...)`
  lists, `is.null`).
- **Automatic pagination** — follows PostgREST `limit`/`offset` pagination to
  fetch all matching rows (up to a safety cap) or up to a configured limit.
- **Exact counts** via the `Prefer: count=exact` header, read from the
  `Content-Range` response header.
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string). Array/object columns are serialised as JSON.
- **Template variable support** in table, filter values and select columns.
- **Health check** validates connectivity and credentials.

## Requirements

- Grafana >= 10.4.0
- A Supabase project (or self-hosted PostgREST) with the **PostgREST API**
  enabled (default for Supabase).

## Configuration

Add a new **Supabase** data source and configure:

| Field            | Required | Description                                                                                     |
| ---------------- | -------- | ----------------------------------------------------------------------------------------------- |
| Project URL      | yes      | Supabase PostgREST endpoint. e.g. `https://<project-ref>.supabase.co/rest/v1` (or a self-hosted PostgREST URL). |
| Service Role Key | yes      | Supabase `anon` or `service_role` key (or a user JWT). Sent as **both** the `apikey` header and `Authorization: Bearer <key>`. Stored encrypted; never sent to the browser. |
| Schema           | no       | Optional Postgres schema, sent via the `Accept-Profile` header. Defaults to `public`.            |

> **anon vs service_role:** with the `anon` key, **row-level security (RLS)**
> policies are enforced, so you only see rows your policies allow. The
> `service_role` key bypasses RLS and can read everything — keep it server-side
> only (this plugin never exposes it to the browser).

## Querying

In the query editor, choose a **Query type** and a **Table**:

- **Records** — returns matching rows. Supports **Select** columns (comma-separated),
  a structured **Filters** builder (AND/OR groups), **Sort** (multi-field), **Limit**,
  and **Offset**.
- **Count** — returns the number of matching rows (respects filters).

### Filter operators

Each editor operator maps to a [PostgREST operator](https://postgrest.org/en/stable/references/api/tables_views.html#operators):

| Editor operator | PostgREST | Description                                              |
| --------------- | --------- | ------------------------------------------------------- |
| `=`             | `eq`      | Equal to                                                 |
| `!=`            | `neq`     | Not equal to                                             |
| `>`             | `gt`      | Greater than                                             |
| `>=`            | `gte`     | Greater than or equal                                    |
| `<`             | `lt`      | Less than                                                |
| `<=`            | `lte`     | Less than or equal                                       |
| `like`          | `like`    | LIKE pattern — use `*` or `%` as the wildcard            |
| `ilike`         | `ilike`   | Case-insensitive LIKE                                    |
| `matches regex` | `match`   | POSIX regex (`~`)                                        |
| `matches regex (ci)` | `imatch` | Case-insensitive POSIX regex (`~*`)                 |
| `in`            | `in`      | One of a comma-separated list, e.g. `a,b,c`             |
| `contains`      | `cs`      | Array/range contains (`@>`), e.g. `{a,b}`               |
| `contained in`  | `cd`      | Array/range contained in (`<@`), e.g. `{1,2,3}`         |
| `is null`       | `is.null` | Is NULL                                                  |
| `is not null`   | `not.is.null` | Is not NULL                                          |
| `is true` / `is false` | `is.true` / `is.false` | Boolean checks                         |

Logical structure compiles to PostgREST as follows: top-level conditions are
combined with **AND** (multiple query params); an **OR** group compiles to a
single `or=(col.op.value,...)` parameter (each condition is field-qualified);
nested groups compile to inline `and(...)`/`or(...)` expressions. Any operator
can be negated with the `not.` prefix.

### Group by / aggregation

PostgREST does **not** expose a general group-by or aggregation endpoint through
this plugin. To group or aggregate records in Grafana, return the records and use
Grafana's **Transformations** (e.g. *Group by*, *Reduce*, *Partition by values*).

## How it talks to Supabase

- **Auth.** Every request carries `apikey: <key>` and `Authorization: Bearer
  <key>` set to the same configured key. The optional schema is selected with the
  `Accept-Profile` header.
- **List records.** `GET /rest/v1/{table}?select=...&order=col.desc&limit=N&offset=M`
  with the compiled filter parameters. Both `200 OK` and `206 Partial Content`
  (returned for ranged reads) are treated as success.
- **Count.** A `HEAD /rest/v1/{table}` request with `Prefer: count=exact`; the
  total is read from the `Content-Range` response header (`0-24/3573` → `3573`).
- **Table discovery.** `GET /rest/v1/` returns the PostgREST OpenAPI document;
  the plugin reads table/view names from its `definitions` (and `paths`),
  excluding `rpc/` functions. If discovery fails the picker still accepts custom
  table names.

## Limitations

- **Table discovery depends on the OpenAPI schema.** Tables/views are read from
  `GET /rest/v1/`; RPC functions are excluded. The picker allows custom values if
  discovery is unavailable.
- **Exact counts can be slow** on very large tables, since `count=exact` performs
  a full count. (PostgREST also supports `count=planned`/`count=estimated`; this
  plugin uses `exact` for accuracy.)
- **Row-level security (RLS).** With the `anon` key, RLS policies apply. Use the
  `service_role` key to bypass RLS.
- **No server-side aggregation / group-by** (use Grafana transformations).

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md). Quick reference:

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn dev                    # frontend watch
go test ./pkg/...           # backend unit tests
```

### Local stack

Build the plugin first, then:

```bash
SUPABASE_API_URL=https://xxx.supabase.co/rest/v1 SUPABASE_SERVICE_KEY=eyJ... docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin (no login).

## License

Apache-2.0
