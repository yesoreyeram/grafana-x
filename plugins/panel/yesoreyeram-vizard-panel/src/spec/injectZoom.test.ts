import { SpecObject } from '../types';
import { extractZoomRange, injectZoom, ZOOM_PARAM } from './injectZoom';

function params(spec: SpecObject): Array<Record<string, unknown>> {
  return (spec.params as Array<Record<string, unknown>>) ?? [];
}

describe('injectZoom', () => {
  it('adds an x interval param for a continuous temporal x', () => {
    const { spec, enabled } = injectZoom({ mark: 'line', encoding: { x: { field: 't', type: 'temporal' } } });
    expect(enabled).toBe(true);
    const p = params(spec).find((x) => x.name === ZOOM_PARAM);
    expect(p).toBeDefined();
    expect(p?.select).toEqual({ type: 'interval', encodings: ['x'] });
  });

  it('does not add zoom for layered specs (avoids duplicate Vega signals)', () => {
    const { enabled } = injectZoom({ layer: [{ mark: 'line', encoding: { x: { field: 't', type: 'temporal' } } }] });
    expect(enabled).toBe(false);
  });

  it('does not add zoom for a non-temporal x', () => {
    const { enabled } = injectZoom({ mark: 'bar', encoding: { x: { field: 'c', type: 'nominal' } } });
    expect(enabled).toBe(false);
  });

  it('does not add zoom for a binned/timeUnit temporal x (discrete scale)', () => {
    expect(injectZoom({ mark: 'bar', encoding: { x: { field: 't', type: 'temporal', timeUnit: 'month' } } }).enabled).toBe(
      false
    );
  });

  it('skips multi-view specs', () => {
    expect(injectZoom({ facet: { field: 'c' }, spec: { mark: 'line', encoding: { x: { field: 't', type: 'temporal' } } } }).enabled).toBe(
      false
    );
  });

  it('does not double-add when an interval selection already exists', () => {
    const { enabled } = injectZoom({
      mark: 'line',
      encoding: { x: { field: 't', type: 'temporal' } },
      params: [{ name: 'brush', select: { type: 'interval' } }],
    });
    expect(enabled).toBe(false);
  });
});

describe('extractZoomRange', () => {
  it('returns a sorted numeric range from a selection value', () => {
    expect(extractZoomRange({ t: [200, 100] })).toEqual([100, 200]);
  });
  it('returns null for empty / non-numeric / zero-width selections', () => {
    expect(extractZoomRange({})).toBeNull();
    expect(extractZoomRange({ t: ['a', 'b'] })).toBeNull();
    expect(extractZoomRange({ t: [5, 5] })).toBeNull();
    expect(extractZoomRange(undefined)).toBeNull();
  });
});
