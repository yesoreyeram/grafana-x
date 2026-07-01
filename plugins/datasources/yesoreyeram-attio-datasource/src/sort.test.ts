import { parseSort, serializeSort } from './sort';

describe('sort', () => {
  it('parses an empty/undefined sort to an empty list', () => {
    expect(parseSort()).toEqual([]);
    expect(parseSort('')).toEqual([]);
  });

  it('parses a JSON array of sort items', () => {
    expect(parseSort('[{"field":"name","direction":"desc"},{"field":"created_at","direction":"asc"}]')).toEqual([
      { field: 'name', direction: 'desc' },
      { field: 'created_at', direction: 'asc' },
    ]);
  });

  it('defaults invalid directions to asc and ignores fieldless items', () => {
    expect(parseSort('[{"field":"name","direction":"sideways"},{"direction":"desc"}]')).toEqual([
      { field: 'name', direction: 'asc' },
    ]);
  });

  it('ignores malformed json', () => {
    expect(parseSort('not-json')).toEqual([]);
  });

  it('serializes valid items and drops fieldless ones', () => {
    expect(
      serializeSort([
        { field: 'name', direction: 'desc' },
        { field: '', direction: 'asc' },
      ])
    ).toBe('[{"field":"name","direction":"desc"}]');
  });

  it('serializes an empty list to an empty string', () => {
    expect(serializeSort([])).toBe('');
    expect(serializeSort([{ field: '', direction: 'asc' }])).toBe('');
  });

  it('round-trips', () => {
    const items = [
      { field: 'name', direction: 'desc' as const },
      { field: 'created_at', direction: 'asc' as const },
    ];
    expect(parseSort(serializeSort(items))).toEqual(items);
  });
});
