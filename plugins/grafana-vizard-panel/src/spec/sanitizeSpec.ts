import { SpecObject } from '../types';
import { isPlainObject } from './merge';

/**
 * Keys removed from every spec, at any depth (outside of inline data content),
 * before it is compiled and rendered.
 *
 * - `url`      blocks remote loading via mark/encoding props (e.g. `image` mark
 *              url). Remote `data.url` is handled separately (see `sanitizeData`).
 * - `href`     blocks clickable links whose target comes from (untrusted) data,
 *              removing a `javascript:`-URI click-XSS vector.
 * - `usermeta` blocks `usermeta.embedOptions`, which Vega-Embed would otherwise
 *              read to override our hardened embed options (loader, actions, ast).
 */
const BLOCKED_KEYS = new Set(['url', 'href', 'usermeta']);

// Keys whose values are inline DATA content. We must not recurse into them and
// strip `url`/`href` (those would be legitimate data values, not loaders), and
// remote `data` sources are neutralized to empty inline data.
const DATA_KEY = 'data';
const DATASETS_KEY = 'datasets';

/**
 * Neutralize a `data` value: a remote source (`{ url }`, or a bare URL string)
 * becomes empty inline data so the spec stays valid and remote-free. Inline data
 * (`values` / `name` / `sequence` / `sphere` / `graticule`) is kept verbatim —
 * its contents are data, not loaders, so we do not descend into it.
 */
function sanitizeData(value: unknown): unknown {
  if (isPlainObject(value)) {
    if ('url' in value) {
      return { values: [] };
    }
    return value;
  }
  // A bare string data value would be a URL — neutralize it.
  if (typeof value === 'string') {
    return { values: [] };
  }
  return value;
}

function walk(node: unknown): unknown {
  if (Array.isArray(node)) {
    return node.map(walk);
  }
  if (isPlainObject(node)) {
    const out: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(node)) {
      if (BLOCKED_KEYS.has(key)) {
        continue;
      }
      if (key === DATA_KEY) {
        out[key] = sanitizeData(value);
        continue;
      }
      if (key === DATASETS_KEY) {
        // Named inline datasets — keep verbatim (data content, not loaders).
        out[key] = value;
        continue;
      }
      out[key] = walk(value);
    }
    return out;
  }
  return node;
}

/**
 * Defense-in-depth sanitizer applied to EVERY spec regardless of how it was
 * produced (builder, spec override, future raw JSON / vega-lite-api). It returns
 * a deep clone with remote sources neutralized and click-XSS / embed-option
 * overrides stripped, so no spec can pull remote resources or escape the hardened
 * Vega-Embed options.
 *
 * Works together with (not instead of) the runtime guards in VegaView: a loader
 * that rejects all remote loads and `ast: true` for CSP-safe expressions.
 */
export function sanitizeSpec(spec: SpecObject): SpecObject {
  return walk(spec) as SpecObject;
}
