# Changelog

## 0.1.0

Initial release.

### Connectivity & config
- Backend data source for the Attio REST API (access token kept server-side,
  sent as `Authorization: Bearer <token>`).
- Config editor: secure **API Token** (required), optional **API URL**
  (defaults to `https://api.attio.com`), optional **Default Object**.
- Health check via the `/v2/self` identify endpoint (verifies the token is
  active).

### Query editor
- **Query types**: Records and Count (filter-aware count for stat panels).
- Live **Object** and **Attributes** pickers, fetched from the Attio API
  (`/v2/objects` and `/v2/objects/{object}/attributes`).
- Structured **filter builder** with type-aware operators (text / number / date /
  boolean) and nested AND/OR groups, consistent with Grafana's inline-row UI.
- Multi-field **Sort** builder, **Limit**, and **Offset** (offset-based pagination).
- Switching object clears object-dependent options (filters, sort, attributes).

### Backend
- Server-side filter compilation (`filterTree` → Attio JSON filter object).
  Negative operators (`!=`, `is empty`) are compiled via `$not` since Attio has
  no native negation. List membership uses `$in`.
- Records fetched via `POST /v2/objects/{object}/records/query` with
  offset-based pagination (500 records per request — Attio's maximum) up to the
  requested limit (or a safety cap of 100,000).
- Count derived by paginating and counting matching records (Attio has no count
  endpoint).
- Attribute-value flattening: each attribute is an array of typed value objects;
  the latest active value is reduced to a scalar based on its `attribute_type`.
  Synthetic `_record_id` and `_created_at` columns are added.
- Data-plane-compliant frames: records → `table` (v0.1), count →
  `numeric-wide` (v0.1). Column type inference (number / boolean / time / string);
  date/timestamp parsed to UTC time fields for time-series panels. Complex values
  serialised as JSON. Row order is preserved so the query sort is honoured.
- Template variable interpolation for filter values, object and attributes.

### Tooling
- Local Docker stack (Grafana with anonymous admin) provisioning the datasource
  against the Attio API.
