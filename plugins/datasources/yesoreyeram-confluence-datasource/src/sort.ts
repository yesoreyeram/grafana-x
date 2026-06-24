export type SortDirection = 'asc' | 'desc';

export interface SortItem {
  field: string;
  direction: SortDirection;
}

/**
 * Parse a Confluence `sort` value into a structured item. Confluence sort orders
 * are a single token where a leading `-` marks descending, e.g. `-modified-date`
 * or `title`.
 */
export function parseSort(sort?: string): SortItem[] {
  if (!sort) {
    return [];
  }
  return sort
    .split(',')
    .map((token) => token.trim())
    .filter((token) => token.length > 0)
    .map((token) => {
      if (token.startsWith('-')) {
        return { field: token.slice(1).trim(), direction: 'desc' as const };
      }
      return { field: token, direction: 'asc' as const };
    })
    .filter((item) => item.field.length > 0);
}

/**
 * Serialize structured sort items back into the `sort` string. Items without a
 * field are dropped.
 */
export function serializeSort(items: SortItem[]): string {
  return items
    .filter((item) => item.field.trim().length > 0)
    .map((item) => (item.direction === 'desc' ? `-${item.field}` : item.field))
    .join(',');
}
