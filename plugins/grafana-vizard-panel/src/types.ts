import type { TopLevelSpec, Config } from 'vega-lite';

/**
 * Public boundary types. The pipeline operates on plain objects (`SpecObject`)
 * and only casts to the strict Vega-Lite types at the very edge (compile/embed),
 * which keeps spec construction ergonomic without sacrificing the typed boundary.
 */
export type VegaLiteSpec = TopLevelSpec;
export type VegaLiteConfig = Config;
export type SpecObject = Record<string, unknown>;

/**
 * The source of the Vega-Lite spec. Only `builder` ships today, but the render
 * pipeline switches on this so additional sources (raw JSON, vega-lite-api) can
 * be added later without touching data/theme/security/size injection.
 */
export type EditorMode = 'builder' | 'code' | 'api';

export type RendererType = 'canvas' | 'svg';

export type VegaLiteFieldType = 'quantitative' | 'temporal' | 'ordinal' | 'nominal' | 'geojson';

export type MarkType =
  | 'bar'
  | 'line'
  | 'area'
  | 'point'
  | 'circle'
  | 'square'
  | 'tick'
  | 'rect'
  | 'rule'
  | 'text'
  | 'arc'
  | 'trail'
  | 'geoshape'
  | 'boxplot'
  | 'errorband'
  | 'errorbar'
  | 'image';

export type EncodingChannelName =
  | 'x'
  | 'y'
  | 'x2'
  | 'y2'
  | 'xOffset'
  | 'yOffset'
  | 'theta'
  | 'theta2'
  | 'radius'
  | 'radius2'
  | 'longitude'
  | 'latitude'
  | 'longitude2'
  | 'latitude2'
  | 'color'
  | 'fill'
  | 'stroke'
  | 'opacity'
  | 'fillOpacity'
  | 'strokeOpacity'
  | 'size'
  | 'angle'
  | 'shape'
  | 'strokeWidth'
  | 'strokeDash'
  | 'text'
  | 'tooltip'
  | 'href'
  | 'description'
  | 'url'
  | 'detail'
  | 'order'
  | 'key'
  | 'row'
  | 'column'
  | 'facet';

export type AggregateOp =
  | 'count'
  | 'valid'
  | 'distinct'
  | 'sum'
  | 'mean'
  | 'average'
  | 'median'
  | 'min'
  | 'max'
  | 'stdev'
  | 'stdevp'
  | 'variance'
  | 'variancep'
  | 'q1'
  | 'q3'
  | 'ci0'
  | 'ci1'
  | 'first'
  | 'last'
  | 'mode'
  | 'product';

export type StackMode = 'zero' | 'normalize' | 'center' | 'none';

/** A structured (non-string) bag of Vega-Lite properties owned by the builder. */
export type PropMap = Record<string, unknown>;

/** A single encoding entry. Channels that accept arrays (tooltip/detail/order) are repeated. */
export interface EncodingModel {
  id: string;
  channel: EncodingChannelName;
  enabled?: boolean;
  /** Field name (column / display name). Mutually exclusive with `value` and `datum`. */
  field?: string;
  type?: VegaLiteFieldType;
  aggregate?: AggregateOp;
  timeUnit?: string;
  bin?: boolean;
  sort?: string;
  stack?: StackMode;
  title?: string;
  /** D3 format string for the channel (axis/legend/text). */
  format?: string;
  /** Constant encoding value (e.g. a fixed color) instead of a field. */
  value?: string;
  /** Constant datum encoding (a value in data space). */
  datum?: string;
  /** Structured scale definition (type/scheme/zero/domain/range/…). */
  scale?: PropMap;
  /** Structured axis definition; `null` removes the axis. */
  axis?: PropMap | null;
  /** Structured legend definition; `null` removes the legend. */
  legend?: PropMap | null;
  /** Structured conditional encoding. */
  condition?: PropMap;
  /**
   * Structured catch-all for channel-def properties without a dedicated control
   * (impute, bandPosition, header, …). Populated by the spec→builder converter
   * so conversions stay lossless; merged under the typed fields above.
   */
  extra?: PropMap;
  /**
   * Per-channel escape hatch: a JSON fragment deep-merged onto this channel
   * definition. Kept for power users; the builder and demos use the typed
   * fields above instead.
   */
  advancedJson?: string;
}

