import { normalizeCQL, escapeCQLValue, spaceCQL, CQL_EXAMPLES } from './filter';

describe('CQL helpers', () => {
  it('normalizeCQL trims', () => {
    expect(normalizeCQL('  type=page  ')).toBe('type=page');
    expect(normalizeCQL(undefined)).toBe('');
  });

  it('escapeCQLValue escapes quotes and backslashes', () => {
    expect(escapeCQLValue('a"b')).toBe('a\\"b');
    expect(escapeCQLValue('a\\b')).toBe('a\\\\b');
  });

  it('spaceCQL builds a scoped clause', () => {
    expect(spaceCQL('ENG')).toBe('space = "ENG"');
    expect(spaceCQL('  ')).toBe('');
  });

  it('exposes example snippets', () => {
    expect(CQL_EXAMPLES.length).toBeGreaterThan(0);
  });
});
