import { getTransformSpec, parseTransformObject } from './transformSchema';

describe('transform schema', () => {
  it('builds a filter transform from values', () => {
    const spec = getTransformSpec('filter');
    expect(spec.build({ filter: 'datum.v > 0' })).toEqual({ filter: 'datum.v > 0' });
  });

  it('wraps a single aggregate op into the array form with derived as', () => {
    const spec = getTransformSpec('aggregate');
    expect(spec.build({ op: 'mean', field: 'v', groupby: ['host'] })).toEqual({
      aggregate: [{ op: 'mean', field: 'v', as: 'mean_v' }],
      groupby: ['host'],
    });
  });

  it('round-trips fold values through build and extract', () => {
    const spec = getTransformSpec('fold');
    const obj = spec.build({ fold: ['a', 'b'], asKey: 'k', asValue: 'val' });
    expect(obj).toEqual({ fold: ['a', 'b'], as: ['k', 'val'] });
    expect(spec.extract(obj)).toEqual({ fold: ['a', 'b'], asKey: 'k', asValue: 'val' });
  });

  it('omits empty values so incomplete transforms produce {}', () => {
    const spec = getTransformSpec('calculate');
    expect(spec.build({ calculate: '', as: '' })).toEqual({});
  });

  it('extracts values from an existing transform object', () => {
    const spec = getTransformSpec('window');
    const values = spec.extract({
      window: [{ op: 'rank', as: 'r' }],
      sort: [{ field: 'v', order: 'descending' }],
      groupby: ['g'],
    });
    expect(values.op).toBe('rank');
    expect(values.sortOrder).toBe('descending');
    expect(values.groupby).toEqual(['g']);
  });

  it('parses invalid JSON to an empty object', () => {
    expect(parseTransformObject('{ not json')).toEqual({});
    expect(parseTransformObject('[1,2]')).toEqual({});
  });
});
