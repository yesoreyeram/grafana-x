import { BuilderModel, EncodingModel, MarkModel, SpecObject, TransformModel } from '../types';
import { escapeFieldName } from './field';
import { deepMerge, parseJsonObject, parseJsonValue } from './merge';

// Channels that accept multiple field definitions are collected into arrays.
const ARRAY_CHANNELS = new Set(['tooltip', 'detail', 'order']);

function parseValue(text: string): unknown {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text; // bare strings like "red" or "#ff8800"
  }
}

function parseSort(text: string): unknown {
  const trimmed = text.trim();
  if (trimmed.startsWith('[') || trimmed.startsWith('{')) {
    try {
      return JSON.parse(trimmed) as unknown;
    } catch {
      return text;
    }
  }
  return text; // "ascending" | "descending" | "-x" | "color" | ...
}

function buildMark(model: MarkModel, errors: string[]): SpecObject | string {
  const mark: SpecObject = { type: model.type };
  if (model.tooltip) {
    mark.tooltip = true;
  }
  if (model.point) {
    mark.point = true;
  }
  if (typeof model.filled === 'boolean') {
    mark.filled = model.filled;
  }
  if (model.interpolate) {
    mark.interpolate = model.interpolate;
  }
  if (typeof model.opacity === 'number') {
    mark.opacity = model.opacity;
  }
  let result: SpecObject = mark;
  if (model.props) {
    // Drop null/undefined props: a "reset" (e.g. the None preset / switching
    // charts) nulls stale keys so Grafana's option-merge clears them — those
    // nulls must not leak into the spec.
    const props = Object.fromEntries(Object.entries(model.props).filter(([, v]) => v !== null && v !== undefined));
    if (Object.keys(props).length > 0) {
      result = deepMerge(result, props);
    }
  }
  if (model.advancedJson) {
    const parsed = parseJsonObject(model.advancedJson);
    if (parsed.error) {
      errors.push(`Mark advanced JSON: ${parsed.error}`);
    } else if (parsed.value) {
      result = deepMerge(result, parsed.value);
    }
  }
  // Collapse to the shorthand string form when only the type is set.
  return Object.keys(result).length === 1 ? model.type : result;
}

function buildChannel(enc: EncodingModel, errors: string[]): SpecObject | null {
  const hasField = Boolean(enc.field && enc.field.length);
  const isCount = enc.aggregate === 'count';
  const hasDatum = enc.datum !== undefined && enc.datum !== '';
  const hasValue = enc.value !== undefined && enc.value !== '';

  // Structured catch-all base (lossless conversion); typed fields override it.
  const base = (): SpecObject => (enc.extra ? { ...enc.extra } : {});

  // Constant value encoding (e.g. a fixed color/size).
  if (!hasField && !isCount && !hasDatum && hasValue) {
    const def = base();
    def.value = parseValue(enc.value!);
    if (enc.condition) {
      def.condition = enc.condition;
    }
    return def;
  }

  // Constant datum encoding (a value in data space).
  if (!hasField && !isCount && hasDatum) {
    const def = base();
    def.datum = parseValue(enc.datum!);
    if (enc.type) {
      def.type = enc.type;
    }
    if (enc.scale) {
      def.scale = enc.scale;
    }
    if (enc.axis !== undefined) {
      def.axis = enc.axis;
    }
    return def;
  }

  if (!hasField && !isCount) {
    return enc.extra ? base() : null; // nothing to encode yet
  }

  const def: SpecObject = base();
  if (hasField) {
    def.field = escapeFieldName(enc.field!);
  }
  if (enc.type) {
    def.type = enc.type;
  } else if (isCount) {
    def.type = 'quantitative';
  }
  if (enc.aggregate) {
    def.aggregate = enc.aggregate;
  }
  if (enc.timeUnit) {
    def.timeUnit = enc.timeUnit;
  }
  if (enc.bin) {
    def.bin = true;
  }
  if (enc.sort) {
    def.sort = parseSort(enc.sort);
  }
  if (enc.stack) {
    def.stack = enc.stack === 'none' ? null : enc.stack;
  }
  if (enc.title) {
    def.title = enc.title;
  }
  if (enc.format) {
    def.format = enc.format;
  }
  if (enc.scale) {
    def.scale = enc.scale;
  }
  if (enc.axis !== undefined) {
    def.axis = enc.axis; // null removes the axis
  }
  if (enc.legend !== undefined) {
    def.legend = enc.legend; // null removes the legend
  }
  if (enc.condition) {
    def.condition = enc.condition;
  }
  if (enc.advancedJson) {
    const parsed = parseJsonObject(enc.advancedJson);
    if (parsed.error) {
      errors.push(`Encoding "${enc.channel}" advanced JSON: ${parsed.error}`);
    } else if (parsed.value) {
      return deepMerge(def, parsed.value);
    }
  }
  return def;
}

