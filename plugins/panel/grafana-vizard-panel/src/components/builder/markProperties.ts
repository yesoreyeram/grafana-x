import { SelectableValue } from '@grafana/data';

import { MarkType } from '../../types';
import { INTERPOLATE_OPTIONS } from './options';

/**
 * Declarative schema for the Mark builder. Each property maps to a Vega-Lite
 * mark-definition property (https://vega.github.io/vega-lite/docs/mark.html#mark-def).
 * Most properties live in the builder's `props` bag (deep-merged onto the mark by
 * `fromBuilder`); the five long-standing typed fields (`opacity`, `tooltip`,
 * `point`, `filled`, `interpolate`) are addressed with `target: 'field'`.
 *
 * Sections and per-property `appliesTo` gate which controls show for the current
 * mark type, so e.g. a `line` shows General/Color/Stroke + Line & area, while an
 * `arc` shows General/Color/Stroke + Arc.
 */
export type MarkControlKind = 'number' | 'slider' | 'text' | 'fill' | 'switch' | 'select' | 'dash';

export interface MarkPropDef {
  /** Property key — a `props` key, or a typed MarkModel field when `target: 'field'`. */
  key: string;
  label: string;
  kind: MarkControlKind;
  /** 'field' writes a typed MarkModel field; 'prop' (default) writes the props bag. */
  target?: 'field' | 'prop';
  description?: string;
  placeholder?: string;
  min?: number;
  max?: number;
  step?: number;
  /** Position shown by a slider when the value is unset. */
  default?: number;
  options?: Array<SelectableValue<string>>;
  /** Restrict this property to specific mark types (within its section). */
  appliesTo?: MarkType[];
}

export interface MarkSection {
  label: string;
  /** Section shown only for these mark types; undefined = every mark. */
  appliesTo?: MarkType[];
  props: MarkPropDef[];
}

/** Build a clearable option list (leading "(default)" maps to an empty value). */
function sel(values: string[]): Array<SelectableValue<string>> {
  return [{ label: '(default)', value: '' }, ...values.map((v) => ({ label: v, value: v }))];
}

export const CURSOR_OPTIONS = sel([
  'auto', 'default', 'none', 'pointer', 'crosshair', 'move', 'text', 'wait', 'help',
  'not-allowed', 'grab', 'grabbing', 'ew-resize', 'ns-resize', 'zoom-in', 'zoom-out',
]);
export const BLEND_OPTIONS = sel([
  'multiply', 'screen', 'overlay', 'darken', 'lighten', 'color-dodge', 'color-burn',
  'hard-light', 'soft-light', 'difference', 'exclusion', 'hue', 'saturation', 'color', 'luminosity',
]);
export const INVALID_OPTIONS = sel([
  'filter', 'break-paths-filter-domains', 'break-paths-show-domains', 'break-paths-show-path-domains', 'show',
]);
export const STROKE_CAP_OPTIONS = sel(['butt', 'round', 'square']);
export const STROKE_JOIN_OPTIONS = sel(['miter', 'round', 'bevel']);
export const ORIENT_OPTIONS = sel(['vertical', 'horizontal']);
export const SHAPE_OPTIONS = sel([
  'circle', 'square', 'cross', 'diamond', 'triangle-up', 'triangle-down',
  'triangle-right', 'triangle-left', 'triangle', 'stroke', 'arrow', 'wedge',
]);
export const ALIGN_OPTIONS = sel(['left', 'center', 'right']);
export const BASELINE_OPTIONS = sel(['top', 'middle', 'bottom', 'alphabetic', 'line-top', 'line-bottom']);
export const FONT_STYLE_OPTIONS = sel(['normal', 'italic']);
export const FONT_WEIGHT_OPTIONS = sel(['normal', 'bold', 'bolder', 'lighter']);

const FILLABLE: MarkType[] = ['point', 'circle', 'square', 'line', 'area', 'trail', 'rule', 'bar', 'arc', 'tick', 'geoshape'];

