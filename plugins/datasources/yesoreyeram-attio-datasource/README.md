# Grafana Attio Data Source

A Grafana data source plugin (with a Go backend) for querying records from
[Attio](https://attio.com) — the AI-native CRM — using the Attio REST API.

## Features

- **Backend data source** — queries run in the Grafana server, so the Attio
  access token never reaches the browser.
- **Records & Count query types** — return matching records, or just the count of
  matching records (filter-aware, ideal for stat panels).
- **Visual query editor** — object picker (People, Companies, Deals and any custom
  object), attributes (multi-select), a structured **filter builder** (type-aware
  operators, nested AND/OR groups), multi-field **sort**, limit, and offset.
  Objects and attributes are fetched live from Attio.
- **Server-side filter building** — the structured filter tree is compiled into an
  Attio JSON filter object on the backend.
- **Automatic value flattening** — Attio returns each attribute as an array of
  historical, deeply-typed value objects; the plugin flattens the latest active
  value of each attribute into a scalar column so it works directly in panels.
- **Automatic pagination** — follows Attio's offset/limit pagination to fetch all
  matching records (up to a safety cap) or up to a configured limit (500 records
  per request — Attio's maximum).
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with date/timestamp columns returned as real
  Grafana time fields for time-series panels. Complex values (e.g. location) are
  serialised as JSON.
- **Template variable support** in object, filter values and attributes.
- **Health check** validates connectivity and credentials via `/v2/self`.

## Requirements

- Grafana >= 10.4.0
- An Attio workspace and an [API key](https://developers.attio.com/) (access
  token) generated under **Settings > Developers > API keys**. The token needs at
  least the `record_permission:read` and `object_configuration:read` scopes.

## Configuration

Add a new **Attio** data source and configure:

| Field          | Required | Description                                                                                                  |
| -------------- | -------- | ------------------------------------------------------------------------------------------------------------ |
| API Token      | yes      | Attio workspace access token, sent as `Authorization: Bearer <token>`. Stored encrypted; never sent to browser. |
| API URL        | no       | Root URL of the Attio API. Defaults to `https://api.attio.com`. Override only to point at a proxy.            |
| Default Object | no       | Optional object api_slug (e.g. `people`). When set, the query editor selects it by default.                    |

## Querying

In the query editor, choose a **Query type** and an **Object**:

- **Records** — returns matching records. Supports **Attributes** (multi-select),
  a structured **Filters** builder (type-aware operators, nested filter groups),
  **Sort** (multi-field), **Limit** (`0` returns all, auto-paginated), and
  **Offset** (skip records).
- **Count** — returns the number of matching records (respects the filters). Handy
  for stat / single-value panels.

Filters are compiled into an Attio JSON filter object **server-side**, and filter
values support Grafana template variables. The operators offered adapt to each
attribute's type (text, number, date, boolean).

date/timestamp columns are returned as proper Grafana time fields, so date/time
columns work directly in time-series panels.

### Attribute-value flattening

Attio attribute values are not plain scalars. Each attribute slug maps to an
**array** of historical value objects, and each value object is a typed structure
(discriminated by `attribute_type`). For example a `currency` value looks like
`{"attribute_type":"currency","currency_value":99.5,"currency_code":"USD"}`.

The plugin flattens the **first (latest active)** value of each attribute to a
single scalar suitable for a table cell:

| Attio attribute type | Flattened to |
| -------------------- | ------------ |
| `text`               | the text value |
| `number`, `rating`   | the numeric value |
| `checkbox`           | the boolean value |
| `date`, `timestamp`  | the ISO date/timestamp string (parsed to a time field) |
| `currency`           | the `currency_value` number |
| `select`             | the option title (or option id) |
| `status`             | the status title (or status id) |
| `record-reference`   | the `target_record_id` |
| `actor-reference`    | the actor name, or `referenced_actor_id` |
| `email-address`      | the `email_address` |
| `phone-number`       | the `phone_number` |
| `domain`             | the `domain` |
| `personal-name`      | the `full_name` |
| `interaction`        | the `interacted_at` timestamp |
| anything else (e.g. `location`) | the value object serialised as JSON |

Every row also gets two synthetic columns: **`_record_id`** (the Attio record id)
and **`_created_at`** (when the record was created, a time field).

### Offset-based pagination

The records query endpoint (`POST /v2/objects/{object}/records/query`) uses
`limit` + `offset` pagination. `limit` is capped at **500** by Attio. When no
limit is set, the plugin fetches pages of 500 records until all matching records
are returned (capped at 100,000 for safety). Use the **Limit** field to cap
results and **Offset** to skip records.

### Filtering

Attio's filter format is a JSON object sent in the query POST body:

- Simple condition: `{"slug": {"$eq": "value"}}`
- Logical groups: `{"$and": [...]}`, `{"$or": [...]}`
- Operators: `$eq`, `$in`, `$contains`, `$starts_with`, `$ends_with`, `$gt`,
  `$gte`, `$lt`, `$lte`, `$not_empty`.

Attio has **no negative operators**, so the editor's "!=" (not equals) and
"is empty" operators are compiled by wrapping a positive condition in `$not`:

- `field != value` → `{"$not": {"field": {"$eq": value}}}`
- `field is empty` → `{"$not": {"field": {"$not_empty": true}}}`

Operator support varies by attribute type (for example `$contains` is only valid
on string-like attributes). The query editor only offers operators appropriate to
the selected attribute's type.

### Group by / aggregation

The Attio API does **not** expose a group-by or per-column aggregation endpoint
(only a filter-aware record count, surfaced as the Count query type). To group or
aggregate records in Grafana, return the records and use Grafana's
**Transformations** (e.g. *Group by*, *Reduce*, *Partition by values*).

## Limitations & notes

- **No native count endpoint.** Count is derived by paginating the matching
  records and counting them. Counting very large result sets issues multiple
  requests.
- **Historical values are not exposed.** Only the latest active value of each
  attribute is flattened. Multi-value attributes (e.g. multiple email addresses)
  surface only the first value.
- **Rate limits.** Attio rate-limits the API. Large unbounded queries and high
  dashboard refresh rates can be throttled (HTTP `429`). Prefer setting a
  **Limit** and a sensible panel refresh interval.
- **Lists vs objects.** This plugin queries **objects** (and their records). Attio
  *lists* and *list entries* are not yet exposed as a query type.
- **`path` filters and sorts** (drilling into related records) are not exposed by
  the visual builder; only attribute-level filters/sorts are supported.

## Troubleshooting

**`401` / unauthorized.** Re-enter the **API Token** and click *Save & test*.
Grafana stores secrets write-only, so saving the config with the token field left
blank blanks the token.

**`403` / forbidden.** Ensure the token has the `record_permission:read` and
`object_configuration:read` scopes.

**`429` / too many requests.** You are being rate-limited. Reduce the query
frequency, set a **Limit**, or increase the panel refresh interval.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup, architecture and
workflow. Quick reference:

```bash
yarn install                # at the monorepo root
yarn build                  # frontend + backend (all platforms) -> dist/
yarn build:frontend         # frontend only -> dist/module.js
yarn build:backend          # backend only -> dist/gpx_attio_* (mage buildAll)
yarn dev                    # frontend watch
yarn test                   # frontend unit tests
yarn lint                   # lint
yarn typecheck              # type check
go test ./pkg/...           # backend unit tests
```

`yarn build` requires Go and [Mage](https://magefile.org) on your PATH. Build
artifacts are written to `dist/`. Point Grafana's `plugins` path at this repo (or
symlink `dist/`) and set
`allow_loading_unsigned_plugins = yesoreyeram-attio-datasource` for local testing.

### Local stack (Docker Compose)

Attio is hosted SaaS, so the stack is just **Grafana** with the plugin mounted and
the datasource auto-provisioned. Build the plugin first, then start it with your
Attio token:

```bash
yarn build   # produce dist/ (frontend + backend)
ATTIO_API_TOKEN=your-token docker compose up
```

This brings up **Grafana** at http://localhost:3000 with **anonymous admin**
enabled, so no login is required. Omit the variable to skip auto-provisioning and
add the datasource manually in the Grafana UI. The provisioning file is
`provisioning/datasources/attio.yaml`;
`provisioning/datasources/attio.yaml.example` shows a manual variant.

## License

Apache-2.0
