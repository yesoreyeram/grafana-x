# Changelog

## 0.2.0

### Fixed

- **Pagination now works.** List queries previously made a single request and
  silently dropped every record beyond the first page. The backend now follows
  v1 offset pagination (`more_items_in_collection` / `next_start`) across pages.
- **Count is now correct.** The Count query previously requested `limit=1` and
  returned the length of the first page (always 0 or 1). It now paginates the
  chosen entity and returns the true total.
- **Person email/phone flattening.** Nested `email`/`phone` arrays flattened to
  the label (e.g. "work") instead of the actual value; they now flatten to the
  email/phone value.
- **Provisioning example.** `pipedrive.yaml.example` was a stray copy of the
  Airtable example; it now documents Pipedrive with both auth methods.
- Single-object list responses are now flattened consistently.

### Added

- **OAuth2 authentication.** In addition to API token (query parameter) auth,
  the plugin now supports OAuth2 access tokens (`Authorization: Bearer`),
  selectable in the config editor.
- **Custom field mapping.** 40-character custom-field hash keys are translated to
  their human-readable names via the `{entity}Fields` endpoints (deals, persons,
  organizations, products), including hash subfields like `{hash}_currency`.
  Toggleable per query.
- **Saved filter id** support (`filter_id`) for server-side filtering.
- **Count entity selector** — count any entity, not just deals.
- Health check moved to `GET /api/v1/users/me`.
- Golden data-frame tests and comprehensive client/frame/model unit tests.

## 0.1.0

- Initial release
- Support for Deals, Persons, Organizations, Products, and Count query types
- Pipeline, stage, user, and status filtering for deals
- Custom field filter builder with EQ, NEQ, GT, GTE, LT, LTE, LIKE, NOT_LIKE operators
- Sort by any field (ascending/descending)
- Offset-based pagination (start + limit)
- Company subdomain-based API URL configuration
- API token authentication (stored securely)
