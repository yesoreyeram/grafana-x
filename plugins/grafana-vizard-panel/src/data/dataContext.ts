import { DataFrame, Field, FieldType, getFieldDisplayName } from '@grafana/data';

import { DataOptions, VegaLiteFieldType } from '../types';
import { detectFrameKind, FrameKind } from './detectKind';
import { vegaLiteTypeForField } from './fieldType';

export type Row = Record<string, unknown>;

export interface FieldInfo {
  /** Column key used inside row objects (the field's display name, incl. labels). */
  name: string;
  vegaLiteType: VegaLiteFieldType;
  grafanaType: FieldType;
  /** True when this column came from a series label / dimension (long format). */
  isLabel?: boolean;
}

export interface FrameData {
  name: string;
  refId?: string;
  kind: FrameKind;
  rows: Row[];
  fields: FieldInfo[];
}

export interface DataContext {
  /** Per selected frame, converted to row objects keyed by display name. */
  frames: FrameData[];
  /** Named datasets (each frame + the merged "series" table) for spec references. */
  datasets: Record<string, Row[]>;
  /** The default data for the chart (merged series table when applicable). */
  primary: FrameData;
  /** Field catalog for the builder dropdowns (from `primary`). */
  fields: FieldInfo[];
  kind: FrameKind;
  /** True when there is more than one series (wide multi-column or merged frames). */
  multiSeries: boolean;
  /** Quantitative value columns in `primary` (used for fold/auto defaults). */
  valueFields: string[];
  /** Temporal (or first) column name in `primary`, used as the default x. */
  indexField?: string;
  /** Series / dimension column name in `primary` (long / merged), default color. */
  seriesField?: string;
  isEmpty: boolean;
}

const EMPTY_FRAME: FrameData = { name: 'empty', kind: 'empty', rows: [], fields: [] };

function normalizeValue(field: Pick<Field, 'type'>, value: unknown): unknown {
  if (value === undefined || value === null) {
    return null;
  }
  switch (field.type) {
    case FieldType.time:
      if (typeof value === 'number') {
        return value;
      }
      if (value instanceof Date) {
        return value.getTime();
      }
      return value; // ISO-8601 string — Vega-Lite parses these as temporal
    case FieldType.number:
      if (typeof value === 'number') {
        return value;
      }
      return value === '' ? null : Number(value);
    case FieldType.boolean:
      return Boolean(value);
    default:
      return value;
  }
}

function numberOrNull(value: unknown): number | null {
  if (value === undefined || value === null || value === '') {
    return null;
  }
  const n = typeof value === 'number' ? value : Number(value);
  return Number.isNaN(n) ? null : n;
}

function frameValues(field: Field): unknown[] {
  return field.values as unknown[];
}

function toFrameData(frame: DataFrame, index: number, allFrames: DataFrame[]): FrameData {
  const fields: FieldInfo[] = frame.fields.map((f) => ({
    name: getFieldDisplayName(f, frame, allFrames),
    vegaLiteType: vegaLiteTypeForField(f),
    grafanaType: f.type,
    isLabel: false,
  }));

  const rows: Row[] = [];
  for (let i = 0; i < frame.length; i++) {
    const row: Row = {};
    frame.fields.forEach((f, fi) => {
      row[fields[fi].name] = normalizeValue(f, frameValues(f)[i]);
    });
    rows.push(row);
  }

  const refId = frame.refId;
  const name = refId && refId.length ? refId : `frame_${index}`;
  return { name, refId, kind: detectFrameKind(frame), rows, fields };
}

/**
 * Merge multiple single-value-field frames (the time series / numeric "multi"
 * formats, which split one series per frame) into one tidy long table:
 * `{ <index>, <value>, <labels...> | series }`. Returns null when the frames are
 * not a clean homogeneous multi set (so the caller falls back to the first frame).
 */
