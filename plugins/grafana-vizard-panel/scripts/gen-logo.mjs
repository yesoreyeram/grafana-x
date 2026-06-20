// Generate the "Grafana logo" demo dashboard: the Grafana brand mark rendered
// MANY creative ways with the Vizard panel — as a Vega-Lite rect/circle/text
// "bitmap" rasterized from the official SVG (scripts/grafana-logo.svg), served
// via the Infinity datasource's INLINE source, and drawn through the Vizard
// builder (or the spec-override escape hatch for the fancier variants). Colour
// follows the logo's yellow→orange gradient.
//
// The dashboard is a Grafana 13 dynamic dashboard (v2 schema, TabsLayout) with two
// tabs: "Grafana" (bitmap, effects and animations) and "Football Fever" (the
// football-themed variants plus a waving flag and a tricolore gloss sweep).
//
// ── Animation ──────────────────────────────────────────────────────────────
// Inspired by MIT's "Animated Vega-Lite" (https://vis.csail.mit.edu/pubs/
// animated-vega-lite/). Vega-Lite 6 ships a native `time` encoding channel from
// that work, but in the pinned vega-lite@6.4.3 it fails to *parse* to a runtime
// dataflow (it references an undefined `data_0_curr` dataset), so it would throw
// in the panel. We therefore drive animation with the primitive that the native
// channel itself compiles down to: a timer-driven Vega-Lite parameter
//   params: [{ name:'anim', value:0, on:[{ events:{type:'timer',throttle},
//              update:'(anim+1)%N' }] }]
// and reference that `anim` signal from `calculate` / `filter` transforms. This
// keeps everything inside the panel's hardened `mode:'vega-lite'` pipeline (no
// network, `ast:true`, no raw Vega) and renders entirely inside the Vega view —
// React never re-renders per frame. Verified to compile AND parse; see
// src/spec/logo.test.ts.
//
// Run: node scripts/gen-logo.mjs  (writes provisioning/dashboards/logo.json)

import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const HERE = dirname(fileURLToPath(import.meta.url));
const SVG = readFileSync(join(HERE, 'grafana-logo.svg'), 'utf8');
const OUT = join(HERE, '..', 'provisioning', 'dashboards', 'logo.json');

const FLAT = 18; // line segments per bézier curve

// The icon is the single gradient-filled path (class "st1"); the text is the gray
// "st0" paths, which we ignore.
const d = SVG.match(/class="st1"\s+d="([^"]+)"/s)[1].replace(/\s+/g, ' ').trim();

// --- SVG path → flattened polygons (subpaths of [x, y] points) ---------------
function parsePath(pathD) {
  const tokens = pathD.match(/[a-zA-Z]|-?\d*\.?\d+(?:[eE][-+]?\d+)?/g);
  let i = 0;
  const num = () => parseFloat(tokens[i++]);
  let cmd = '';
  let cx = 0, cy = 0, sx = 0, sy = 0, pcx = 0, pcy = 0;
  const subpaths = [];
  let cur = null;
  const bez = (x0, y0, x1, y1, x2, y2, x3, y3) => {
    for (let t = 1; t <= FLAT; t++) {
      const u = t / FLAT, m = 1 - u;
      cur.push([m * m * m * x0 + 3 * m * m * u * x1 + 3 * m * u * u * x2 + u * u * u * x3, m * m * m * y0 + 3 * m * m * u * y1 + 3 * m * u * u * y2 + u * u * u * y3]);
    }
  };
  while (i < tokens.length) {
    if (/[a-zA-Z]/.test(tokens[i])) {
      cmd = tokens[i++];
    }
    switch (cmd) {
      case 'M': cx = num(); cy = num(); sx = cx; sy = cy; cur = [[cx, cy]]; subpaths.push(cur); cmd = 'L'; break;
      case 'm': cx += num(); cy += num(); sx = cx; sy = cy; cur = [[cx, cy]]; subpaths.push(cur); cmd = 'l'; break;
      case 'L': cx = num(); cy = num(); cur.push([cx, cy]); break;
      case 'l': cx += num(); cy += num(); cur.push([cx, cy]); break;
      case 'H': cx = num(); cur.push([cx, cy]); break;
      case 'h': cx += num(); cur.push([cx, cy]); break;
      case 'V': cy = num(); cur.push([cx, cy]); break;
      case 'v': cy += num(); cur.push([cx, cy]); break;
      case 'C': { const a = num(), b = num(), c = num(), dd = num(), e = num(), f = num(); bez(cx, cy, a, b, c, dd, e, f); pcx = c; pcy = dd; cx = e; cy = f; break; }
      case 'c': { const a = cx + num(), b = cy + num(), c = cx + num(), dd = cy + num(), e = cx + num(), f = cy + num(); bez(cx, cy, a, b, c, dd, e, f); pcx = c; pcy = dd; cx = e; cy = f; break; }
      case 'S': { const a = 2 * cx - pcx, b = 2 * cy - pcy, c = num(), dd = num(), e = num(), f = num(); bez(cx, cy, a, b, c, dd, e, f); pcx = c; pcy = dd; cx = e; cy = f; break; }
      case 's': { const a = 2 * cx - pcx, b = 2 * cy - pcy, c = cx + num(), dd = cy + num(), e = cx + num(), f = cy + num(); bez(cx, cy, a, b, c, dd, e, f); pcx = c; pcy = dd; cx = e; cy = f; break; }
      case 'Z': case 'z': cur.push([sx, sy]); cx = sx; cy = sy; break;
      default: i++; break;
    }
  }
  // Ensure every subpath is closed (winding needs closed loops).
  for (const sp of subpaths) {
    const a = sp[0], b = sp[sp.length - 1];
    if (a[0] !== b[0] || a[1] !== b[1]) sp.push([a[0], a[1]]);
  }
  return subpaths;
}

