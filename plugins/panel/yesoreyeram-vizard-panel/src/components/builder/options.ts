import { SelectableValue } from '@grafana/data';

import { AggregateOp, EncodingChannelName, MarkType, StackMode, TransformKind, VegaLiteFieldType } from '../../types';

export const MARK_OPTIONS: Array<SelectableValue<MarkType>> = [
  { label: 'Line', value: 'line' },
  { label: 'Bar', value: 'bar' },
  { label: 'Area', value: 'area' },
  { label: 'Point', value: 'point' },
  { label: 'Circle', value: 'circle' },
  { label: 'Square', value: 'square' },
  { label: 'Tick', value: 'tick' },
  { label: 'Rect (heatmap)', value: 'rect' },
  { label: 'Rule', value: 'rule' },
  { label: 'Text', value: 'text' },
  { label: 'Arc (pie)', value: 'arc' },
  { label: 'Trail', value: 'trail' },
  { label: 'Geoshape', value: 'geoshape' },
  { label: 'Box plot', value: 'boxplot' },
  { label: 'Error band', value: 'errorband' },
  { label: 'Error bar', value: 'errorbar' },
  { label: 'Image', value: 'image' },
];

export const CHANNEL_OPTIONS: Array<SelectableValue<EncodingChannelName>> = [
  { label: 'x', value: 'x', description: 'Horizontal position' },
  { label: 'y', value: 'y', description: 'Vertical position' },
  { label: 'x2', value: 'x2', description: 'Secondary x (ranges)' },
  { label: 'y2', value: 'y2', description: 'Secondary y (ranges)' },
  { label: 'xOffset', value: 'xOffset', description: 'Sub-position (grouped bars)' },
  { label: 'yOffset', value: 'yOffset', description: 'Sub-position' },
  { label: 'theta', value: 'theta', description: 'Arc angle (pie)' },
  { label: 'theta2', value: 'theta2' },
  { label: 'radius', value: 'radius' },
  { label: 'radius2', value: 'radius2' },
  { label: 'color', value: 'color' },
  { label: 'fill', value: 'fill' },
  { label: 'stroke', value: 'stroke' },
  { label: 'opacity', value: 'opacity' },
  { label: 'fillOpacity', value: 'fillOpacity' },
  { label: 'strokeOpacity', value: 'strokeOpacity' },
  { label: 'size', value: 'size' },
  { label: 'angle', value: 'angle' },
  { label: 'shape', value: 'shape' },
  { label: 'strokeWidth', value: 'strokeWidth' },
  { label: 'strokeDash', value: 'strokeDash' },
  { label: 'text', value: 'text' },
  { label: 'tooltip', value: 'tooltip', description: 'Repeatable' },
  { label: 'detail', value: 'detail', description: 'Repeatable; grouping' },
  { label: 'order', value: 'order', description: 'Repeatable; sort/stack order' },
  { label: 'key', value: 'key' },
  { label: 'longitude', value: 'longitude' },
  { label: 'latitude', value: 'latitude' },
  { label: 'longitude2', value: 'longitude2' },
  { label: 'latitude2', value: 'latitude2' },
  { label: 'row (facet)', value: 'row' },
  { label: 'column (facet)', value: 'column' },
  { label: 'facet', value: 'facet' },
  { label: 'description', value: 'description' },
];

export const TYPE_OPTIONS: Array<SelectableValue<VegaLiteFieldType>> = [
  { label: 'Quantitative', value: 'quantitative' },
  { label: 'Temporal', value: 'temporal' },
  { label: 'Ordinal', value: 'ordinal' },
  { label: 'Nominal', value: 'nominal' },
  { label: 'GeoJSON', value: 'geojson' },
];

export const AGGREGATE_OPTIONS: Array<SelectableValue<AggregateOp | ''>> = [
  { label: '(none)', value: '' },
  { label: 'count', value: 'count' },
  { label: 'sum', value: 'sum' },
  { label: 'mean', value: 'mean' },
  { label: 'average', value: 'average' },
  { label: 'median', value: 'median' },
  { label: 'min', value: 'min' },
  { label: 'max', value: 'max' },
  { label: 'distinct', value: 'distinct' },
  { label: 'valid', value: 'valid' },
  { label: 'stdev', value: 'stdev' },
  { label: 'stdevp', value: 'stdevp' },
  { label: 'variance', value: 'variance' },
  { label: 'variancep', value: 'variancep' },
  { label: 'q1', value: 'q1' },
  { label: 'q3', value: 'q3' },
  { label: 'ci0', value: 'ci0' },
  { label: 'ci1', value: 'ci1' },
  { label: 'first', value: 'first' },
  { label: 'last', value: 'last' },
  { label: 'mode', value: 'mode' },
  { label: 'product', value: 'product' },
];

