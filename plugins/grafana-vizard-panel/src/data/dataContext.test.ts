import { FieldType, toDataFrame } from '@grafana/data';

import { buildDataContext } from './dataContext';

describe('buildDataContext', () => {
  it('converts a wide time series into row objects keyed by display name', () => {
    const frame = toDataFrame({
      refId: 'A',
      fields: [
        { name: 'time', type: FieldType.time, values: [1000, 2000] },
        { name: 'cpu', type: FieldType.number, values: [1, 2] },
        { name: 'mem', type: FieldType.number, values: [3, 4] },
      ],
    });
    const ctx = buildDataContext([frame], { source: 'auto' });

    expect(ctx.isEmpty).toBe(false);
    expect(ctx.primary.rows).toEqual([
      { time: 1000, cpu: 1, mem: 3 },
      { time: 2000, cpu: 2, mem: 4 },
    ]);
    expect(ctx.indexField).toBe('time');
    expect(ctx.valueFields).toEqual(['cpu', 'mem']);
    expect(ctx.multiSeries).toBe(true);
  });

  it('merges multi-frame single-series data into one long table using labels', () => {
    const mk = (host: string, values: number[]) =>
      toDataFrame({
        refId: 'A',
        fields: [
          { name: 'time', type: FieldType.time, values: [1000, 2000] },
          { name: 'value', type: FieldType.number, values, labels: { host } },
        ],
      });
    const ctx = buildDataContext([mk('a', [1, 2]), mk('b', [3, 4])], { source: 'auto' });

    expect(ctx.primary.name).toBe('merged');
    expect(ctx.primary.rows).toHaveLength(4);
    expect(ctx.seriesField).toBe('host');
    expect(ctx.primary.rows[0]).toEqual({ time: 1000, value: 1, host: 'a' });
    expect(ctx.primary.rows[2]).toEqual({ time: 1000, value: 3, host: 'b' });
  });

  it('synthesizes a series column when multi frames have no labels', () => {
    const mk = (name: string, values: number[]) =>
      toDataFrame({ fields: [{ name, type: FieldType.number, values }] });
    const ctx = buildDataContext([mk('a', [1]), mk('b', [2])], { source: 'auto' });

    expect(ctx.primary.name).toBe('merged');
    expect(ctx.seriesField).toBe('series');
    // Differently-named value fields normalize to a unified `value` column.
    expect(ctx.primary.rows).toEqual([
      { value: 1, series: 'a' },
      { value: 2, series: 'b' },
    ]);
  });

  it('normalizes numbers and nulls', () => {
    const frame = toDataFrame({
      fields: [
        { name: 'label', type: FieldType.string, values: ['x', null] },
        { name: 'n', type: FieldType.number, values: [5, null] },
      ],
    });
    const ctx = buildDataContext([frame], { source: 'auto' });
    expect(ctx.primary.rows).toEqual([
      { label: 'x', n: 5 },
      { label: null, n: null },
    ]);
  });

  it('pins a single series by refId', () => {
    const a = toDataFrame({ refId: 'A', fields: [{ name: 'v', type: FieldType.number, values: [1] }] });
    const b = toDataFrame({ refId: 'B', fields: [{ name: 'v', type: FieldType.number, values: [2] }] });
    const ctx = buildDataContext([a, b], { source: 'series', seriesRefId: 'B' });
    expect(ctx.primary.rows).toEqual([{ v: 2 }]);
  });

  it('reports empty when there is no data', () => {
    const ctx = buildDataContext([], { source: 'auto' });
    expect(ctx.isEmpty).toBe(true);
  });
});