const subpaths = parsePath(d);

// bounding box (vector space — shared by every raster resolution + the outline)
let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
for (const sp of subpaths) for (const [x, y] of sp) {
  if (x < minX) minX = x;
  if (x > maxX) maxX = x;
  if (y < minY) minY = y;
  if (y > maxY) maxY = y;
}
const bw = maxX - minX, bh = maxY - minY;
const r3 = (n) => Math.round(n * 1000) / 1000;

// --- nonzero-winding point-in-polygon (excludes the ring hole) ---------------
const cross = (x0, y0, x1, y1, px, py) => (x1 - x0) * (py - y0) - (px - x0) * (y1 - y0);
function inside(px, py) {
  let wn = 0;
  for (const sp of subpaths) {
    for (let k = 0; k < sp.length - 1; k++) {
      const [x0, y0] = sp[k], [x1, y1] = sp[k + 1];
      if (y0 <= py) {
        if (y1 > py && cross(x0, y0, x1, y1, px, py) > 0) wn++;
      } else if (y1 <= py && cross(x0, y0, x1, y1, px, py) < 0) wn--;
    }
  }
  return wn !== 0;
}

// --- rasterize the icon at a given grid resolution ---------------------------
// Each cell carries position plus three colour/animation drivers:
//   v   = vertical gradient (0 at the top → 1 at the bottom),
//   ang = angle from the centroid [0,1] (for swirls / conic effects),
//   rad = distance from the centroid [0,1] (for rings / radial reveals).
// y = row keeps the icon upright: Vega-Lite ordinal y renders value 0 at the top.
function rasterize(cols) {
  const rows = Math.round(cols * (bh / bw));
  const lit = [];
  let preview = '';
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < cols; col++) {
      const sx = minX + ((col + 0.5) / cols) * bw;
      const sy = minY + ((row + 0.5) / rows) * bh;
      if (inside(sx, sy)) {
        lit.push({ col, row, sy });
        preview += '#';
      } else {
        preview += ' ';
      }
    }
    preview += '\n';
  }
  const cgx = lit.reduce((s, c) => s + c.col, 0) / lit.length;
  const cgy = lit.reduce((s, c) => s + c.row, 0) / lit.length;
  const maxRad = Math.max(...lit.map((c) => Math.hypot(c.col - cgx, c.row - cgy)));
  const cells = lit.map(({ col, row, sy }) => ({
    x: col,
    y: row,
    v: r3((sy - minY) / bh),
    ang: r3((Math.atan2(row - cgy, col - cgx) + Math.PI) / (2 * Math.PI)),
    rad: r3(Math.hypot(col - cgx, row - cgy) / maxRad),
  }));
  return { cells, cols, rows, preview };
}

// Three resolutions: a faithful high-res bitmap for "Classic", a lighter grid for
// the effect / animation panels (smaller payload, smoother animation), and an even
// lighter grid for the flag panels (big colour blocks read fine at low res).
const HI = rasterize(58);
const LO = rasterize(30);
const FLAG = rasterize(24);

