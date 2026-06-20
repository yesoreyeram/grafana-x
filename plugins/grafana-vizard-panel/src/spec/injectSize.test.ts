import { injectSize } from './injectSize';

describe('injectSize', () => {
  it('sets width/height and autosize for single views', () => {
    const out = injectSize({ mark: 'line', encoding: {} }, 400.7, 300.2);
    expect(out.width).toBe(400);
    expect(out.height).toBe(300);
    expect(out.autosize).toEqual({ type: 'fit', contains: 'padding' });
  });

  it('respects an explicit size already on the spec', () => {
    const out = injectSize({ mark: 'bar', width: 123, height: 45 }, 400, 300);
    expect(out.width).toBe(123);
    expect(out.height).toBe(45);
  });

  it('does not force size on multi-view specs', () => {
    const out = injectSize({ facet: { field: 'c' }, spec: { mark: 'bar' } }, 400, 300);
    expect(out.width).toBeUndefined();
    expect(out.height).toBeUndefined();
    expect(out.autosize).toBeUndefined();
  });

  it('does not force size on encoding-level facets (row/column/facet)', () => {
    for (const channel of ['row', 'column', 'facet']) {
      const out = injectSize({ mark: 'area', encoding: { x: { field: 'x' }, [channel]: { field: 's' } } }, 400, 300);
      expect(out.width).toBeUndefined();
      expect(out.height).toBeUndefined();
    }
  });

  it('sizes layered specs', () => {
    const out = injectSize({ layer: [{ mark: 'line' }] }, 200, 100);
    expect(out.width).toBe(200);
    expect(out.height).toBe(100);
  });
});
