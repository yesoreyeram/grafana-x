import { BuilderModel, defaultMark } from '../types';
import { isPlainObject } from './merge';
import { specToBuilder } from './specToBuilder';

/**
 * Builds the Infinity-backed demo gallery dashboards from the Vega-Lite example
 * corpus. Every included panel is 100% builder-native (no spec-override JSON):
 * each example is converted to a `BuilderModel` via `specToBuilder`; examples
 * that can't be represented natively (multi-view facet/concat/repeat, geo) are
 * excluded from the demo (they remain covered by the compile/parse tests).
 *
 * Data is fetched at render time via the Infinity datasource (URL/inline) from
 * the Vega data repository, so datasets are not hardcoded into the dashboards.
 */

type Json = Record<string, unknown>;

const DATA_BASE = 'https://raw.githubusercontent.com/vega/vega/main/docs/data/';
const DS = { type: 'yesoreyeram-infinity-datasource', uid: 'vizard-infinity' };
const ROW_SIZE = 12;
const COLS = 2;
const PW = 24 / COLS;
const PH = 9;

// Datasets / examples the Infinity backend parser can't handle (invalid/mixed).
const DENY_FILES = new Set(['movies.json']);
const DENY_EXAMPLES = new Set(['boxplot_1D_invalid']);

type DataRef = { mode: 'url'; url: string; type: string } | { mode: 'inline'; data: string; type: 'json' };

function infinityType(file: string): string {
  if (file.endsWith('.csv')) {
    return 'csv';
  }
  if (file.endsWith('.tsv')) {
    return 'tsv';
  }
  return 'json';
}

function resolveData(spec: Json): DataRef | null {
  const d = spec.data;
  if (isPlainObject(d) && typeof d.url === 'string' && d.url.startsWith('data/')) {
    const file = d.url.slice('data/'.length);
    if (DENY_FILES.has(file)) {
      return null;
    }
    return { mode: 'url', url: DATA_BASE + file, type: infinityType(file) };
  }
  if (isPlainObject(d) && Array.isArray(d.values)) {
    return { mode: 'inline', data: JSON.stringify(d.values), type: 'json' };
  }
  return null;
}

function category(spec: Json): string {
  if (spec.layer) {
    return 'layered';
  }
  const json = JSON.stringify(spec);
  if (json.includes('"params"') && (json.includes('"select"') || json.includes('"selection"'))) {
    return 'interactive';
  }
  if (json.includes('"boxplot"') || json.includes('"errorbar"') || json.includes('"errorband"')) {
    return 'composite';
  }
  return 'single';
}

function infinityTarget(d: DataRef): Json {
  const base: Json = {
    refId: 'A',
    datasource: DS,
    type: d.type,
    format: 'table',
    parser: 'backend',
    root_selector: '',
    columns: [],
    filters: [],
  };
  if (d.mode === 'url') {
    return { ...base, source: 'url', url: d.url, url_options: { method: 'GET', data: '' } };
  }
  return { ...base, source: 'inline', data: d.data };
}

/** Remove example color scales so panels inherit the Grafana theme palette. */
function stripColorScales(node: unknown): void {
  if (Array.isArray(node)) {
    node.forEach(stripColorScales);
    return;
  }
  if (!isPlainObject(node)) {
    return;
  }
  if (isPlainObject(node.encoding)) {
    for (const ch of ['color', 'fill', 'stroke']) {
      const def = (node.encoding as Json)[ch];
      const defs = Array.isArray(def) ? def : [def];
      for (const dd of defs) {
        if (isPlainObject(dd) && isPlainObject(dd.scale)) {
          delete (dd.scale as Json).scheme;
          delete (dd.scale as Json).range;
          if (Object.keys(dd.scale as Json).length === 0) {
            delete (dd as Json).scale;
          }
        }
      }
    }
  }
  for (const key of Object.keys(node)) {
    stripColorScales(node[key]);
  }
}

