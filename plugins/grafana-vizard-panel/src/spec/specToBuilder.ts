import {
  AggregateOp,
  BuilderModel,
  defaultMark,
  EncodingChannelName,
  EncodingModel,
  LayerModel,
  MarkModel,
  MarkType,
  ParamModel,
  PropMap,
  StackMode,
  TransformKind,
  TransformModel,
  VegaLiteFieldType,
} from '../types';
import { isPlainObject } from './merge';

let counter = 0;
function nid(prefix: string): string {
  counter += 1;
  return `${prefix}-${counter}`;
}

const MULTI_VIEW_KEYS = ['facet', 'concat', 'hconcat', 'vconcat', 'repeat'];
const KNOWN_TRANSFORM_KINDS: TransformKind[] = [
  'filter',
  'calculate',
  'aggregate',
  'bin',
  'timeUnit',
  'fold',
  'pivot',
  'window',
  'joinaggregate',
  'stack',
  'flatten',
  'sample',
  'density',
  'regression',
  'loess',
  'quantile',
  'lookup',
  'impute',
  'extent',
];
// Top-level keys handled explicitly or intentionally dropped (data/size/theme).
const TOP_HANDLED = new Set([
  'mark',
  'encoding',
  'transform',
  'layer',
  'params',
  'title',
  'resolve',
  '$schema',
  'data',
  'datasets',
  'config',
  'width',
  'height',
  'autosize',
]);

function isGeo(spec: PropMap): boolean {
  const json = JSON.stringify(spec);
  return json.includes('"geoshape"') || json.includes('"projection"') || json.includes('"topojson"');
}

function toMark(value: unknown): MarkModel {
  if (typeof value === 'string') {
    return { type: value as MarkType };
  }
  if (!isPlainObject(value) || typeof value.type !== 'string') {
    return { ...defaultMark };
  }
  const model: MarkModel = { type: value.type as MarkType };
  const props: PropMap = {};
  for (const [k, v] of Object.entries(value)) {
    if (k === 'type') {
      continue;
    }
    if (k === 'tooltip' && v === true) {
      model.tooltip = true;
    } else if (k === 'point' && v === true) {
      model.point = true;
    } else if (k === 'filled' && typeof v === 'boolean') {
      model.filled = v;
    } else if (k === 'interpolate' && typeof v === 'string') {
      model.interpolate = v;
    } else if (k === 'opacity' && typeof v === 'number') {
      model.opacity = v;
    } else {
      props[k] = v;
    }
  }
  if (Object.keys(props).length > 0) {
    model.props = props;
  }
  return model;
}

function valueToString(v: unknown): string {
  return typeof v === 'string' ? v : JSON.stringify(v);
}

