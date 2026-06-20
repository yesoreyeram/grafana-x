import { readFileSync } from 'fs';
import { join } from 'path';

import { createTheme, FieldType, toDataFrame } from '@grafana/data';
import { parse as vegaParse } from 'vega';
import { compile } from 'vega-lite';

import { buildDataContext } from '../data/dataContext';
import { buildVegaConfig } from '../theme/grafanaTheme';
import { defaultBuilder, defaultPanelOptions, PanelOptions } from '../types';
import { buildSpec } from './index';

// The generated logo dashboard (scripts/gen-logo.mjs) is a v2 tabbed dashboard
// whose every panel is a Vizard rendering of the Grafana mark. This test drives
// each variant through the SAME pipeline the panel uses at runtime (buildSpec),
// then `vega-lite.compile` + `vega.parse` — parse catches runtime-construction
// errors (e.g. the broken native `time` channel) that compile alone misses. It
// also asserts the animated variants actually emit a timer-driven Vega signal.

type CompileSpec = Parameters<typeof compile>[0];
type Row = Record<string, number | string>;

interface DataQuery {
  group: string;
  spec: { data?: string; panelId?: number };
}
interface PanelElement {
  spec: {
    id: number;
    title: string;
    data: { spec: { queries: Array<{ spec: { query: DataQuery } } > } };
    vizConfig: { spec: { options: PanelOptions } };
  };
}
interface LogoDashboard {
  spec: {
    layout: { spec: { tabs: Array<{ spec: { title: string } }> } };
    elements: Record<string, PanelElement>;
  };
}

const dashboard = JSON.parse(
  readFileSync(join(__dirname, '..', '..', 'provisioning', 'dashboards', 'logo.json'), 'utf8')
) as LogoDashboard;

interface Variant {
  title: string;
  options: PanelOptions;
  rows: Row[];
  animated: boolean;
}

// Panels reuse data via the Dashboard datasource (group `datasource`, by panelId);
// resolve those references back to the origin panel's inline rows so each variant
// is exercised against the data it actually renders.
function rowsByPanelId(): Map<number, Row[]> {
  const map = new Map<number, Row[]>();
  for (const el of Object.values(dashboard.spec.elements)) {
    const q = el.spec.data.spec.queries[0].spec.query;
    if (q.group !== 'datasource' && typeof q.spec.data === 'string') {
      map.set(el.spec.id, JSON.parse(q.spec.data) as Row[]);
    }
  }
  return map;
}

function variants(): Variant[] {
  const inlineRows = rowsByPanelId();
  return Object.values(dashboard.spec.elements).map((el) => {
    const saved = el.spec.vizConfig.spec.options;
    const options: PanelOptions = {
      ...defaultPanelOptions,
      ...saved,
      builder: { ...defaultBuilder, ...saved.builder },
    };
    const q = el.spec.data.spec.queries[0].spec.query;
    const rows = q.group === 'datasource' ? inlineRows.get(q.spec.panelId as number)! : (JSON.parse(q.spec.data as string) as Row[]);
    const override = options.builder.specOverrideJson
      ? (JSON.parse(options.builder.specOverrideJson) as { params?: Array<{ on?: unknown }> })
      : null;
    const animated = Boolean(override?.params?.some((p) => p.on !== undefined));
    return { title: el.spec.title, options, rows, animated };
  });
}

function frameFromRows(rows: Row[]) {
  const keys = Object.keys(rows[0]);
  return toDataFrame({
    refId: 'A',
    fields: keys.map((name) => ({
      name,
      type: typeof rows[0][name] === 'string' ? FieldType.string : FieldType.number,
      values: rows.map((r) => r[name]),
    })),
  });
}

function hasTimerSignal(spec: ReturnType<typeof compile>['spec']): boolean {
  const signals = (spec.signals ?? []) as Array<{ on?: unknown }>;
  return signals.some((s) => JSON.stringify(s.on ?? '').includes('timer'));
}

const theme = createTheme();
const themeConfig = buildVegaConfig(theme, defaultPanelOptions.theme);
const size = { width: 400, height: 440 };
const all = variants();

describe('Grafana logo dashboard', () => {
  beforeAll(() => {
    // Vega-Lite logs non-fatal warnings for some specs; keep test output clean.
    jest.spyOn(console, 'warn').mockImplementation(() => {});
    jest.spyOn(console, 'error').mockImplementation(() => {});
  });
  afterAll(() => {
    jest.restoreAllMocks();
  });

  it('is a v2 tabbed dashboard with "Grafana" and "Football Fever" tabs', () => {
    expect(all.length).toBeGreaterThanOrEqual(20);
    expect(dashboard.spec.layout.spec.tabs.map((t) => t.spec.title)).toEqual(['Grafana', 'Football Fever']);
  });

  it.each(all.map((v) => [v.title, v] as const))(
    'builds, compiles and parses "%s" through the panel pipeline',
    (_title, v) => {
      const ctx = buildDataContext([frameFromRows(v.rows)], { source: 'auto' });
      const { spec } = buildSpec(v.options, ctx, themeConfig, size);
      const compiled = compile(spec as unknown as CompileSpec);
      expect(compiled.spec).toBeTruthy();
      // parse the compiled Vega to a runtime dataflow — catches errors compile misses.
      expect(() => vegaParse(compiled.spec)).not.toThrow();
    }
  );

  it('compiles every animated variant to a timer-driven Vega signal', () => {
    const animated = all.filter((v) => v.animated);
    expect(animated.length).toBeGreaterThanOrEqual(7);
    for (const v of animated) {
      const ctx = buildDataContext([frameFromRows(v.rows)], { source: 'auto' });
      const { spec } = buildSpec(v.options, ctx, themeConfig, size);
      const compiled = compile(spec as unknown as CompileSpec);
      expect(hasTimerSignal(compiled.spec)).toBe(true);
    }
  });
});
