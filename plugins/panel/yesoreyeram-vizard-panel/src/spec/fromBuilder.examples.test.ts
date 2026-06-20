import { createTheme } from '@grafana/data';
import { compile } from 'vega-lite';

import { buildDataContext } from '../data/dataContext';
import { buildVegaConfig } from '../theme/grafanaTheme';
import { BuilderModel, defaultPanelOptions, EncodingModel, MarkModel, PanelOptions } from '../types';
import { fromBuilder } from './fromBuilder';
import { buildSpec } from './index';

type CompileSpec = Parameters<typeof compile>[0];

const theme = createTheme();
const themeConfig = buildVegaConfig(theme, defaultPanelOptions.theme);
const ctx = buildDataContext([], { source: 'auto' });
const size = { width: 400, height: 300 };

let encId = 0;
function enc(partial: Omit<EncodingModel, 'id'>): EncodingModel {
  encId += 1;
  return { id: `e${encId}`, ...partial };
}

function model(mark: MarkModel, encodings: EncodingModel[], extra: Partial<BuilderModel> = {}): BuilderModel {
  return { mark, encodings, transforms: [], ...extra };
}

/** Build through the full pipeline (typed builder path) and compile to Vega. */
function compileModel(m: BuilderModel) {
  const options: PanelOptions = { ...defaultPanelOptions, builder: { ...defaultPanelOptions.builder, ...m } };
  const { spec, warnings } = buildSpec(options, ctx, themeConfig, size);
  const result = compile(spec as unknown as CompileSpec);
  return { spec, warnings, vega: result.spec };
}

beforeAll(() => {
  jest.spyOn(console, 'warn').mockImplementation(() => {});
});
afterAll(() => jest.restoreAllMocks());

/**
 * Each case is a gallery example reproduced with the typed builder. We assert the
 * encoding the builder emits and that the whole thing compiles to Vega.
 */
