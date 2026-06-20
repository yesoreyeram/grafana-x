# AGENTS.md

Guidance for AI coding agents working in this plugin. Humans should read
[README.md](./README.md) and [CONTRIBUTING.md](./CONTRIBUTING.md); this file is
the fast, factual map.

## What this is

A Grafana **panel plugin** (frontend only — no Go backend) that renders any
Grafana data frame as a [Vega-Lite](https://vega.github.io/vega-lite/)
visualization through a visual builder. Plugin id: `yesoreyeram-vizard-panel`.
Vega-Lite is compiled to Vega and rendered with `vega-embed`.

## Layout

```
src/
  module.ts                 PanelPlugin entry; setPanelOptions (Data / Chart / Appearance)
  types.ts                  PanelOptions (discriminated by editorMode), BuilderModel, enums, defaults
  components/
    Vizard.tsx            Panel root: builds DataContext + theme config, runs the pipeline, renders
    VegaView.tsx            Hardened vega-embed wrapper (lazy-loaded): ast, blocking loader, actions, finalize
    ErrorView.tsx           Theme-aware error/empty surface (Grafana Alert)
    builder/                Visual builder (one custom option editor per section)
      sections.tsx          StandardEditorProps wrappers -> each is its own options category
                            (Mark / Encoding / Layers / Transforms / Parameters / Advanced)
      MarkEditor.tsx        Mark type + properties
      EncodingEditor.tsx    Repeatable encoding rows (all channels) + ChannelStyleEditor
      ChannelStyleEditor.tsx  Typed scale / axis / legend controls per channel
      LayerEditor.tsx       Layers (multiple marks): per-layer mark + encodings
      ParamEditor.tsx       Parameters / selections (interval / point / variable)
      TransformEditor.tsx   Structured transform builder per kind + per-transform raw-JSON toggle
      transformSchema.ts    Per-kind field schema + build/extract (UI values <-> transform JSON)
      PreviewEditor.tsx     Read-only "Generated Vega-Lite JSON" preview + modal
      JsonInput.tsx         Validated JSON escape-hatch text area (Advanced only)
      options.ts            Selectable option lists (marks, channels, color schemes, transform templates)
  data/
    dataContext.ts          DataFrame[] -> row objects, named datasets, merged long table, field catalog
    detectKind.ts           Data-plane kind (meta.type + inference)
    fieldType.ts            Grafana FieldType -> Vega-Lite type
  spec/
    index.ts                buildSpec(): source -> injectData -> sanitize -> injectTheme -> injectSize
    __fixtures__/vegaLiteExamples.json  All 627 Vega-Lite gallery example specs (test corpus)
    examples.test.ts        Compiles EVERY example through the pipeline (coverage + bug net)
    fromBuilder.examples.test.ts  Builder reproduces representative gallery examples
    fromBuilder.ts          BuilderModel -> Vega-Lite spec (single + layered, params, scale/axis/legend)
    specToBuilder.ts        Vega-Lite spec -> BuilderModel (lossless; ok=false for multi-view/geo)
    galleries.ts            Build the Infinity-backed, builder-native demo dashboards
    suggest.ts              Smart-default spec from the detected data shape
    injectData.ts           Attach primary rows + named datasets
    sanitizeSpec.ts         SECURITY: neutralize remote data + strip href/usermeta
    injectTheme.ts          Merge theme < user config < spec config
    injectSize.ts           Single/layered views get width/height + autosize:fit
    injectZoom.ts           Add an x interval selection for time-range zoom (single-view temporal)
    merge.ts                deepMerge + JSON parse helpers
  theme/grafanaTheme.ts     GrafanaTheme2 (+ Grafana color registry) -> Vega-Lite config
  plugin.json               Panel manifest (type: panel)
scripts/gen-demo-dashboards.mjs  Generates the Infinity-backed example galleries
scripts/gen-showcase.mjs    Generates showcase.json (Grafana 13 tabs, v2 schema, inline data)
scripts/gen-logo.mjs        Generates logo.json (the Grafana mark, 27 variants in two
                            v2 tabs — "Grafana" (bitmap/effects/animated) + "Football
                            Fever" — incl. timer-driven animations). See logo.test.ts.
provisioning/
  datasources/              TestData + Infinity (yesoreyeram-infinity-datasource)
  dashboards/               demo.json (TestData quickstart) + gallery-*.json (Infinity examples)
                            + showcase.json (tabbed v2 dashboard; brilliant inline-data charts)
                            + logo.json (the Grafana mark drawn many creative + animated ways)
docker-compose.yaml         Grafana + the panel + Infinity (GF_INSTALL_PLUGINS) + provisioning
webpack.config.ts           Frontend build (self-contained)
```

## Commands

Workspace in the **`grafana-x` Yarn 4 monorepo**. Run from this directory or with
`yarn workspace yesoreyeram-vizard-panel <script>`.

| Task           | Command                  |
| -------------- | ------------------------ |
| Install deps   | `yarn install` (at root) |
| Build          | `yarn build`             |
| Watch          | `yarn dev`               |
| Typecheck      | `yarn typecheck`         |
| Lint           | `yarn lint`              |
| Tests          | `yarn test`              |
| Local stack    | `docker compose up` (build `dist/` first) |

Before declaring work done, run: `yarn typecheck && yarn lint && yarn test && yarn build`.

## Key architecture facts (do not regress these)

- **It is a pipeline.** `spec/index.ts::buildSpec` is the single entry point:
  pick a **source** (builder today; the `editorMode` switch leaves room for raw
  JSON / vega-lite-api), then run the shared stages `injectData -> sanitizeSpec
  -> injectTheme -> injectSize`. New modes must reuse the shared stages.
- **Security lives in the core, not the UI.** `sanitizeSpec` runs on the
  fully-assembled spec (after data + override merge) for EVERY mode. It is
  data-aware: remote `data` sources (`data.url`, `lookup.from.data.url`, bare
  URL strings) are neutralized to empty inline data (`{ values: [] }`) so the
  spec stays valid AND remote-free; `url`/`href`/`usermeta` are stripped from
  non-data positions (image marks, links, embed-option overrides). Inline data
  content (`data.values`, `datasets`) is kept verbatim. `VegaView` adds runtime
  guards: a blocking Vega loader, `ast: true` (CSP-safe interpreter), and
  `actions: false` (the Vega-Embed action menu is disabled). Never weaken these.
  the override merges onto the builder; otherwise (a complete spec such as a
  multi-view / layered example) it is used verbatim. This is why the example
  galleries embed each spec as `specOverrideJson`. Don't merge a full spec onto
  the single-view suggestion (it produces invalid `mark`+`layer` specs).
- **Cover all examples.** `examples.test.ts` runs all 627 gallery specs through
  the pipeline, then `vega-lite` `compile` AND `vega` `parse` (parse catches
  runtime-construction errors like duplicate signals that compile alone misses).
  Run it when changing the pipeline/sanitizer/zoom — it is the regression net.
- **Time-range zoom is single-view only.** `injectZoom` adds an x interval
  selection ONLY for single (non-layered, non-multi-view) continuous-temporal x,
  and skips specs with their own selections. Layered/multi x duplicates Vega
  signals. `VegaView` debounces the brush and calls `onBrush` -> the panel calls
  `onChangeTimeRange`. Colors: categorical -> Grafana palette (distinct);
  continuous -> sequential gradient (not the categorical rainbow).
- **Data is uniform.** Every frame becomes row objects keyed by
  `getFieldDisplayName`; time stays epoch-ms (`temporal`). The detected
  data-plane **kind only drives smart defaults**, never the row shape.
- **Multi-frame merge.** `dataContext.mergeSeriesFrames` merges homogeneous
  single-value-field frames (the multi formats) into one long table with label
  columns; wide multi-column frames are handled by a `fold` in `suggestSpec`.
- **Comprehensive builder via escape hatches.** Typed controls cover the common
  grammar; per-channel/mark/config/spec-override JSON inputs (deep-merged) cover
  the long tail. `deepMerge` precedence for config is theme < user < spec.
- **The builder mark always applies.** When no encodings are configured,
  `buildSpec` auto-suggests the encoding from the data but keeps the builder's
  `mark` (and explicit transforms). Do not regress this — it is the difference
  between "changing the mark does nothing" and a working fresh panel.
- **Transforms are JSON under the hood.** `transformSchema.ts` maps typed UI
  values to/from the transform object; `TransformModel.json` is the source of
  truth (`mode` only toggles the editor). Changing a transform's kind rebuilds
  its JSON so the change actually takes effect.
- **Color scheme uses Grafana's registry.** `grafanaTheme.ts` resolves Grafana
  scheme ids (`palette-classic`, `continuous-*`) via `fieldColorModeRegistry`
  `getColors(theme)` (theme-aware); other values are Vega scheme names. Fonts and
  axis colors always follow Grafana so charts stay readable.
- **Sizing.** Single and layered views get numeric width/height + `autosize:
  fit`; multi-view (facet/concat/repeat) specs size by content.
- **Frontend only.** No backend, no `pkg/`, no Magefile; `plugin.json` has
  `type: panel` and no `backend`/`executable`.

## Conventions

- TypeScript/React; use stable `@grafana/ui` components (`Select`, `Switch`,
  `Input`, `Field`, `InlineField`, `IconButton`, `Button`, `FieldSet`) — the
  `Select`-is-deprecated lint rule is a repo-wide **warning**, matching the other
  plugins.
- The spec pipeline operates on plain `SpecObject` (`Record<string, unknown>`)
  and casts to `TopLevelSpec` only at the embed boundary — keep it that way to
  avoid fighting Vega-Lite's strict types. Avoid `any`.
- Pure logic (data conversion, kind detection, builder->spec, sanitizer, theme,
  size) is unit-tested under `src/**/<name>.test.ts`. Add tests there.
- Vega/Vega-Lite/Vega-Embed are bundled (not Grafana externals); `VegaView` is
  lazy-loaded so `module.js` stays small. Keep it lazy.
- Toolchain pinned: Node in `.nvmrc`; all JS deps pinned to exact versions.
