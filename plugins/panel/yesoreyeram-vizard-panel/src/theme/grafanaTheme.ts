import { fieldColorModeRegistry, GrafanaTheme2 } from '@grafana/data';

import { SpecObject, ThemeOptions } from '../types';

// Back-compat alias for the original custom value.
const GRAFANA_CLASSIC_ALIASES = new Set(['grafana-classic', '']);

/**
 * Apply the chosen color scheme to the Vega-Lite config.
 *  - Grafana modes (`palette-classic`, `continuous-*`, ...) resolve to an
 *    explicit, theme-aware color array via the Grafana color registry, and set
 *    the default mark color so single-series marks also match.
 *  - Any other value is treated as a Vega color scheme name (`{ scheme }`),
 *    resolved by Vega at render time.
 *
 * The colors are applied to category/ordinal (discrete) and ramp/heatmap
 * (continuous) ranges so they drive both nominal and quantitative color.
 */
function applyColorScheme(config: SpecObject, colorScheme: string, theme: GrafanaTheme2): void {
  const id = GRAFANA_CLASSIC_ALIASES.has(colorScheme) ? 'palette-classic' : colorScheme;

  const mode = fieldColorModeRegistry.getIfExists(id);
  const grafanaColors = mode?.getColors?.(theme);
  if (grafanaColors && grafanaColors.length) {
    if (mode?.isContinuous) {
      // A gradient scheme drives both discrete (sampled) and continuous color.
      config.range = {
        category: grafanaColors,
        ordinal: grafanaColors,
        ramp: grafanaColors,
        heatmap: grafanaColors,
      };
      // Endpoints are near-white/near-black (poor contrast for a single mark),
      // so use a mid-gradient color.
      config.mark = { ...(config.mark as SpecObject), color: grafanaColors[Math.floor(grafanaColors.length / 2)] };
      return;
    }
    // Categorical palette: distinct colors for discrete (category/ordinal); use a
    // theme-aware sequential gradient for continuous (ramp/heatmap) color so
    // quantitative color isn't a rainbow of the categorical palette.
    const range: Record<string, unknown> = { category: grafanaColors, ordinal: grafanaColors };
    const sequential = fieldColorModeRegistry.getIfExists('continuous-blues')?.getColors?.(theme);
    if (sequential && sequential.length) {
      range.ramp = sequential;
      range.heatmap = sequential;
    }
    config.range = range;
    config.mark = { ...(config.mark as SpecObject), color: grafanaColors[0] };
    return;
  }

  // A Vega color scheme by name (applied to discrete and continuous ranges).
  config.range = {
    category: { scheme: id },
    ordinal: { scheme: id },
    ramp: { scheme: id },
    heatmap: { scheme: id },
  };
}

/**
 * Build a Vega-Lite `config` from the active Grafana theme. Structural theming
 * (fonts, text/grid/axis colors, transparent background, no view border) always
 * follows Grafana so charts stay readable; the categorical palette is selectable
 * via `options.colorScheme`. User `config` overrides are deep-merged on top of
 * this in `injectTheme`.
 */
export function buildVegaConfig(theme: GrafanaTheme2, options: ThemeOptions): SpecObject {
  const text = theme.colors.text.primary;
  const textWeak = theme.colors.text.secondary;
  const grid = theme.colors.border.weak;
  const axisLine = theme.colors.border.medium;
  const font = theme.typography.fontFamily;
  const fontSize = theme.typography.fontSize;

  const guideText = {
    labelColor: textWeak,
    titleColor: text,
    labelFont: font,
    titleFont: font,
    labelFontSize: Math.max(fontSize - 1, 10),
    titleFontSize: fontSize,
  };

  const config: SpecObject = {
    // Transparent so the Grafana panel background (incl. the native "Transparent
    // background" panel option) shows through.
    background: 'transparent',
    font,
    axis: {
      ...guideText,
      domainColor: axisLine,
      tickColor: axisLine,
      gridColor: grid,
      gridOpacity: 0.6,
      titleFontWeight: 'normal',
      labelOverlap: true,
    },
    legend: {
      ...guideText,
      titleFontWeight: 'normal',
      symbolType: 'circle',
    },
    header: guideText,
    title: {
      color: text,
      subtitleColor: textWeak,
      font,
      fontSize: fontSize + 2,
      fontWeight: 'bold' as const,
      anchor: 'start' as const,
    },
    view: {
      stroke: null,
      continuousWidth: 300,
      continuousHeight: 200,
    },
    mark: {},
    text: { color: text },
    style: {
      'guide-label': { fill: textWeak },
      'guide-title': { fill: text },
      'group-title': { fill: text },
    },
  };

  applyColorScheme(config, options.colorScheme, theme);

  return config;
}
