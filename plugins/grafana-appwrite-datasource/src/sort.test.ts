import { parseSort, serializeSort } from './sort';

describe('sort serialization', () => {
  it('parses an empty/undefined sort to an empty list', () => {
    expect(parseSort(undefined)).toEqual([]);
    expect(parseSort('')).toEqual([]);
  });

  it('parses valid JSON sort items', () => {
    expect(parseSort('[{"attribute":"age","direction":"desc"}]')).toEqual([{ attribute: 'age', direction: 'desc' }]);
  });

  it('defaults invalid directions to asc and drops items without an attribute', () => {
    expect(parseSort('[{"attribute":"a","direction":"weird"},{"direction":"desc"}]')).toEqual([
      { attribute: 'a', direction: 'asc' },
    ]);
  });

  it('returns an empty list for malformed JSON', () => {
    expect(parseSort('not-json')).toEqual([]);
  });

  it('serializes valid items and drops empty attributes', () => {
    expect(serializeSort([{ attribute: 'age', direction: 'desc' }])).toBe('[{"attribute":"age","direction":"desc"}]');
    expect(serializeSort([{ attribute: '  ', direction: 'asc' }])).toBe('');
  });

  it('serializes an empty list to an empty string', () => {
    expect(serializeSort([])).toBe('');
  });
});
