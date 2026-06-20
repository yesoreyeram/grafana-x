# Vizard — visual chart builder for Grafana

**Vizard** turns any Grafana data frame into a rich, custom visualization through
a point-and-click **visual builder** — no code required to get started, and a
full grammar-of-graphics underneath when you need it. Build the chart Grafana's
built-in panels don't have: radial bars, stacked/diverging bars, streamgraph charts,
trellis/faceted small multiples, layered composites, error bars, connected
scatter plots, and much more.

Vizard is powered by [Vega-Lite](https://vega.github.io/vega-lite/) (compiled to
[Vega](https://vega.github.io/vega/) and rendered with `vega-embed`), matches the
active Grafana theme, is fully responsive, and ships hardened against remote data
loading and spec-based XSS.

- **Plugin id:** `yesoreyeram-vizard-panel`
- **Type:** panel (frontend only — no backend)
- **Grafana:** `>= 10.4.0`

---

## Table of contents

- [Why Vizard](#why-vizard)
- [Features](#features)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Usage guide](#usage-guide)
  - [Options reference](#options-reference)
  - [The builder, section by section](#the-builder-section-by-section)
  - [Escape hatches (full grammar)](#escape-hatches-full-grammar)
- [Data handling](#data-handling)
- [Theming](#theming)
- [Interactions & time-range zoom](#interactions--time-range-zoom)
- [Security](#security)
- [Demo dashboards](#demo-dashboards)
- [Development](#development)
- [Architecture](#architecture)
- [License](#license)

---

## Why Vizard

Grafana's native panels are great for the common cases. When you need a chart
that isn't on the menu, you normally reach for code. Vizard gives you the
**expressiveness of Vega-Lite** with the **ergonomics of a Grafana options
editor**: pick a chart type, map your fields to channels, and refine — while a
live, read-only **Generated Vega-Lite JSON** preview shows exactly what's being
produced. Every typed control has a JSON escape hatch, so you are never boxed in.

## Features

- **Visual builder** for the single-view and layered Vega-Lite grammar:
  - **Chart-type presets** — start from bar, line, area, scatter, pie/donut,
    heatmap, histogram, streamgraph, radial, trellis (faceted), error bar,
    horizontal/diverging stacked bar, and more; each maps your current data onto
    a mark + encodings you can then refine.
  - **Marks** — `bar`, `line`, `area`, `point`, `circle`, `square`, `tick`,
    `rect`, `rule`, `text`, `arc`, `trail`, `geoshape`, `boxplot`, `errorband`,
    `errorbar`, `image` — with per-mark properties (interpolation, point overlay,
    fill/stroke, opacity, corner radius, gradients, …).
  - **Encodings** — every channel (`x`, `y`, `x2/y2`, `xOffset/yOffset`,
    `theta`, `radius`, `color`, `fill`, `stroke`, `opacity`, `size`, `angle`,
    `shape`, `strokeWidth`, `strokeDash`, `text`, `tooltip`, `detail`, `order`,
    `key`, `row`/`column`/`facet`, …) with field, type, aggregate, time unit,
    bin, sort, stack, title, and format.
  - **Layers** — draw multiple marks on shared axes (e.g. a line with a points
    overlay, or a bar with a rule).
  - **Transforms** — a structured pipeline editor for `filter`, `calculate`,
    `aggregate`, `bin`, `timeUnit`, `fold`, `pivot`, `window`, `joinaggregate`,
    `stack`, `flatten`, `sample`, `density`, `regression`, `loess`, `quantile`,
    `lookup`, `impute`, and `extent` — each with a **Builder ⇄ JSON** toggle.
  - **Parameters** — selections (interval/point) and input bindings for
    interactivity.
- **Any data shape** — converts every
  [data-plane](https://grafana.com/developers/dataplane) format to Vega-Lite:
  time series (wide / multi / long), numeric (wide / multi / long), logs, and
  plain tables. Multi-frame series are merged into one tidy long table; wide
  multi-column frames are folded automatically.
- **Smart defaults** — with no encodings configured, Vizard picks a sensible
  chart from the detected data shape so the panel renders immediately.
- **Grafana theming** — fonts and text/grid/axis colors always follow the active
  theme; the **color scheme** is selectable (Grafana's theme-aware schemes or any
  Vega scheme).
- **Responsive** — the chart fits the panel and re-fits on resize; multi-view
  (facet/concat/repeat) charts scroll instead of being clipped.
- **Time-range zoom** — brushing a temporal x-axis updates the dashboard time
  range, just like the native time series panel.
- **Secure by design** — remote loading blocked, `href`/`usermeta` stripped, a
  CSP-safe expression interpreter, and the Vega action menu disabled (see
  [Security](#security)).

## Installation

### Grafana Cloud / self-hosted (released build)

Install from the plugin catalog or with the Grafana CLI:

```bash
grafana-cli plugins install yesoreyeram-vizard-panel
```

Then restart Grafana and add a **Vizard** visualization to any panel.

### From a local build (unsigned)

Build the plugin and point Grafana at the `dist/` output:

```bash
yarn workspace yesoreyeram-vizard-panel build   # produces dist/
```

Copy/symlink `dist/` to `<grafana-data>/plugins/yesoreyeram-vizard-panel` and
allow the unsigned plugin to load:

```ini
# grafana.ini / custom.ini
[plugins]
allow_loading_unsigned_plugins = yesoreyeram-vizard-panel
```

### Docker (bundled demo stack)

The fastest way to try Vizard with provisioned datasources and demo dashboards:

```bash
yarn workspace yesoreyeram-vizard-panel build
docker compose -f plugins/grafana-vizard-panel/docker-compose.yaml up
# Grafana → http://localhost:3000 (anonymous admin)
```

The Compose stack installs the signed
[Infinity datasource](https://grafana.com/grafana/plugins/yesoreyeram-infinity-datasource/)
(via `GF_INSTALL_PLUGINS`), provisions TestData + Infinity, and loads the
[demo dashboards](#demo-dashboards).

## Quick start

1. Create or edit a dashboard panel and run a query against **any** datasource.
2. In the visualization picker, choose **Vizard**. It renders a smart default
   chart immediately from your data's shape.
3. Open **Chart → Chart type** and pick a starting visualization.
4. Refine in **Mark** and **Encoding** (map fields to `x`, `y`, `color`, …).
5. Watch the **Generated Vega-Lite JSON** preview at the bottom to see the exact
   grammar being produced.

## Usage guide

### Options reference

Options are grouped into categories in the panel editor:

| Category        | Option                      | Description                                                                 |
| --------------- | --------------------------- | --------------------------------------------------------------------------- |
| **Data**        | Data source frames          | Use every returned frame (first is the default data) or pin a single series. |
| **Data**        | Series refId                | The refId to pin when **Single series** is selected.                        |
| **Chart**       | Chart type                  | Pick a starting chart type that maps the current data onto a mark + encodings. |
| **Chart**       | Color scheme                | Grafana standard scheme (theme-aware) or a Vega color scheme.               |
| **Chart**       | Renderer                    | Canvas (default) or SVG.                                                     |
| **Chart**       | Tooltips                    | Toggle hover tooltips.                                                       |
| **Chart**       | Legend                      | Toggle the legend.                                                          |
| **Mark**        | Mark                        | The mark type and its properties (single-mark charts).                      |
| **Encoding**    | Encoding                    | Map fields to channels (`x`, `y`, `color`, `size`, …). Shared across layers. |
| **Layers**      | Layers                      | Draw multiple marks on shared axes.                                          |
| **Transforms**  | Transforms                  | Filter, aggregate, bin, fold, window, … applied before encoding.            |
| **Parameters**  | Parameters                  | Selections (brush/click) and input bindings for interactions.               |
| **Advanced**    | Vega-Lite config            | Vega-Lite `config` overrides (deep-merged).                                  |
| **Advanced**    | Spec override               | A top-level spec fragment, deep-merged last — the full-grammar catch-all.    |
| **Preview JSON**| Generated Vega-Lite JSON    | Read-only preview of the grammar the builder generates.                      |

### The builder, section by section

- **Chart** — the fast lane. **Chart type** rewrites the mark + encodings to a
  known-good starting point; the remaining controls (color scheme, renderer,
  tooltips, legend) tune appearance without touching the grammar.
- **Mark** — choose the mark and its visual properties. For single-mark charts
  this is all you need; for composites, use **Layers**.
- **Encoding** — add one row per channel and bind it to a field. Set the field
  **type** (quantitative / temporal / ordinal / nominal), plus aggregate, time
  unit, bin, sort, stack, title and format as needed.
- **Layers** — when one mark isn't enough, add layers; each layer has its own
  mark and encodings drawn on shared axes.
- **Transforms** — shape the data before it's encoded. Build each step with typed
  fields, or flip a step to raw JSON for anything the UI doesn't cover.
- **Parameters** — add interval/point selections and bindings to make the chart
  interactive.

### Escape hatches (full grammar)

Vizard's typed controls cover the common grammar; the long tail is reachable via
JSON inputs that are **deep-merged** into the generated spec:

- **Per-channel JSON** (in each encoding row) — `scale` / `axis` / `legend` /
  `condition` overrides.
- **Vega-Lite config** (Advanced) — global `config` overrides.
- **Spec override** (Advanced) — a top-level spec fragment merged last. When the
  builder has no encodings/layers/params configured and a spec override is
  present, the override **is** the spec (your query frame is still injected as the
  data), so you can paste a complete Vega-Lite spec and have it themed, sized, and
  sanitized like everything else.

## Data handling

Every Grafana frame is converted to uniform row objects keyed by the field
display name; time stays as epoch-milliseconds (`temporal`). The detected
data-plane **kind only drives smart defaults** — it never changes the row shape,
so you can always re-map fields freely.

- **Multi-frame series** (the time-series/numeric *multi* and *long* formats) are
  merged into a single long table with label columns.
- **Wide multi-column frames** are folded automatically so each numeric column
  becomes a series.
- Named datasets are exposed for transforms (e.g. `lookup`).

## Theming

Fonts and text/grid/axis colors always follow the active Grafana theme so charts
stay legible in light and dark modes. The **Color scheme** option accepts:

- Grafana's standard, **theme-aware** schemes — Classic palette, Classic by
  series name, and the continuous gradients (Green-Yellow-Red, Blues, …),
  resolved through Grafana's color registry; or
- any **Vega color scheme** name (`tableau10`, `viridis`, `category20`, …).

Anything else is overridable through the **Vega-Lite config** input
(precedence: theme < your config < spec).

## Interactions & time-range zoom

For a single-view chart with a continuous **temporal x-axis**, Vizard adds an x
interval selection. Brushing a range is debounced and applied to the **dashboard
time range** — the same behavior as the native time series panel. (Layered and
multi-view specs, and specs that define their own selections, are left untouched
to avoid duplicate Vega signals.)

## Security

Security is enforced in the render core for **every** spec — not in the UI — so
it holds for builder output and pasted specs alike:

- **No remote loading.** Remote `data` sources (`data.url`, `lookup.from.data.url`,
  bare URL strings) are neutralized to empty inline data, **and** the Vega loader
  rejects all network/file loads — no SSRF, no data exfiltration.
- **No script injection.** `href` is stripped (removes a `javascript:`-URI
  click-XSS vector) and `usermeta.embedOptions` is stripped so a spec can't
  override the hardened embed options.
- **CSP-safe expressions.** Expressions run through the interpreter (`ast: true`),
  never `new Function`.
- **No data exfiltration via the action menu.** The Vega export/source/"Open in Vega
  Editor" menu is disabled (`actions: false`), so specs never leave Grafana.

Inline data you provide (`data.values`, `datasets`) is kept verbatim.

## Demo dashboards

The bundled stack provisions these (TestData + Infinity datasources, all
**builder-native** — no hand-written spec JSON in the galleries):

| Dashboard                       | UID                          | What it shows                                              |
| ------------------------------- | ---------------------------- | --------------------------------------------------------- |
| Vizard — Vega-Lite demos        | `vizard-demos`               | Quickstart charts on TestData (smart defaults + builder). |
| Vizard — Showcase               | `vizard-showcase`            | A tabbed tour of brilliant charts on Infinity inline data. |
| Vizard — Single-view gallery    | `vizard-gallery-single`      | Single-view examples from the Vega-Lite corpus.           |
| Vizard — Composite marks gallery| `vizard-gallery-composite`   | Box plots, error bars/bands.                              |
| Vizard — Layered gallery        | `vizard-gallery-layered`     | Multi-mark layered charts.                                |
| Vizard — Interactive gallery    | `vizard-gallery-interactive` | Charts with selections/params.                            |
| Vizard — Grafana logo           | `vizard-logo`                | The Grafana mark drawn 8 creative ways (a fun stress test). |

The galleries fetch real datasets live via the Infinity datasource from the
[Vega data repository](https://github.com/vega/vega/tree/main/docs/data).

## Development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo.

```bash
yarn install                                      # at the monorepo root
yarn workspace yesoreyeram-vizard-panel typecheck
yarn workspace yesoreyeram-vizard-panel lint
yarn workspace yesoreyeram-vizard-panel test
yarn workspace yesoreyeram-vizard-panel build
yarn workspace yesoreyeram-vizard-panel spellcheck
```

Regenerate the provisioned demo dashboards after changing the generators:

```bash
# Galleries (from the Vega-Lite example corpus)
GEN_GALLERIES=true yarn workspace yesoreyeram-vizard-panel test src/spec/galleries.test.ts
# Showcase + logo dashboards
node plugins/grafana-vizard-panel/scripts/gen-showcase.mjs
node plugins/grafana-vizard-panel/scripts/gen-logo.mjs
```

The plugin is tested against the **entire Vega-Lite example corpus** (627 specs):
each is run through the pipeline and compiled to Vega
(`src/spec/examples.test.ts`), and the builder is separately verified by
reproducing representative examples (`src/spec/fromBuilder.examples.test.ts`).

## Architecture

See [`AGENTS.md`](./AGENTS.md) for the full map. In short, a **spec pipeline**
turns options + data + theme into a Vega-Lite spec:

```
source (builder) → injectData → sanitize (security) → injectTheme → injectSize → <VegaView>
```

The `source` stage is pluggable (builder today; raw JSON and the
[vega-lite-api](https://vega.github.io/vega-lite-api/) later); every other stage
is shared and mode-independent.

## License

[Apache-2.0](./LICENSE)