// --- the icon outline (flattened path) for the wireframe variants -------------
// `gi` is a global draw order across all subpaths so the stroke can be revealed
// progressively; `part` keeps separate subpaths from being joined by `line`.
// The full flattening (FLAT segments/curve) is overkill for a visible stroke, so
// we downsample it here (rasterization above still uses the full polygon).
const OUTLINE_STEP = 5;
let giCounter = 0;
const OUTLINE = subpaths.flatMap((sp, part) => {
  const pts = sp.filter((_, seq) => seq % OUTLINE_STEP === 0 || seq === sp.length - 1);
  return pts.map(([x, y], seq) => ({ ox: r3(x), oy: r3(y), part, seq, gi: giCounter++ }));
});
const OX0 = r3(minX), OX1 = r3(maxX), OY0 = r3(minY), OY1 = r3(maxY);
const GI_MAX = giCounter - 1;

// ---------------------------------------------------------------------------
// Builder + Infinity-inline assembly (v2 schema, TabsLayout — same shape the
// Showcase dashboard uses, so every panel carries its own inline data).
// ---------------------------------------------------------------------------

// Hide axes with a non-null object (false flags): Grafana's scenes runtime strips
// null option values, so `axis: null` would not survive as a builder option.
const HIDE = { labels: false, ticks: false, domain: false, grid: false };
const ORANGE = ['#F15B2A', '#F8B723'];

// builder encoding that hides both axes
const AX = (channel, field) => ({ id: channel, channel, field, type: 'ordinal', title: ' ', axis: HIDE });
// a pure-builder model (typed encodings drive the chart)
const builder = (mark, encodings) => ({ mark, encodings, transforms: [], layers: [], params: [] });
// a custom-Vega-Lite model: the spec IS the chart (frame data is injected by the
// pipeline as `data.values`, so the override must NOT declare its own data).
const custom = (spec) => ({ mark: { type: 'rect' }, encodings: [], transforms: [], layers: [], params: [], specOverrideJson: JSON.stringify(spec) });

// A timer-driven Vega-Lite parameter: `name` steps 0→n-1 every `throttle` ms.
// (`on` is not in vega-lite's VariableParameter TS type but the compiler forwards
//  it verbatim to the Vega signal — see the header note + logo.test.ts.)
const timer = (name, n, throttle) => ({ name, value: 0, on: [{ events: { type: 'timer', throttle }, update: `(${name} + 1) % ${n}` }] });
// axis-less positional channels for quantitative (motion) specs
const qx = (field, domain) => ({ field, type: 'quantitative', axis: null, scale: { domain, nice: false } });
const qy = (field, domain) => ({ field, type: 'quantitative', axis: null, scale: { domain, nice: false, reverse: true } });
// axis-less ordinal channel (grid position)
const ord = (field) => ({ field, type: 'ordinal', axis: null });

// --- flag helpers (used by the waving flag / tricolore gloss in Football Fever) ---
// Normalize the grid position to [0,1] so a flag's geometry maps onto the icon's
// bounding box (the mark silhouette is then "cut out" of the flag).
const fxT = { calculate: `datum.x / ${FLAG.cols - 1}`, as: 'fx' };
const fyT = { calculate: `datum.y / ${FLAG.rows - 1}`, as: 'fy' };
// Map an integer region code (0..n-1, produced by a `calculate`) to the flag colours.
const flagColors = (colors) => ({ field: 'region', type: 'ordinal', scale: { domain: colors.map((_, i) => i), range: colors }, legend: null });

let pid = 0;
const elements = {};

// --- query reuse (file-size reduction) ---------------------------------------
// Each dataset (HI / LO / FLAG / OUTLINE) is inlined ONCE per tab on its first
// panel ("origin"); every other panel in that tab that needs the same dataset
// pulls it from the origin via Grafana's built-in **Dashboard datasource**
// (`-- Dashboard --`, group `datasource`, referencing the origin by `panelId`).
// This is the authoritative v2beta1 serialization (round-tripped through Grafana
// 13) and is verified to render in the Vizard panel. Reuse is kept WITHIN a tab
// so the origin's query has run by the time consumers read it (tabs lazy-load).
// `DATASETS` maps a rows array (by reference) to a stable dataset key.
const DATASETS = new Map([[HI.cells, 'HI'], [LO.cells, 'LO'], [FLAG.cells, 'FLAG'], [OUTLINE, 'OUT']]);

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

