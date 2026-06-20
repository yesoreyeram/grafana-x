# Changelog

## 0.1.0

Initial release.

### Visualization

- Grafana **panel plugin** that renders any data frame as a
  [Vega-Lite](https://vega.github.io/vega-lite/) visualization (compiled to Vega
  and rendered with `vega-embed`).
- **Visual builder** covering the single-view Vega-Lite grammar: all mark types
  (with mark-definition properties), every encoding channel (field/value,
  type, aggregate, time unit, bin, sort, stack, title, format, and a per-channel
  scale/axis/legend/condition JSON escape hatch), a **structured transform
  pipeline** (filter, calculate, aggregate, bin, timeUnit, fold, pivot, window,
  joinaggregate, stack, flatten, sample, density, regression, loess, quantile,
  lookup, impute, extent) with a per-transform Builder ⇄ JSON toggle, a Vega-Lite
  `config` override, and a top-level spec override (deep-merged last) for
  full-grammar coverage. The selected mark always applies, even before any
  encoding is configured.

### Data

- Converts every [data plane](https://grafana.com/developers/dataplane) format
  to inline Vega-Lite data keyed by field display name: time series
  (wide / multi / long), numeric (wide / multi / long), logs, and tables.
- Multi-frame single-series responses are merged into one tidy long table
  (labels become dimension columns); wide multi-column frames are folded.
- Every frame is also exposed as a named dataset for spec references.
- **Smart defaults**: an unconfigured panel derives a sensible chart from the
  detected data shape.

### Theme & layout

- Vega-Lite `config` derived from the active Grafana theme (fonts, text / grid /
  axis colors, no view border). Selectable **color scheme**: Grafana's standard,
  theme-aware schemes (Classic palette, Classic by series name, and the
  continuous Green-Yellow-Red / Blues / … gradients via `fieldColorModeRegistry`)
  or a Vega color scheme (tableau10, viridis, …).
- **Distinct series colors by default** — categorical color uses the full Grafana
  palette (distinct per series); continuous (quantitative) color uses a
  theme-aware sequential gradient instead of a rainbow of the categorical palette.
- Transparent or solid (card) background.
- Responsive sizing from the panel dimensions with `autosize: fit`.

### Interactivity

- **Time-range zoom**: drag to brush a continuous temporal x axis to update the
  dashboard time range (`onChangeTimeRange`), like the native time series panel.
  Single-view temporal charts only; specs with their own selections are untouched.
- The Vega tooltip is themed (dark/light) to match the Grafana theme.

### Security

- Spec sanitizer strips `url` (remote data / images), `href` (click-XSS), and
  `usermeta` (embed-option override) at any depth.
- A Vega loader that rejects all remote/file loads (defense in depth).
- CSP-safe expression interpreter (`ast: true`).
- The Vega export/source/editor menu is disabled, so specs never leave Grafana.

### Builder grammar coverage

- The visual builder now covers **layers (multiple marks)**, **parameters /
  selections**, per-channel **scale / axis / legend** (typed controls, no JSON),
  structured **mark properties**, and a view **title** — in addition to mark,
  encodings, and the structured transform pipeline.
- Each builder area is its own collapsible options **section**, ordered
  Data → Chart → Mark → Encoding → Layers → Transforms → Parameters → Advanced →
  Preview JSON (the read-only generated grammar, at the bottom). The JSON preview
  opens in a word-wrapped, viewport-fitting modal.
- The **Chart** section combines the chart-type preset with appearance: color
  scheme, renderer, and the global tooltip / legend toggles.
- Single mark is the default; layering is an opt-in section (multi-mark). Adding
  the first layer migrates the current mark into it (no longer reset to default).
- **Mark properties** are comprehensive, schema-driven builder controls laid out
  one per row and grouped by category — **General** (opacity, tooltip, clip,
   invalid-data mode, cursor, blend, ARIA, description, style), **Color** (color,
   filled, fill, stroke, fill/stroke opacity — color/fill/stroke each support a
   **solid color or a linear / radial gradient**), **Stroke** (width, cap, dash, dash
  offset, join, miter limit), plus type-specific groups that appear for the
  current mark: **Line & area** (interpolation, tension, points, orientation),
  **Bar** (corner radius, band sizes, bin spacing, orientation, width),
  **Rectangle**, **Point & symbol** (size, shape), **Arc** (inner/outer radius,
  corner radius, pad angle), **Text** (align, baseline, dx/dy, angle, font,
   size, weight, style, limit, line height), and **Tick**. Each uses the right
   control (color / gradient picker, slider, dropdown, switch, number/text). An
   "Advanced mark properties" JSON override covers the long tail.
- **Chart type presets**: a dropdown of ready-made chart types that map the
  current data shape onto a complete mark + encodings (folding wide measures or
  using an existing series dimension as needed) — line, multi-line, area, stacked
  area, **streamgraph**, **trellis area**, bar, grouped / stacked / horizontal
  bar, **horizontal stacked bar**, **diverging stacked bar**, pie, donut,
  **radial plot**, scatter, bubble, points, **ternary**, histogram, heatmap,
  **error bar**, plus **None** to start blank. Switching presets fully replaces the
  chart — encodings, layers, params and stale mark properties are cleared (so no
  leftovers carry over) — while keeping the Advanced escape-hatch JSON. **None**
  is a complete reset to a single default mark, clearing the escape-hatch JSON
  too. The chart then stays fully editable in the sections below.
- Global **Tooltip** and **Legend** toggles are symmetric, so both states always
  take effect: ON enables value tooltips (`config.mark.tooltip`) and legends even
  when the spec declared none; OFF strips tooltips and disables all legends —
  including nulling channel-level `legend` objects, which `config.legend.disable`
  alone does not override (fixes the legend toggle on converted specs).
- The panel uses a transparent background (so Grafana's native panel background
  and the "Transparent background" option apply); it is responsive to the panel
  size with no separate padding control.
- A spec→builder converter makes the demo galleries **100% builder-native**
  (no spec-override / config / advanced JSON): each example is converted to a
  builder model; conversions are lossless (long-tail props persist in structured
  `extra`/`props` maps, never JSON-string fields).

### Vega-Lite example coverage

- Tested against the entire Vega-Lite example corpus (627 gallery specs): each is
  run through the panel pipeline, compiled to Vega, AND parsed to a runtime
  dataflow (so errors like duplicate signals are caught, not just compile errors).
  The visual builder is also tested by reproducing representative examples for
  every mark/transform/encoding.
- The Infinity-backed demo galleries strip example-defined color scales so every
  panel uses the Grafana palette; datasets Infinity can't parse are excluded.
- The data-aware spec sanitizer neutralizes remote `data` sources to empty inline
  data (keeping specs valid + remote-free) instead of leaving broken data objects
  — fixes layered / lookup / geo example compilation.
- A full spec override is used verbatim (not merged onto the auto-suggestion), so
  multi-view / layered specs render correctly.

### Tooling

- Local Docker stack: Grafana with the panel, provisioned TestData and Infinity
  (`yesoreyeram-infinity-datasource`, preinstalled via `GF_INSTALL_PLUGINS`)
  datasources, a TestData quickstart dashboard, and Infinity-backed example
  galleries (one per category) that fetch the Vega example datasets by URL.
- **Showcase dashboard** (`provisioning/dashboards/showcase.json`): a curated
  tour of 30+ brilliant charts (streamgraph, gradient area, spiral, Nightingale
  rose, calendar heatmap, step / multi-line, stacked / normalized / diverging
  bars, pie / donut / radial, scatter, histogram, 2D density heatmap, box / strip
  plots, error bars, ternary, candlestick, slope, connected scatter, trellis, …)
  organized with **Grafana 13 tabs** (the dynamic dashboard `TabsLayout` v2
  schema). Every panel is driven by the **Infinity datasource's inline source**
  (data lives in the query, not the spec) and configured with the **builder**
  (mark + encodings, plus builder layers / transforms for composite charts) — no
  spec-override JSON. Generated by `scripts/gen-showcase.mjs`.
- **Grafana logo demo** (`provisioning/dashboards/logo.json`): the Grafana brand
  mark (icon only, no text) rendered as eight genuinely different visualizations of
  the same pixel grid — Classic (orange→amber `rect` bitmap), Halftone (gradient-sized
  circles), Rainbow swirl (hue by angle) are pure builder; Fire mosaic 🔥 (a flame
  made of flame emojis), Neon glow (layered halo + core), Isometric 3D (extruded
  shaded layers), Particle burst (cells flung from the centroid) and Wireframe (the
  traced icon outline) showcase the spec-override escape hatch (`calculate`
  transforms, multi-layer specs, text marks). The pixel grid is served once by the
  Infinity datasource's inline source on the first panel; the rest reuse it via the
  built-in "-- Dashboard --" datasource (no per-panel data duplication; Wireframe
  carries the outline path as its own inline query). `scripts/gen-logo.mjs` converts
  the official SVG to the pixel grid (point-in-polygon fill, so the ring's hole is
  preserved) and tags each cell with a vertical gradient, an angle and a radius so
  variants can color by ramp, swirl or ring.
- `scripts/gen-demo-dashboards.mjs` regenerates the galleries from the example
  corpus.