export const MARK_SECTIONS: MarkSection[] = [
  {
    label: 'General',
    props: [
      { key: 'opacity', label: 'Opacity', kind: 'slider', target: 'field', min: 0, max: 1, step: 0.05, default: 1, description: 'Overall mark opacity (0–1).' },
      { key: 'tooltip', label: 'Tooltip', kind: 'switch', target: 'field', description: 'Per-mark tooltip. The global toggle is in the Chart section.' },
      { key: 'clip', label: 'Clip', kind: 'switch', description: 'Clip the mark to the enclosing group’s width and height.' },
      { key: 'invalid', label: 'Invalid data mode', kind: 'select', options: INVALID_OPTIONS, description: 'How null / NaN values are drawn (path breaks, filtering, …).' },
      { key: 'cursor', label: 'Cursor', kind: 'select', options: CURSOR_OPTIONS, description: 'Mouse cursor shown over the mark.' },
      { key: 'blend', label: 'Blend mode', kind: 'select', options: BLEND_OPTIONS, description: 'CSS mix-blend-mode used to composite the mark.' },
      { key: 'aria', label: 'ARIA', kind: 'switch', description: 'Include ARIA attributes (SVG only).' },
      { key: 'description', label: 'Description', kind: 'text', placeholder: 'ARIA label', description: 'ARIA accessibility label (SVG only).' },
      { key: 'style', label: 'Style', kind: 'text', placeholder: 'style name(s)', description: 'Named config style(s) to apply (comma-separated).' },
    ],
  },
  {
    label: 'Color',
    props: [
      { key: 'color', label: 'Color', kind: 'fill', description: 'Default color or gradient. Clear to use the color encoding or palette.' },
      { key: 'filled', label: 'Filled', kind: 'switch', target: 'field', appliesTo: FILLABLE, description: 'Use the color as fill instead of stroke.' },
      { key: 'fill', label: 'Fill', kind: 'fill', description: 'Fill color or gradient (overrides color). Clear for none.' },
      { key: 'stroke', label: 'Stroke', kind: 'fill', description: 'Stroke color or gradient (overrides color). Clear for none.' },
      { key: 'fillOpacity', label: 'Fill opacity', kind: 'slider', min: 0, max: 1, step: 0.05, default: 1 },
      { key: 'strokeOpacity', label: 'Stroke opacity', kind: 'slider', min: 0, max: 1, step: 0.05, default: 1 },
    ],
  },
  {
    label: 'Stroke',
    props: [
      { key: 'strokeWidth', label: 'Stroke width', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'strokeCap', label: 'Stroke cap', kind: 'select', options: STROKE_CAP_OPTIONS, description: 'Line ending style.' },
      { key: 'strokeDash', label: 'Stroke dash', kind: 'dash', placeholder: '6,4', description: 'Dash pattern: alternating stroke,space lengths.' },
      { key: 'strokeDashOffset', label: 'Stroke dash offset', kind: 'number', placeholder: '0' },
      { key: 'strokeJoin', label: 'Stroke join', kind: 'select', options: STROKE_JOIN_OPTIONS, description: 'Line join method.' },
      { key: 'strokeMiterLimit', label: 'Stroke miter limit', kind: 'number', placeholder: 'auto' },
    ],
  },
  {
    label: 'Line & area',
    appliesTo: ['line', 'area', 'trail'],
    props: [
      { key: 'interpolate', label: 'Interpolation', kind: 'select', target: 'field', options: INTERPOLATE_OPTIONS, description: 'Line/area interpolation method.' },
      { key: 'tension', label: 'Tension', kind: 'number', placeholder: 'auto', min: 0, max: 1, step: 0.1, description: 'Interpolation tension (0–1).' },
      { key: 'point', label: 'Show points', kind: 'switch', target: 'field', description: 'Draw a point at each vertex.' },
      { key: 'orient', label: 'Orientation', kind: 'select', options: ORIENT_OPTIONS },
    ],
  },
  {
    label: 'Bar',
    appliesTo: ['bar'],
    props: [
      { key: 'cornerRadius', label: 'Corner radius', kind: 'number', placeholder: '0', min: 0 },
      { key: 'cornerRadiusEnd', label: 'Corner radius (end)', kind: 'number', placeholder: '0', min: 0, description: 'Round only the end of the bar (e.g. tops of vertical bars).' },
      { key: 'binSpacing', label: 'Bin spacing', kind: 'number', placeholder: '1', min: 0, description: 'Offset between bars for binned fields.' },
      { key: 'continuousBandSize', label: 'Continuous band size', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'discreteBandSize', label: 'Discrete band size', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'width', label: 'Width', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'orient', label: 'Orientation', kind: 'select', options: ORIENT_OPTIONS },
    ],
  },
  {
    label: 'Rectangle',
    appliesTo: ['rect'],
    props: [
      { key: 'width', label: 'Width', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'height', label: 'Height', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'cornerRadius', label: 'Corner radius', kind: 'number', placeholder: '0', min: 0 },
    ],
  },
  {
    label: 'Point & symbol',
    appliesTo: ['point', 'circle', 'square', 'tick'],
    props: [
      { key: 'size', label: 'Size', kind: 'number', placeholder: 'auto', min: 0, description: 'Symbol area in square pixels.' },
      { key: 'shape', label: 'Shape', kind: 'select', options: SHAPE_OPTIONS, appliesTo: ['point'] },
    ],
  },
  {
    label: 'Arc',
    appliesTo: ['arc'],
    props: [
      { key: 'innerRadius', label: 'Inner radius', kind: 'number', placeholder: '0', min: 0, description: 'Inner radius (use > 0 for a donut).' },
      { key: 'outerRadius', label: 'Outer radius', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'cornerRadius', label: 'Corner radius', kind: 'number', placeholder: '0', min: 0 },
      { key: 'padAngle', label: 'Pad angle', kind: 'number', placeholder: '0', min: 0, step: 0.01, description: 'Angular padding between slices (radians).' },
      { key: 'radius', label: 'Radius', kind: 'number', placeholder: 'auto', min: 0 },
    ],
  },
  {
    label: 'Text',
    appliesTo: ['text'],
    props: [
      { key: 'text', label: 'Text', kind: 'text', placeholder: 'constant text', description: 'Constant text (otherwise use the text encoding).' },
      { key: 'align', label: 'Align', kind: 'select', options: ALIGN_OPTIONS },
      { key: 'baseline', label: 'Baseline', kind: 'select', options: BASELINE_OPTIONS },
      { key: 'dx', label: 'Offset X (dx)', kind: 'number', placeholder: '0' },
      { key: 'dy', label: 'Offset Y (dy)', kind: 'number', placeholder: '0' },
      { key: 'angle', label: 'Angle', kind: 'number', placeholder: '0', description: 'Rotation in degrees.' },
      { key: 'font', label: 'Font', kind: 'text', placeholder: 'inherit' },
      { key: 'fontSize', label: 'Font size', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'fontWeight', label: 'Font weight', kind: 'select', options: FONT_WEIGHT_OPTIONS },
      { key: 'fontStyle', label: 'Font style', kind: 'select', options: FONT_STYLE_OPTIONS },
      { key: 'limit', label: 'Limit', kind: 'number', placeholder: 'none', min: 0, description: 'Max text length in pixels before truncation.' },
      { key: 'lineHeight', label: 'Line height', kind: 'number', placeholder: 'auto', min: 0 },
      { key: 'ellipsis', label: 'Ellipsis', kind: 'text', placeholder: '…' },
    ],
  },
  {
    label: 'Tick',
    appliesTo: ['tick'],
    props: [
      { key: 'thickness', label: 'Thickness', kind: 'number', placeholder: '1', min: 0 },
      { key: 'bandSize', label: 'Band size', kind: 'number', placeholder: 'auto', min: 0 },
    ],
  },
];
