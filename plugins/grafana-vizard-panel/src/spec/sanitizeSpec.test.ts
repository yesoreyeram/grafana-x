import { SpecObject } from '../types';
import { sanitizeSpec } from './sanitizeSpec';

function get(obj: unknown, path: string[]): unknown {
  return path.reduce<unknown>((acc, key) => {
    if (acc && typeof acc === 'object') {
      return (acc as Record<string, unknown>)[key];
    }
    return undefined;
  }, obj);
}

describe('sanitizeSpec', () => {
  it('neutralizes remote data sources to empty inline data at any depth', () => {
    const spec: SpecObject = {
      data: { url: 'http://evil.example/data.json', format: { type: 'json' } },
      transform: [{ lookup: 'k', from: { data: { url: 'http://evil.example/lk.json' }, key: 'id', fields: ['n'] } }],
      layer: [{ data: { url: 'http://evil.example/layer.json' }, mark: 'line' }],
    };
    const out = sanitizeSpec(spec);
    expect(JSON.stringify(out)).not.toContain('evil.example');
    // Remote sources become empty inline data (valid + remote-free).
    expect(get(out, ['data'])).toEqual({ values: [] });
    expect(get(out, ['transform', '0', 'from', 'data'])).toEqual({ values: [] });
    // The lookup's key/fields are preserved.
    expect(get(out, ['transform', '0', 'from', 'key'])).toBe('id');
    expect(get(out, ['layer', '0', 'data'])).toEqual({ values: [] });
  });

  it('removes usermeta (blocks embedOptions override)', () => {
    const out = sanitizeSpec({ usermeta: { embedOptions: { actions: true, loader: {} } }, mark: 'bar' });
    expect(out.usermeta).toBeUndefined();
    expect(out.mark).toBe('bar');
  });

  it('strips href from marks/encodings (click-XSS vector)', () => {
    const out = sanitizeSpec({
      layer: [{ mark: 'point', encoding: { href: { field: 'link' }, x: { field: 'a' } } }],
    });
    const layer = out.layer as Array<Record<string, unknown>>;
    const encoding = layer[0].encoding as Record<string, unknown>;
    expect(encoding.href).toBeUndefined();
    expect(encoding.x).toEqual({ field: 'a' });
  });

  it('keeps safe inline data untouched', () => {
    const out = sanitizeSpec({ data: { values: [{ a: 1 }] }, mark: 'line' });
    expect(out).toEqual({ data: { values: [{ a: 1 }] }, mark: 'line' });
  });

  it('preserves inline data content (url is a data value, not a loader)', () => {
    const out = sanitizeSpec({ data: { values: [{ name: 'x', url: 'http://img/x.png' }] }, mark: 'image' });
    expect(get(out, ['data', 'values', '0', 'url'])).toBe('http://img/x.png');
  });

  it('preserves named datasets verbatim', () => {
    const out = sanitizeSpec({ datasets: { a: [{ v: 1 }] }, data: { name: 'a' }, mark: 'bar' });
    expect(out.datasets).toEqual({ a: [{ v: 1 }] });
    expect(out.data).toEqual({ name: 'a' });
  });
});
