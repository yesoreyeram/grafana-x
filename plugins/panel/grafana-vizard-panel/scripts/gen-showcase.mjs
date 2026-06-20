// Generate the Vizard "Showcase" dashboard: a Grafana 13 dynamic dashboard
// (v2 schema) using the new TABS layout. Every panel is data-driven by the
// Infinity datasource's *inline* source (the data lives in the query, NOT in the
// Vega-Lite spec) and configured with the Vizard *builder* (mark + encodings,
// plus builder layers / transforms for composite charts) — no spec-override JSON.
// The Grafana theme and palette are applied automatically by the panel pipeline.
//
// Run: node scripts/gen-showcase.mjs  (writes provisioning/dashboards/showcase.json)

import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const OUT = join(dirname(fileURLToPath(import.meta.url)), '..', 'provisioning', 'dashboards', 'showcase.json');

// ---------------------------------------------------------------------------
// Deterministic data generators (seeded → reproducible). Each returns a flat
// array of row objects, which Infinity parses into a typed data frame.
// ---------------------------------------------------------------------------
const R = (n) => Math.round(n * 100) / 100;
function makeRng(seed) {
  let s = seed >>> 0;
  return () => {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
    return s / 4294967296;
  };
}
const gauss = (rng) => (rng() + rng() + rng() + rng() - 2) / 2;

function flowSeries(names, n, seed) {
  const rng = makeRng(seed);
  const p = names.map(() => ({ base: 3 + rng() * 9, amp: 2 + rng() * 7, period: 6 + rng() * 12, phase: rng() * 6.28, w: rng() * 2 }));
  const rows = [];
  for (let x = 0; x < n; x++) {
    names.forEach((s, i) => rows.push({ x, v: R(Math.max(0.2, p[i].base + p[i].amp * Math.sin(x / p[i].period + p[i].phase) + p[i].w * Math.sin(x / 2.3 + i))), s }));
  }
  return rows;
}
function sineGrid(cols, rows) {
  const out = [];
  for (let i = 0; i < cols; i++) for (let j = 0; j < rows; j++) out.push({ x: i, y: j, v: R(Math.sin(i / 3) * Math.cos(j / 3) + Math.sin((i + j) / 5)) });
  return out;
}
function clusters(groups, per, seed) {
  const rng = makeRng(seed);
  const out = [];
  groups.forEach((g) => {
    const cx = 25 + rng() * 55, cy = 25 + rng() * 55, sp = 5 + rng() * 7;
    for (let k = 0; k < per; k++) out.push({ x: R(cx + gauss(rng) * sp * 4), y: R(cy + gauss(rng) * sp * 4), size: R(5 + rng() * 45), g });
  });
  return out;
}
function groupedCats(cats, groups, seed) {
  const rng = makeRng(seed);
  const out = [];
  cats.forEach((cat) => groups.forEach((grp) => out.push({ cat, grp, v: R(5 + rng() * 45) })));
  return out;
}
const RESP = ['Strongly disagree', 'Disagree', 'Neutral', 'Agree', 'Strongly agree'];
function likert(questions, seed) {
  const rng = makeRng(seed);
  const out = [];
  questions.forEach((q) => RESP.forEach((resp, idx) => out.push({ q, resp, v: (idx < 2 ? -1 : 1) * (5 + Math.round(rng() * 28)) })));
  return out;
}
function gaussVals(n, seed) {
  const rng = makeRng(seed);
  return Array.from({ length: n }, () => ({ v: R(60 + gauss(rng) * 70) }));
}
function boxGroups(groups, per, seed) {
  const rng = makeRng(seed);
  const out = [];
  groups.forEach((g, i) => {
    for (let k = 0; k < per; k++) out.push({ g, v: R(30 + i * 12 + gauss(rng) * 45) });
  });
  return out;
}
function ternaryPts(groups, per, seed) {
  const rng = makeRng(seed);
  const out = [];
  groups.forEach((g, i) => {
    for (let k = 0; k < per; k++) out.push({ a: R(rng() + (i === 0 ? 0.6 : 0)), b: R(rng() + (i === 1 ? 0.6 : 0)), c: R(rng() + (i === 2 ? 0.6 : 0)), g });
  });
  return out;
}
function ohlc(n, seed) {
  const rng = makeRng(seed);
  const out = [];
  let price = 100;
  for (let i = 0; i < n; i++) {
    const o = price, c = R(o + gauss(rng) * 7);
    out.push({ date: `2024-02-${String(i + 1).padStart(2, '0')}`, o: R(o), c, h: R(Math.max(o, c) + rng() * 4), l: R(Math.min(o, c) - rng() * 4) });
    price = c;
  }
  return out;
}
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const gradientWave = (n) => Array.from({ length: n }, (_, x) => ({ x, v: R(22 + 12 * Math.sin(x / 6) + 6 * Math.sin(x / 2.4)) }));
const spiral = (n) => Array.from({ length: n }, (_, i) => ({ x: R(i * 0.95 * Math.cos(i * 0.5)), y: R(i * 0.95 * Math.sin(i * 0.5)), i }));
const rose = (seed) => {
  const rng = makeRng(seed);
  return MONTHS.map((month, mi) => ({ month, v: R(6 + (1 + Math.sin(mi / 2)) * 12 + rng() * 8) }));
};
function calendar(weeks, seed) {
  const rng = makeRng(seed);
  const out = [];
  for (let w = 0; w < weeks; w++) for (let d = 0; d < 7; d++) out.push({ week: w, day: DAYS[d], v: Math.max(0, Math.round((Math.sin(w / 6) + 1) * 4 + gauss(rng) * 4)) });
  return out;
}
function slope(seed) {
  const rng = makeRng(seed);
  const out = [];
  ['North', 'South', 'East', 'West', 'Central'].forEach((grp) => {
    const before = R(20 + rng() * 45);
    out.push({ stage: 'Q1', val: before, grp });
    out.push({ stage: 'Q4', val: R(Math.max(2, before + (rng() - 0.4) * 35)), grp });
  });
  return out;
}
const loop = (n) => Array.from({ length: n }, (_, t) => ({ x: R(Math.cos((t / n) * 6.28) * 10 + Math.cos(t / 5) * 2.5), y: R(Math.sin((t / n) * 6.28) * 10 + Math.sin(t / 4) * 2.5), t }));

