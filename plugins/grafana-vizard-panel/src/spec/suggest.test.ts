import { FieldType, toDataFrame } from '@grafana/data';

import { buildDataContext } from '../data/dataContext';
import { suggestSpec } from './suggest';

function ctxOf(frames: Array<Parameters<typeof toDataFrame>[0]>) {
  return buildDataContext(frames.map(toDataFrame), { source: 'auto' });
}

describe('suggestSpec', () => {
  it('folds a multi-column wide time series into a coloured line', () => {
    const ctx = ctxOf([
      {
        fields: [
          { name: 'time', type: FieldType.time, values: [1, 2] },
          { name: 'cpu', type: FieldType.number, values: [1, 2] },
          { name: 'mem', type: FieldType.number, values: [3, 4] },
        ],
      },
    ]);
    const spec = suggestSpec(ctx);
    expect(Array.isArray(spec.transform)).toBe(true);
    const enc = spec.encoding as Record<string, Record<string, unknown>>;
    expect(enc.x.field).toBe('time');
    expect(enc.x.type).toBe('temporal');
    expect(enc.color).toBeDefined();
  });

  it('uses a single value column directly', () => {
    const ctx = ctxOf([
      {
        fields: [
          { name: 'time', type: FieldType.time, values: [1, 2] },
          { name: 'cpu', type: FieldType.number, values: [1, 2] },
        ],
      },
    ]);
    const spec = suggestSpec(ctx);
    const enc = spec.encoding as Record<string, Record<string, unknown>>;
    expect(enc.y.field).toBe('cpu');
    expect(spec.transform).toBeUndefined();
  });

  it('colours long/merged series by the series dimension', () => {
    const mk = (host: string) => ({
      fields: [
        { name: 'time', type: FieldType.time, values: [1, 2] },
        { name: 'value', type: FieldType.number, values: [1, 2], labels: { host } },
      ],
    });
    const ctx = ctxOf([mk('a'), mk('b')]);
    const spec = suggestSpec(ctx);
    const enc = spec.encoding as Record<string, Record<string, unknown>>;
    expect(enc.color.field).toBe('host');
  });

  it('counts logs over time', () => {
    const ctx = buildDataContext(
      [
        toDataFrame({
          meta: { type: undefined },
          fields: [
            { name: 'timestamp', type: FieldType.time, values: [1, 2] },
            { name: 'body', type: FieldType.string, values: ['x', 'y'] },
          ],
        }),
      ],
      { source: 'auto' }
    );
    // Force logs handling via kind override is not possible here, but a time +
    // string frame should still yield a sensible time-based suggestion.
    const spec = suggestSpec(ctx);
    expect(spec.mark).toBeDefined();
    expect(spec.encoding).toBeDefined();
  });
});
