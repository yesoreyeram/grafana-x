import { DataFrame, DataFrameType, FieldType } from '@grafana/data';

/**
 * Normalized data-plane kind for a single frame. Mirrors the Grafana data plane
 * contract (https://grafana.com/developers/dataplane) with a `table` fallback
 * for untyped/wide tabular frames and `empty` for field-less frames.
 */
export type FrameKind =
  | 'timeseries-wide'
  | 'timeseries-multi'
  | 'timeseries-long'
  | 'numeric-wide'
  | 'numeric-multi'
  | 'numeric-long'
  | 'logs'
  | 'table'
  | 'empty';

/**
 * Determine a frame's kind. Prefers the explicit data-plane declaration on
 * `frame.meta.type`; otherwise infers a kind from the field types so the panel
 * still does something sensible for data sources that don't tag their frames.
 */
export function detectFrameKind(frame: DataFrame): FrameKind {
  switch (frame.meta?.type) {
    case DataFrameType.TimeSeriesWide:
      return 'timeseries-wide';
    case DataFrameType.TimeSeriesMulti:
    // `TimeSeriesMany` is the deprecated alias of multi; treat it the same.
    case DataFrameType.TimeSeriesMany:
      return 'timeseries-multi';
    case DataFrameType.TimeSeriesLong:
      return 'timeseries-long';
    case DataFrameType.NumericWide:
      return 'numeric-wide';
    case DataFrameType.NumericMulti:
      return 'numeric-multi';
    case DataFrameType.NumericLong:
      return 'numeric-long';
    case DataFrameType.LogLines:
      return 'logs';
    default:
      break;
  }

  if (!frame.fields.length) {
    return 'empty';
  }

  const hasTime = frame.fields.some((f) => f.type === FieldType.time);
  const numberCount = frame.fields.filter((f) => f.type === FieldType.number).length;
  const stringCount = frame.fields.filter((f) => f.type === FieldType.string).length;

  if (hasTime && numberCount > 0) {
    // A string dimension column alongside time+value is the SQL-like long shape.
    return stringCount > 0 ? 'timeseries-long' : 'timeseries-wide';
  }

  if (!hasTime && numberCount > 0 && stringCount === 0) {
    return 'numeric-wide';
  }

  if (!hasTime && numberCount > 0 && stringCount > 0) {
    return 'numeric-long';
  }

  return 'table';
}