function buildEncoding(encodings: EncodingModel[] | undefined, errors: string[]): SpecObject {
  const encoding: SpecObject = {};
  for (const enc of encodings ?? []) {
    if (enc.enabled === false) {
      continue;
    }
    const def = buildChannel(enc, errors);
    if (!def) {
      continue;
    }
    if (ARRAY_CHANNELS.has(enc.channel)) {
      const existing = Array.isArray(encoding[enc.channel]) ? (encoding[enc.channel] as unknown[]) : [];
      existing.push(def);
      encoding[enc.channel] = existing;
    } else {
      encoding[enc.channel] = def;
    }
  }
  return encoding;
}

function buildTransforms(transforms: TransformModel[] | undefined, errors: string[]): unknown[] {
  const out: unknown[] = [];
  for (const transform of transforms ?? []) {
    if (transform.enabled === false || !transform.json || !transform.json.trim()) {
      continue;
    }
    const parsed = parseJsonValue(transform.json);
    if (parsed.error) {
      errors.push(`Transform "${transform.kind}": ${parsed.error}`);
      continue;
    }
    // Skip incomplete transforms (an empty `{}`) so they never break the spec.
    if (
      parsed.value === undefined ||
      (typeof parsed.value === 'object' &&
        parsed.value !== null &&
        !Array.isArray(parsed.value) &&
        Object.keys(parsed.value).length === 0)
    ) {
      continue;
    }
    out.push(parsed.value);
  }
  return out;
}

/**
 * Convert the visual builder model into a Vega-Lite spec fragment. Supports
 * single views and layered (multi-mark) views, structured encoding sub-defs
 * (scale/axis/legend/condition), transforms, parameters, and a view title.
 * Returns collected, non-fatal errors (invalid escape-hatch JSON).
 */
export function fromBuilder(model: BuilderModel): { spec: SpecObject; errors: string[] } {
  const errors: string[] = [];
  const sharedEncoding = buildEncoding(model.encodings, errors);
  const topTransforms = buildTransforms(model.transforms, errors);
  const layers = (model.layers ?? []).filter((l) => Boolean(l));

  let spec: SpecObject;
  if (layers.length > 0) {
    spec = {};
    if (Object.keys(sharedEncoding).length > 0) {
      spec.encoding = sharedEncoding;
    }
    spec.layer = layers.map((layer) => {
      const unit: SpecObject = { mark: buildMark(layer.mark, errors) };
      const layerEncoding = buildEncoding(layer.encodings, errors);
      if (Object.keys(layerEncoding).length > 0) {
        unit.encoding = layerEncoding;
      }
      const layerTransforms = buildTransforms(layer.transforms, errors);
      if (layerTransforms.length > 0) {
        unit.transform = layerTransforms;
      }
      if (layer.params && layer.params.length > 0) {
        unit.params = layer.params.map((p) => ({ name: p.name, ...p.spec }));
      }
      return unit;
    });
    if (model.resolve) {
      spec.resolve = model.resolve;
    }
  } else {
    spec = { mark: buildMark(model.mark, errors), encoding: sharedEncoding };
  }

  if (topTransforms.length > 0) {
    spec.transform = topTransforms;
  }
  if (model.params && model.params.length > 0) {
    spec.params = model.params.map((p) => ({ name: p.name, ...p.spec }));
  }
  if (model.extra) {
    // Top-level catch-all (lossless); the built structure wins over it.
    spec = deepMerge(model.extra, spec);
  }

  return { spec, errors };
}
