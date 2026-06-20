import { DataFrameType, FieldType, toDataFrame } from '@grafana/data';

import { detectFrameKind } from './detectKind';

describe('detectFrameKind', () => {
  it('honours an explicit data-plane meta type', () => {
    const frame = toDataFrame({
      meta: { type: DataFrameType.NumericWide },
      fields: [{ name: 'cpu', type: FieldType.number, values: [1] }],
    });
    expect(detectFrameKind(frame)).toBe('numeric-wide');
  });

  it('treats the deprecated TimeSeriesMany as multi', () => {
    const frame = toDataFrame({
      meta: { type: DataFrameType.TimeSeriesMany },
      fields: [
        { name: 'time', type: FieldType.time, values: [1] },
        { name: 'v', type: FieldType.number, values: [1] },
      ],
    });
    expect(detectFrameKind(frame)).toBe('timeseries-multi');
  });

  it('infers wide time series from time + numeric fields', () => {
    const frame = toDataFrame({
      fields: [
        { name: 'time', type: FieldType.time, values: [1, 2] },
        { name: 'a', type: FieldType.number, values: [1, 2] },
        { name: 'b', type: FieldType.number, values: [3, 4] },
      ],
    });
    expect(detectFrameKind(frame)).toBe('timeseries-wide');
  });

  it('infers long time series when a string dimension is present', () => {
    const frame = toDataFrame({
      fields: [
        { name: 'time', type: FieldType.time, values: [1, 1] },
        { name: 'host', type: FieldType.string, values: ['a', 'b'] },
        { name: 'v', type: FieldType.number, values: [1, 2] },
      ],
    });
    expect(detectFrameKind(frame)).toBe('timeseries-long');
  });

  it('infers numeric kinds without a time field', () => {
    expect(
      detectFrameKind(
        toDataFrame({ fields: [{ name: 'cpu', type: FieldType.number, values: [1] }] })
      )
    ).toBe('numeric-wide');
    expect(
      detectFrameKind(
        toDataFrame({
          fields: [
            { name: 'host', type: FieldType.string, values: ['a'] },
            { name: 'cpu', type: FieldType.number, values: [1] },
          ],
        })
      )
    ).toBe('numeric-long');
  });

  it('returns empty for a field-less frame', () => {
    expect(detectFrameKind(toDataFrame({ fields: [] }))).toBe('empty');
  });
});
