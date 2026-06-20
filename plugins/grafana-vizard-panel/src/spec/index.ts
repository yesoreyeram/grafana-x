import { DataContext } from '../data/dataContext';
import { PanelOptions, SpecObject, VegaLiteSpec } from '../types';
import { fromBuilder } from './fromBuilder';
import { injectData } from './injectData';
import { injectSize, isMultiView } from './injectSize';
import { injectTheme } from './injectTheme';
import { injectZoom } from './injectZoom';
import { deepMerge, isPlainObject, parseJsonObject } from './merge';
import { sanitizeSpec } from './sanitizeSpec';
import { suggestSpec } from './suggest';

// Encoding channels that produce a legend. `config.legend.disable` alone does
// NOT remove a legend when the channel carries an explicit `legend` object
// (which the spec→builder converter adds), so we also null those out.
const LEGEND_CHANNELS = new Set([
  'color',
  'fill',
  'stroke',
  'size',
  'shape',
  'opacity',
  'fillOpacity',
  'strokeOpacity',
  'strokeWidth',
  'strokeDash',
  'angle',
]);

/** Remove all legends from a spec: set `legend: null` on every legend-bearing channel. */
function disableLegend(node: unknown): unknown {
  if (Array.isArray(node)) {
    return node.map(disableLegend);
  }
  if (!isPlainObject(node)) {
    return node;
  }
  const out: SpecObject = {};
  for (const [key, value] of Object.entries(node)) {
    if (key === 'data' || key === 'datasets') {
      out[key] = value;
      continue;
    }
    if (key === 'encoding' && isPlainObject(value)) {
      const enc: SpecObject = {};
      for (const [ch, def] of Object.entries(value)) {
        if (LEGEND_CHANNELS.has(ch) && isPlainObject(def)) {
          enc[ch] = { ...(disableLegend(def) as SpecObject), legend: null };
        } else {
          enc[ch] = disableLegend(def);
        }
      }
      out[key] = enc;
    } else {
      out[key] = disableLegend(value);
    }
  }
  return out;
}

/** Remove all tooltips from a spec: drop `tooltip` encodings and set marks' tooltip off. */
function disableTooltip(node: unknown): unknown {
  if (Array.isArray(node)) {
    return node.map(disableTooltip);
  }
  if (!isPlainObject(node)) {
    return node;
  }
  const out: SpecObject = {};
  for (const [key, value] of Object.entries(node)) {
    if (key === 'encoding' && isPlainObject(value)) {
      const enc: SpecObject = {};
      for (const [ch, def] of Object.entries(value)) {
        if (ch !== 'tooltip') {
          enc[ch] = disableTooltip(def);
        }
      }
      out[key] = enc;
    } else if (key === 'mark') {
      if (typeof value === 'string') {
        out[key] = { type: value, tooltip: false };
      } else if (isPlainObject(value)) {
        out[key] = { ...(disableTooltip(value) as SpecObject), tooltip: false };
      } else {
        out[key] = value;
      }
    } else {
      out[key] = disableTooltip(value);
    }
  }
  return out;
}

export interface BuildResult {
  spec: VegaLiteSpec;
  warnings: string[];
  /** True when an x time-range zoom selection was added (temporal single-view spec). */
  zoomEnabled: boolean;
  /** True for facet/concat/repeat compositions, which size by content (the panel
   *  scrolls them instead of clipping); single/layered views fit the panel. */
  multiView: boolean;
}

export interface SizeInput {
  width: number;
  height: number;
}

function activeEncodingCount(options: PanelOptions): number {
  return (options.builder?.encodings ?? []).filter(
    (e) => e.enabled !== false && (Boolean(e.field) || Boolean(e.value) || e.aggregate === 'count')
  ).length;
}

/** The builder is "configured" when it has encodings, layers, or params to build from. */
function isBuilderConfigured(options: PanelOptions): boolean {
  const b = options.builder;
  return activeEncodingCount(options) > 0 || (b?.layers?.length ?? 0) > 0 || (b?.params?.length ?? 0) > 0;
}

