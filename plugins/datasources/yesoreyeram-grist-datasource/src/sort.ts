export type SortDirection = 'asc' | 'desc';

export interface SortItem {
  field: string;
  direction: SortDirection;
}

export function parseSort(sort?: string): SortItem[] {
  if (!sort) {
    return [];
  }
  try {
    const parsed = JSON.parse(sort);
    if (Array.isArray(parsed)) {
      return parsed
        .filter((item) => item && typeof item.field === 'string')
        .map((item) => ({
          field: item.field,
          direction: item.direction === 'desc' ? 'desc' : 'asc',
        }));
    }
  } catch {
    // ignore malformed persisted state
  }
  return [];
}

export function serializeSort(items: SortItem[]): string {
  const valid = items.filter((item) => item.field.trim().length > 0);
  if (valid.length === 0) {
    return '';
  }
  return JSON.stringify(valid);
}
