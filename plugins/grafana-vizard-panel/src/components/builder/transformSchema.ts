import { SelectableValue } from '@grafana/data';

import { TransformKind } from '../../types';
import {
  AGGREGATE_OP_OPTIONS,
  IMPUTE_METHOD_OPTIONS,
  REGRESSION_METHOD_OPTIONS,
  SORT_ORDER_OPTIONS,
  STACK_OFFSET_OPTIONS,
  TIME_UNIT_OPTIONS,
  WINDOW_OP_OPTIONS,
} from './options';

export type TFieldKind = 'expr' | 'text' | 'number' | 'field' | 'fields' | 'select';

export interface TField {
  key: string;
  label: string;
  kind: TFieldKind;
  options?: Array<SelectableValue<string>>;
  placeholder?: string;
  tooltip?: string;
}

/** UI state for a transform's structured fields. Always strings / string[]. */
export type TValues = Record<string, string | string[]>;

export interface TransformSpec {
  fields: TField[];
  /** Build the Vega-Lite transform object from the UI values. */
  build: (values: TValues) => Record<string, unknown>;
  /** Extract UI values from a parsed transform object. */
  extract: (obj: Record<string, unknown>) => TValues;
}

const TIME_UNIT_CHOICES = TIME_UNIT_OPTIONS.filter((o) => o.value !== '');

// --- value coercion helpers -------------------------------------------------

const str = (v: unknown): string => (typeof v === 'string' ? v : v == null ? '' : String(v));
const strArr = (v: unknown): string[] => (Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []);
const numOrUndef = (v: unknown): number | undefined => {
  if (typeof v === 'number') {
    return v;
  }
  if (v == null || v === '') {
    return undefined;
  }
  const n = Number(v);
  return Number.isNaN(n) ? undefined : n;
};
const csvToList = (v: unknown): string[] =>
  str(v)
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean);
const listToCsv = (v: unknown): string => (Array.isArray(v) ? v.map(str).join(', ') : '');
const maybeNumber = (v: string): string | number => {
  const n = Number(v);
  return v !== '' && !Number.isNaN(n) ? n : v;
};

/** Drop empty values so incomplete transforms stay out of the spec. */
function compact(obj: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(obj)) {
    if (v === undefined || v === '' || (Array.isArray(v) && v.length === 0)) {
      continue;
    }
    out[k] = v;
  }
  return out;
}

const asObj = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const firstOf = (v: unknown): Record<string, unknown> => (Array.isArray(v) ? asObj(v[0]) : {});

const field = (key: string, label: string): TField => ({ key, label, kind: 'field' });
const fields = (key: string, label: string): TField => ({ key, label, kind: 'fields' });
const text = (key: string, label: string, placeholder?: string): TField => ({ key, label, kind: 'text', placeholder });
const expr = (key: string, label: string, placeholder?: string): TField => ({ key, label, kind: 'expr', placeholder });
const select = (key: string, label: string, options: Array<SelectableValue<string>>): TField => ({
  key,
  label,
  kind: 'select',
  options,
});

