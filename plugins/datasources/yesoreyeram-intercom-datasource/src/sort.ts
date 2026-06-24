export type SortDirection = 'asc' | 'desc';

export interface SortItem {
  field: string;
  direction: SortDirection;
}

/**
 * Parse a `sort` string into a structured sort item. The query editor encodes
 * sort as a single field name where a leading `-` marks a descending sort,
 * e.g. `-created_at`. Intercom supports a single sort field per query.
 */
export function parseSort(sort?: string): SortItem | undefined {
  if (!sort) {
    return undefined;
  }
  const token = sort.trim();
  if (token.length === 0) {
    return undefined;
  }
  if (token.startsWith('-')) {
    const field = token.slice(1).trim();
    return field.length > 0 ? { field, direction: 'desc' } : undefined;
  }
  return { field: token, direction: 'asc' };
}

/**
 * Serialize a structured sort item back into the `sort` string. Returns an empty
 * string when no field is set.
 */
export function serializeSort(item?: SortItem): string {
  if (!item || item.field.trim().length === 0) {
    return '';
  }
  return item.direction === 'desc' ? `-${item.field}` : item.field;
}