/** A Dashboard-datasource query reusing another panel's results (by panelId). */
function dashboardReuse(panelId) {
  return {
    kind: 'PanelQuery',
    spec: {
      refId: 'A',
      hidden: false,
      query: {
        kind: 'DataQuery',
        group: 'datasource',
        version: 'v0',
        datasource: { name: '-- Dashboard --' },
        spec: { panelId, withTransforms: false },
      },
    },
  };
}

/** Declare a Vizard logo variant (data is wired per-tab so it can be reused). */
function panel(title, rows, builderModel, opts = {}) {
  return { title, rows, key: DATASETS.get(rows), builderModel, opts };
}

/** Register a panel element with a specific query (inline or reuse). */
function registerPanel(id, descriptor, query) {
  const name = `panel-${id}`;
  elements[name] = {
    kind: 'Panel',
    spec: {
      id,
      title: descriptor.title,
      description: descriptor.opts.description ?? '',
      links: [],
      data: { kind: 'QueryGroup', spec: { queries: [query], transformations: [], queryOptions: {} } },
      vizConfig: {
        kind: 'VizConfig',
        group: 'yesoreyeram-vizard-panel',
        version: '',
        spec: {
          options: {
            editorMode: 'builder',
            renderer: 'canvas',
            tooltip: false,
            legend: false,
            theme: { colorScheme: descriptor.opts.color ?? 'palette-classic' },
            data: { source: 'auto' },
            builder: descriptor.builderModel,
          },
          fieldConfig: { defaults: {}, overrides: [] },
        },
      },
    },
  };
  return { name, w: descriptor.opts.w ?? 6, h: descriptor.opts.h ?? 9 };
}

function tab(title, descriptors) {
  const originByKey = {}; // dataset key -> origin panelId (within this tab)
  const placed = descriptors.map((d) => {
    pid++;
    const id = pid;
    let query;
    if (d.key !== undefined && originByKey[d.key] !== undefined) {
      query = dashboardReuse(originByKey[d.key]); // reuse the in-tab origin
    } else {
      query = infinityInline(d.rows); // first of its dataset in this tab → origin
      if (d.key !== undefined) {
        originByKey[d.key] = id;
      }
    }
    return registerPanel(id, d, query);
  });
  let x = 0, y = 0, rowH = 0;
  const items = placed.map((p) => {
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

// ===========================================================================
// GRAFANA TAB · Bitmap — the logo as a grid of marks, coloured from the data.
// ===========================================================================
const classic = builder({ type: 'rect' }, [AX('x', 'x'), AX('y', 'y'), { id: 'color', channel: 'color', field: 'v', type: 'quantitative', legend: null, scale: { range: ORANGE } }]);
const halftone = builder({ type: 'circle' }, [
  AX('x', 'x'), AX('y', 'y'),
  { id: 'size', channel: 'size', field: 'v', type: 'quantitative', legend: null, scale: { range: [110, 8] } },
  { id: 'color', channel: 'color', field: 'v', type: 'quantitative', legend: null, scale: { range: ORANGE } },
]);
const swirl = builder({ type: 'rect' }, [AX('x', 'x'), AX('y', 'y'), { id: 'color', channel: 'color', field: 'ang', type: 'quantitative', legend: null, scale: { scheme: 'rainbow' } }]);
const contour = custom({
  transform: [{ calculate: 'floor(datum.rad * 7)', as: 'band' }],
  mark: { type: 'rect' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'band', type: 'ordinal', scale: { scheme: 'oranges', reverse: true }, legend: null } },
});
const conic = custom({
  mark: { type: 'rect' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'ang', type: 'quantitative', scale: { scheme: 'sinebow', domain: [0, 1] }, legend: null } },
});
const ascii = custom({
  transform: [{ calculate: "['·',':','-','=','+','*','#','@'][floor(datum.v * 7.999)]", as: 'ch' }],
  mark: { type: 'text', baseline: 'middle', align: 'center', font: 'monospace', fontSize: 13, fontWeight: 'bold' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, text: { field: 'ch', type: 'nominal' }, color: { field: 'v', type: 'quantitative', scale: { range: ORANGE }, legend: null } },
});

// ===========================================================================
// GRAFANA TAB · Effects — layered / stylised renderings via the spec-override hatch.
// ===========================================================================
const glow = custom({
  layer: [
    { mark: { type: 'circle', color: '#F15B2A', opacity: 0.05, size: 900 }, encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null } } },
    { mark: { type: 'circle', color: '#FF7A1A', opacity: 0.35, size: 90 }, encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null } } },
    { mark: { type: 'circle', color: '#FFE6A8', size: 8 }, encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null } } },
  ],
});

