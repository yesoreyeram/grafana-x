import { createTheme, FieldType, toDataFrame } from '@grafana/data';
import { parse as vegaParse } from 'vega';
import { compile } from 'vega-lite';

import { buildDataContext, DataContext } from '../data/dataContext';
import { buildVegaConfig } from '../theme/grafanaTheme';
import { BuilderModel, defaultPanelOptions, EncodingChannelName, PanelOptions } from '../types';
import { fromBuilder } from './fromBuilder';
import { buildSpec } from './index';
import { applyPreset, PRESETS, PresetId } from './presets';

type CompileSpec = Parameters<typeof compile>[0];

const theme = createTheme();
const themeConfig = buildVegaConfig(theme, defaultPanelOptions.theme);
const size = { width: 600, height: 400 };

// --- Representative data shapes -------------------------------------------------

function wideTimeseries(): DataContext {
  return buildDataContext(
    [
      toDataFrame({
        fields: [
          { name: 'time', type: FieldType.time, values: [1, 2, 3] },
          { name: 'cpu', type: FieldType.number, values: [1, 2, 3] },
          { name: 'mem', type: FieldType.number, values: [4, 5, 6] },
        ],
      }),
    ],
    { source: 'auto' }
  );
}

function longSeries(): DataContext {
  return buildDataContext(
    [
      toDataFrame({
        refId: 'A',
        fields: [
          { name: 'time', type: FieldType.time, values: [1, 2] },
          { name: 'value', type: FieldType.number, values: [1, 2], labels: { host: 'a' } },
        ],
      }),
      toDataFrame({
        refId: 'B',
        fields: [
          { name: 'time', type: FieldType.time, values: [1, 2] },
          { name: 'value', type: FieldType.number, values: [3, 4], labels: { host: 'b' } },
        ],
      }),
    ],
    { source: 'auto' }
  );
}

function categorical(): DataContext {
  return buildDataContext(
    [
      toDataFrame({
        fields: [
          { name: 'category', type: FieldType.string, values: ['a', 'b', 'c'] },
          { name: 'amount', type: FieldType.number, values: [10, 20, 30] },
        ],
      }),
    ],
    { source: 'auto' }
  );
}

function numericMatrix(): DataContext {
  return buildDataContext(
    [
      toDataFrame({
        fields: [
          { name: 'x', type: FieldType.number, values: [1, 2, 3] },
          { name: 'y', type: FieldType.number, values: [4, 5, 6] },
          { name: 'z', type: FieldType.number, values: [7, 8, 9] },
        ],
      }),
    ],
    { source: 'auto' }
  );
}

function emptyCtx(): DataContext {
  return buildDataContext([], { source: 'auto' });
}

const SHAPES: Array<[string, () => DataContext]> = [
  ['wide timeseries', wideTimeseries],
  ['long series', longSeries],
  ['categorical', categorical],
  ['numeric matrix', numericMatrix],
  ['empty', emptyCtx],
];

function channelDef(model: BuilderModel, channel: EncodingChannelName) {
  return model.encodings.find((enc) => enc.channel === channel);
}

// --- Tests ----------------------------------------------------------------------

