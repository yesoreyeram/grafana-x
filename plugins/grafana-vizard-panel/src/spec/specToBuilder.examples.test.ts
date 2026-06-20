import { createTheme } from '@grafana/data';
import { parse as vegaParse } from 'vega';
import { compile } from 'vega-lite';

import { buildDataContext } from '../data/dataContext';
import { buildVegaConfig } from '../theme/grafanaTheme';
import { defaultPanelOptions, PanelOptions } from '../types';
import { buildSpec } from './index';
import { specToBuilder } from './specToBuilder';
import examples from './__fixtures__/vegaLiteExamples.json';

type CompileSpec = Parameters<typeof compile>[0];

const theme = createTheme();
const themeConfig = buildVegaConfig(theme, defaultPanelOptions.theme);
const ctx = buildDataContext([], { source: 'auto' });
const size = { width: 600, height: 400 };
const entries = Object.entries(examples as Record<string, unknown>);

beforeAll(() => {
  jest.spyOn(console, 'warn').mockImplementation(() => {});
  jest.spyOn(console, 'error').mockImplementation(() => {});
});
afterAll(() => jest.restoreAllMocks());

describe('specToBuilder reproduces examples through the builder model', () => {
  // Every example that converts to a builder model must render (compile + parse)
  // through the full pipeline — this is what makes the demo galleries builder-native.
  const convertible = entries.filter(([, spec]) => specToBuilder(spec).ok);

  it('converts a large majority of examples to a native builder model', () => {
    // Report and guard the coverage so regressions are visible.
    expect(convertible.length).toBeGreaterThan(entries.length * 0.6);
  });

  it.each(convertible)('builder-renders example "%s"', (_name, spec) => {
    const { builder } = specToBuilder(spec);
    const options: PanelOptions = { ...defaultPanelOptions, builder };
    const { spec: built } = buildSpec(options, ctx, themeConfig, size);
    const result = compile(built as unknown as CompileSpec);
    expect(result.spec).toBeTruthy();
    expect(() => vegaParse(result.spec)).not.toThrow();
  });
});
