import { PanelPlugin } from '@grafana/data';

import { COLOR_SCHEME_OPTIONS } from './components/builder/options';
import { PresetEditor } from './components/builder/PresetEditor';
import { PreviewEditor } from './components/builder/PreviewEditor';
import {
  ConfigJsonEditor,
  EncodingSectionEditor,
  LayersSectionEditor,
  MarkSectionEditor,
  ParamsSectionEditor,
  SpecOverrideEditor,
  TransformsSectionEditor,
} from './components/builder/sections';
import { Vizard } from './components/Vizard';
import { defaultMark, PanelOptions } from './types';

export const plugin = new PanelPlugin<PanelOptions>(Vizard).setPanelOptions((builder) => {
  builder
    // --- Data ---------------------------------------------------------------
    .addRadio({
      path: 'data.source',
      name: 'Data source frames',
      description: 'Use every returned frame (first is the default data) or pin a single series by refId.',
      defaultValue: 'auto',
      category: ['Data'],
      settings: {
        options: [
          { label: 'All frames', value: 'auto' },
          { label: 'Single series', value: 'series' },
        ],
      },
    })
    .addTextInput({
      path: 'data.seriesRefId',
      name: 'Series refId',
      defaultValue: '',
      category: ['Data'],
      showIf: (opts) => opts.data?.source === 'series',
    })
    // --- Chart (type preset + appearance) ----------------------------------
    .addCustomEditor({
      id: 'preset',
      path: 'builder',
      name: 'Chart type',
      description: 'Pick a starting chart type that maps the current data onto a mark and encodings. Refine it in the sections below.',
      editor: PresetEditor,
      category: ['Chart'],
    })
    .addSelect({
      path: 'theme.colorScheme',
      name: 'Color scheme',
      description: 'Grafana standard color scheme (theme-aware) or a Vega color scheme. Fonts and axis colors always follow Grafana.',
      defaultValue: 'palette-classic',
      category: ['Chart'],
      settings: { options: COLOR_SCHEME_OPTIONS },
    })
    .addRadio({
      path: 'renderer',
      name: 'Renderer',
      defaultValue: 'canvas',
      category: ['Chart'],
      settings: {
        options: [
          { label: 'Canvas', value: 'canvas' },
          { label: 'SVG', value: 'svg' },
        ],
      },
    })
    .addBooleanSwitch({ path: 'tooltip', name: 'Tooltips', defaultValue: true, category: ['Chart'] })
    .addBooleanSwitch({ path: 'legend', name: 'Legend', defaultValue: true, category: ['Chart'] })
    // --- Mark ---------------------------------------------------------------
    .addCustomEditor({
      id: 'mark',
      path: 'builder.mark',
      name: 'Mark',
      description: 'The mark type and its properties (used for single-mark charts).',
      editor: MarkSectionEditor,
      defaultValue: { ...defaultMark },
      category: ['Mark'],
    })
    // --- Encoding -----------------------------------------------------------
    .addCustomEditor({
      id: 'encoding',
      path: 'builder.encodings',
      name: 'Encoding',
      description: 'Map fields to channels (x, y, color, size, …). Shared across layers when layering.',
      editor: EncodingSectionEditor,
      defaultValue: [],
      category: ['Encoding'],
    })
    // --- Layers -------------------------------------------------------------
    .addCustomEditor({
      id: 'layers',
      path: 'builder.layers',
      name: 'Layers',
      description: 'Draw multiple marks on shared axes.',
      editor: LayersSectionEditor,
      defaultValue: [],
      category: ['Layers'],
    })
    // --- Transforms ---------------------------------------------------------
    .addCustomEditor({
      id: 'transforms',
      path: 'builder.transforms',
      name: 'Transforms',
      description: 'Filter, aggregate, bin, fold, window, … applied before encoding.',
      editor: TransformsSectionEditor,
      defaultValue: [],
      category: ['Transforms'],
    })
    // --- Parameters ---------------------------------------------------------
    .addCustomEditor({
      id: 'params',
      path: 'builder.params',
      name: 'Parameters',
      description: 'Selections (brush/click) and input bindings for interactions.',
      editor: ParamsSectionEditor,
      defaultValue: [],
      category: ['Parameters'],
    })
    // --- Advanced (escape hatches) -----------------------------------------
    .addCustomEditor({
      id: 'configJson',
      path: 'builder.configJson',
      name: 'Vega-Lite config',
      editor: ConfigJsonEditor,
      defaultValue: '',
      category: ['Advanced'],
    })
    .addCustomEditor({
      id: 'specOverride',
      path: 'builder.specOverrideJson',
      name: 'Spec override',
      editor: SpecOverrideEditor,
      defaultValue: '',
      category: ['Advanced'],
    })
    // --- Preview (bottom) ---------------------------------------------------
    .addCustomEditor({
      id: 'previewJson',
      path: '__previewJson',
      name: 'Generated Vega-Lite JSON',
      description: 'Read-only preview of the grammar the builder generates.',
      editor: PreviewEditor,
      category: ['Preview JSON'],
    });
});