function toChannel(channel: EncodingChannelName, def: unknown): EncodingModel {
  const enc: EncodingModel = { id: nid('enc'), channel };
  if (!isPlainObject(def)) {
    enc.extra = { __value: def };
    return enc;
  }
  const extra: PropMap = {};
  for (const [k, v] of Object.entries(def)) {
    switch (k) {
      case 'field':
        if (typeof v === 'string') {
          enc.field = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'type':
        enc.type = v as VegaLiteFieldType;
        break;
      case 'aggregate':
        if (typeof v === 'string') {
          enc.aggregate = v as AggregateOp;
        } else {
          extra[k] = v; // argmin/argmax object
        }
        break;
      case 'timeUnit':
        if (typeof v === 'string') {
          enc.timeUnit = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'bin':
        if (v === true) {
          enc.bin = true;
        } else {
          extra[k] = v; // bin params object / false
        }
        break;
      case 'sort':
        if (typeof v === 'string') {
          enc.sort = v;
        } else {
          extra[k] = v; // sort array / object
        }
        break;
      case 'stack':
        if (v === 'zero' || v === 'normalize' || v === 'center') {
          enc.stack = v as StackMode;
        } else if (v === null || v === false) {
          enc.stack = 'none';
        } else {
          extra[k] = v;
        }
        break;
      case 'title':
        if (typeof v === 'string' || v === null) {
          enc.title = v === null ? '' : v;
        } else {
          extra[k] = v;
        }
        break;
      case 'format':
        if (typeof v === 'string') {
          enc.format = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'value':
        enc.value = valueToString(v);
        break;
      case 'datum':
        enc.datum = valueToString(v);
        break;
      case 'scale':
        if (isPlainObject(v) || v === null) {
          enc.scale = (v ?? undefined) as PropMap | undefined;
          if (v === null) {
            extra.scale = null;
          }
        } else {
          extra[k] = v;
        }
        break;
      case 'axis':
        if (isPlainObject(v) || v === null) {
          enc.axis = v as PropMap | null;
        } else {
          extra[k] = v;
        }
        break;
      case 'legend':
        if (isPlainObject(v) || v === null) {
          enc.legend = v as PropMap | null;
        } else {
          extra[k] = v;
        }
        break;
      case 'condition':
        if (isPlainObject(v)) {
          enc.condition = v;
        } else {
          extra[k] = v;
        }
        break;
      default:
        extra[k] = v;
    }
  }
  if (Object.keys(extra).length > 0) {
    enc.extra = extra;
  }
  return enc;
}

function toEncodings(encoding: unknown): EncodingModel[] {
  if (!isPlainObject(encoding)) {
    return [];
  }
  const out: EncodingModel[] = [];
  for (const [channel, def] of Object.entries(encoding)) {
    if (Array.isArray(def)) {
      for (const d of def) {
        out.push(toChannel(channel as EncodingChannelName, d));
      }
    } else {
      out.push(toChannel(channel as EncodingChannelName, def));
    }
  }
  return out;
}

function transformKind(t: PropMap): TransformKind {
  for (const kind of KNOWN_TRANSFORM_KINDS) {
    if (kind in t) {
      return kind;
    }
  }
  return 'filter';
}

function toTransforms(transform: unknown): TransformModel[] {
  if (!Array.isArray(transform)) {
    return [];
  }
  return transform
    .filter(isPlainObject)
    .map((t) => ({ id: nid('tf'), kind: transformKind(t), mode: 'builder' as const, json: JSON.stringify(t, null, 2) }));
}

function toParams(params: unknown): ParamModel[] {
  if (!Array.isArray(params)) {
    return [];
  }
  return params.filter(isPlainObject).map((p) => {
    const { name, ...rest } = p;
    return { id: nid('param'), name: typeof name === 'string' ? name : nid('param'), spec: rest };
  });
}

function toLayer(unit: unknown): LayerModel | null {
  if (!isPlainObject(unit) || 'layer' in unit) {
    return null; // nested layers not modeled
  }
  const layer: LayerModel = {
    id: nid('layer'),
    mark: toMark(unit.mark),
    encodings: toEncodings(unit.encoding),
    transforms: toTransforms(unit.transform),
  };
  const params = toParams(unit.params);
  if (params.length > 0) {
    layer.params = params;
  }
  return layer;
}

// Field names containing expression-like characters need escaping the typed
// builder doesn't model; let the override path handle those few specs.
function hasComplexFieldName(spec: PropMap): boolean {
  return /"field"\s*:\s*"[^"]*[\\'()][^"]*"/.test(JSON.stringify(spec));
}

function topLevelExtra(spec: PropMap): PropMap | undefined {
  const extra: PropMap = {};
  for (const [k, v] of Object.entries(spec)) {
    if (!TOP_HANDLED.has(k)) {
      extra[k] = v;
    }
  }
  return Object.keys(extra).length > 0 ? extra : undefined;
}

export interface ConvertResult {
  builder: BuilderModel;
  ok: boolean;
}

/**
 * Convert a Vega-Lite spec into a builder model. Returns `ok: false` for specs
 * the builder can't represent natively (multi-view facet/concat/repeat, geo),
 * so the caller can fall back to the spec-override path. Conversions are
 * lossless: properties without a dedicated field land in structured `extra`/
 * `props` maps (never raw JSON strings).
 */
export function specToBuilder(input: unknown): ConvertResult {
  const empty: BuilderModel = { mark: { ...defaultMark }, encodings: [], transforms: [] };
  if (!isPlainObject(input)) {
    return { builder: empty, ok: false };
  }
  const spec = input;
  if (MULTI_VIEW_KEYS.some((k) => k in spec) || isGeo(spec) || hasComplexFieldName(spec)) {
    return { builder: empty, ok: false };
  }

  try {
    const builder: BuilderModel = { mark: { ...defaultMark }, encodings: [], transforms: [] };

    if (Array.isArray(spec.layer)) {
      const layers: LayerModel[] = [];
      for (const unit of spec.layer) {
        const layer = toLayer(unit);
        if (!layer) {
          return { builder: empty, ok: false }; // nested layer / unsupported
        }
        layers.push(layer);
      }
      builder.layers = layers;
      builder.encodings = toEncodings(spec.encoding); // shared encodings
    } else {
      builder.mark = toMark(spec.mark);
      builder.encodings = toEncodings(spec.encoding);
    }

    builder.transforms = toTransforms(spec.transform);
    const params = toParams(spec.params);
    if (params.length > 0) {
      builder.params = params;
    }
    if (isPlainObject(spec.resolve)) {
      builder.resolve = spec.resolve;
    }
    const extra = topLevelExtra(spec);
    if (extra) {
      builder.extra = extra;
    }

    return { builder, ok: true };
  } catch {
    return { builder: empty, ok: false };
  }
}
