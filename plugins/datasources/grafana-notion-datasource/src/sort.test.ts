import { parseSort, serializeSort } from './sort';

describe('sort helpers', () => {
  it('parses empty/undefined to []', () => {
    expect(parseSort(undefined)).toEqual([]);
    expect(parseSort('')).toEqual([]);
  });

  it('parses asc and desc fields', () => {
    expect(parseSort('-CreatedAt,Title')).toEqual([
      { field: 'CreatedAt', direction: 'desc' },
      { field: 'Title', direction: 'asc' },
    ]);
  });

  it('trims whitespace and drops empty tokens', () => {
    expect(parseSort(' Name , , -Age ')).toEqual([
      { field: 'Name', direction: 'asc' },
      { field: 'Age', direction: 'desc' },
    ]);
  });

  it('serializes items back to a sort string', () => {
    expect(
      serializeSort([
        { field: 'CreatedAt', direction: 'desc' },
        { field: 'Title', direction: 'asc' },
      ])
    ).toBe('-CreatedAt,Title');
  });

  it('serialize drops items without a field', () => {
    expect(
      serializeSort([
        { field: '', direction: 'asc' },
        { field: 'Age', direction: 'desc' },
      ])
    ).toBe('-Age');
  });

  it('round-trips', () => {
    const s = '-CreatedAt,Title,-Age';
    expect(serializeSort(parseSort(s))).toBe(s);
  });
});