// ---------------------------------------------------------------------------
// Builder + Infinity-inline assembly (v2 schema, TabsLayout).
// ---------------------------------------------------------------------------
const GRADIENT = {
  gradient: 'linear',
  x1: 1, y1: 1, x2: 1, y2: 0,
  stops: [{ offset: 0, color: '#3a1c71' }, { offset: 0.5, color: '#d76d77' }, { offset: 1, color: '#ffaf7b' }],
};

/** A builder encoding row (channel doubles as id). */
const enc = (channel, o = {}) => ({ id: channel, channel, ...o });
/** A builder model. */
const B = (mark, encodings, extra = {}) => ({ mark, encodings, transforms: extra.transforms ?? [], layers: extra.layers ?? [], params: [] });
const mark = (type, o = {}) => ({ type, tooltip: true, ...o });

let pid = 0;
const elements = {};

function infinityInline(rows) {
  return {
    kind: 'PanelQuery',
    spec: {
      refId: 'A',
      hidden: false,
      query: {
        kind: 'DataQuery',
        group: 'yesoreyeram-infinity-datasource',
        version: 'v0',
        datasource: { name: 'vizard-infinity' },
        spec: { type: 'json', format: 'table', parser: 'backend', root_selector: '', columns: [], filters: [], source: 'inline', data: JSON.stringify(rows) },
      },
    },
  };
}

