import { createTheme } from '@grafana/data';
import { parse as vegaParse } from 'vega';
import { compile } from 'vega-lite';

import { buildDataContext } from '../data/dataContext';
import { buildVegaConfig } from '../theme/grafanaTheme';
import { defaultPanelOptions, PanelOptions } from '../types';
import { buildSpec } from './index';
import examples from './__fixtures__/vegaLiteExamples.json';

type CompileSpec = Parameters<typeof compile>[0];

const theme = createTheme();
const themeConfig = buildVegaConfig(theme, defaultPanelOptions.theme);
// Empty Grafana data: each example carries its own data, and in the demo the
// Infinity datasource supplies it. We assert the spec compiles to Vega.
const ctx = buildDataContext([], { source: 'auto' });
const size = { width: 600, height: 400 };

const entries = Object.entries(examples as Record<string, unknown>);

/** Run an example spec through the panel pipeline exactly as the demo does. */
function buildExample(spec: unknown) {
  const options: PanelOptions = {
    ...defaultPanelOptions,
    builder: { ...defaultPanelOptions.builder, specOverrideJson: JSON.stringify(spec) },
  };
  return buildSpec(options, ctx, themeConfig, size);
}

describe('Vega-Lite examples coverage', () => {
  beforeAll(() => {
    // Vega-Lite logs (non-fatal) warnings for some specs; keep test output clean.
    jest.spyOn(console, 'warn').mockImplementation(() => {});
    jest.spyOn(console, 'error').mockImplementation(() => {});
  });
  afterAll(() => {
    jest.restoreAllMocks();
  });

  it('loads the full example corpus', () => {
    expect(entries.length).toBeGreaterThan(600);
  });

  it.each(entries)('compiles + parses example "%s" through the panel pipeline', (_name, spec) => {
    const { spec: built } = buildExample(spec);
    const result = compile(built as unknown as CompileSpec);
    expect(result.spec).toBeTruthy();
    expect(typeof result.spec).toBe('object');
    // Also parse the compiled Vega spec to a runtime dataflow — this catches
    // errors (e.g. duplicate signals) that compile alone does not.
    expect(() => vegaParse(result.spec)).not.toThrow();
  });
});
