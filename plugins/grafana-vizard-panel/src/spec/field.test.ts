import { escapeFieldName } from './field';

describe('escapeFieldName', () => {
  it('leaves plain names and label-style names untouched', () => {
    expect(escapeFieldName('cpu')).toBe('cpu');
    expect(escapeFieldName('value {host=a}')).toBe('value {host=a}');
  });

  it('escapes dots and brackets so they are matched literally', () => {
    expect(escapeFieldName('response.time')).toBe('response\\.time');
    expect(escapeFieldName('a[0]')).toBe('a\\[0\\]');
  });

  it('escapes backslashes first', () => {
    expect(escapeFieldName('a\\b')).toBe('a\\\\b');
  });
});
