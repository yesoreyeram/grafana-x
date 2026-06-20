import { SpecObject } from '../types';
import { deepMerge, isPlainObject } from './merge';

/**
 * Merge the Vega-Lite `config` with precedence:
 *   Grafana theme  <  user config (builder `configJson`)  <  spec's own `config`
 *
 * The theme is therefore a base that users can refine, and an explicit `config`
 * carried by the spec always wins.
 */
export function injectTheme(
  spec: SpecObject,
  themeConfig: SpecObject,
  userConfig?: Record<string, unknown>
): SpecObject {
  let config = themeConfig;
  if (userConfig) {
    config = deepMerge(config, userConfig);
  }
  if (isPlainObject(spec.config)) {
    config = deepMerge(config, spec.config);
  }
  return { ...spec, config };
}