const hx = (n) => Math.max(0, Math.min(255, Math.round(n))).toString(16).padStart(2, '0');
function blend(a, b, t) {
  const pa = [1, 3, 5].map((i) => parseInt(a.slice(i, i + 2), 16));
  const pb = [1, 3, 5].map((i) => parseInt(b.slice(i, i + 2), 16));
  return '#' + pa.map((c, i) => hx(c + (pb[i] - c) * t)).join('');
}
const DEPTH = 6;
const isoLayers = [];
for (let i = DEPTH; i >= 0; i--) {
  isoLayers.push({
    transform: [
      { calculate: `datum.x + ${(i * 0.5).toFixed(2)}`, as: 'xo' },
      { calculate: `datum.y - ${(i * 0.5).toFixed(2)}`, as: 'yo' },
    ],
    mark: { type: 'square', size: 26, color: i === 0 ? '#FFC24B' : blend('#5a1e0a', '#F15B2A', 1 - i / DEPTH) },
    encoding: { x: qx('xo'), y: { field: 'yo', type: 'quantitative', axis: null, scale: { nice: false, reverse: true } } },
  });
}
const iso = custom({ layer: isoLayers });

const duotone = custom({
  layer: [
    { transform: [{ calculate: 'datum.x + 0.8', as: 'sx' }, { calculate: 'datum.y + 0.8', as: 'sy' }], mark: { type: 'square', size: 34, color: '#2a1405', opacity: 0.55 }, encoding: { x: qx('sx'), y: qy('sy') } },
    { mark: { type: 'square', size: 34 }, encoding: { x: qx('x'), y: qy('y'), color: { field: 'v', type: 'quantitative', scale: { range: ORANGE }, legend: null } } },
  ],
});

const glass = custom({
  transform: [{ calculate: 'floor(datum.ang * 7) * 5 + floor(datum.rad * 4)', as: 'pane' }],
  mark: { type: 'square', size: 95, stroke: '#140a04', strokeWidth: 0.6 },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'pane', type: 'nominal', scale: { scheme: 'plasma' }, legend: null } },
});

const fire = custom({
  transform: [{ filter: 'datum.x % 2 === 0 && datum.y % 2 === 0' }],
  mark: { type: 'text', size: 16, baseline: 'middle', align: 'center' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, text: { value: '🔥' } },
});

const wireframe = custom({
  mark: { type: 'line', strokeWidth: 2.5, color: '#F15B2A', strokeCap: 'round', strokeJoin: 'round' },
  encoding: {
    x: qx('ox', [OX0, OX1]),
    y: qy('oy', [OY0, OY1]),
    order: { field: 'gi', type: 'quantitative' },
    detail: { field: 'part', type: 'nominal' },
  },
});

// ===========================================================================
// GRAFANA TAB · Animated — timer-driven parameters animate the same logo data.
// ===========================================================================
const N_REVEAL = 46;
const radialReveal = custom({
  params: [timer('anim', N_REVEAL, 55)],
  transform: [{ calculate: 'clamp((anim/40 - datum.rad) * 6, 0, 1)', as: 'o' }],
  mark: { type: 'rect' },
  encoding: {
    x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null },
    color: { field: 'v', type: 'quantitative', scale: { range: ORANGE }, legend: null },
    opacity: { field: 'o', type: 'quantitative', scale: { domain: [0, 1], range: [0, 1] }, legend: null },
  },
});

const N_SPIN = 60;
const hueSpin = custom({
  params: [timer('anim', N_SPIN, 45)],
  transform: [{ calculate: `(datum.ang + anim/${N_SPIN}) % 1`, as: 'hue' }],
  mark: { type: 'rect' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'hue', type: 'quantitative', scale: { scheme: 'sinebow', domain: [0, 1] }, legend: null } },
});