export type TransformKind =
  | 'filter'
  | 'calculate'
  | 'aggregate'
  | 'bin'
  | 'timeUnit'
  | 'fold'
  | 'pivot'
  | 'window'
  | 'joinaggregate'
  | 'stack'
  | 'flatten'
  | 'sample'
  | 'density'
  | 'regression'
  | 'loess'
  | 'quantile'
  | 'lookup'
  | 'impute'
  | 'extent';

/**
 * A transform entry. `mode` selects how it is edited: `builder` renders typed
 * fields per kind, `raw` exposes the JSON directly. Either way the emitted
 * transform is `JSON.parse(json)`, so `json` is always the source of truth.
 */
export interface TransformModel {
  id: string;
  kind: TransformKind;
  enabled?: boolean;
  mode?: 'builder' | 'raw';
  json: string;
}

export interface MarkModel {
  type: MarkType;
  tooltip?: boolean;
  point?: boolean;
  filled?: boolean;
  interpolate?: string;
  opacity?: number;
  /** Structured extra mark-definition properties (cornerRadius, line, …). */
  props?: PropMap;
  /** Escape hatch: extra mark-definition properties deep-merged onto the mark. */
  advancedJson?: string;
}

/** One layer of a layered (multi-mark) view: its own mark + encodings + transforms. */
export interface LayerModel {
  id: string;
  mark: MarkModel;
  encodings: EncodingModel[];
  transforms?: TransformModel[];
  /** Parameters / selections scoped to this layer. */
  params?: ParamModel[];
}

/** A parameter / selection. `spec` is the param body (select/bind/value/expr). */
export interface ParamModel {
  id: string;
  name: string;
  spec: PropMap;
}

export interface BuilderModel {
  mark: MarkModel;
  /** Top-level encodings. For a layered view these are the SHARED encodings. */
  encodings: EncodingModel[];
  transforms: TransformModel[];
  /** When non-empty, the view is layered (each layer is its own mark + encodings). */
  layers?: LayerModel[];
  /** Parameters / selections (interval/point selections, input bindings, …). */
  params?: ParamModel[];
  /** Structured scale/axis resolution for layered views. */
  resolve?: PropMap;
  /**
   * Structured catch-all for top-level spec properties without a dedicated
   * control. Populated by the spec→builder converter so conversions stay
   * lossless; merged under the structured fields above.
   */
  extra?: PropMap;
  /** Vega-Lite `config` overrides (deep-merged on top of the Grafana theme config). */
  configJson?: string;
  /** Top-level spec overrides deep-merged last — the comprehensive catch-all. */
  specOverrideJson?: string;
}

export interface DataOptions {
  /** `auto` uses every returned frame (first is primary); `series` pins one frame. */
  source: 'auto' | 'series';
  seriesRefId?: string;
}

export interface ThemeOptions {
  /**
   * Color scheme. A Grafana color-scheme id (`palette-classic`,
   * `palette-classic-by-name`, `continuous-GrYlRd`, ...) resolves to the active
   * theme's palette/gradient; any other value is a Vega color scheme name (e.g.
   * `tableau10`, `viridis`). Structural theming (fonts, text/grid/axis colors)
   * always follows Grafana.
   */
  colorScheme: string;
}

export interface PanelOptions {
  editorMode: EditorMode;
  builder: BuilderModel;
  /** Reserved for the future raw-spec editor mode. */
  code: { spec: string };
  data: DataOptions;
  theme: ThemeOptions;
  renderer: RendererType;
  /** Global tooltip toggle (off strips tooltips from the spec). */
  tooltip: boolean;
  /** Global legend toggle (off disables all legends via config). */
  legend: boolean;
}

export const defaultMark: MarkModel = {
  type: 'line',
  tooltip: true,
  point: false,
};

export const defaultBuilder: BuilderModel = {
  mark: defaultMark,
  encodings: [],
  transforms: [],
};

export const defaultPanelOptions: PanelOptions = {
  editorMode: 'builder',
  builder: defaultBuilder,
  code: { spec: '' },
  data: { source: 'auto' },
  theme: { colorScheme: 'palette-classic' },
  renderer: 'canvas',
  tooltip: true,
  legend: true,
};