const SPECS: Record<TransformKind, TransformSpec> = {
  filter: {
    fields: [expr('filter', 'Filter expression', 'datum.value > 0')],
    build: (v) => compact({ filter: str(v.filter) }),
    extract: (o) => ({ filter: str(o.filter) }),
  },
  calculate: {
    fields: [expr('calculate', 'Expression', 'datum.a + datum.b'), text('as', 'As (new field)', 'result')],
    build: (v) => compact({ calculate: str(v.calculate), as: str(v.as) }),
    extract: (o) => ({ calculate: str(o.calculate), as: str(o.as) }),
  },
  aggregate: {
    fields: [
      select('op', 'Operation', AGGREGATE_OP_OPTIONS),
      field('field', 'Field'),
      text('as', 'As'),
      fields('groupby', 'Group by'),
    ],
    build: (v) => {
      const op = str(v.op);
      const f = str(v.field);
      const as = str(v.as) || (op && f ? `${op}_${f}` : op);
      const entry = compact({ op, field: f, as });
      return compact({ aggregate: op ? [entry] : undefined, groupby: strArr(v.groupby) });
    },
    extract: (o) => {
      const first = firstOf(o.aggregate);
      return { op: str(first.op), field: str(first.field), as: str(first.as), groupby: strArr(o.groupby) };
    },
  },
  bin: {
    fields: [field('field', 'Field'), text('as', 'As'), { key: 'maxbins', label: 'Max bins', kind: 'number' }],
    build: (v) => {
      const maxbins = numOrUndef(v.maxbins);
      return compact({ bin: maxbins != null ? { maxbins } : true, field: str(v.field), as: str(v.as) });
    },
    extract: (o) => ({ field: str(o.field), as: str(o.as), maxbins: str(asObj(o.bin).maxbins) }),
  },
  timeUnit: {
    fields: [select('timeUnit', 'Time unit', TIME_UNIT_CHOICES), field('field', 'Field'), text('as', 'As')],
    build: (v) => compact({ timeUnit: str(v.timeUnit), field: str(v.field), as: str(v.as) }),
    extract: (o) => ({ timeUnit: str(o.timeUnit), field: str(o.field), as: str(o.as) }),
  },
  fold: {
    fields: [fields('fold', 'Fields to fold'), text('asKey', 'As key', 'key'), text('asValue', 'As value', 'value')],
    build: (v) => {
      const asKey = str(v.asKey);
      const asValue = str(v.asValue);
      const as = asKey || asValue ? [asKey || 'key', asValue || 'value'] : undefined;
      return compact({ fold: strArr(v.fold), as });
    },
    extract: (o) => {
      const as = Array.isArray(o.as) ? o.as : [];
      return { fold: strArr(o.fold), asKey: str(as[0]), asValue: str(as[1]) };
    },
  },
  pivot: {
    fields: [field('pivot', 'Pivot field'), field('value', 'Value field'), fields('groupby', 'Group by')],
    build: (v) => compact({ pivot: str(v.pivot), value: str(v.value), groupby: strArr(v.groupby) }),
    extract: (o) => ({ pivot: str(o.pivot), value: str(o.value), groupby: strArr(o.groupby) }),
  },
  window: {
    fields: [
      select('op', 'Operation', WINDOW_OP_OPTIONS),
      field('field', 'Field'),
      text('as', 'As'),
      field('sortField', 'Sort field'),
      select('sortOrder', 'Sort order', SORT_ORDER_OPTIONS),
      fields('groupby', 'Group by'),
    ],
    build: (v) => {
      const op = str(v.op);
      const f = str(v.field);
      const as = str(v.as) || op;
      const win = op ? [compact({ op, field: f, as })] : undefined;
      const sortField = str(v.sortField);
      const sort = sortField ? [compact({ field: sortField, order: str(v.sortOrder) || 'ascending' })] : undefined;
      return compact({ window: win, sort, groupby: strArr(v.groupby) });
    },
    extract: (o) => {
      const first = firstOf(o.window);
      const sort0 = firstOf(o.sort);
      return {
        op: str(first.op),
        field: str(first.field),
        as: str(first.as),
        sortField: str(sort0.field),
        sortOrder: str(sort0.order),
        groupby: strArr(o.groupby),
      };
    },
  },
  joinaggregate: {
    fields: [
      select('op', 'Operation', AGGREGATE_OP_OPTIONS),
      field('field', 'Field'),
      text('as', 'As'),
      fields('groupby', 'Group by'),
    ],
    build: (v) => {
      const op = str(v.op);
      const f = str(v.field);
      const as = str(v.as) || (op && f ? `${op}_${f}` : op);
      const entry = compact({ op, field: f, as });
      return compact({ joinaggregate: op ? [entry] : undefined, groupby: strArr(v.groupby) });
    },
    extract: (o) => {
      const first = firstOf(o.joinaggregate);
      return { op: str(first.op), field: str(first.field), as: str(first.as), groupby: strArr(o.groupby) };
    },
  },
  stack: {
    fields: [
      field('stack', 'Stack field'),
      fields('groupby', 'Group by'),
      select('offset', 'Offset', STACK_OFFSET_OPTIONS),
      text('asStart', 'As (start)', 'v_start'),
      text('asEnd', 'As (end)', 'v_end'),
    ],
    build: (v) => {
      const asStart = str(v.asStart);
      const asEnd = str(v.asEnd);
      const as = asStart || asEnd ? [asStart || 'v0', asEnd || 'v1'] : undefined;
      return compact({ stack: str(v.stack), groupby: strArr(v.groupby), offset: str(v.offset), as });
    },
    extract: (o) => {
      const as = Array.isArray(o.as) ? o.as : [];
      return {
        stack: str(o.stack),
        groupby: strArr(o.groupby),
        offset: str(o.offset),
        asStart: str(as[0]),
        asEnd: str(as[1]),
      };
    },
  },
  flatten: {
    fields: [fields('flatten', 'Fields to flatten'), text('as', 'As (comma-separated)', 'a, b')],
    build: (v) => {
      const as = csvToList(v.as);
      return compact({ flatten: strArr(v.flatten), as: as.length ? as : undefined });
    },
    extract: (o) => ({ flatten: strArr(o.flatten), as: listToCsv(o.as) }),
  },
  sample: {
    fields: [{ key: 'sample', label: 'Sample size', kind: 'number' }],
    build: (v) => compact({ sample: numOrUndef(v.sample) }),
    extract: (o) => ({ sample: str(o.sample) }),
  },
  density: {
    fields: [field('density', 'Field'), fields('groupby', 'Group by')],
    build: (v) => compact({ density: str(v.density), groupby: strArr(v.groupby) }),
    extract: (o) => ({ density: str(o.density), groupby: strArr(o.groupby) }),
  },
  regression: {
    fields: [
      field('regression', 'Y field'),
      field('on', 'X field'),
      select('method', 'Method', REGRESSION_METHOD_OPTIONS),
      fields('groupby', 'Group by'),
    ],
    build: (v) =>
      compact({ regression: str(v.regression), on: str(v.on), method: str(v.method), groupby: strArr(v.groupby) }),
    extract: (o) => ({
      regression: str(o.regression),
      on: str(o.on),
      method: str(o.method),
      groupby: strArr(o.groupby),
    }),
  },
  loess: {
    fields: [field('loess', 'Y field'), field('on', 'X field'), fields('groupby', 'Group by')],
    build: (v) => compact({ loess: str(v.loess), on: str(v.on), groupby: strArr(v.groupby) }),
    extract: (o) => ({ loess: str(o.loess), on: str(o.on), groupby: strArr(o.groupby) }),
  },
  quantile: {
    fields: [
      field('quantile', 'Field'),
      text('probs', 'Probabilities (comma)', '0.25, 0.5, 0.75'),
      fields('groupby', 'Group by'),
    ],
    build: (v) => {
      const probs = csvToList(v.probs)
        .map((x) => Number(x))
        .filter((x) => !Number.isNaN(x));
      return compact({ quantile: str(v.quantile), probs: probs.length ? probs : undefined, groupby: strArr(v.groupby) });
    },
    extract: (o) => ({ quantile: str(o.quantile), probs: listToCsv(o.probs), groupby: strArr(o.groupby) }),
  },
  impute: {
    fields: [
      field('impute', 'Field'),
      field('key', 'Key field'),
      select('method', 'Method', IMPUTE_METHOD_OPTIONS),
      text('value', 'Value (when method = value)'),
      fields('groupby', 'Group by'),
    ],
    build: (v) => {
      const method = str(v.method);
      const value = method === 'value' && str(v.value) !== '' ? maybeNumber(str(v.value)) : undefined;
      return compact({ impute: str(v.impute), key: str(v.key), method, value, groupby: strArr(v.groupby) });
    },
    extract: (o) => ({
      impute: str(o.impute),
      key: str(o.key),
      method: str(o.method),
      value: o.value == null ? '' : String(o.value),
      groupby: strArr(o.groupby),
    }),
  },
  lookup: {
    fields: [
      field('lookup', 'Local key field'),
      text('fromName', 'From dataset name', 'other'),
      text('fromKey', 'From key field', 'id'),
      text('fromFields', 'From fields (comma)', 'name, label'),
    ],
    build: (v) => {
      const fromName = str(v.fromName);
      const fromKey = str(v.fromKey);
      const fromFields = csvToList(v.fromFields);
      const from = compact({
        data: fromName ? { name: fromName } : undefined,
        key: fromKey,
        fields: fromFields.length ? fromFields : undefined,
      });
      return compact({ lookup: str(v.lookup), from: Object.keys(from).length ? from : undefined });
    },
    extract: (o) => {
      const from = asObj(o.from);
      const data = asObj(from.data);
      return {
        lookup: str(o.lookup),
        fromName: str(data.name),
        fromKey: str(from.key),
        fromFields: listToCsv(from.fields),
      };
    },
  },
  extent: {
    fields: [field('extent', 'Field'), text('param', 'Param name', 'value_extent')],
    build: (v) => compact({ extent: str(v.extent), param: str(v.param) }),
    extract: (o) => ({ extent: str(o.extent), param: str(o.param) }),
  },
};

export function getTransformSpec(kind: TransformKind): TransformSpec {
  return SPECS[kind];
}

/** Parse a transform JSON body into an object, tolerating invalid input. */
export function parseTransformObject(json: string): Record<string, unknown> {
  try {
    const v: unknown = JSON.parse(json);
    return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

/** Serialize a transform object to a pretty JSON string. */
export function serializeTransform(obj: Record<string, unknown>): string {
  return JSON.stringify(obj, null, 2);
}
