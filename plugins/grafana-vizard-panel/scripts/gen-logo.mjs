// Generate the "Grafana logo" demo dashboard: a single Vizard that renders the
// Grafana brand mark (the icon, ignoring the "Grafana" text) as a Vega-Lite rect
// "bitmap" — NO custom Vega-Lite JSON. The logo is rasterized from the official
// SVG (scripts/grafana-logo.svg) into a grid of lit cells, served via the Infinity
// datasource's INLINE source, and drawn with the Vizard builder (rect mark with
// x / y / color encodings). Colour follows the logo's yellow→orange gradient.
//
// Run: node scripts/gen-logo.mjs  (writes provisioning/dashboards/logo.json)

import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const HERE = dirname(fileURLToPath(import.meta.url));
const SVG = readFileSync(join(HERE, 'grafana-logo.svg'), 'utf8');
const OUT = join(HERE, '..', 'provisioning', 'dashboards', 'logo.json');

const COLS = 58; // grid resolution (height derived from the icon aspect ratio)
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

// bounding box
let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
for (const sp of subpaths) for (const [x, y] of sp) {
  if (x < minX) minX = x;
  if (x > maxX) maxX = x;
  if (y < minY) minY = y;
  if (y > maxY) maxY = y;
}
const bw = maxX - minX, bh = maxY - minY;
const ROWS = Math.round(COLS * (bh / bw));

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

// --- rasterize (pass 1: lit cells; pass 2: add gradient / angle / radius) -----
const lit = [];
let preview = '';
for (let row = 0; row < ROWS; row++) {
  for (let col = 0; col < COLS; col++) {
    const sx = minX + ((col + 0.5) / COLS) * bw;
    const sy = minY + ((row + 0.5) / ROWS) * bh;
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
const r3 = (n) => Math.round(n * 1000) / 1000;

// Each cell carries position plus three colour drivers:
//   v   = vertical gradient (0 at the top → 1 at the bottom),
//   ang = angle from the centroid [0,1] (for swirls),
//   rad = distance from the centroid [0,1] (for rings / radial fills).
// y = row keeps the icon upright: Vega-Lite ordinal y renders value 0 at the top.
const cells = lit.map(({ col, row, sy }) => ({
  x: col,
  y: row,
  v: r3((sy - minY) / bh),
  ang: r3((Math.atan2(row - cgy, col - cgx) + Math.PI) / (2 * Math.PI)),
  rad: r3(Math.hypot(col - cgx, row - cgy) / maxRad),
}));

// --- the icon outline (flattened path) for the wireframe variant ---------------
const DATA_OUTLINE = JSON.stringify(
  subpaths.flatMap((sp, part) => sp.map(([x, y], seq) => ({ ox: r3(x), oy: r3(y), part, seq })))
);

// --- dashboard: creative variants of the same logo (classic schema) -----------
const DS = { type: 'yesoreyeram-infinity-datasource', uid: 'vizard-infinity' };
const DASH = { type: 'datasource', uid: '-- Dashboard --' };
// Hide axes with a non-null object (false flags): Grafana's scenes runtime strips
// null option values, so `axis: null` would not survive.
const HIDE = { labels: false, ticks: false, domain: false, grid: false };
const DATA = JSON.stringify(cells);
const W = 6, H = 12, PER_ROW = 4;
let pid = 0;

const hx = (n) => Math.max(0, Math.min(255, Math.round(n))).toString(16).padStart(2, '0');
function blend(a, b, t) {
  const pa = [1, 3, 5].map((i) => parseInt(a.slice(i, i + 2), 16));
  const pb = [1, 3, 5].map((i) => parseInt(b.slice(i, i + 2), 16));
  return '#' + pa.map((c, i) => hx(c + (pb[i] - c) * t)).join('');
}

// builder encodings that hide both axes
const AX = (channel, field) => ({ id: channel, channel, field, type: 'ordinal', title: ' ', axis: HIDE });
// a pure-builder model
const builder = (mark, encodings) => ({ mark, encodings, transforms: [], layers: [], params: [] });
// a custom-Vega-Lite model: the spec IS the chart (data is injected from the frame)
const custom = (spec) => ({ mark: { type: 'rect' }, encodings: [], transforms: [], layers: [], params: [], specOverrideJson: JSON.stringify(spec) });

/** Register a logo variant. dataset 'cells' reuses the first panel's frame via the
 *  "-- Dashboard --" datasource; 'outline' carries its own inline query. */
function panel(title, builderModel, { dataset = 'cells', theme = 'palette-classic' } = {}) {
  pid++;
  const inline = (data) => ({ refId: 'A', datasource: DS, type: 'json', format: 'table', parser: 'backend', root_selector: '', columns: [], filters: [], source: 'inline', data });
  let targets, ds;
  if (dataset === 'outline') {
    targets = [inline(DATA_OUTLINE)];
    ds = DS;
  } else if (pid === 1) {
    targets = [inline(DATA)];
    ds = DS;
  } else {
    targets = [{ refId: 'A', datasource: DASH, panelId: 1 }];
    ds = DASH;
  }
  return {
    id: pid,
    title,
    type: 'yesoreyeram-vizard-panel',
    datasource: ds,
    gridPos: { h: H, w: W, x: ((pid - 1) % PER_ROW) * W, y: Math.floor((pid - 1) / PER_ROW) * H },
    targets,
    options: { editorMode: 'builder', renderer: 'canvas', tooltip: false, legend: false, theme: { colorScheme: theme }, data: { source: 'auto' }, builder: builderModel },
  };
}

// 1. Classic — the faithful rect bitmap with the logo's real orange→amber ramp.
const classic = builder({ type: 'rect' }, [AX('x', 'x'), AX('y', 'y'), { id: 'color', channel: 'color', field: 'v', type: 'quantitative', legend: null, scale: { range: ['#F15B2A', '#F8B723'] } }]);

// 2. Halftone — circles sized by the gradient (newspaper-dot look).
const halftone = builder({ type: 'circle' }, [
  AX('x', 'x'), AX('y', 'y'),
  { id: 'size', channel: 'size', field: 'v', type: 'quantitative', legend: null, scale: { range: [70, 6] } },
  { id: 'color', channel: 'color', field: 'v', type: 'quantitative', legend: null, scale: { range: ['#F15B2A', '#F8B723'] } },
]);

// 3. Rainbow swirl — hue by angle around the centroid.
const swirl = builder({ type: 'rect' }, [AX('x', 'x'), AX('y', 'y'), { id: 'color', channel: 'color', field: 'ang', type: 'quantitative', legend: null, scale: { scheme: 'rainbow' } }]);

// 4. Fire mosaic — every 3rd cell drawn as a 🔥 (a flame made of flames).
const fire = custom({
  transform: [{ filter: 'datum.x % 3 === 0 && datum.y % 3 === 0' }],
  mark: { type: 'text', size: 16, baseline: 'middle', align: 'center' },
  encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null }, text: { value: '🔥' } },
});