export const TIME_UNIT_OPTIONS: Array<SelectableValue<string>> = [
  { label: '(none)', value: '' },
  { label: 'year', value: 'year' },
  { label: 'quarter', value: 'quarter' },
  { label: 'month', value: 'month' },
  { label: 'date', value: 'date' },
  { label: 'day', value: 'day' },
  { label: 'hours', value: 'hours' },
  { label: 'minutes', value: 'minutes' },
  { label: 'seconds', value: 'seconds' },
  { label: 'yearquarter', value: 'yearquarter' },
  { label: 'yearmonth', value: 'yearmonth' },
  { label: 'yearmonthdate', value: 'yearmonthdate' },
  { label: 'yearmonthdatehours', value: 'yearmonthdatehours' },
  { label: 'yearmonthdatehoursminutes', value: 'yearmonthdatehoursminutes' },
  { label: 'monthdate', value: 'monthdate' },
  { label: 'hoursminutes', value: 'hoursminutes' },
  { label: 'utcyearmonth', value: 'utcyearmonth' },
];

export const INTERPOLATE_OPTIONS: Array<SelectableValue<string>> = [
  { label: '(default)', value: '' },
  { label: 'linear', value: 'linear' },
  { label: 'monotone', value: 'monotone' },
  { label: 'basis', value: 'basis' },
  { label: 'cardinal', value: 'cardinal' },
  { label: 'natural', value: 'natural' },
  { label: 'step', value: 'step' },
  { label: 'step-before', value: 'step-before' },
  { label: 'step-after', value: 'step-after' },
];

export const STACK_OPTIONS: Array<SelectableValue<StackMode>> = [
  { label: 'zero', value: 'zero' },
  { label: 'normalize', value: 'normalize' },
  { label: 'center', value: 'center' },
  { label: 'none', value: 'none' },
];

/**
 * Color scheme options. The "Grafana" entries map to Grafana's standard
 * color-scheme modes (FieldColorModeId) and are resolved at render time via
 * `fieldColorModeRegistry.getColors(theme)`, so they use the active theme's
 * palette and gradients. The "Vega" entries are Vega color scheme names resolved
 * by Vega. Labels are prefixed instead of grouped so the panel option Select
 * renders them as a single flat, searchable list.
 */
export const COLOR_SCHEME_OPTIONS: Array<SelectableValue<string>> = [
  // Grafana categorical palettes (theme-aware)
  { label: 'Grafana · Classic palette', value: 'palette-classic' },
  { label: 'Grafana · Classic palette (by series name)', value: 'palette-classic-by-name' },
  // Grafana multiple continuous colors (by value)
  { label: 'Grafana · Green-Yellow-Red', value: 'continuous-GrYlRd' },
  { label: 'Grafana · Red-Yellow-Green', value: 'continuous-RdYlGr' },
  { label: 'Grafana · Blue-Yellow-Red', value: 'continuous-BlYlRd' },
  { label: 'Grafana · Yellow-Red', value: 'continuous-YlRd' },
  { label: 'Grafana · Blue-Purple', value: 'continuous-BlPu' },
  { label: 'Grafana · Yellow-Blue', value: 'continuous-YlBl' },
  // Grafana single continuous colors (by value)
  { label: 'Grafana · Blues', value: 'continuous-blues' },
  { label: 'Grafana · Reds', value: 'continuous-reds' },
  { label: 'Grafana · Greens', value: 'continuous-greens' },
  { label: 'Grafana · Purples', value: 'continuous-purples' },
  // Vega categorical schemes
  { label: 'Vega · Tableau 10', value: 'tableau10' },
  { label: 'Vega · Tableau 20', value: 'tableau20' },
  { label: 'Vega · Category 10', value: 'category10' },
  { label: 'Vega · Category 20', value: 'category20' },
  { label: 'Vega · Accent', value: 'accent' },
  { label: 'Vega · Dark 2', value: 'dark2' },
  { label: 'Vega · Paired', value: 'paired' },
  { label: 'Vega · Set 1', value: 'set1' },
  { label: 'Vega · Set 2', value: 'set2' },
  { label: 'Vega · Set 3', value: 'set3' },
  { label: 'Vega · Observable 10', value: 'observable10' },
  // Vega sequential / diverging schemes
  { label: 'Vega · Viridis', value: 'viridis' },
  { label: 'Vega · Magma', value: 'magma' },
  { label: 'Vega · Inferno', value: 'inferno' },
  { label: 'Vega · Plasma', value: 'plasma' },
  { label: 'Vega · Turbo', value: 'turbo' },
  { label: 'Vega · Blues', value: 'blues' },
  { label: 'Vega · Greens', value: 'greens' },
  { label: 'Vega · Oranges', value: 'oranges' },
  { label: 'Vega · Spectral', value: 'spectral' },
  { label: 'Vega · Red-Blue', value: 'redblue' },
];

/** Aggregate ops without the "(none)" entry, for the structured transform builder. */
export const AGGREGATE_OP_OPTIONS: Array<SelectableValue<string>> = AGGREGATE_OPTIONS.filter(
  (o) => o.value !== ''
) as Array<SelectableValue<string>>;

