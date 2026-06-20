import { createTheme, FieldType, toDataFrame } from '@grafana/data';

import { buildDataContext } from '../data/dataContext';
import { buildVegaConfig } from '../theme/grafanaTheme';
import { defaultPanelOptions, PanelOptions } from '../types';
import { buildSpec } from './index';

const theme = createTheme();

function tsContext() {
  return buildDataContext(
    [
      toDataFrame({
        fields: [
          { name: 'time', type: FieldType.time, values: [1, 2] },
          { name: 'cpu', type: FieldType.number, values: [1, 2] },
        ],
      }),
    ],
    { source: 'auto' }
  );
}

function rec(value: unknown): Record<string, unknown> {
  return value as Record<string, unknown>;
}

function markType(spec: unknown): string {
  const mark = rec(spec).mark;
  return typeof mark === 'string' ? mark : String(rec(mark).type);
}

describe('buildSpec', () => {
  const themeConfig = buildVegaConfig(theme, defaultPanelOptions.theme);
  const size = { width: 300, height: 200 };

  it('honours the builder mark even when no encodings are configured (regression)', () => {
    const options: PanelOptions = {
      ...defaultPanelOptions,
      builder: { ...defaultPanelOptions.builder, mark: { type: 'bar', tooltip: true }, encodings: [] },
    };
    const { spec } = buildSpec(options, tsContext(), themeConfig, size);
    expect(markType(spec)).toBe('bar');
    // data + size + theme are injected
    expect(Array.isArray(rec(rec(spec).data).values)).toBe(true);
    expect(rec(spec).width).toBe(300);
    expect(rec(rec(spec).config).background).toBe('transparent');
  });

  it('uses configured encodings when present', () => {
    const options: PanelOptions = {
      ...defaultPanelOptions,
      builder: {
        ...defaultPanelOptions.builder,
        mark: { type: 'point' },
        encodings: [
          { id: 'x', channel: 'x', field: 'time', type: 'temporal' },
          { id: 'y', channel: 'y', field: 'cpu', type: 'quantitative' },
        ],
      },
    };
    const { spec } = buildSpec(options, tsContext(), themeConfig, size);
    expect(markType(spec)).toBe('point');
    const encoding = rec(rec(spec).encoding);
    expect(rec(encoding.x).field).toBe('time');
    expect(rec(encoding.y).field).toBe('cpu');
  });

  it('strips tooltips when the tooltip toggle is off', () => {
    const options: PanelOptions = {
      ...defaultPanelOptions,
      tooltip: false,
      builder: {
        ...defaultPanelOptions.builder,
        mark: { type: 'line', tooltip: true },
        encodings: [
          { id: 'x', channel: 'x', field: 'time', type: 'temporal' },
          { id: 'y', channel: 'y', field: 'cpu', type: 'quantitative' },
          { id: 't', channel: 'tooltip', field: 'cpu', type: 'quantitative' },
        ],
      },
    };
    const { spec } = buildSpec(options, tsContext(), themeConfig, size);
    expect(rec(rec(spec).mark).tooltip).toBe(false);
    expect(rec(rec(spec).encoding).tooltip).toBeUndefined();
  });

  it('disables all legends when the legend toggle is off', () => {
    const options: PanelOptions = { ...defaultPanelOptions, legend: false };
    const { spec } = buildSpec(options, tsContext(), themeConfig, size);
    expect(rec(rec(rec(spec).config).legend).disable).toBe(true);
  });

  it('nulls channel-level legends when off (regression: config.legend.disable alone is overridden)', () => {
    const options: PanelOptions = {
      ...defaultPanelOptions,
      legend: false,
      builder: {
        ...defaultPanelOptions.builder,
        mark: { type: 'point' },
        encodings: [
          { id: 'x', channel: 'x', field: 'time', type: 'temporal' },
          { id: 'y', channel: 'y', field: 'cpu', type: 'quantitative' },
          // An explicit channel-level legend object — config.legend.disable does
          // NOT remove this on its own, so the walker must null it.
          { id: 'c', channel: 'color', field: 'cpu', type: 'quantitative', legend: { title: 'CPU' } },
        ],
      },
    };
    const { spec } = buildSpec(options, tsContext(), themeConfig, size);
    expect(rec(rec(rec(spec).config).legend).disable).toBe(true);
    expect(rec(rec(rec(spec).encoding).color).legend).toBeNull();
  });

  it('keeps channel-level legends when on', () => {
    const options: PanelOptions = {
      ...defaultPanelOptions,
      legend: true,
      builder: {
        ...defaultPanelOptions.builder,
        mark: { type: 'point' },
        encodings: [
          { id: 'x', channel: 'x', field: 'time', type: 'temporal' },
          { id: 'y', channel: 'y', field: 'cpu', type: 'quantitative' },
          { id: 'c', channel: 'color', field: 'cpu', type: 'quantitative', legend: { title: 'CPU' } },
        ],
      },
    };
    const { spec } = buildSpec(options, tsContext(), themeConfig, size);
    expect(rec(rec(rec(spec).encoding).color).legend).not.toBeNull();
    expect(rec(rec(rec(rec(spec).encoding).color).legend).title).toBe('CPU');
  });

  it('turns tooltips and legends on by default via config (ON has effect even with no tooltip encoding)', () => {
    const { spec } = buildSpec(defaultPanelOptions, tsContext(), themeConfig, size);
    const config = rec(rec(spec).config);
    // ON is symmetric with OFF: it always writes config so the toggle has an
    // effect even when the spec declared no tooltip encoding / no legend.
    expect(rec(config.mark).tooltip).toBe(true);
    expect(rec(config.legend).disable).toBe(false);
  });

  it('enables tooltips through config when toggled on (regression: ON was a no-op)', () => {
    const options: PanelOptions = {
      ...defaultPanelOptions,
      tooltip: true,
      builder: {
        ...defaultPanelOptions.builder,
        // A mark that does NOT declare tooltip and encodings with no tooltip channel.
        mark: { type: 'line' },
        encodings: [
          { id: 'x', channel: 'x', field: 'time', type: 'temporal' },
          { id: 'y', channel: 'y', field: 'cpu', type: 'quantitative' },
        ],
      },
    };
    const { spec } = buildSpec(options, tsContext(), themeConfig, size);
    expect(rec(rec(rec(spec).config).mark).tooltip).toBe(true);
    expect(rec(rec(rec(spec).config).legend).disable).toBe(false);
  });

  it('flags facet (multi-view) specs so the panel scrolls them; single views are not flagged', () => {
    // Single view → not multi-view (the panel clips it to fit, no scrollbar).
    expect(buildSpec(defaultPanelOptions, tsContext(), themeConfig, size).multiView).toBe(false);

    // A row-encoding facet → multi-view (the panel makes it scrollable).
    const faceted: PanelOptions = {
      ...defaultPanelOptions,
      builder: {
        ...defaultPanelOptions.builder,
        mark: { type: 'area' },
        encodings: [
          { id: 'x', channel: 'x', field: 'time', type: 'temporal' },
          { id: 'y', channel: 'y', field: 'cpu', type: 'quantitative' },
          { id: 'r', channel: 'row', field: 'cpu', type: 'nominal' },
        ],
      },
    };
    expect(buildSpec(faceted, tsContext(), themeConfig, size).multiView).toBe(true);
  });
});