const N_PULSE = 60;
const pulse = custom({
  params: [timer('anim', N_PULSE, 40)],
  transform: [
    { calculate: `0.55 + 0.45 * sin(anim/${N_PULSE} * 2 * PI)`, as: 'p' },
    { calculate: 'datum.p * (1.15 - datum.rad)', as: 'sz' },
  ],
  mark: { type: 'circle' },
  encoding: {
    x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null },
    size: { field: 'sz', type: 'quantitative', scale: { domain: [0, 1.15], range: [0, 150] }, legend: null },
    color: { field: 'v', type: 'quantitative', scale: { range: ORANGE }, legend: null },
  },
});

const N_SHIMMER = 60;
const shimmer = custom({
  params: [timer('anim', N_SHIMMER, 45)],
  transform: [
    { calculate: `(datum.x / ${LO.cols}) - (anim/${N_SHIMMER})`, as: 'd' },
    { calculate: 'max(0, 1 - abs(datum.d) * 5)', as: 'sheen' },
  ],
  mark: { type: 'rect' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'sheen', type: 'quantitative', scale: { range: ['#7a3413', '#FFE9B0'], domain: [0, 1] }, legend: null } },
});

const N_ASM = 70;
const SPREAD = Math.round(LO.cols * 0.9);
const assembly = custom({
  params: [timer('anim', N_ASM, 40)],
  transform: [
    { calculate: `1 - abs(1 - 2 * (anim/${N_ASM}))`, as: 't' },
    { calculate: 'datum.ang * 2 * PI - PI', as: 'theta' },
    { calculate: `datum.x + cos(datum.theta) * datum.rad * ${SPREAD} * (1 - datum.t)`, as: 'px' },
    { calculate: `datum.y + sin(datum.theta) * datum.rad * ${SPREAD} * (1 - datum.t)`, as: 'py' },
  ],
  mark: { type: 'circle' },
  encoding: {
    x: qx('px', [-SPREAD, LO.cols + SPREAD]),
    y: qy('py', [-SPREAD, LO.rows + SPREAD]),
    color: { field: 'v', type: 'quantitative', scale: { range: ORANGE }, legend: null },
    opacity: { field: 't', type: 'quantitative', scale: { domain: [0, 1], range: [0.12, 1] }, legend: null },
  },
});

const drawOn = custom({
  params: [timer('anim', GI_MAX + 14, 16)],
  transform: [{ filter: 'datum.gi <= anim' }],
  mark: { type: 'line', strokeWidth: 2.5, color: '#F15B2A', strokeCap: 'round', strokeJoin: 'round' },
  encoding: {
    x: qx('ox', [OX0, OX1]),
    y: qy('oy', [OY0, OY1]),
    order: { field: 'gi', type: 'quantitative' },
    detail: { field: 'part', type: 'nominal' },
  },
});

const N_RAIN = 22;
const matrix = custom({
  params: [timer('anim', N_RAIN, 70)],
  transform: [
    { calculate: `((datum.y + anim) % ${N_RAIN}) / ${N_RAIN}`, as: 'fall' },
    { calculate: "['0','1'][(datum.x + datum.y) % 2]", as: 'bit' },
  ],
  mark: { type: 'text', font: 'monospace', fontSize: 11, fontWeight: 'bold', baseline: 'middle', align: 'center' },
  encoding: {
    x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null },
    text: { field: 'bit', type: 'nominal' },
    color: { field: 'fall', type: 'quantitative', scale: { range: ['#063b0d', '#27d63f', '#d6ffd9'], domain: [0, 0.5, 1] }, legend: null },
  },
});