export const WINDOW_OP_OPTIONS: Array<SelectableValue<string>> = [
  { label: 'row_number', value: 'row_number' },
  { label: 'rank', value: 'rank' },
  { label: 'dense_rank', value: 'dense_rank' },
  { label: 'percent_rank', value: 'percent_rank' },
  { label: 'cume_dist', value: 'cume_dist' },
  { label: 'ntile', value: 'ntile' },
  { label: 'lag', value: 'lag' },
  { label: 'lead', value: 'lead' },
  { label: 'first_value', value: 'first_value' },
  { label: 'last_value', value: 'last_value' },
  { label: 'nth_value', value: 'nth_value' },
  { label: 'sum', value: 'sum' },
  { label: 'mean', value: 'mean' },
  { label: 'count', value: 'count' },
  { label: 'min', value: 'min' },
  { label: 'max', value: 'max' },
];

export const IMPUTE_METHOD_OPTIONS: Array<SelectableValue<string>> = [
  { label: 'value', value: 'value' },
  { label: 'mean', value: 'mean' },
  { label: 'median', value: 'median' },
  { label: 'max', value: 'max' },
  { label: 'min', value: 'min' },
];

export const REGRESSION_METHOD_OPTIONS: Array<SelectableValue<string>> = [
  { label: 'linear', value: 'linear' },
  { label: 'log', value: 'log' },
  { label: 'exp', value: 'exp' },
  { label: 'pow', value: 'pow' },
  { label: 'quad', value: 'quad' },
  { label: 'poly', value: 'poly' },
];

export const STACK_OFFSET_OPTIONS: Array<SelectableValue<string>> = [
  { label: 'zero', value: 'zero' },
  { label: 'normalize', value: 'normalize' },
  { label: 'center', value: 'center' },
];

export const SORT_ORDER_OPTIONS: Array<SelectableValue<string>> = [
  { label: 'ascending', value: 'ascending' },
  { label: 'descending', value: 'descending' },
];

export const TRANSFORM_OPTIONS: Array<SelectableValue<TransformKind>> = [
  { label: 'filter', value: 'filter' },
  { label: 'calculate', value: 'calculate' },
  { label: 'aggregate', value: 'aggregate' },
  { label: 'bin', value: 'bin' },
  { label: 'timeUnit', value: 'timeUnit' },
  { label: 'fold', value: 'fold' },
  { label: 'pivot', value: 'pivot' },
  { label: 'window', value: 'window' },
  { label: 'joinaggregate', value: 'joinaggregate' },
  { label: 'stack', value: 'stack' },
  { label: 'flatten', value: 'flatten' },
  { label: 'sample', value: 'sample' },
  { label: 'density', value: 'density' },
  { label: 'regression', value: 'regression' },
  { label: 'loess', value: 'loess' },
  { label: 'quantile', value: 'quantile' },
  { label: 'lookup', value: 'lookup' },
  { label: 'impute', value: 'impute' },
  { label: 'extent', value: 'extent' },
];

export const TRANSFORM_TEMPLATES: Record<TransformKind, string> = {
  filter: '{\n  "filter": "datum.value != null"\n}',
  calculate: '{\n  "calculate": "datum.value * 2",\n  "as": "doubled"\n}',
  aggregate: '{\n  "aggregate": [{ "op": "mean", "field": "value", "as": "mean_value" }],\n  "groupby": ["category"]\n}',
  bin: '{\n  "bin": true,\n  "field": "value",\n  "as": "binned_value"\n}',
  timeUnit: '{\n  "timeUnit": "yearmonth",\n  "field": "time",\n  "as": "month"\n}',
  fold: '{\n  "fold": ["a", "b"],\n  "as": ["key", "value"]\n}',
  pivot: '{\n  "pivot": "category",\n  "value": "value",\n  "groupby": ["time"]\n}',
  window:
    '{\n  "window": [{ "op": "rank", "as": "rank" }],\n  "sort": [{ "field": "value", "order": "descending" }]\n}',
  joinaggregate: '{\n  "joinaggregate": [{ "op": "sum", "field": "value", "as": "total" }],\n  "groupby": ["category"]\n}',
  stack: '{\n  "stack": "value",\n  "groupby": ["time"],\n  "as": ["v0", "v1"]\n}',
  flatten: '{\n  "flatten": ["arrayField"]\n}',
  sample: '{\n  "sample": 500\n}',
  density: '{\n  "density": "value",\n  "groupby": ["category"]\n}',
  regression: '{\n  "regression": "y",\n  "on": "x",\n  "groupby": ["series"]\n}',
  loess: '{\n  "loess": "y",\n  "on": "x",\n  "groupby": ["series"]\n}',
  quantile: '{\n  "quantile": "value",\n  "probs": [0.25, 0.5, 0.75]\n}',
  lookup: '{\n  "lookup": "key",\n  "from": { "data": { "name": "other" }, "key": "id", "fields": ["name"] }\n}',
  impute: '{\n  "impute": "value",\n  "key": "time",\n  "method": "value",\n  "value": 0\n}',
  extent: '{\n  "extent": "value",\n  "param": "value_extent"\n}',
};
