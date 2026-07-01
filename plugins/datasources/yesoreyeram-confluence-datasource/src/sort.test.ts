import { parseSort, serializeSort } from './sort';

describe('sort helpers', () => {
  it('parses empty/undefined to []', () => {
    expect(parseSort(undefined)).toEqual([]);
    expect(parseSort('')).toEqual([]);
  });

  it('parses asc and desc tokens', () => {
    expect(parseSort('-modified-date')).toEqual([{ field: 'modified-date', direction: 'desc' }]);
    expect(parseSort('title')).toEqual([{ field: 'title', direction: 'asc' }]);
  });

  it('serializes items back to a sort string', () => {
    expect(serializeSort([{ field: 'created-date', direction: 'desc' }])).toBe('-created-date');
    expect(serializeSort([{ field: 'title', direction: 'asc' }])).toBe('title');
  });

  it('round-trips', () => {
    expect(serializeSort(parseSort('-modified-date'))).toBe('-modified-date');
  });
});