function panel(id: number, title: string, builder: BuilderModel, data: DataRef, x: number, y: number): Json {
  return {
    id,
    title,
    type: 'yesoreyeram-vizard-panel',
    datasource: DS,
    gridPos: { h: PH, w: PW, x, y },
    targets: [infinityTarget(data)],
    options: {
      editorMode: 'builder',
      renderer: 'canvas',
      tooltip: true,
      legend: true,
      theme: { colorScheme: 'palette-classic' },
      data: { source: 'auto' },
      builder,
    },
  };
}

interface Item {
  name: string;
  builder: BuilderModel;
  data: DataRef;
}

function buildDashboard(title: string, uid: string, items: Item[]): Json {
  let id = 1;
  const panels: Json[] = [];
  for (let r = 0; r < items.length; r += ROW_SIZE) {
    const chunk = items.slice(r, r + ROW_SIZE);
    const rowY = r / ROW_SIZE;
    const children = chunk.map((it, i) =>
      panel(id++, it.name, it.builder, it.data, (i % COLS) * PW, rowY + 1 + Math.floor(i / COLS) * PH)
    );
    panels.push({
      id: id++,
      type: 'row',
      title: `${chunk[0].name} … ${chunk[chunk.length - 1].name} (${chunk.length})`,
      collapsed: true,
      gridPos: { h: 1, w: 24, x: 0, y: rowY },
      panels: children,
    });
  }
  return {
    annotations: { list: [] },
    editable: true,
    graphTooltip: 0,
    panels,
    schemaVersion: 39,
    tags: ['vizard', 'vega-lite', 'gallery'],
    templating: { list: [] },
    time: { from: 'now-6h', to: 'now' },
    timezone: 'browser',
    title,
    uid,
    version: 1,
  };
}

export interface GalleryStats {
  total: number;
  skipped: number;
  perCategory: Record<string, number>;
}

const META: Array<[string, string, string]> = [
  ['single', 'Vizard — Single-view gallery', 'vizard-gallery-single'],
  ['composite', 'Vizard — Composite marks gallery', 'vizard-gallery-composite'],
  ['layered', 'Vizard — Layered gallery', 'vizard-gallery-layered'],
  ['interactive', 'Vizard — Interactive gallery', 'vizard-gallery-interactive'],
];

export function buildGalleries(examples: Record<string, unknown>): {
  files: Record<string, Json>;
  stats: GalleryStats;
} {
  const buckets: Record<string, Item[]> = { single: [], composite: [], layered: [], interactive: [] };
  let skipped = 0;

  for (const name of Object.keys(examples).sort()) {
    const original = examples[name];
    if (DENY_EXAMPLES.has(name) || !isPlainObject(original)) {
      skipped++;
      continue;
    }
    const data = resolveData(original);
    if (!data) {
      skipped++;
      continue;
    }
    // Clone, strip color scales (inherit Grafana palette), then convert.
    const cleaned = JSON.parse(JSON.stringify(original)) as Json;
    stripColorScales(cleaned);
    const { builder, ok } = specToBuilder(cleaned);
    if (!ok) {
      skipped++; // multi-view / geo / complex — not builder-native
      continue;
    }
    const normalized: BuilderModel = {
      ...builder,
      mark: builder.mark ?? { ...defaultMark },
      encodings: builder.encodings ?? [],
      transforms: builder.transforms ?? [],
    };
    const cat = category(cleaned);
    const bucket = buckets[cat] ?? buckets.single;
    bucket.push({ name, builder: normalized, data });
  }

  const files: Record<string, Json> = {};
  const perCategory: Record<string, number> = {};
  let total = 0;
  for (const [key, title, uid] of META) {
    const items = buckets[key];
    perCategory[key] = items.length;
    total += items.length;
    files[`gallery-${key}.json`] = buildDashboard(title, uid, items);
  }

  return { files, stats: { total, skipped, perCategory } };
}
