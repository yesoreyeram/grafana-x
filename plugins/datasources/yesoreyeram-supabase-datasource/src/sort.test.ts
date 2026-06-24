import { parseSort, serializeSort } from './sort';

describe('sort persistence', () => {
  it('parses empty/invalid input to an empty list', () => {
    expect(parseSort()).toEqual([]);
    expect(parseSort('')).toEqual([]);
    expect(parseSort('not-json')).toEqual([]);
  });

  it('parses structured sort items and defaults the direction to asc', () => {
    expect(parseSort('[{"field":"name","direction":"desc"},{"field":"age"}]')).toEqual([
      { field: 'name', direction: 'desc' },
      { field: 'age', direction: 'asc' },
    ]);
  });

  it('drops malformed entries', () => {
    expect(parseSort('[{"direction":"desc"},{"field":"name","direction":"asc"}]')).toEqual([
      { field: 'name', direction: 'asc' },
    ]);
  });

  it('serializes items, dropping those without a field', () => {
    expect(serializeSort([{ field: 'name', direction: 'desc' }])).toBe('[{"field":"name","direction":"desc"}]');
    expect(serializeSort([{ field: '   ', direction: 'asc' }])).toBe('');
    expect(serializeSort([])).toBe('');
  });

  it('round-trips', () => {
    const items = [
      { field: 'created_at', direction: 'desc' as const },
      { field: 'name', direction: 'asc' as const },
    ];
    expect(parseSort(serializeSort(items))).toEqual(items);
  });
});
