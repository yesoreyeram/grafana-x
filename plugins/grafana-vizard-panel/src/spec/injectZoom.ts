import { SpecObject } from '../types';
import { isPlainObject } from './merge';

/** Name of the interval-selection param used for time-range zooming. */
export const ZOOM_PARAM = '__grafana_zoom';

// Composition operators we never add zoom to. `layer` is included because a
// top-level interval selection on a multi-layer view that shares x compiles to
// duplicate Vega signals; restricting to single views keeps it robust.
const COMPOSITION_KEYS = ['facet', 'hconcat', 'vconcat', 'concat', 'repeat', 'layer'];

function temporalXField(spec: SpecObject): string | undefined {
  const encoding = spec.encoding;
  if (isPlainObject(encoding)) {
    const x = encoding.x;
    // Only a continuous temporal x (no timeUnit/bin/aggregate -> continuous
    // scale) supports a meaningful interval brush.
    if (
      isPlainObject(x) &&
      x.type === 'temporal' &&
      typeof x.field === 'string' &&
      x.timeUnit === undefined &&
      x.bin === undefined &&
      x.aggregate === undefined
    ) {
      return x.field;
    }
  }
  return undefined;
}

/** True if the spec already defines a selection param. */
function hasAnySelection(spec: SpecObject): boolean {
  return Array.isArray(spec.params) && spec.params.some((p) => isPlainObject(p) && isPlainObject(p.select));
}

/**
 * Enable native-style time-range zooming: for single (or layered) views with a
 * temporal x and no existing interval selection, add an x interval-selection
 * param. `VegaView` listens for it and propagates the brushed range to the
 * dashboard time range (`onChangeTimeRange`). Multi-view specs and specs that
 * already define their own interval interaction are left untouched.
 */
export function injectZoom(spec: SpecObject): { spec: SpecObject; enabled: boolean } {
  if (COMPOSITION_KEYS.some((key) => key in spec)) {
    return { spec, enabled: false };
  }
  // Don't interfere with specs that already define their own interactions.
  if (hasAnySelection(spec)) {
    return { spec, enabled: false };
  }
  if (!temporalXField(spec)) {
    return { spec, enabled: false };
  }
  const params = Array.isArray(spec.params) ? [...spec.params] : [];
  params.push({ name: ZOOM_PARAM, select: { type: 'interval', encodings: ['x'] } });
  return { spec: { ...spec, params }, enabled: true };
}

/**
 * Extract a numeric [from, to] range from a Vega interval-selection signal value
 * (shape `{ <field>: [lo, hi] }`). Returns null when there is no valid range.
 */
export function extractZoomRange(value: unknown): [number, number] | null {
  if (!isPlainObject(value)) {
    return null;
  }
  for (const v of Object.values(value)) {
    if (Array.isArray(v) && v.length === 2 && typeof v[0] === 'number' && typeof v[1] === 'number') {
      const lo = Math.min(v[0], v[1]);
      const hi = Math.max(v[0], v[1]);
      if (hi > lo) {
        return [lo, hi];
      }
    }
  }
  return null;
}
