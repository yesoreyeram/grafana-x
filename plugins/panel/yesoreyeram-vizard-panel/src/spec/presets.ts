import { DataContext } from '../data/dataContext';
import {
  BuilderModel,
  defaultMark,
  EncodingChannelName,
  EncodingModel,
  LayerModel,
  MarkModel,
  PropMap,
  StackMode,
  TransformModel,
  VegaLiteFieldType,
} from '../types';
import { escapeFieldName } from './field';

/**
 * Chart-type presets. Each preset turns the *detected data shape* into a complete
 * single-view {@link BuilderModel} (mark + encodings + any needed fold transform)
 * that the user can then refine in the builder sections. They are best-effort
 * starting points: every preset always produces a VALID builder model for any
 * data shape (including empty), so applying one never breaks the panel.
 */
export type PresetId =
  | 'line'
  | 'multi-line'
  | 'area'
  | 'stacked-area'
  | 'streamgraph'
  | 'trellis-area'
  | 'bar'
  | 'grouped-bar'
  | 'stacked-bar'
  | 'horizontal-bar'
  | 'horizontal-stacked-bar'
  | 'diverging-stacked-bar'
  | 'pie'
  | 'donut'
  | 'radial'
  | 'scatter'
  | 'bubble'
  | 'point'
  | 'ternary'
  | 'histogram'
  | 'heatmap'
  | 'error-bar';

export type PresetGroup = 'Lines & areas' | 'Bars' | 'Parts of a whole' | 'Points' | 'Distribution';

export interface PresetDef {
  id: PresetId;
  label: string;
  group: PresetGroup;
  description: string;
}

/** Display order (grouped). The editor groups by `group` in this order. */
export const PRESETS: PresetDef[] = [
  { id: 'line', label: 'Line', group: 'Lines & areas', description: 'A single line over the index.' },
  { id: 'multi-line', label: 'Multi-line', group: 'Lines & areas', description: 'One line per series.' },
  { id: 'area', label: 'Area', group: 'Lines & areas', description: 'A filled area over the index.' },
  { id: 'stacked-area', label: 'Stacked area', group: 'Lines & areas', description: 'Series stacked as areas.' },
  { id: 'streamgraph', label: 'Streamgraph', group: 'Lines & areas', description: 'Series stacked and centered around a baseline.' },
  { id: 'trellis-area', label: 'Trellis area', group: 'Lines & areas', description: 'Small-multiple areas, one row per series.' },
  { id: 'bar', label: 'Bar', group: 'Bars', description: 'Bars over the index or category.' },
  { id: 'grouped-bar', label: 'Grouped bar', group: 'Bars', description: 'Series drawn side by side.' },
  { id: 'stacked-bar', label: 'Stacked bar', group: 'Bars', description: 'Series stacked into one vertical bar.' },
  { id: 'horizontal-bar', label: 'Horizontal bar', group: 'Bars', description: 'Bars along the x-axis.' },
  { id: 'horizontal-stacked-bar', label: 'Horizontal stacked bar', group: 'Bars', description: 'Series stacked into one horizontal bar.' },
  { id: 'diverging-stacked-bar', label: 'Diverging stacked bar', group: 'Bars', description: 'Stacked bars centered on a baseline.' },
  { id: 'pie', label: 'Pie', group: 'Parts of a whole', description: 'Proportions as slices.' },
  { id: 'donut', label: 'Donut', group: 'Parts of a whole', description: 'A pie with a hollow center.' },
  { id: 'radial', label: 'Radial plot', group: 'Parts of a whole', description: 'Arcs whose angle and radius encode the value.' },
  { id: 'scatter', label: 'Scatter', group: 'Points', description: 'Two measures as points.' },
  { id: 'bubble', label: 'Bubble', group: 'Points', description: 'A scatter sized by a measure.' },
  { id: 'point', label: 'Points', group: 'Points', description: 'Points over the index.' },
  { id: 'ternary', label: 'Ternary', group: 'Points', description: 'Three measures projected onto a triangle.' },
  { id: 'histogram', label: 'Histogram', group: 'Distribution', description: 'Binned counts of a measure.' },
  { id: 'heatmap', label: 'Heatmap', group: 'Distribution', description: 'A color-encoded matrix.' },
  { id: 'error-bar', label: 'Error bar', group: 'Distribution', description: 'Mean points with error bars per category.' },
];