/** Register a Vizard element: Infinity inline data + a builder model. */
function panel(title, rows, builderModel, opts = {}) {
  pid++;
  const name = `panel-${pid}`;
  elements[name] = {
    kind: 'Panel',
    spec: {
      id: pid,
      title,
      description: opts.description ?? '',
      links: [],
      data: { kind: 'QueryGroup', spec: { queries: [infinityInline(rows)], transformations: [], queryOptions: {} } },
      vizConfig: {
        kind: 'VizConfig',
        group: 'yesoreyeram-vizard-panel',
        version: '',
        spec: {
          options: {
            editorMode: 'builder',
            renderer: 'canvas',
            tooltip: opts.tooltip !== false,
            legend: opts.legend !== false,
            theme: { colorScheme: opts.color ?? 'palette-classic' },
            data: { source: 'auto' },
            builder: builderModel,
          },
          fieldConfig: { defaults: {}, overrides: [] },
        },
      },
    },
  };
  return { name, w: opts.w ?? 12, h: opts.h ?? 9 };
}

function tab(title, panels) {
  let x = 0, y = 0, rowH = 0;
  const items = panels.map((p) => {
    if (x + p.w > 24) {
      x = 0;
      y += rowH;
      rowH = 0;
    }
    const item = { kind: 'GridLayoutItem', spec: { x, y, width: p.w, height: p.h, element: { kind: 'ElementReference', name: p.name } } };
    x += p.w;
    rowH = Math.max(rowH, p.h);
    return item;
  });
  return { kind: 'TabsLayoutTab', spec: { title, layout: { kind: 'GridLayout', spec: { items } } } };
}

// --- datasets ---
const flow = flowSeries(['alpha', 'beta', 'gamma', 'delta', 'epsilon'], 60, 7);
const waves = flowSeries(['north', 'south', 'east', 'west'], 50, 11);
const heat = sineGrid(28, 14);
const bub = clusters(['cluster A', 'cluster B', 'cluster C'], 45, 21);
const fruit = [
  { c: 'Apples', v: 38 }, { c: 'Bananas', v: 27 }, { c: 'Cherries', v: 18 },
  { c: 'Dates', v: 11 }, { c: 'Elderberry', v: 9 }, { c: 'Figs', v: 14 },
];
const grouped = groupedCats(['Q1', 'Q2', 'Q3', 'Q4'], ['Online', 'Retail', 'Wholesale'], 33);
const survey = likert(['Onboarding', 'Performance', 'Support', 'Pricing'], 44);
const dist = gaussVals(400, 55);
const boxes = boxGroups(['A', 'B', 'C', 'D'], 40, 66);
const tern = ternaryPts(['ore X', 'ore Y', 'ore Z'], 30, 77);
const candles = ohlc(22, 88);
const gradWave = gradientWave(80);
const spiralPts = spiral(170);
const roseData = rose(99);
const cal = calendar(53, 101);
const slopeData = slope(123);
const loopData = loop(120);

// --- common encodings ---
const X = enc('x', { field: 'x', type: 'quantitative' });
const Yv = enc('y', { field: 'v', type: 'quantitative' });
const Cs = enc('color', { field: 's', type: 'nominal' });

