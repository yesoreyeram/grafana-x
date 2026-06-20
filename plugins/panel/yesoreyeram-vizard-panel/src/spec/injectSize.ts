import { SpecObject } from '../types';
import { isPlainObject } from './merge';

// Multi-view operators size themselves from their content, so we must not force a
// top-level width/height on them (it would be ignored or cause warnings). `layer`
// is intentionally excluded — layered single views accept width/height.
const MULTI_VIEW_KEYS = ['facet', 'hconcat', 'vconcat', 'concat', 'repeat'];
// Encoding-level facet channels also turn a unit spec into a faceted (multi-view)
// composition; a top-level width/height would otherwise be applied per facet cell.
const FACET_CHANNELS = ['row', 'column', 'facet'];

/**
 * True for facet / concat / repeat compositions (including encoding-level
 * `row`/`column`/`facet`). These size by content rather than to the panel, so the
 * panel renders them in a scrollable area instead of clipping the overflow.
 */
export function isMultiView(spec: SpecObject): boolean {
  if (MULTI_VIEW_KEYS.some((key) => key in spec)) {
    return true;
  }
  const encoding = spec.encoding;
  return isPlainObject(encoding) && FACET_CHANNELS.some((channel) => channel in encoding);
}

/**
 * Make the chart fill the panel. For single (and layered) views we set numeric
 * width/height from the panel size and `autosize: fit` so axes/legends are drawn
 * inside the box. Any width/height already on the spec is respected. Multi-view
 * specs are left to size by content.
 */
export function injectSize(spec: SpecObject, width: number, height: number): SpecObject {
  const out: SpecObject = { ...spec };

  if (isMultiView(out)) {
    return out;
  }

  if (out.width === undefined) {
    out.width = Math.max(0, Math.floor(width));
  }
  if (out.height === undefined) {
    out.height = Math.max(0, Math.floor(height));
  }
  if (out.autosize === undefined) {
    out.autosize = { type: 'fit', contains: 'padding' };
  }

  return out;
}
