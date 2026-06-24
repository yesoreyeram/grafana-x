# Changelog

## 1.0.0 (unreleased)

- Initial release.
- Records query type: lists rows from a Grist table.
  - Simple equality/membership filters use the fast records endpoint
    (`filter={"Col":["v1","v2"]}`); richer operators (`!=`, `>`, `<`, `contains`,
    OR logic, nested groups) compile to a **parameterized** Grist SQL `WHERE`
    clause (no SQL injection).
  - Multi-column sort via the records `sort` csv (`Name,-Age`); field projection
    via SQL (the records endpoint has no projection param).
  - `limit` caps results; `0` returns all rows. The Grist records endpoint has
    **no offset/cursor pagination**.
- Count query type: derived from a SQL `SELECT COUNT(*)` (Grist has no count
  endpoint).
- SQL query type: run a raw read-only Grist SQL `SELECT` against the document.
- Date/DateTime handling: Grist returns those columns as Unix **epoch seconds**;
  the backend classifies them from column metadata (`/columns` → `fields.type`)
  and converts them to UTC time fields (no string sniffing).
- Resource discovery: documents (enumerated via orgs → workspaces), tables, and
  columns (label + type).
- Data-plane compliant frames: records → `FrameTypeTable` (v0.1); count →
  `FrameTypeNumericWide` (v0.1). Nullable pointer fields; time fields first.
- Visual filter editor with type-aware operators and AND/OR groups.
