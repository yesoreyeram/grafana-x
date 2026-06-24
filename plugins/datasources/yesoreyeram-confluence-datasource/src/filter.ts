// Confluence search uses CQL (Confluence Query Language) — a free-form query
// string rather than a structured filter object. These helpers keep the small
// amount of CQL-string handling the frontend performs in one tested place.
//
// See https://developer.atlassian.com/cloud/confluence/advanced-searching-using-cql/

/** Example CQL snippets surfaced in the query editor as guidance. */
export const CQL_EXAMPLES: string[] = [
  'type = page AND space = "ENG"',
  'text ~ "release notes"',
  'lastmodified >= now("-7d")',
  'creator = currentUser() ORDER BY created DESC',
];

/** Trim a CQL string before persisting it on the query. */
export function normalizeCQL(cql?: string): string {
  return (cql ?? '').trim();
}

/** Escape a raw value for safe inclusion in a double-quoted CQL string literal. */
export function escapeCQLValue(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

/** Build a CQL clause scoping a search to a single space key. */
export function spaceCQL(spaceKey?: string): string {
  const key = (spaceKey ?? '').trim();
  if (!key) {
    return '';
  }
  return `space = "${escapeCQLValue(key)}"`;
}