// ===========================================================================
// FOOTBALL FEVER TAB — football-themed renderings of the same mark. ⚽🏆
// ===========================================================================
const footballs = custom({
  transform: [{ filter: 'datum.x % 2 === 0 && datum.y % 2 === 0' }],
  mark: { type: 'text', size: 16, baseline: 'middle', align: 'center' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, text: { value: '⚽' } },
});
const ball = custom({
  transform: [{ calculate: '(floor(datum.ang * 6) + floor(datum.rad * 3)) % 2', as: 'panel' }],
  mark: { type: 'square', size: 95, stroke: '#cfcfcf', strokeWidth: 0.4 },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'panel', type: 'nominal', scale: { domain: [0, 1], range: ['#141414', '#fbfbfb'] }, legend: null } },
});
const pitch = custom({
  transform: [{ calculate: 'floor(datum.x / 3) % 2', as: 'stripe' }],
  mark: { type: 'rect' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'stripe', type: 'nominal', scale: { domain: [0, 1], range: ['#2e7d32', '#43a047'] }, legend: null } },
});
const N_WAVE = 60;
const mexicanWave = custom({
  params: [timer('anim', N_WAVE, 45)],
  transform: [
    { calculate: `(datum.x / ${LO.cols}) - (anim/${N_WAVE})`, as: 'd' },
    { calculate: 'min(abs(datum.d), abs(datum.d + 1))', as: 'dist' },
    { calculate: 'max(0, 1 - datum.dist * 4)', as: 'lift' },
  ],
  mark: { type: 'circle' },
  encoding: {
    x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null },
    size: { field: 'lift', type: 'quantitative', scale: { domain: [0, 1], range: [18, 170] }, legend: null },
    color: { field: 'lift', type: 'quantitative', scale: { range: ['#1565c0', '#F15B2A', '#FFEB3B'], domain: [0, 0.5, 1] }, legend: null },
  },
});
const N_FLAGWAVE = 60;
const flagWave = custom({
  params: [timer('anim', N_FLAGWAVE, 40)],
  transform: [{ calculate: `datum.y + sin(datum.x/3 - anim/${N_FLAGWAVE} * 2 * PI) * 1.6`, as: 'wy' }],
  mark: { type: 'square', size: 70 },
  encoding: { x: qx('x', [0, LO.cols - 1]), y: qy('wy', [-3, LO.rows + 2]), color: { field: 'v', type: 'quantitative', scale: { range: ORANGE }, legend: null } },
});
const N_TROPHY = 60;
const trophy = custom({
  params: [timer('anim', N_TROPHY, 45)],
  transform: [
    { calculate: `abs(((datum.ang - anim/${N_TROPHY} + 1.5) % 1) - 0.5)`, as: 'da' },
    { calculate: 'max(0, 1 - datum.da * 4)', as: 'glint' },
    { calculate: 'min(1, (1 - datum.rad) * 0.5 + datum.glint)', as: 'shine' },
  ],
  mark: { type: 'rect' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, color: { field: 'shine', type: 'quantitative', scale: { range: ['#8a6d1a', '#FFD24A', '#FFFDE7'], domain: [0, 0.6, 1] }, legend: null } },
});

// ===========================================================================
// FOOTBALL FEVER extras — a waving national flag and a tricolore gloss sweep.
// ===========================================================================
const N_BRAZIL = 60;
const brazilWaving = custom({
  params: [timer('anim', N_BRAZIL, 40)],
  transform: [
    fxT, fyT,
    { calculate: 'datum.rad < 0.30 ? 2 : (((abs(datum.fx - 0.5) / 0.46) + (abs(datum.fy - 0.5) / 0.46)) <= 1 ? 1 : 0)', as: 'region' },
    { calculate: `datum.y + sin(datum.x / 3 - anim/${N_BRAZIL} * 2 * PI) * 1.6`, as: 'wy' },
  ],
  mark: { type: 'square', size: 80 },
  encoding: { x: qx('x', [0, FLAG.cols - 1]), y: qy('wy', [-3, FLAG.rows + 2]), color: flagColors(['#009C3B', '#FFDF00', '#002776']) },
});
const N_GLOSS = 60;
const tricoloreGloss = custom({
  params: [timer('anim', N_GLOSS, 45)],
  transform: [fxT],
  layer: [
    { transform: [{ calculate: 'datum.fx < 0.34 ? 0 : (datum.fx < 0.67 ? 1 : 2)', as: 'region' }], mark: { type: 'rect' }, encoding: { x: ord('x'), y: ord('y'), color: flagColors(['#0055A4', '#FFFFFF', '#EF4135']) } },
    { transform: [{ calculate: `(datum.x / ${FLAG.cols}) - (anim/${N_GLOSS})`, as: 'd' }, { calculate: 'max(0, 1 - abs(datum.d) * 6)', as: 'sheen' }], mark: { type: 'rect', color: '#ffffff' }, encoding: { x: ord('x'), y: ord('y'), opacity: { field: 'sheen', type: 'quantitative', scale: { domain: [0, 1], range: [0, 0.7] }, legend: null } } },
  ],
});