// 5. Neon glow — translucent halo circles under a bright core.
const glow = custom({
  layer: [
    { mark: { type: 'circle', color: '#F15B2A', opacity: 0.05, size: 500 }, encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null } } },
    { mark: { type: 'circle', color: '#FF7A1A', opacity: 0.35, size: 55 }, encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null } } },
    { mark: { type: 'circle', color: '#FFE6A8', size: 5 }, encoding: { x: { field: 'x', type: 'ordinal', axis: null }, y: { field: 'y', type: 'ordinal', axis: null } } },
  ],
});

// 6. Isometric 3D — the bitmap extruded by stacking offset, shaded copies.
const isoLayers = [];
const DEPTH = 6;
for (let i = DEPTH; i >= 0; i--) {
  isoLayers.push({
    transform: [
      { calculate: `datum.x + ${(i * 0.5).toFixed(2)}`, as: 'xo' },
      { calculate: `datum.y - ${(i * 0.5).toFixed(2)}`, as: 'yo' },
    ],
    mark: { type: 'square', size: 16, color: i === 0 ? '#FFC24B' : blend('#5a1e0a', '#F15B2A', 1 - i / DEPTH) },
    encoding: {
      x: { field: 'xo', type: 'quantitative', axis: null, scale: { nice: false } },
      y: { field: 'yo', type: 'quantitative', axis: null, scale: { nice: false, reverse: true } },
    },
  });
}
const iso = custom({ layer: isoLayers });

// 7. Particle burst — cells flung outward from the centroid, fading with distance.
const burst = custom({
  transform: [
    { calculate: 'datum.ang * 2 * PI - PI', as: 'theta' },
    { calculate: 'datum.x + cos(datum.theta) * datum.rad * 12', as: 'px' },
    { calculate: 'datum.y + sin(datum.theta) * datum.rad * 12', as: 'py' },
  ],
  mark: { type: 'circle', filled: true },
  encoding: {
    x: { field: 'px', type: 'quantitative', axis: null, scale: { nice: false } },
    y: { field: 'py', type: 'quantitative', axis: null, scale: { nice: false, reverse: true } },
    size: { field: 'rad', type: 'quantitative', legend: null, scale: { range: [45, 3] } },
    color: { field: 'rad', type: 'quantitative', legend: null, scale: { range: ['#FFE08A', '#F15B2A'] } },
    opacity: { field: 'rad', type: 'quantitative', legend: null, scale: { range: [1, 0.15] } },
  },
});

// 8. Wireframe — the actual icon outline traced as glowing vector strokes.
const wireframe = custom({
  mark: { type: 'line', strokeWidth: 2.5, color: '#F15B2A', strokeCap: 'round', strokeJoin: 'round' },
  encoding: {
    x: { field: 'ox', type: 'quantitative', axis: null, scale: { nice: false } },
    y: { field: 'oy', type: 'quantitative', axis: null, scale: { nice: false, reverse: true } },
    order: { field: 'seq', type: 'quantitative' },
    detail: { field: 'part', type: 'nominal' },
  },
});

const panels = [
  panel('Classic', classic),
  panel('Halftone', halftone),
  panel('Rainbow swirl', swirl),
  panel('Fire mosaic 🔥', fire),
  panel('Neon glow', glow),
  panel('Isometric 3D', iso),
  panel('Particle burst', burst),
  panel('Wireframe', wireframe, { dataset: 'outline' }),
];

const dashboard = {
  annotations: { list: [] },
  editable: true,
  graphTooltip: 0,
  links: [],
  panels,
  schemaVersion: 39,
  tags: ['vizard', 'vega-lite', 'logo'],
  templating: { list: [] },
  time: { from: 'now-6h', to: 'now' },
  timepicker: {},
  timezone: 'browser',
  title: 'Vizard — Grafana logo',
  uid: 'vizard-logo',
  version: 1,
  weekStart: '',
};

writeFileSync(OUT, JSON.stringify(dashboard, null, 2) + '\n');
console.log(preview);
console.log(`grid ${COLS}x${ROWS} · ${cells.length} lit cells · ${panels.length} variants · wrote ${OUT}`);