describe('chart-type presets', () => {
  beforeAll(() => {
    jest.spyOn(console, 'warn').mockImplementation(() => {});
    jest.spyOn(console, 'error').mockImplementation(() => {});
  });
  afterAll(() => {
    jest.restoreAllMocks();
  });

  const cases: Array<[PresetId, string, () => DataContext]> = PRESETS.flatMap((p) =>
    SHAPES.map(([shapeName, make]): [PresetId, string, () => DataContext] => [p.id, shapeName, make])
  );

  it.each(cases)('preset "%s" on %s compiles + parses to a valid Vega spec', (id, _shape, make) => {
    const ctx = make();
    const model = applyPreset(id, ctx);
    const options: PanelOptions = { ...defaultPanelOptions, builder: model };
    const { spec } = buildSpec(options, ctx, themeConfig, size);
    const result = compile(spec as unknown as CompileSpec);
    expect(result.spec).toBeTruthy();
    expect(() => vegaParse(result.spec)).not.toThrow();
  });

  it('folds wide measures into a colored series for multi-line', () => {
    const model = applyPreset('multi-line', wideTimeseries());
    expect(model.transforms).toHaveLength(1);
    expect(model.transforms[0].kind).toBe('fold');
    const fold = JSON.parse(model.transforms[0].json) as { fold: string[]; as: string[] };
    expect(fold.fold).toEqual(['cpu', 'mem']);
    expect(fold.as).toEqual(['key', 'value']);
    expect(channelDef(model, 'color')?.field).toBe('key');
    expect(channelDef(model, 'y')?.field).toBe('value');
  });

  it('uses the existing series dimension for multi-line on long data (no fold)', () => {
    const model = applyPreset('multi-line', longSeries());
    expect(model.transforms).toHaveLength(0);
    expect(channelDef(model, 'color')?.field).toBe('host');
    expect(channelDef(model, 'x')?.field).toBe('time');
  });

  it('keeps a single line uncolored', () => {
    const model = applyPreset('line', longSeries());
    expect(channelDef(model, 'color')).toBeUndefined();
    expect(model.transforms).toHaveLength(0);
  });

  it('stacks series for stacked-area and stacked-bar', () => {
    expect(channelDef(applyPreset('stacked-area', wideTimeseries()), 'y')?.stack).toBe('zero');
    expect(channelDef(applyPreset('stacked-bar', wideTimeseries()), 'y')?.stack).toBe('zero');
  });

  it('offsets series for grouped-bar', () => {
    const model = applyPreset('grouped-bar', longSeries());
    expect(channelDef(model, 'xOffset')?.field).toBe('host');
    expect(channelDef(model, 'color')?.field).toBe('host');
  });

  it('swaps axes for horizontal bars', () => {
    const model = applyPreset('horizontal-bar', categorical());
    expect(channelDef(model, 'x')?.type).toBe('quantitative');
    expect(channelDef(model, 'x')?.field).toBe('amount');
    expect(channelDef(model, 'y')?.type).toBe('nominal');
    expect(channelDef(model, 'y')?.field).toBe('category');
  });

  it('uses theta + color for pie and a hollow center for donut', () => {
    const pie = applyPreset('pie', categorical());
    expect(pie.mark.type).toBe('arc');
    expect(channelDef(pie, 'theta')?.field).toBe('amount');
    expect(channelDef(pie, 'color')?.field).toBe('category');

    const donut = applyPreset('donut', categorical());
    expect(donut.mark.props?.innerRadius).toBe(60);
  });

  it('bins a measure and counts for histogram', () => {
    const model = applyPreset('histogram', numericMatrix());
    expect(channelDef(model, 'x')?.bin).toBe(true);
    expect(channelDef(model, 'y')?.aggregate).toBe('count');
  });

  it('sizes points by a third measure for bubble', () => {
    const model = applyPreset('bubble', numericMatrix());
    expect(channelDef(model, 'x')?.field).toBe('x');
    expect(channelDef(model, 'y')?.field).toBe('y');
    expect(channelDef(model, 'size')?.field).toBe('z');
  });

  it('centers a smooth stacked area for streamgraph', () => {
    const model = applyPreset('streamgraph', wideTimeseries());
    expect(model.mark.type).toBe('area');
    expect(model.mark.interpolate).toBe('monotone');
    expect(channelDef(model, 'y')?.stack).toBe('center');
    expect(channelDef(model, 'color')?.field).toBe('key');
  });

  it('facets areas by series for trellis-area', () => {
    const model = applyPreset('trellis-area', longSeries());
    expect(model.mark.type).toBe('area');
    expect(channelDef(model, 'row')?.field).toBe('host');
  });

  it('builds a horizontal stacked bar (measure on x, index on y)', () => {
    const model = applyPreset('horizontal-stacked-bar', wideTimeseries());
    expect(channelDef(model, 'y')?.field).toBe('time');
    expect(channelDef(model, 'x')?.stack).toBe('zero');
  });

  it('centers the stack for a diverging stacked bar', () => {
    const model = applyPreset('diverging-stacked-bar', wideTimeseries());
    expect(channelDef(model, 'x')?.stack).toBe('center');
  });

  it('encodes both angle and radius for a radial plot', () => {
    const model = applyPreset('radial', categorical());
    expect(model.mark.type).toBe('arc');
    expect(channelDef(model, 'theta')?.field).toBe('amount');
    expect(channelDef(model, 'radius')?.field).toBe('amount');
    expect((channelDef(model, 'radius')?.scale as Record<string, unknown>)?.type).toBe('sqrt');
  });

  it('projects three measures with calculate transforms for ternary', () => {
    const model = applyPreset('ternary', numericMatrix());
    expect(model.transforms).toHaveLength(2);
    expect(model.transforms.every((t) => t.kind === 'calculate')).toBe(true);
    expect(channelDef(model, 'x')?.field).toBe('ternary_x');
    expect(channelDef(model, 'y')?.field).toBe('ternary_y');
  });

  it('falls back to a scatter when ternary lacks three measures', () => {
    const model = applyPreset('ternary', longSeries()); // one value field
    expect(model.mark.type).toBe('point');
    expect(model.transforms).toHaveLength(0);
  });

  it('layers error bars with a mean point for error-bar', () => {
    const model = applyPreset('error-bar', longSeries());
    expect(model.layers).toHaveLength(2);
    expect(model.layers?.[0].mark.type).toBe('errorbar');
    expect(model.layers?.[1].mark.type).toBe('point');
    expect(model.layers?.[1].encodings[0].aggregate).toBe('mean');
  });

  it('preserves the escape-hatch JSON when switching presets', () => {
    const current: BuilderModel = {
      ...defaultPanelOptions.builder,
      configJson: '{ "axis": { "grid": false } }',
      specOverrideJson: '{ "title": "x" }',
    };
    const model = applyPreset('bar', categorical(), current);
    expect(model.configJson).toBe('{ "axis": { "grid": false } }');
    expect(model.specOverrideJson).toBe('{ "title": "x" }');
    // ...but the chart structure is fully replaced (layers cleared, not omitted,
    // so Grafana's option-merge actually clears them).
    expect(model.layers).toEqual([]);
  });

  it('"none" is a full reset to a single default mark (clears props, layers, and escape hatches)', () => {
    const current: BuilderModel = {
      mark: { type: 'arc', tooltip: true, filled: true, opacity: 0.5, props: { innerRadius: 60, strokeDash: [6, 4] } },
      encodings: [{ id: 'theta', channel: 'theta', field: 'v', type: 'quantitative' }],
      transforms: [{ id: 'f', kind: 'fold', json: '{"fold":["a","b"]}' }],
      layers: [{ id: 'l', mark: { type: 'bar' }, encodings: [] }],
      params: [{ id: 'p', name: 'sel', spec: {} }],
      resolve: { scale: { color: 'independent' } },
      extra: { title: 'x' },
      configJson: '{ "axis": { "grid": false } }',
      specOverrideJson: '{ "layer": [{ "mark": "line" }, { "mark": "point" }] }',
    };
    const model = applyPreset('none', wideTimeseries(), current);

    // Structural fields are cleared with explicit empty values (not omitted), so
    // Grafana's deep object-merge actually drops the old values.
    expect(model.encodings).toEqual([]);
    expect(model.transforms).toEqual([]);
    expect(model.layers).toEqual([]);
    expect(model.params).toEqual([]);
    expect(model.resolve).toBeNull();
    expect(model.extra).toBeNull();
    expect(model.configJson).toBe('');
    expect(model.specOverrideJson).toBe('');

    // Stale mark props/typed-fields are nulled so the merge clears them; the
    // EFFECTIVE mark (after fromBuilder drops nulls) is a clean single mark.
    const { spec } = fromBuilder(model);
    expect(spec.mark).toEqual({ type: 'line', tooltip: true });
    expect(spec.layer).toBeUndefined();
  });
});
