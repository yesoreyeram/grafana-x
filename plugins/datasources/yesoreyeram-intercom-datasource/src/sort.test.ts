import { parseSort, serializeSort } from './sort';

describe('sort helpers', () => {
  it('parses empty/undefined to undefined', () => {
    expect(parseSort(undefined)).toBeUndefined();
    expect(parseSort('')).toBeUndefined();
    expect(parseSort('   ')).toBeUndefined();
  });

  it('parses asc and desc fields', () => {
    expect(parseSort('created_at')).toEqual({ field: 'created_at', direction: 'asc' });
    expect(parseSort('-created_at')).toEqual({ field: 'created_at', direction: 'desc' });
  });

  it('trims whitespace', () => {
    expect(parseSort(' -updated_at ')).toEqual({ field: 'updated_at', direction: 'desc' });
  });

  it('serializes items back to a sort string', () => {
    expect(serializeSort({ field: 'created_at', direction: 'desc' })).toBe('-created_at');
    expect(serializeSort({ field: 'created_at', direction: 'asc' })).toBe('created_at');
  });

  it('serialize drops items without a field', () => {
    expect(serializeSort(undefined)).toBe('');
    expect(serializeSort({ field: '', direction: 'asc' })).toBe('');
  });

  it('round-trips', () => {
    expect(serializeSort(parseSort('-created_at'))).toBe('-created_at');
    expect(serializeSort(parseSort('updated_at'))).toBe('updated_at');
  });
});
