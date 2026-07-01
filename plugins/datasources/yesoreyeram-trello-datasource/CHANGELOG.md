# Changelog

## 0.1.0 (unreleased)

- Initial release
- Cards and Count query types
- Board/list/member/label resource endpoints
- API key + token authentication (both secrets, sent as query parameters)
- Cursor pagination over cards via the `before` creation-date cursor (Trello has
  no offset/page parameter; max 1000 per page)
- Count via minimal-field cursor pagination (no native count endpoint)
- Creation-date filter (any / dashboard range / custom) mapped to `since`/`before`
- Card flattening: labels/members/checklists joined, badge counts expanded to
  `badges_*` columns, derived `dateCreated`, `customFieldItems` as JSON