const LINE: MarkModel = { type: 'line', tooltip: true };
const AREA: MarkModel = { type: 'area', tooltip: true };
const BAR: MarkModel = { type: 'bar', tooltip: true };
const POINT: MarkModel = { type: 'point', tooltip: true, filled: true };
const ARC: MarkModel = { type: 'arc', tooltip: true };
const RECT: MarkModel = { type: 'rect', tooltip: true };

interface PresetResult {
  mark: MarkModel;
  encodings: EncodingModel[];
  transforms: TransformModel[];
  /** Layers for multi-mark presets (e.g. error bar = error bar + mean point). */
  layers?: LayerModel[];
}

/** Build an encoding row. The channel doubles as the id (channels are unique per preset). */
function e(channel: EncodingChannelName, opts: Omit<Partial<EncodingModel>, 'id' | 'channel'>): EncodingModel {
  return { id: channel, channel, ...opts };
}

function typeOf(ctx: DataContext, name: string | undefined): VegaLiteFieldType {
  return ctx.fields.find((f) => f.name === name)?.vegaLiteType ?? 'nominal';
}

function nonColliding(base: string, used: Set<string>): string {
  if (!used.has(base)) {
    return base;
  }
  let i = 2;
  while (used.has(`${base}${i}`)) {
    i++;
  }
  return `${base}${i}`;
}

/**
 * Resolve how to express "value over index, optionally split by series" for the
 * current data. Wide multi-measure frames are folded into a long key/value table
 * (only when `multi`); long/merged frames already carry a series dimension.
 */
interface Series {
  index?: string;
  indexType: VegaLiteFieldType;
  /** The y/measure field name (a real column, or the folded value column). */
  value: string;
  /** True when there is no measure to plot, so y should be an aggregate count. */
  valueIsCount: boolean;
  /** The series/color field, present only when splitting into multiple series. */
  series?: string;
  /** A fold transform, present only when wide measures were folded. */
  fold?: TransformModel;
}

function seriesContext(ctx: DataContext, multi: boolean): Series {
  const index = ctx.indexField;
  const indexType = typeOf(ctx, index);
  const value0 = ctx.valueFields[0];

  // Already a long / merged series: one measure plus a series dimension.
  if (ctx.seriesField) {
    return {
      index,
      indexType,
      value: value0 ?? 'value',
      valueIsCount: !value0,
      series: multi ? ctx.seriesField : undefined,
    };
  }

  // Wide multiple measures: fold them into a long table so they can share a color.
  if (multi && ctx.valueFields.length > 1) {
    const used = new Set(ctx.fields.map((f) => f.name));
    const key = nonColliding('key', used);
    const value = nonColliding('value', used);
    return {
      index,
      indexType,
      value,
      valueIsCount: false,
      series: key,
      fold: {
        id: 'fold',
        kind: 'fold',
        json: JSON.stringify({ fold: ctx.valueFields.map(escapeFieldName), as: [key, value] }),
      },
    };
  }

  return { index, indexType, value: value0 ?? 'value', valueIsCount: !value0 };
}

function foldList(s: Series): TransformModel[] {
  return s.fold ? [s.fold] : [];
}

function yEnc(s: Series, stack?: StackMode): EncodingModel {
  if (s.valueIsCount) {
    return e('y', { aggregate: 'count', type: 'quantitative', ...(stack ? { stack } : {}) });
  }
  return e('y', { field: s.value, type: 'quantitative', ...(stack ? { stack } : {}) });
}

/** The best categorical field for a single-series bar/pie (prefers a real dimension). */
function categoryField(ctx: DataContext): { name?: string; type: VegaLiteFieldType } {
  const nominal = ctx.fields.find((f) => f.vegaLiteType === 'nominal' || f.vegaLiteType === 'ordinal');
  const name = ctx.seriesField ?? nominal?.name ?? ctx.indexField;
  return { name, type: typeOf(ctx, name) };
}

function colorEnc(name: string | undefined): EncodingModel[] {
  return name ? [e('color', { field: name, type: 'nominal' })] : [];
}