function mergeSeriesFrames(frames: DataFrame[]): FrameData | null {
  if (frames.length < 2) {
    return null;
  }

  const specs = frames.map((f) => {
    const timeField = f.fields.find((x) => x.type === FieldType.time);
    const valueFields = f.fields.filter((x) => x.type === FieldType.number);
    return { frame: f, timeField, valueField: valueFields[0], valueCount: valueFields.length };
  });

  // Every frame must contribute exactly one numeric value field.
  if (specs.some((s) => !s.valueField || s.valueCount !== 1)) {
    return null;
  }

  const valueNames = new Set(specs.map((s) => s.valueField!.name));
  const valueName = valueNames.size === 1 ? specs[0].valueField!.name || 'value' : 'value';
  const hasTime = specs.every((s) => Boolean(s.timeField));
  const indexName = hasTime ? specs[0].timeField!.name || 'time' : undefined;

  const labelKeys = new Set<string>();
  specs.forEach((s) => Object.keys(s.valueField!.labels ?? {}).forEach((k) => labelKeys.add(k)));
  const useSeriesName = labelKeys.size === 0;
  const labelKeyList = Array.from(labelKeys);

  const rows: Row[] = [];
  specs.forEach((s) => {
    const seriesLabel = getFieldDisplayName(s.valueField!, s.frame, frames);
    for (let i = 0; i < s.frame.length; i++) {
      const row: Row = {};
      if (indexName && s.timeField) {
        row[indexName] = normalizeValue(s.timeField, frameValues(s.timeField)[i]);
      }
      row[valueName] = numberOrNull(frameValues(s.valueField!)[i]);
      if (useSeriesName) {
        row.series = seriesLabel;
      } else {
        labelKeyList.forEach((k) => {
          row[k] = s.valueField!.labels?.[k] ?? null;
        });
      }
      rows.push(row);
    }
  });

  const fields: FieldInfo[] = [];
  if (indexName) {
    fields.push({ name: indexName, vegaLiteType: 'temporal', grafanaType: FieldType.time });
  }
  fields.push({ name: valueName, vegaLiteType: 'quantitative', grafanaType: FieldType.number });
  if (useSeriesName) {
    fields.push({ name: 'series', vegaLiteType: 'nominal', grafanaType: FieldType.string, isLabel: true });
  } else {
    labelKeyList.forEach((k) =>
      fields.push({ name: k, vegaLiteType: 'nominal', grafanaType: FieldType.string, isLabel: true })
    );
  }

  return { name: 'merged', kind: hasTime ? 'timeseries-long' : 'numeric-long', rows, fields };
}

function uniqueKey(base: string, used: Set<string>): string {
  if (!used.has(base)) {
    used.add(base);
    return base;
  }
  let i = 2;
  while (used.has(`${base}_${i}`)) {
    i++;
  }
  const key = `${base}_${i}`;
  used.add(key);
  return key;
}

/**
 * Convert a query response into the uniform representation the spec pipeline and
 * builder consume. Every frame becomes row objects keyed by display name; the
 * detected data-plane kind only influences smart defaults, never the shape.
 */
export function buildDataContext(series: DataFrame[] | undefined, selection: DataOptions): DataContext {
  const allFrames = series ?? [];

  let selected = allFrames;
  if (selection?.source === 'series' && selection.seriesRefId) {
    const matched = allFrames.filter((f) => (f.refId ?? '') === selection.seriesRefId);
    selected = matched.length ? matched : allFrames;
  }

  const frames = selected.map((f, i) => toFrameData(f, i, allFrames));

  const datasets: Record<string, Row[]> = {};
  const usedNames = new Set<string>();
  frames.forEach((fd) => {
    const key = uniqueKey(fd.name, usedNames);
    fd.name = key;
    datasets[key] = fd.rows;
  });

  const merged = mergeSeriesFrames(selected);
  let primary: FrameData;
  if (merged) {
    const key = uniqueKey(merged.name, usedNames);
    merged.name = key;
    datasets[key] = merged.rows;
    primary = merged;
  } else {
    primary = frames[0] ?? EMPTY_FRAME;
  }

  const valueFields = primary.fields.filter((f) => f.vegaLiteType === 'quantitative').map((f) => f.name);
  const indexField =
    primary.fields.find((f) => f.vegaLiteType === 'temporal')?.name ?? primary.fields[0]?.name;
  const seriesField = primary.fields.find((f) => f.isLabel)?.name;

  return {
    frames,
    datasets,
    primary,
    fields: primary.fields,
    kind: primary.kind,
    multiSeries: Boolean(merged) || valueFields.length > 1,
    valueFields,
    indexField,
    seriesField,
    isEmpty: primary.rows.length === 0,
  };
}