/**
 * The render pipeline. Picks a spec source (builder today; the switch leaves room
 * for raw-JSON / vega-lite-api later), then applies the shared, mode-independent
 * stages: data injection, security sanitization, theme, and sizing.
 *
 *   source -> injectData -> sanitize (security) -> injectTheme -> injectSize
 */
export function buildSpec(
  options: PanelOptions,
  ctx: DataContext,
  themeConfig: SpecObject,
  size: SizeInput
): BuildResult {
  const warnings: string[] = [];

  const builderResult = fromBuilder(options.builder);
  warnings.push(...builderResult.errors);

  const override = parseJsonObject(options.builder?.specOverrideJson);
  if (override.error) {
    warnings.push(`Spec override JSON: ${override.error}`);
  }
  const overrideValue = override.value;
  const hasOverride = Boolean(overrideValue) && Object.keys(overrideValue ?? {}).length > 0;
  const builderConfigured = isBuilderConfigured(options);

  let inner: SpecObject;
  if (builderConfigured) {
    // Typed builder (encodings / layers / params) drives the chart.
    inner = builderResult.spec;
  } else if (hasOverride) {
    // A full spec override (e.g. a multi-view / layered spec) IS the chart — start
    // from an empty base so the override is used verbatim, not merged onto a
    // single-view suggestion (which would produce invalid mark+layer specs).
    inner = {};
  } else {
    // No encodings and no override: auto-derive the encoding/transforms from the
    // data, but always honour the builder's mark and any explicit transforms so
    // changing the mark type takes effect immediately on a fresh panel.
    const suggestion = suggestSpec(ctx);
    inner = { ...suggestion, mark: builderResult.spec.mark };
    const suggested = Array.isArray(suggestion.transform) ? suggestion.transform : [];
    const fromBuilderTransforms = Array.isArray(builderResult.spec.transform) ? builderResult.spec.transform : [];
    const transforms = [...suggested, ...fromBuilderTransforms];
    if (transforms.length) {
      inner.transform = transforms;
    }
  }

  // Comprehensive catch-all: deep-merge the override (onto the builder when
  // encodings are set; it is the whole spec otherwise).
  if (hasOverride && overrideValue) {
    inner = deepMerge(inner, overrideValue);
  }

  // Shared, mode-independent stages.
  let spec = injectData(inner, ctx);
  spec = sanitizeSpec(spec); // security: applied to the fully-assembled spec

  const userConfig = parseJsonObject(options.builder?.configJson);
  if (userConfig.error) {
    warnings.push(`Config JSON: ${userConfig.error}`);
  }
  spec = injectTheme(spec, themeConfig, userConfig.value);

  spec = injectSize(spec, size.width, size.height);

  // Global legend / tooltip toggles. These are symmetric so the toggle ALWAYS
  // has an effect, even when the spec declared nothing:
  //  - tooltip ON  -> config.mark.tooltip:true shows encoded values on hover
  //    (no tooltip encoding needed); OFF -> strip tooltip encodings + mark off.
  //  - legend  ON  -> config.legend.disable:false; OFF -> config.legend.disable
  //    AND null every channel's `legend` (config.disable alone is overridden by
  //    an explicit channel-level legend object).
  const tooltipOn = options.tooltip !== false;
  const legendOn = options.legend !== false;
  if (!tooltipOn) {
    spec = disableTooltip(spec) as SpecObject;
  }
  if (!legendOn) {
    spec = disableLegend(spec) as SpecObject;
  }
  const baseConfig = isPlainObject(spec.config) ? spec.config : {};
  spec.config = deepMerge(baseConfig, {
    mark: { tooltip: tooltipOn },
    legend: { disable: !legendOn },
  });

  const zoom = injectZoom(spec);
  spec = zoom.spec;

  return {
    spec: spec as unknown as VegaLiteSpec,
    warnings,
    zoomEnabled: zoom.enabled,
    multiView: isMultiView(spec),
  };
}