/** Line / area / point over the index, optionally split by series. */
function overIndex(ctx: DataContext, mark: MarkModel, multi: boolean, stack?: StackMode): PresetResult {
  const s = seriesContext(ctx, multi);
  const encodings = [e('x', { field: s.index, type: s.indexType }), yEnc(s, stack), ...colorEnc(s.series)];
  return { mark, encodings, transforms: foldList(s) };
}

function scatter(ctx: DataContext): PresetResult {
  const q = ctx.valueFields;
  const xField = q[0] ?? ctx.indexField;
  const xType: VegaLiteFieldType = q[0] ? 'quantitative' : typeOf(ctx, xField);
  const yPick = q[1] ?? q[0];
  const yField = yPick ?? ctx.indexField;
  const yType: VegaLiteFieldType = yPick ? 'quantitative' : typeOf(ctx, yField);
  const encodings = [
    e('x', { field: xField, type: xType }),
    e('y', { field: yField, type: yType }),
    ...colorEnc(ctx.seriesField),
  ];
  return { mark: POINT, encodings, transforms: [] };
}

function pie(ctx: DataContext, donut: boolean): PresetResult {
  const cat = categoryField(ctx);
  const s = seriesContext(ctx, false);
  const theta = s.valueIsCount
    ? e('theta', { aggregate: 'count', type: 'quantitative' })
    : e('theta', { field: s.value, type: 'quantitative' });
  const color = cat.name ? [e('color', { field: cat.name, type: cat.type })] : [];
  const mark: MarkModel = donut ? { type: 'arc', tooltip: true, props: { innerRadius: 60 } } : ARC;
  return { mark, encodings: [theta, ...color], transforms: [] };
}

function histogram(ctx: DataContext): PresetResult {
  const field = ctx.valueFields[0];
  if (!field) {
    const cat = categoryField(ctx);
    return {
      mark: BAR,
      encodings: [e('x', { field: cat.name, type: cat.type }), e('y', { aggregate: 'count', type: 'quantitative' })],
      transforms: [],
    };
  }
  return {
    mark: BAR,
    encodings: [e('x', { field, type: 'quantitative', bin: true }), e('y', { aggregate: 'count', type: 'quantitative' })],
    transforms: [],
  };
}

function heatmap(ctx: DataContext): PresetResult {
  const x = ctx.indexField;
  const xType = typeOf(ctx, x);
  const cat = categoryField(ctx);
  const s = seriesContext(ctx, false);
  const color = s.valueIsCount
    ? e('color', { aggregate: 'count', type: 'quantitative' })
    : e('color', { field: s.value, type: 'quantitative' });
  const encodings: EncodingModel[] = [e('x', { field: x, type: xType })];
  if (cat.name && cat.name !== x) {
    encodings.push(e('y', { field: cat.name, type: cat.type }));
  }
  encodings.push(color);
  return { mark: RECT, encodings, transforms: [] };
}

/** Stacked bars/areas along the index, with an optional explicit orientation. */
function stacked(ctx: DataContext, mark: MarkModel, horizontal: boolean, stack: StackMode): PresetResult {
  const s = seriesContext(ctx, true);
  const measure = s.valueIsCount
    ? { aggregate: 'count' as const, type: 'quantitative' as const, stack }
    : { field: s.value, type: 'quantitative' as const, stack };
  const index = { field: s.index, type: s.indexType };
  const encodings = horizontal
    ? [e('y', index), e('x', measure), ...colorEnc(s.series)]
    : [e('x', index), e('y', measure), ...colorEnc(s.series)];
  return { mark, encodings, transforms: foldList(s) };
}

/** A streamgraph: stacked areas centered on a baseline with smooth interpolation. */
function streamgraph(ctx: DataContext): PresetResult {
  const s = seriesContext(ctx, true);
  const mark: MarkModel = { type: 'area', tooltip: true, interpolate: 'monotone' };
  const y = s.valueIsCount
    ? e('y', { aggregate: 'count', type: 'quantitative', stack: 'center' })
    : e('y', { field: s.value, type: 'quantitative', stack: 'center' });
  return { mark, encodings: [e('x', { field: s.index, type: s.indexType }), y, ...colorEnc(s.series)], transforms: foldList(s) };
}

/** Trellis (small multiples): one area row per series. */
function trellisArea(ctx: DataContext): PresetResult {
  const s = seriesContext(ctx, true);
  const encodings = [e('x', { field: s.index, type: s.indexType }), yEnc(s), ...colorEnc(s.series)];
  if (s.series) {
    encodings.push(e('row', { field: s.series, type: 'nominal' }));
  }
  return { mark: AREA, encodings, transforms: foldList(s) };
}

