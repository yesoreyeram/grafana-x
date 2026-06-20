import React, { Suspense, useMemo } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2, PanelProps } from '@grafana/data';
import { useStyles2, useTheme2 } from '@grafana/ui';

import { buildDataContext } from '../data/dataContext';
import { buildSpec } from '../spec';
import { buildVegaConfig } from '../theme/grafanaTheme';
import { defaultBuilder, defaultPanelOptions, PanelOptions } from '../types';
import { ErrorView } from './ErrorView';

// Lazy so the heavy vega/vega-lite/vega-embed bundle is split out of module.js.
const VegaView = React.lazy(() => import('./VegaView'));

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** True if the spec (or any child) carries its own renderable data. */
function specHasOwnData(node: unknown): boolean {
  if (Array.isArray(node)) {
    return node.some(specHasOwnData);
  }
  if (!isRecord(node)) {
    return false;
  }
  const data = node.data;
  if (isRecord(data)) {
    if (Array.isArray(data.values) && data.values.length > 0) {
      return true;
    }
    // Data generators produce their own data.
    if ('sequence' in data || 'sphere' in data || 'graticule' in data || 'name' in data) {
      return true;
    }
  }
  if (isRecord(node.datasets) && Object.keys(node.datasets).length > 0) {
    return true;
  }
  // Recurse into composition operators.
  return ['layer', 'hconcat', 'vconcat', 'concat', 'spec'].some((key) => specHasOwnData(node[key]));
}

/** Fill any missing options with defaults so partially-saved panels still work. */
function normalizeOptions(options: PanelOptions | undefined): PanelOptions {
  return {
    ...defaultPanelOptions,
    ...(options ?? {}),
    builder: { ...defaultBuilder, ...(options?.builder ?? {}) },
    data: { ...defaultPanelOptions.data, ...(options?.data ?? {}) },
    theme: { ...defaultPanelOptions.theme, ...(options?.theme ?? {}) },
    code: { ...defaultPanelOptions.code, ...(options?.code ?? {}) },
  };
}

export function Vizard(props: PanelProps<PanelOptions>) {
  const { data, width, height } = props;
  const theme = useTheme2();
  const styles = useStyles2(getStyles);

  const options = useMemo(() => normalizeOptions(props.options), [props.options]);
  const ctx = useMemo(() => buildDataContext(data.series, options.data), [data.series, options.data]);
  const themeConfig = useMemo(() => buildVegaConfig(theme, options.theme), [theme, options.theme]);

  const { spec, warnings, zoomEnabled, multiView } = useMemo(
    () => buildSpec(options, ctx, themeConfig, { width, height }),
    [options, ctx, themeConfig, width, height]
  );

  const onChangeTimeRange = props.onChangeTimeRange;
  const onBrush = useMemo(
    () =>
      zoomEnabled && onChangeTimeRange
        ? (from: number, to: number) => onChangeTimeRange({ from, to })
        : undefined,
    [zoomEnabled, onChangeTimeRange]
  );

  // Show "No data" only when there is genuinely nothing to draw: the query is
  // empty AND the spec doesn't carry its own data (a pasted spec override may
  // include inline data / datasets and should still render).
  if (ctx.isEmpty && !specHasOwnData(spec)) {
    return (
      <ErrorView
        title="No data"
        severity="info"
        message={data.errors?.[0]?.message ?? 'The query returned no rows to visualize.'}
      />
    );
  }

  return (
    <div className={styles.wrap} style={{ width, height }}>
      <Suspense fallback={<div className={styles.fill} />}>
        <VegaView
          spec={spec}
          renderer={options.renderer}
          tooltip={options.tooltip}
          tooltipTheme={theme.isDark ? 'dark' : 'light'}
          scrollable={multiView}
          onBrush={onBrush}
        />
      </Suspense>
      {warnings.length > 0 && (
        <div className={styles.warnings}>
          {warnings.map((w, i) => (
            <div key={i}>{w}</div>
          ))}
        </div>
      )}
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({
    position: 'relative',
    boxSizing: 'border-box',
    overflow: 'hidden',
  }),
  fill: css({
    width: '100%',
    height: '100%',
  }),
  warnings: css({
    position: 'absolute',
    left: theme.spacing(1),
    bottom: theme.spacing(1),
    maxWidth: '92%',
    maxHeight: '40%',
    overflow: 'auto',
    padding: theme.spacing(0.5, 1),
    background: theme.colors.warning.transparent,
    color: theme.colors.warning.text,
    border: `1px solid ${theme.colors.warning.borderTransparent}`,
    borderRadius: theme.shape.radius.default,
    fontSize: theme.typography.bodySmall.fontSize,
    pointerEvents: 'none',
    zIndex: 1,
  }),
});