// ---------------------------------------------------------------------------
const tabs = [
  tab('Grafana', [
    // Bitmap
    panel('Classic', HI.cells, classic, { description: 'The faithful rect bitmap with the logo’s real orange→amber ramp.' }),
    panel('Halftone', LO.cells, halftone, { description: 'Circles sized by the gradient — a newspaper-dot look.' }),
    panel('Rainbow swirl', LO.cells, swirl, { color: 'rainbow', description: 'Hue mapped to the angle around the centroid.' }),
    panel('Contour rings', LO.cells, contour, { description: 'Quantized radius → concentric topographic bands.' }),
    panel('Conic gradient', LO.cells, conic, { description: 'A smooth cyclic sinebow sweep around the centre.' }),
    panel('ASCII shading', LO.cells, ascii, { description: 'Each lit cell is a character ramped by brightness.' }),
    // Effects
    panel('Neon glow', LO.cells, glow, { description: 'Translucent halo circles under a bright core.' }),
    panel('Isometric 3D', LO.cells, iso, { description: 'The bitmap extruded by stacking shaded copies.' }),
    panel('Duotone shadow', LO.cells, duotone, { description: 'An offset dark copy under the bright bitmap.' }),
    panel('Stained glass', LO.cells, glass, { description: 'Coloured panes with dark leading.' }),
    panel('Fire mosaic 🔥', LO.cells, fire, { description: 'A flame mark drawn out of flame marks.' }),
    panel('Wireframe', OUTLINE, wireframe, { description: 'The icon outline traced as vector strokes.' }),
    // Animated
    panel('Radial reveal', LO.cells, radialReveal, { description: 'The logo draws itself from the centre outward, then loops. Timer-driven param + a radial opacity ramp.' }),
    panel('Hue spin', LO.cells, hueSpin, { description: 'A sinebow conic gradient rotates continuously. color = (angle + anim/N) % 1.' }),
    panel('Breathing pulse', LO.cells, pulse, { description: 'Dot size swells and shrinks on a sine wave.' }),
    panel('Shimmer sweep', LO.cells, shimmer, { description: 'A metallic highlight band sweeps across the mark.' }),
    panel('Particle assembly', LO.cells, assembly, { description: 'Cells fly out from the centre and re-converge (ping-pong). Positions interpolated by the frame signal.' }),
    panel('Wireframe draw-on', OUTLINE, drawOn, { description: 'The outline stroke draws progressively along its path (filter by draw order ≤ anim).' }),
    panel('Matrix rain', LO.cells, matrix, { description: 'Falling 0/1 columns light up the icon in phosphor green.' }),
  ]),
  tab('Football Fever', [
    panel('Football mosaic ⚽', LO.cells, footballs, { description: 'The mark drawn out of soccer-ball emojis.' }),
    panel('Soccer ball', LO.cells, ball, { description: 'Black & white panels — a classic football.' }),
    panel('Pitch stripes', LO.cells, pitch, { description: 'The mown-grass stripes of a football field.' }),
    panel('Mexican wave', LO.cells, mexicanWave, { description: 'La Ola — a crowd wave of fans sweeps across, and loops.' }),
    panel('Flag wave', LO.cells, flagWave, { description: 'The mark ripples like a national flag in the wind.' }),
    panel('Trophy shine 🏆', LO.cells, trophy, { description: 'A polished-gold mark with a glint orbiting around it.' }),
    panel('Brazil waving', FLAG.cells, brazilWaving, { description: 'The Brazil flag rippling in the wind (timer-driven).' }),
    panel('Tricolore gloss', FLAG.cells, tricoloreGloss, { description: 'A white sheen sweeps across the French flag (timer-driven).' }),
  ]),
];

const dashboard = {
  apiVersion: 'dashboard.grafana.app/v2beta1',
  kind: 'Dashboard',
  metadata: { name: 'vizard-logo' },
  spec: {
    title: 'Vizard — Grafana logo',
    description: 'The Grafana mark rendered many creative ways with the Vizard builder — static bitmaps, layered effects and timer-driven animations in the "Grafana" tab, plus football-themed variants (and a waving flag / tricolore gloss) in "Football Fever".',
    tags: ['vizard', 'vega-lite', 'logo'],
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
console.log(HI.preview);
console.log(`hi ${HI.cols}x${HI.rows} (${HI.cells.length} cells) · lo ${LO.cols}x${LO.rows} (${LO.cells.length} cells) · flag ${FLAG.cols}x${FLAG.rows} (${FLAG.cells.length} cells) · outline ${OUTLINE.length} pts`);
console.log(`${Object.keys(elements).length} variants across ${tabs.length} tabs · wrote ${OUT}`);