/**
 * Radial plot: one arc per category whose angle and radius both encode the
 * aggregated value. The value is aggregated (sum/count) so the chart stays
 * readable even on raw, many-row data.
 */
function radial(ctx: DataContext): PresetResult {
  const cat = categoryField(ctx);
  const s = seriesContext(ctx, false);
  const valueOpts = s.valueIsCount
    ? { aggregate: 'count' as const, type: 'quantitative' as const }
    : { aggregate: 'sum' as const, field: s.value, type: 'quantitative' as const };
  const encodings = [
    e('theta', { ...valueOpts, stack: 'zero' }),
    e('radius', { ...valueOpts, scale: { type: 'sqrt', zero: true, rangeMin: 20 } }),
    ...(cat.name ? [e('color', { field: cat.name, type: cat.type })] : []),
  ];
  return { mark: { type: 'arc', tooltip: true, props: { stroke: '#ffffff' } }, encodings, transforms: [] };
}

/** Ternary plot: three measures projected onto a 2D triangle via calculate transforms. */
function ternary(ctx: DataContext): PresetResult {
  const q = ctx.valueFields;
  if (q.length < 3) {
    return scatter(ctx);
  }
  const ref = (n: string) => `datum[${JSON.stringify(n)}]`;
  const total = `(${ref(q[0])} + ${ref(q[1])} + ${ref(q[2])})`;
  const transforms: TransformModel[] = [
    { id: 'tx', kind: 'calculate', json: JSON.stringify({ calculate: `(${ref(q[1])} / ${total}) + (${ref(q[2])} / ${total}) / 2`, as: 'ternary_x' }) },
    { id: 'ty', kind: 'calculate', json: JSON.stringify({ calculate: `(${ref(q[2])} / ${total}) * 0.8660254`, as: 'ternary_y' }) },
  ];
  const encodings = [
    e('x', { field: 'ternary_x', type: 'quantitative', scale: { domain: [0, 1] }, axis: null }),
    e('y', { field: 'ternary_y', type: 'quantitative', scale: { domain: [0, 0.9] }, axis: null }),
    ...colorEnc(ctx.seriesField),
  ];
  return { mark: POINT, encodings, transforms };
}

/** Error bar: per-category error bars layered with the mean point. */
function errorBar(ctx: DataContext): PresetResult {
  const cat = categoryField(ctx);
  const s = seriesContext(ctx, false);
  const xOpts = cat.name ? { field: cat.name, type: cat.type } : { field: s.index, type: s.indexType };
  const yField = s.value;
  const layers: LayerModel[] = [
    {
      id: 'errorbar',
      mark: { type: 'errorbar', tooltip: true, props: { ticks: true } },
      encodings: [e('y', { field: yField, type: 'quantitative' })],
    },
    {
      id: 'mean',
      mark: { type: 'point', tooltip: true, filled: true, props: { color: 'black', size: 40 } },
      encodings: [e('y', { field: yField, type: 'quantitative', aggregate: 'mean' })],
    },
  ];
  return { mark: POINT, encodings: [e('x', xOpts)], transforms: [], layers };
}

