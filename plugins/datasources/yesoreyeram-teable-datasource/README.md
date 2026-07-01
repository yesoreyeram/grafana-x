# Grafana Teable Data Source

A Grafana data source plugin (with a Go backend) for querying records from
[Teable](https://teable.io) tables using the
[Teable API](https://help.teable.ai/en/api-doc/overview).

## Features

- **Backend data source** — queries run in the Grafana server, so the Teable
  API token never reaches the browser.
- **Records & Count query types** — return matching records, or just the count of
  matching records (filter-aware, ideal for stat panels).
- **Visual query editor** — base ID, table picker, fields (multi-select), a structured
  **filter builder** (type-aware operators, nested AND/OR groups), multi-field
  **sort**, and limit. Tables and fields are fetched live from Teable.
- **Server-side filter building** — the structured filter tree is compiled into a
  Teable JSON `filter` object (`{conjunction, filterSet:[…]}`) on the backend and
  sent as the `filter` query parameter (not the deprecated `filterByTql`).
- **Offset-based pagination** — automatically pages through all matching records
  with Teable's `skip`/`take` parameters (page size capped at 1000) up to a safety
  cap, or up to a configured limit.
- **Accurate counts** — the Count query type uses Teable's dedicated
  `aggregation/row-count` endpoint, so no records are paginated to count.
- **Human-readable columns** — records are fetched with `fieldKeyType=name`, so
  columns are the real field names. Each row also carries the synthetic `_id`,
  `_createdTime` and `_lastModifiedTime` columns.
- **Data-plane-compliant frames** — automatic column type inference
  (number / boolean / time / string), with date/dateTime columns returned as real
  Grafana time fields for time-series panels. Array/object cells are serialised as
  JSON.
- **Template variable support** in base ID, table ID, view ID, filter values and
  fields.
- **Health check** validates connectivity and credentials via the
  `GET /api/auth/user` endpoint.

## Requirements

- Grafana >= 10.4.0
- A Teable [personal access token](https://help.teable.ai/en/api-doc/token) with
  access to the table(s) you want to query.

## Configuration

Add a new **Teable** data source and configure:

| Field           | Required | Description                                                                                          |
| --------------- | -------- | ---------------------------------------------------------------------------------------------------- |
| API Token       | yes      | Teable API token, sent as `Authorization: Bearer <token>`. Stored encrypted; never sent to browser.  |
| Server URL      | no       | Teable API base URL. Defaults to `https://app.teable.io` (cloud); set your own domain when self-hosted. |
| Default Base ID | no       | Optional base ID. When set, the query editor lists that base's tables without typing a base ID.       |

Generate a token from your Teable account settings (Settings → Personal access
tokens).

## Querying

In the query editor, choose a **Query type**, pick a **Table** (the **Base ID** is
only used to list tables; queries are addressed by table id):

- **Records** — returns matching records. Supports a structured **Filters**
  builder (type-aware operators, nested filter groups), **Sort** (multi-field),
  **Fields** (multi-select, compiled into the `projection` parameter), and a
  **Limit** (`0` returns all, auto-paginated).
- **Count** — returns the number of matching records (respects the filters). Handy
  for stat / single-value panels.

Each record frame includes synthetic `_id`, `_createdTime` and `_lastModifiedTime`
columns alongside the record's fields.

### Filtering

Filters are compiled into a Teable JSON `filter` object **server-side**:

```json
{ "conjunction": "and", "filterSet": [ { "fieldId": "Status", "operator": "is", "value": "Done" } ] }
```

Because the query uses `fieldKeyType=name`, the `fieldId` slot holds the field
**name**. The operators offered adapt to each field's type:

| Category        | Operators |
| --------------- | --------- |
| Text            | `is`, `isNot`, `contains`, `doesNotContain`, `isEmpty`, `isNotEmpty` |
| Number          | `is`, `isNot`, `isGreater`, `isGreaterEqual`, `isLess`, `isLessEqual`, `isEmpty`, `isNotEmpty` |
| Checkbox        | `is` |
| Date            | `is`, `isNot`, `isBefore`, `isAfter`, `isOnOrBefore`, `isOnOrAfter`, `isEmpty`, `isNotEmpty` |
| Single select   | `is`, `isNot`, `isAnyOf`, `isNoneOf`, `isEmpty`, `isNotEmpty` |
| Multiple select | `hasAnyOf`, `hasAllOf`, `hasNoneOf`, `isExactly`, `isEmpty`, `isNotEmpty` |
| Attachment      | `isEmpty`, `isNotEmpty` |

List operators (`isAnyOf`, `hasAnyOf`, …) take a comma-separated value that is
compiled into an array. Date operators compile the value into Teable's
`{ "mode": "exactDate", "exactDate": <value>, "timeZone": "UTC" }` shape. Filter
values support Grafana template variables.

Sorting is sent via the `orderBy` parameter (`[{"fieldId":"<name>","order":"asc"}]`).

Date/dateTime columns are returned as proper Grafana time fields, so they work
directly in time-series panels.

### Group by / aggregation

The plugin does not expose Teable's group-by endpoints. To group or aggregate
records in Grafana, return the records and use Grafana's **Transformations**
(e.g. *Group by*, *Reduce*, *Partition by values*).

## Finding IDs

- **Base ID** is visible in the Teable workspace URL — e.g.,
  `https://app.teable.io/base/{baseId}`.
- **Table ID** can be found using the query editor's table picker (fetched from the
  Teable API), or by inspecting the URL when viewing a table.

The query editor fetches tables and fields for you when the API token has access to
the base.

## API endpoints used

| Purpose      | Endpoint |
| ------------ | -------- |
| Health/Ping  | `GET /api/auth/user` |
| List tables  | `GET /api/base/{baseId}/table` (bare array) |
| List fields  | `GET /api/table/{tableId}/field` (bare array) |
| List records | `GET /api/table/{tableId}/record` (`fieldKeyType`, `take`, `skip`, `viewId`, `filter`, `orderBy`, `projection`) |
| Count        | `GET /api/table/{tableId}/aggregation/row-count` → `{ "rowCount": N }` |

## Troubleshooting

**`401` / unauthorized.** Re-enter the **API Token** and click *Save & test*.
Grafana stores secrets write-only, so saving the config with the token field left
blank blanks the token.

**`403` / forbidden.** The token is valid but lacks access to the table. Verify the
token has the necessary permissions in your Teable account settings.

**`404` / not found.** Verify the **Server URL** and the **Table ID** are correct
and the token can access them.

**`422` / unprocessable.** Check the filter conditions, sort fields and field
names.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full setup, architecture and
workflow.

## License

Apache-2.0
