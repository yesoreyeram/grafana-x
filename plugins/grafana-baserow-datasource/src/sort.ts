export type SortDirection = 'asc' | 'desc';

export interface SortItem {
  field: string;
  direction: SortDirection;
}

/**
 * Parse a Baserow `order_by` string into structured sort items. Baserow encodes
 * sort as a comma-separated list of field names where a leading `-` marks a
 * descending sort, e.g. `-CreatedAt,Title`.
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
 * Serialize structured sort items back into the Baserow `order_by` string. Items
 * without a field are dropped.
 */
export function serializeSort(items: SortItem[]): string {
  return items
    .filter((item) => item.field.trim().length > 0)
    .map((item) => (item.direction === 'desc' ? `-${item.field}` : item.field))
    .join(',');
}
