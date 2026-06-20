export type SortDirection = 'asc' | 'desc';

export interface SortItem {
  attribute: string;
  direction: SortDirection;
}

/**
 * Parse the persisted sort (a JSON array of {attribute, direction}) into
 * structured sort items. Returns an empty list for empty/invalid input.
 */
export function parseSort(sort?: string): SortItem[] {
  if (!sort) {
    return [];
  }
  try {
    const parsed = JSON.parse(sort);
    if (Array.isArray(parsed)) {
      return parsed
        .filter((item) => item && typeof item.attribute === 'string')
        .map((item) => ({
          attribute: item.attribute,
          direction: item.direction === 'desc' ? 'desc' : 'asc',
        }));
    }
  } catch {
    // ignore malformed persisted state
  }
  return [];
}

/**
 * Serialize structured sort items into the persisted JSON string. Items without
 * an attribute are dropped; an empty result serializes to an empty string to
 * keep the query clean.
 */
export function serializeSort(items: SortItem[]): string {
  const valid = items.filter((item) => item.attribute.trim().length > 0);
  if (valid.length === 0) {
    return '';
  }
  return JSON.stringify(valid);
}
