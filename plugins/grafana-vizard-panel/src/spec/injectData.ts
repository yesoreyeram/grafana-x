import { DataContext } from '../data/dataContext';
import { SpecObject } from '../types';
import { isPlainObject } from './merge';

function hasInlineData(data: unknown): boolean {
  return isPlainObject(data) && ('values' in data || 'name' in data || 'sequence' in data);
}

/**
 * Attach the converted data frames to a spec:
 * - every frame (and the merged series table) is exposed as a named dataset so
 *   specs can reference them by name, and
 * - if the spec doesn't already declare its own data, the primary frame's rows
 *   become the default inline `data`.
 *
 * Top-level data is inherited by child views, so this also feeds future
 * layer/facet/concat specs.
 */
export function injectData(spec: SpecObject, ctx: DataContext): SpecObject {
  const out: SpecObject = { ...spec };

  if (Object.keys(ctx.datasets).length > 0) {
    const existing = isPlainObject(out.datasets) ? out.datasets : {};
    out.datasets = { ...ctx.datasets, ...existing };
  }

  if (!hasInlineData(out.data)) {
    out.data = { values: ctx.primary.rows };
  }

  return out;
}