const tabs = [
  tab('Home', [
    panel('Streamgraph', flow, B(mark('area', { interpolate: 'monotone' }), [X, enc('y', { field: 'v', type: 'quantitative', stack: 'center' }), Cs]), { w: 24, h: 10, legend: false, description: 'Five flowing series stacked around a centered baseline.' }),
    panel('Gradient area', gradWave, B(mark('area', { props: { color: GRADIENT, line: { color: '#ffaf7b', strokeWidth: 2 } } }), [enc('x', { field: 'x', type: 'quantitative', axis: null }), enc('y', { field: 'v', type: 'quantitative', axis: null })]), { w: 8, h: 8, legend: false, description: 'An area filled with a linear color gradient.' }),
    panel('Spiral', spiralPts, B(mark('circle', { opacity: 0.85 }), [enc('x', { field: 'x', type: 'quantitative', axis: null }), enc('y', { field: 'y', type: 'quantitative', axis: null }), enc('color', { field: 'i', type: 'quantitative' }), enc('size', { field: 'i', type: 'quantitative', scale: { range: [8, 220] } })]), { w: 8, h: 8, color: 'plasma', legend: false, description: 'Points along an Archimedean spiral.' }),
    panel('Nightingale rose', roseData, B(mark('arc', { props: { stroke: '#ffffff', strokeWidth: 1, innerRadius: 8 } }), [enc('theta', { field: 'month', type: 'nominal' }), enc('radius', { field: 'v', type: 'quantitative', scale: { type: 'sqrt', zero: true } }), enc('color', { field: 'month', type: 'nominal' })]), { w: 8, h: 8, legend: false, description: 'A coxcomb / polar-area chart.' }),
    panel('Calendar heatmap', cal, B(mark('rect', { props: { cornerRadius: 2 } }), [enc('x', { field: 'week', type: 'ordinal', axis: null }), enc('y', { field: 'day', type: 'ordinal', sort: JSON.stringify(DAYS), axis: { title: null, domain: false, ticks: false } }), enc('color', { field: 'v', type: 'quantitative' })]), { w: 12, h: 7, color: 'continuous-greens', description: 'A year of activity, GitHub-style.' }),
    panel('Sine heatmap', heat, B(mark('rect'), [enc('x', { field: 'x', type: 'ordinal', axis: null }), enc('y', { field: 'y', type: 'ordinal', axis: null }), enc('color', { field: 'v', type: 'quantitative' })]), { w: 6, h: 7, color: 'continuous-GrYlRd', legend: false }),
    panel('Bubble clusters', bub, B(mark('circle', { opacity: 0.7 }), [X, enc('y', { field: 'y', type: 'quantitative' }), enc('size', { field: 'size', type: 'quantitative' }), enc('color', { field: 'g', type: 'nominal' })]), { w: 6, h: 7 }),
  ]),
  tab('Lines & areas', [
    panel('Multi-line', waves, B(mark('line', { props: { strokeWidth: 2 } }), [X, Yv, Cs]), { w: 12, h: 9 }),
    panel('Step line', waves, B(mark('line', { interpolate: 'step' }), [X, Yv, Cs]), { w: 12, h: 9 }),
    panel('Area', gradWave, B(mark('area', { opacity: 0.85 }), [X, Yv]), { w: 12, h: 9, legend: false }),
    panel('Stacked area', flow, B(mark('area', { opacity: 0.85 }), [X, enc('y', { field: 'v', type: 'quantitative', stack: 'zero' }), Cs]), { w: 12, h: 9 }),
    panel('Streamgraph', flow, B(mark('area', { interpolate: 'monotone' }), [X, enc('y', { field: 'v', type: 'quantitative', stack: 'center' }), Cs]), { w: 12, h: 9, legend: false }),
    panel('Trellis area (small multiples)', waves, B(mark('area', { opacity: 0.85 }), [X, Yv, Cs, enc('column', { field: 's', type: 'nominal' })]), { w: 24, h: 9, description: 'One area per series, faceted into columns.' }),
  ]),
  tab('Bars', [
    panel('Bar', fruit, B(mark('bar'), [enc('x', { field: 'c', type: 'nominal' }), Yv, enc('color', { field: 'c', type: 'nominal' })]), { w: 12, h: 9, legend: false }),
    panel('Grouped bar', grouped, B(mark('bar'), [enc('x', { field: 'cat', type: 'nominal' }), Yv, enc('xOffset', { field: 'grp', type: 'nominal' }), enc('color', { field: 'grp', type: 'nominal' })]), { w: 12, h: 9 }),
    panel('Stacked bar', grouped, B(mark('bar'), [enc('x', { field: 'cat', type: 'nominal' }), enc('y', { field: 'v', type: 'quantitative', stack: 'zero' }), enc('color', { field: 'grp', type: 'nominal' })]), { w: 12, h: 9 }),
    panel('Normalized stacked bar', grouped, B(mark('bar'), [enc('x', { field: 'cat', type: 'nominal' }), enc('y', { field: 'v', type: 'quantitative', stack: 'normalize' }), enc('color', { field: 'grp', type: 'nominal' })]), { w: 12, h: 9 }),
    panel('Horizontal bar', fruit, B(mark('bar'), [enc('y', { field: 'c', type: 'nominal', sort: '-x' }), enc('x', { field: 'v', type: 'quantitative' }), enc('color', { field: 'c', type: 'nominal' })]), { w: 12, h: 9, legend: false }),
    panel('Diverging stacked bar (Likert)', survey, B(mark('bar'), [enc('x', { field: 'v', type: 'quantitative', stack: 'zero' }), enc('y', { field: 'q', type: 'nominal' }), enc('color', { field: 'resp', type: 'nominal', sort: JSON.stringify(RESP) })]), { w: 12, h: 9, color: 'spectral' }),
  ]),
  tab('Parts of a whole', [
    panel('Pie', fruit, B(mark('arc'), [enc('theta', { field: 'v', type: 'quantitative' }), enc('color', { field: 'c', type: 'nominal' })]), { w: 12, h: 9 }),
    panel('Donut', fruit, B(mark('arc', { props: { innerRadius: 60 } }), [enc('theta', { field: 'v', type: 'quantitative' }), enc('color', { field: 'c', type: 'nominal' })]), { w: 12, h: 9 }),
    panel('Radial plot', fruit, B(mark('arc', { props: { stroke: '#ffffff' } }), [enc('theta', { field: 'v', type: 'quantitative', stack: 'zero' }), enc('radius', { field: 'v', type: 'quantitative', scale: { type: 'sqrt', zero: true, rangeMin: 20 } }), enc('color', { field: 'c', type: 'nominal' })]), { w: 12, h: 9 }),
    panel('Nightingale rose', roseData, B(mark('arc', { props: { stroke: '#ffffff', innerRadius: 8 } }), [enc('theta', { field: 'month', type: 'nominal' }), enc('radius', { field: 'v', type: 'quantitative', scale: { type: 'sqrt', zero: true } }), enc('color', { field: 'month', type: 'nominal' })]), { w: 12, h: 9, legend: false }),
  ]),
  tab('Points & distribution', [
    panel('Scatter', bub, B(mark('point', { filled: true }), [X, enc('y', { field: 'y', type: 'quantitative' }), enc('color', { field: 'g', type: 'nominal' })]), { w: 12, h: 9 }),
    panel('Histogram', dist, B(mark('bar'), [enc('x', { field: 'v', type: 'quantitative', bin: true }), enc('y', { aggregate: 'count', type: 'quantitative' })]), { w: 12, h: 9, color: 'continuous-blues', legend: false }),
    panel('2D density heatmap', bub, B(mark('rect'), [enc('x', { field: 'x', type: 'quantitative', bin: true }), enc('y', { field: 'y', type: 'quantitative', bin: true }), enc('color', { aggregate: 'count', type: 'quantitative' })]), { w: 12, h: 9, color: 'continuous-GrYlRd', legend: false }),
    panel('Box plot', boxes, B(mark('boxplot'), [enc('x', { field: 'g', type: 'nominal' }), enc('y', { field: 'v', type: 'quantitative' }), enc('color', { field: 'g', type: 'nominal' })]), { w: 12, h: 9, legend: false }),
    panel('Strip plot', boxes, B(mark('tick', { opacity: 0.5 }), [enc('x', { field: 'v', type: 'quantitative' }), enc('y', { field: 'g', type: 'nominal' }), enc('color', { field: 'g', type: 'nominal' })]), { w: 12, h: 9, legend: false }),
    panel('Error bars', boxes, B(mark('point'), [enc('x', { field: 'g', type: 'nominal' })], {
      layers: [
        { id: 'eb', mark: mark('errorbar', { props: { extent: 'ci', ticks: true } }), encodings: [enc('y', { field: 'v', type: 'quantitative' })] },
        { id: 'mean', mark: mark('point', { filled: true, props: { color: 'black', size: 50 } }), encodings: [enc('y', { field: 'v', type: 'quantitative', aggregate: 'mean' })] },
      ],
    }), { w: 12, h: 9 }),
  ]),
  tab('Specials', [
    panel('Ternary plot', tern, B(mark('point', { filled: true, opacity: 0.7 }), [
      enc('x', { field: 'tx', type: 'quantitative', scale: { domain: [0, 1] }, axis: null }),
      enc('y', { field: 'ty', type: 'quantitative', scale: { domain: [0, 0.9] }, axis: null }),
      enc('color', { field: 'g', type: 'nominal' }),
    ], {
      transforms: [
        { id: 'tx', kind: 'calculate', json: JSON.stringify({ calculate: '(datum.b/(datum.a+datum.b+datum.c)) + (datum.c/(datum.a+datum.b+datum.c))/2', as: 'tx' }) },
        { id: 'ty', kind: 'calculate', json: JSON.stringify({ calculate: '(datum.c/(datum.a+datum.b+datum.c)) * 0.8660254', as: 'ty' }) },
      ],
    }), { w: 12, h: 9, description: 'Three measures projected onto a triangle via builder transforms.' }),
    panel('Candlestick (OHLC)', candles, B(mark('bar'), [enc('x', { field: 'date', type: 'ordinal', axis: { labelAngle: -45 } })], {
      layers: [
        { id: 'wick', mark: mark('rule'), encodings: [enc('y', { field: 'l', type: 'quantitative', scale: { zero: false } }), enc('y2', { field: 'h' })] },
        { id: 'body', mark: mark('bar', { props: { size: 6 } }), encodings: [enc('y', { field: 'o', type: 'quantitative' }), enc('y2', { field: 'c' }), enc('color', { value: '#c4162a', condition: { test: 'datum.o < datum.c', value: '#3ba33b' } })] },
      ],
    }), { w: 12, h: 9, legend: false }),
    panel('Slope chart', slopeData, B(mark('line', { point: true, props: { strokeWidth: 2 } }), [enc('x', { field: 'stage', type: 'ordinal', sort: JSON.stringify(['Q1', 'Q4']) }), enc('y', { field: 'val', type: 'quantitative' }), enc('color', { field: 'grp', type: 'nominal' })]), { w: 12, h: 9, description: 'Change between two points, one line per group.' }),
    panel('Connected scatter', loopData, B(mark('line', { point: true, props: { strokeWidth: 1.5 } }), [enc('x', { field: 'x', type: 'quantitative', axis: null }), enc('y', { field: 'y', type: 'quantitative', axis: null }), enc('order', { field: 't', type: 'quantitative' }), enc('color', { field: 't', type: 'quantitative' })]), { w: 12, h: 9, color: 'continuous-BlPu', legend: false, description: 'A path traced through 2D space.' }),
  ]),
];

const dashboard = {
  apiVersion: 'dashboard.grafana.app/v2beta1',
  kind: 'Dashboard',
  metadata: { name: 'vizard-showcase' },
  spec: {
    title: 'Vizard — Showcase',
    description: 'A tour of brilliant Vega-Lite charts built with the Vizard builder, driven by Infinity inline data, organized into Grafana 13 tabs.',
    tags: ['vizard', 'vega-lite', 'showcase'],
    editable: true,
    preload: false,
    liveNow: false,
    cursorSync: 'Off',
    links: [],
    annotations: [],
    variables: [],
    timeSettings: { timezone: 'browser', from: 'now-6h', to: 'now', autoRefresh: '', autoRefreshIntervals: ['5s', '10s', '30s', '1m', '5m', '15m', '30m', '1h', '2h', '1d'], hideTimepicker: true, fiscalYearStartMonth: 0 },
    elements,
    layout: { kind: 'TabsLayout', spec: { tabs } },
  },
};

writeFileSync(OUT, JSON.stringify(dashboard, null, 2) + '\n');
console.log(`Wrote ${OUT} — ${Object.keys(elements).length} panels across ${tabs.length} tabs`);