const cases: Array<{ name: string; model: BuilderModel; encoding: Record<string, unknown>; mark?: unknown }> = [
  {
    name: 'bar',
    model: model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'a', type: 'nominal' }),
      enc({ channel: 'y', field: 'b', type: 'quantitative' }),
    ]),
    mark: 'bar',
    encoding: { x: { field: 'a', type: 'nominal' }, y: { field: 'b', type: 'quantitative' } },
  },
  {
    name: 'bar_aggregate (mean)',
    model: model({ type: 'bar' }, [
      enc({ channel: 'y', field: 'precipitation', type: 'quantitative', aggregate: 'mean' }),
      enc({ channel: 'x', field: 'date', type: 'ordinal', timeUnit: 'month' }),
    ]),
    encoding: {
      y: { field: 'precipitation', type: 'quantitative', aggregate: 'mean' },
      x: { field: 'date', type: 'ordinal', timeUnit: 'month' },
    },
  },
  {
    name: 'stacked_bar_weather (count + color)',
    model: model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'date', type: 'ordinal', timeUnit: 'month' }),
      enc({ channel: 'y', aggregate: 'count' }),
      enc({ channel: 'color', field: 'weather', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'date', type: 'ordinal', timeUnit: 'month' },
      y: { aggregate: 'count', type: 'quantitative' },
      color: { field: 'weather', type: 'nominal' },
    },
  },
  {
    name: 'bar_grouped (xOffset)',
    model: model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'category', type: 'nominal' }),
      enc({ channel: 'y', field: 'value', type: 'quantitative' }),
      enc({ channel: 'xOffset', field: 'group', type: 'nominal' }),
      enc({ channel: 'color', field: 'group', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'category', type: 'nominal' },
      y: { field: 'value', type: 'quantitative' },
      xOffset: { field: 'group', type: 'nominal' },
      color: { field: 'group', type: 'nominal' },
    },
  },
  {
    name: 'stacked_bar_normalize',
    model: model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'date', type: 'ordinal', timeUnit: 'month' }),
      enc({ channel: 'y', aggregate: 'count', stack: 'normalize' }),
      enc({ channel: 'color', field: 'weather', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'date', type: 'ordinal', timeUnit: 'month' },
      y: { aggregate: 'count', type: 'quantitative', stack: 'normalize' },
      color: { field: 'weather', type: 'nominal' },
    },
  },
  {
    name: 'histogram (bin + count)',
    model: model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'IMDB_Rating', type: 'quantitative', bin: true }),
      enc({ channel: 'y', aggregate: 'count' }),
    ]),
    encoding: {
      x: { field: 'IMDB_Rating', type: 'quantitative', bin: true },
      y: { aggregate: 'count', type: 'quantitative' },
    },
  },
  {
    name: 'line',
    model: model({ type: 'line' }, [
      enc({ channel: 'x', field: 'date', type: 'temporal' }),
      enc({ channel: 'y', field: 'price', type: 'quantitative' }),
    ]),
    mark: 'line',
    encoding: { x: { field: 'date', type: 'temporal' }, y: { field: 'price', type: 'quantitative' } },
  },
  {
    name: 'line_color (multi-series)',
    model: model({ type: 'line' }, [
      enc({ channel: 'x', field: 'date', type: 'temporal' }),
      enc({ channel: 'y', field: 'price', type: 'quantitative' }),
      enc({ channel: 'color', field: 'symbol', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'date', type: 'temporal' },
      y: { field: 'price', type: 'quantitative' },
      color: { field: 'symbol', type: 'nominal' },
    },
  },
  {
    name: 'area',
    model: model({ type: 'area' }, [
      enc({ channel: 'x', field: 'date', type: 'temporal' }),
      enc({ channel: 'y', field: 'count', type: 'quantitative' }),
    ]),
    mark: 'area',
    encoding: { x: { field: 'date', type: 'temporal' }, y: { field: 'count', type: 'quantitative' } },
  },
  {
    name: 'stacked_area',
    model: model({ type: 'area' }, [
      enc({ channel: 'x', field: 'date', type: 'temporal', timeUnit: 'yearmonth' }),
      enc({ channel: 'y', field: 'count', type: 'quantitative', aggregate: 'sum' }),
      enc({ channel: 'color', field: 'series', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'date', type: 'temporal', timeUnit: 'yearmonth' },
      y: { field: 'count', type: 'quantitative', aggregate: 'sum' },
      color: { field: 'series', type: 'nominal' },
    },
  },
  {
    name: 'point_2d (scatter)',
    model: model({ type: 'point' }, [
      enc({ channel: 'x', field: 'Horsepower', type: 'quantitative' }),
      enc({ channel: 'y', field: 'Miles_per_Gallon', type: 'quantitative' }),
    ]),
    mark: 'point',
    encoding: {
      x: { field: 'Horsepower', type: 'quantitative' },
      y: { field: 'Miles_per_Gallon', type: 'quantitative' },
    },
  },
  {
    name: 'circle_bubble (size)',
    model: model({ type: 'circle' }, [
      enc({ channel: 'x', field: 'Horsepower', type: 'quantitative' }),
      enc({ channel: 'y', field: 'Miles_per_Gallon', type: 'quantitative' }),
      enc({ channel: 'size', field: 'Acceleration', type: 'quantitative' }),
    ]),
    mark: 'circle',
    encoding: {
      x: { field: 'Horsepower', type: 'quantitative' },
      y: { field: 'Miles_per_Gallon', type: 'quantitative' },
      size: { field: 'Acceleration', type: 'quantitative' },
    },
  },
  {
    name: 'point_color_with_shape',
    model: model({ type: 'point' }, [
      enc({ channel: 'x', field: 'sepalWidth', type: 'quantitative' }),
      enc({ channel: 'y', field: 'petalLength', type: 'quantitative' }),
      enc({ channel: 'color', field: 'species', type: 'nominal' }),
      enc({ channel: 'shape', field: 'species', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'sepalWidth', type: 'quantitative' },
      y: { field: 'petalLength', type: 'quantitative' },
      color: { field: 'species', type: 'nominal' },
      shape: { field: 'species', type: 'nominal' },
    },
  },
  {
    name: 'tick_strip',
    model: model({ type: 'tick' }, [
      enc({ channel: 'x', field: 'Horsepower', type: 'quantitative' }),
      enc({ channel: 'y', field: 'Cylinders', type: 'ordinal' }),
    ]),
    mark: 'tick',
    encoding: { x: { field: 'Horsepower', type: 'quantitative' }, y: { field: 'Cylinders', type: 'ordinal' } },
  },
  {
    name: 'rect_heatmap',
    model: model({ type: 'rect' }, [
      enc({ channel: 'x', field: 'date', type: 'ordinal', timeUnit: 'date' }),
      enc({ channel: 'y', field: 'date', type: 'ordinal', timeUnit: 'month' }),
      enc({ channel: 'color', field: 'temp_max', type: 'quantitative', aggregate: 'max' }),
    ]),
    encoding: {
      x: { field: 'date', type: 'ordinal', timeUnit: 'date' },
      y: { field: 'date', type: 'ordinal', timeUnit: 'month' },
      color: { field: 'temp_max', type: 'quantitative', aggregate: 'max' },
    },
  },
  {
    name: 'arc_pie (theta + color)',
    model: model({ type: 'arc' }, [
      enc({ channel: 'theta', field: 'value', type: 'quantitative' }),
      enc({ channel: 'color', field: 'category', type: 'nominal' }),
    ]),
    mark: 'arc',
    encoding: { theta: { field: 'value', type: 'quantitative' }, color: { field: 'category', type: 'nominal' } },
  },
  {
    name: 'text_scatterplot_colored',
    model: model({ type: 'text' }, [
      enc({ channel: 'x', field: 'Horsepower', type: 'quantitative' }),
      enc({ channel: 'y', field: 'Miles_per_Gallon', type: 'quantitative' }),
      enc({ channel: 'color', field: 'Origin', type: 'nominal' }),
      enc({ channel: 'text', field: 'Origin', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'Horsepower', type: 'quantitative' },
      y: { field: 'Miles_per_Gallon', type: 'quantitative' },
      color: { field: 'Origin', type: 'nominal' },
      text: { field: 'Origin', type: 'nominal' },
    },
  },
  {
    name: 'square',
    model: model({ type: 'square' }, [
      enc({ channel: 'x', field: 'a', type: 'ordinal' }),
      enc({ channel: 'y', field: 'b', type: 'ordinal' }),
    ]),
    mark: 'square',
    encoding: { x: { field: 'a', type: 'ordinal' }, y: { field: 'b', type: 'ordinal' } },
  },
  {
    name: 'rule_color_mean',
    model: model({ type: 'rule' }, [enc({ channel: 'y', field: 'Acceleration', type: 'quantitative', aggregate: 'mean' })]),
    mark: 'rule',
    encoding: { y: { field: 'Acceleration', type: 'quantitative', aggregate: 'mean' } },
  },
  {
    name: 'boxplot_2D_vertical',
    model: model({ type: 'boxplot', advancedJson: '{"extent":"min-max"}' }, [
      enc({ channel: 'x', field: 'Origin', type: 'nominal' }),
      enc({ channel: 'y', field: 'Miles_per_Gallon', type: 'quantitative' }),
    ]),
    encoding: { x: { field: 'Origin', type: 'nominal' }, y: { field: 'Miles_per_Gallon', type: 'quantitative' } },
  },
  {
    name: 'tooltip_multi (array channel)',
    model: model({ type: 'point' }, [
      enc({ channel: 'x', field: 'Horsepower', type: 'quantitative' }),
      enc({ channel: 'y', field: 'Miles_per_Gallon', type: 'quantitative' }),
      enc({ channel: 'tooltip', field: 'Name', type: 'nominal' }),
      enc({ channel: 'tooltip', field: 'Origin', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'Horsepower', type: 'quantitative' },
      y: { field: 'Miles_per_Gallon', type: 'quantitative' },
      tooltip: [
        { field: 'Name', type: 'nominal' },
        { field: 'Origin', type: 'nominal' },
      ],
    },
  },
  {
    name: 'trellis_bar (facet via row)',
    model: model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'population', type: 'quantitative', aggregate: 'sum' }),
      enc({ channel: 'y', field: 'age', type: 'ordinal' }),
      enc({ channel: 'row', field: 'gender', type: 'nominal' }),
    ]),
    encoding: {
      x: { field: 'population', type: 'quantitative', aggregate: 'sum' },
      y: { field: 'age', type: 'ordinal' },
      row: { field: 'gender', type: 'nominal' },
    },
  },
];

describe('builder reproduces gallery examples', () => {
  it.each(cases)('$name', ({ model: m, encoding, mark }) => {
    const { spec, errors } = fromBuilder(m);
    expect(errors).toHaveLength(0);
    if (mark !== undefined) {
      expect(spec.mark).toEqual(mark);
    }
    expect(spec.encoding).toEqual(encoding);
  });

  it.each(cases)('compiles: $name', ({ model: m }) => {
    expect(() => compileModel(m)).not.toThrow();
  });
});

describe('builder transforms reproduce gallery examples', () => {
  it('bar_aggregate_transform (aggregate transform)', () => {
    const m = model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'mean_acc', type: 'quantitative' }),
      enc({ channel: 'y', field: 'Cylinders', type: 'ordinal' }),
    ], {
      transforms: [
        {
          id: 't1',
          kind: 'aggregate',
          json: '{"aggregate":[{"op":"mean","field":"Acceleration","as":"mean_acc"}],"groupby":["Cylinders"]}',
        },
      ],
    });
    const { spec, errors } = fromBuilder(m);
    expect(errors).toHaveLength(0);
    expect(spec.transform).toEqual([
      { aggregate: [{ op: 'mean', field: 'Acceleration', as: 'mean_acc' }], groupby: ['Cylinders'] },
    ]);
    expect(() => compileModel(m)).not.toThrow();
  });

  it('bar_filter_calc (filter + calculate)', () => {
    const m = model({ type: 'bar' }, [
      enc({ channel: 'x', field: 'AorB', type: 'nominal' }),
      enc({ channel: 'y', field: 'b', type: 'quantitative' }),
    ], {
      transforms: [
        { id: 't1', kind: 'filter', json: '{"filter":"datum.b > 0"}' },
        { id: 't2', kind: 'calculate', json: '{"calculate":"datum.a","as":"AorB"}' },
      ],
    });
    const { spec, errors } = fromBuilder(m);
    expect(errors).toHaveLength(0);
    expect(spec.transform).toEqual([{ filter: 'datum.b > 0' }, { calculate: 'datum.a', as: 'AorB' }]);
    expect(() => compileModel(m)).not.toThrow();
  });

  it('window_cumulative (window transform)', () => {
    const m = model({ type: 'area' }, [
      enc({ channel: 'x', field: 'date', type: 'temporal' }),
      enc({ channel: 'y', field: 'cumulative', type: 'quantitative' }),
    ], {
      transforms: [
        {
          id: 't1',
          kind: 'window',
          json: '{"window":[{"op":"sum","field":"count","as":"cumulative"}],"sort":[{"field":"date"}]}',
        },
      ],
    });
    const { errors } = fromBuilder(m);
    expect(errors).toHaveLength(0);
    expect(() => compileModel(m)).not.toThrow();
  });

  it('fold (fold transform)', () => {
    const m = model({ type: 'line' }, [
      enc({ channel: 'x', field: 'x', type: 'quantitative' }),
      enc({ channel: 'y', field: 'value', type: 'quantitative' }),
      enc({ channel: 'color', field: 'key', type: 'nominal' }),
    ], {
      transforms: [{ id: 't1', kind: 'fold', json: '{"fold":["a","b"],"as":["key","value"]}' }],
    });
    const { spec, errors } = fromBuilder(m);
    expect(errors).toHaveLength(0);
    expect(spec.transform).toEqual([{ fold: ['a', 'b'], as: ['key', 'value'] }]);
    expect(() => compileModel(m)).not.toThrow();
  });
});