const BUILDERS: Record<PresetId, (ctx: DataContext) => PresetResult> = {
  line: (ctx) => overIndex(ctx, LINE, false),
  'multi-line': (ctx) => overIndex(ctx, LINE, true),
  area: (ctx) => overIndex(ctx, AREA, false),
  'stacked-area': (ctx) => overIndex(ctx, AREA, true, 'zero'),
  streamgraph,
  'trellis-area': trellisArea,
  bar: (ctx) => {
    const cat = categoryField(ctx);
    const s = seriesContext(ctx, false);
    const x = cat.name ? e('x', { field: cat.name, type: cat.type }) : e('x', { field: s.index, type: s.indexType });
    return { mark: BAR, encodings: [x, yEnc(s)], transforms: [] };
  },
  'grouped-bar': (ctx) => {
    const s = seriesContext(ctx, true);
    const encodings = [e('x', { field: s.index, type: s.indexType }), yEnc(s), ...colorEnc(s.series)];
    if (s.series) {
      encodings.push(e('xOffset', { field: s.series, type: 'nominal' }));
    }
    return { mark: BAR, encodings, transforms: foldList(s) };
  },
  'stacked-bar': (ctx) => {
    const s = seriesContext(ctx, true);
    return {
      mark: BAR,
      encodings: [e('x', { field: s.index, type: s.indexType }), yEnc(s, 'zero'), ...colorEnc(s.series)],
      transforms: foldList(s),
    };
  },
  'horizontal-bar': (ctx) => {
    const cat = categoryField(ctx);
    const s = seriesContext(ctx, false);
    const y = cat.name ? e('y', { field: cat.name, type: cat.type }) : e('y', { field: s.index, type: s.indexType });
    const x = s.valueIsCount
      ? e('x', { aggregate: 'count', type: 'quantitative' })
      : e('x', { field: s.value, type: 'quantitative' });
    return { mark: BAR, encodings: [x, y], transforms: [] };
  },
  'horizontal-stacked-bar': (ctx) => stacked(ctx, BAR, true, 'zero'),
  'diverging-stacked-bar': (ctx) => stacked(ctx, BAR, true, 'center'),
  pie: (ctx) => pie(ctx, false),
  donut: (ctx) => pie(ctx, true),
  radial,
  scatter,
  bubble: (ctx) => {
    const sc = scatter(ctx);
    const q = ctx.valueFields;
    const sizeField = q[2] ?? q[1] ?? q[0];
    const encodings = sizeField
      ? [...sc.encodings, e('size', { field: sizeField, type: 'quantitative' })]
      : sc.encodings;
    return { ...sc, encodings };
  },
  point: (ctx) => overIndex(ctx, POINT, true),
  ternary,
  histogram,
  heatmap,
  'error-bar': errorBar,
};

/**
 * Null any key present in `prev` but absent in `next`. Grafana's panel-options
 * update deep-MERGES objects (it only replaces arrays and assigns null/values,
 * skipping `undefined`), so simply omitting a key keeps the old value. Nulling
 * forces the merge to clear it; `fromBuilder` then ignores null mark props.
 */
function withCleared<T extends Record<string, unknown>>(next: T, prev?: Record<string, unknown>): T {
  if (!prev) {
    return next;
  }
  const out: Record<string, unknown> = { ...next };
  for (const key of Object.keys(prev)) {
    if (!(key in out)) {
      out[key] = null;
    }
  }
  return out as T;
}

/** A mark that fully replaces the previous one under Grafana's object-merge. */
function replacementMark(next: MarkModel, prev?: MarkModel): MarkModel {
  const props = withCleared({ ...(next.props ?? {}) }, prev?.props as PropMap | undefined);
  const merged = withCleared(
    { ...next, props: Object.keys(props).length > 0 ? props : undefined } as Record<string, unknown>,
    prev as unknown as Record<string, unknown> | undefined
  );
  return merged as unknown as MarkModel;
}

/**
 * Apply a preset to the current data, returning a fresh single-view builder
 * model. A preset is a whole new chart, so layers / params / resolve / extra and
 * stale mark properties are cleared; the escape-hatch JSON (`configJson`,
 * `specOverrideJson`) is preserved when switching between chart presets.
 *
 * `'none'` is a full reset to a single default mark: it additionally clears the
 * escape-hatch JSON (a spec override can define a layered / multi-mark chart),
 * leaving exactly one default mark.
 *
 * Cleared fields are set to explicit empty values (arrays -> `[]`, objects ->
 * null, strings -> '') rather than omitted, because Grafana's option-merge keeps
 * omitted keys (see {@link withCleared}).
 */
export function applyPreset(id: PresetId | 'none', ctx: DataContext, current?: BuilderModel): BuilderModel {
  const result: PresetResult =
    id === 'none' ? { mark: { ...defaultMark }, encodings: [], transforms: [] } : BUILDERS[id](ctx);
  const cleared = {
    mark: replacementMark(result.mark, current?.mark),
    encodings: result.encodings,
    transforms: result.transforms,
    layers: result.layers ?? [],
    params: [],
    resolve: null,
    extra: null,
    configJson: id === 'none' ? '' : current?.configJson ?? '',
    specOverrideJson: id === 'none' ? '' : current?.specOverrideJson ?? '',
  };
  return cleared as unknown as BuilderModel;
}
