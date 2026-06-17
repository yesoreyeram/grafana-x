#!/usr/bin/env node
/**
 * Idempotent NocoDB seeder for local development.
 *
 * On `docker compose up` this:
 *   1. waits for NocoDB to be ready,
 *   2. ensures an admin user exists (the first signup becomes super admin),
 *   3. ensures a "Sample" base with several tables covering different data
 *      shapes (relational rows, time series, logs, numeric metrics, sales),
 *   4. mints an API token,
 *   5. writes a Grafana datasource provisioning file (with the token + base id)
 *      into the shared provisioning volume so Grafana picks it up on startup.
 *
 * Re-running is safe: existing base/tables/rows are detected and reused.
 */

import { writeFile, mkdir } from 'node:fs/promises';
import { dirname } from 'node:path';

const NOCODB_URL = process.env.NOCODB_URL ?? 'http://nocodb:8080';
const ADMIN_EMAIL = process.env.NOCODB_ADMIN_EMAIL ?? 'admin@example.com';
const ADMIN_PASSWORD = process.env.NOCODB_ADMIN_PASSWORD ?? 'Password123.';
const BASE_TITLE = process.env.NOCODB_BASE_TITLE ?? 'Sample';
const DS_OUTPUT = process.env.DS_OUTPUT ?? '/provisioning/datasources/nocodb.generated.yaml';
const GRAFANA_INTERNAL_NOCODB_URL = process.env.GRAFANA_INTERNAL_NOCODB_URL ?? 'http://nocodb:8080';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function req(path, { method = 'GET', headers = {}, body } = {}) {
  const res = await fetch(`${NOCODB_URL}${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', ...headers },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json;
  try {
    json = text ? JSON.parse(text) : undefined;
  } catch {
    json = undefined;
  }
  return { status: res.status, ok: res.ok, json, text };
}

async function waitForNocoDB() {
  process.stdout.write('Waiting for NocoDB');
  for (let i = 0; i < 120; i++) {
    try {
      const res = await fetch(`${NOCODB_URL}/api/v1/health`).catch(() => null);
      if (res && res.ok) {
        console.log(' ready.');
        return;
      }
    } catch {
      // ignore
    }
    process.stdout.write('.');
    await sleep(2000);
  }
  throw new Error('NocoDB did not become ready in time');
}

async function getAuthToken() {
  // Try sign-in first; if the user does not exist, sign up (first user => super admin).
  let res = await req('/api/v1/auth/user/signin', {
    method: 'POST',
    body: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  });
  if (res.ok && res.json?.token) {
    console.log('Signed in as existing admin.');
    return res.json.token;
  }
  res = await req('/api/v1/auth/user/signup', {
    method: 'POST',
    body: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  });
  if (res.ok && res.json?.token) {
    console.log('Created admin user.');
    return res.json.token;
  }
  // Sign up may fail if the user already exists but the password differs; retry sign-in.
  res = await req('/api/v1/auth/user/signin', {
    method: 'POST',
    body: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  });
  if (res.ok && res.json?.token) {
    return res.json.token;
  }
  throw new Error(`Unable to authenticate with NocoDB: ${res.status} ${res.text}`);
}

async function findOrCreateBase(auth) {
  const headers = { 'xc-auth': auth };
  const list = await req('/api/v2/meta/bases', { headers });
  const existing = (list.json?.list ?? []).find((b) => b.title === BASE_TITLE);
  if (existing) {
    console.log(`Reusing base "${BASE_TITLE}" (${existing.id}).`);
    return existing.id;
  }
  const created = await req('/api/v2/meta/bases', {
    method: 'POST',
    headers,
    body: { title: BASE_TITLE },
  });
  if (!created.ok || !created.json?.id) {
    throw new Error(`Failed to create base: ${created.status} ${created.text}`);
  }
  console.log(`Created base "${BASE_TITLE}" (${created.json.id}).`);
  return created.json.id;
}

async function findOrCreateTable(auth, baseId, spec) {
  const headers = { 'xc-auth': auth };
  const list = await req(`/api/v2/meta/bases/${baseId}/tables`, { headers });
  const existing = (list.json?.list ?? []).find((t) => t.title === spec.title);
  if (existing) {
    console.log(`Reusing table "${spec.title}" (${existing.id}).`);
    return { id: existing.id, created: false };
  }
  const created = await req(`/api/v2/meta/bases/${baseId}/tables`, {
    method: 'POST',
    headers,
    body: { title: spec.title, columns: spec.columns },
  });
  if (!created.ok || !created.json?.id) {
    throw new Error(`Failed to create table "${spec.title}": ${created.status} ${created.text}`);
  }
  console.log(`Created table "${spec.title}" (${created.json.id}).`);
  return { id: created.json.id, created: true };
}

async function ensureRows(token, tableId, spec) {
  const headers = { 'xc-token': token };
  const existing = await req(`/api/v2/tables/${tableId}/records?limit=1`, { headers });
  const total = existing.json?.pageInfo?.totalRows ?? (existing.json?.list?.length ?? 0);
  if (total > 0) {
    console.log(`Table "${spec.title}" already has ${total} row(s); skipping insert.`);
    return;
  }
  const rows = spec.rows();
  // NocoDB caps bulk insert payloads; chunk to stay well under the limit.
  const chunkSize = 100;
  for (let i = 0; i < rows.length; i += chunkSize) {
    const chunk = rows.slice(i, i + chunkSize);
    const inserted = await req(`/api/v2/tables/${tableId}/records`, {
      method: 'POST',
      headers,
      body: chunk,
    });
    if (!inserted.ok) {
      throw new Error(`Failed to insert rows into "${spec.title}": ${inserted.status} ${inserted.text}`);
    }
  }
  console.log(`Inserted ${rows.length} rows into "${spec.title}".`);
}

// --------------------------------------------------------------------------
// Sample data generators
// --------------------------------------------------------------------------

const sel = (uidt, options) => ({ uidt, colOptions: { options: options.map((title) => ({ title })) } });

// Deterministic pseudo-random so seeds are stable across runs.
function makeRng(seedValue) {
  let s = seedValue >>> 0;
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0;
    return s / 0xffffffff;
  };
}

const pad = (n) => String(n).padStart(2, '0');
function isoMinute(d) {
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(
    d.getUTCMinutes()
  )}:${pad(d.getUTCSeconds())}`;
}
const dateOnly = (d) => `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;

const TABLES = [
  // 1) Relational / mixed-type rows.
  {
    title: 'Customers',
    columns: [
      { title: 'Name', uidt: 'SingleLineText' },
      { title: 'Email', uidt: 'Email' },
      { title: 'Age', uidt: 'Number' },
      { title: 'Active', uidt: 'Checkbox' },
      { title: 'Plan', ...sel('SingleSelect', ['free', 'pro', 'enterprise']) },
      { title: 'Country', ...sel('SingleSelect', ['US', 'UK', 'DE', 'IN', 'BR']) },
      { title: 'MRR', uidt: 'Currency' },
      { title: 'SignedUp', uidt: 'Date' },
    ],
    rows() {
      const rng = makeRng(11);
      const plans = ['free', 'pro', 'enterprise'];
      const countries = ['US', 'UK', 'DE', 'IN', 'BR'];
      const firstNames = ['Alice', 'Bob', 'Charlie', 'Dana', 'Evan', 'Fiona', 'George', 'Hana', 'Ivan', 'Julia'];
      const lastNames = ['Johnson', 'Smith', 'Diaz', 'Lee', 'Wright', 'Patel', 'Chen', 'Mueller', 'Costa', 'Kim'];
      const out = [];
      for (let i = 0; i < 40; i++) {
        const fn = firstNames[i % firstNames.length];
        const ln = lastNames[Math.floor(rng() * lastNames.length)];
        const plan = plans[Math.floor(rng() * plans.length)];
        const mrr = plan === 'free' ? 0 : plan === 'pro' ? 49 + Math.floor(rng() * 50) : 499 + Math.floor(rng() * 500);
        const d = new Date(Date.UTC(2023, 0, 1));
        d.setUTCDate(d.getUTCDate() + Math.floor(rng() * 600));
        out.push({
          Name: `${fn} ${ln}`,
          Email: `${fn.toLowerCase()}.${ln.toLowerCase()}${i}@example.com`,
          Age: 18 + Math.floor(rng() * 50),
          Active: rng() > 0.4,
          Plan: plan,
          Country: countries[Math.floor(rng() * countries.length)],
          MRR: mrr,
          SignedUp: dateOnly(d),
        });
      }
      return out;
    },
  },

  // 2) Time series metrics (DateTime + numeric measures across services).
  {
    title: 'Metrics',
    columns: [
      { title: 'Timestamp', uidt: 'DateTime' },
      { title: 'Service', ...sel('SingleSelect', ['api', 'web', 'worker', 'db']) },
      { title: 'CPU', uidt: 'Decimal' },
      { title: 'Memory', uidt: 'Decimal' },
      { title: 'Requests', uidt: 'Number' },
      { title: 'LatencyMs', uidt: 'Decimal' },
    ],
    rows() {
      const rng = makeRng(23);
      const services = ['api', 'web', 'worker', 'db'];
      const out = [];
      // 48 points per service, hourly, ending now.
      const now = Date.now();
      for (const service of services) {
        let cpu = 30 + rng() * 20;
        for (let i = 47; i >= 0; i--) {
          const ts = new Date(now - i * 3600 * 1000);
          cpu = Math.max(2, Math.min(98, cpu + (rng() - 0.5) * 18));
          const mem = Math.max(5, Math.min(95, cpu * 0.7 + (rng() - 0.5) * 10));
          const req = Math.floor(50 + rng() * 950 + (service === 'api' ? 500 : 0));
          const latency = +(20 + rng() * 180 + (cpu > 80 ? 120 : 0)).toFixed(1);
          out.push({
            Timestamp: isoMinute(ts),
            Service: service,
            CPU: +cpu.toFixed(2),
            Memory: +mem.toFixed(2),
            Requests: req,
            LatencyMs: latency,
          });
        }
      }
      return out;
    },
  },

  // 3) Log lines (level, service, message, duration).
  {
    title: 'Logs',
    columns: [
      { title: 'Timestamp', uidt: 'DateTime' },
      { title: 'Level', ...sel('SingleSelect', ['debug', 'info', 'warn', 'error']) },
      { title: 'Service', ...sel('SingleSelect', ['api', 'web', 'worker', 'db']) },
      { title: 'Message', uidt: 'LongText' },
      { title: 'StatusCode', uidt: 'Number' },
      { title: 'DurationMs', uidt: 'Decimal' },
    ],
    rows() {
      const rng = makeRng(37);
      const levels = ['debug', 'info', 'info', 'info', 'warn', 'error'];
      const services = ['api', 'web', 'worker', 'db'];
      const messages = {
        debug: ['cache hit for key user:42', 'connection pool size 8', 'feature flag evaluated'],
        info: ['request completed', 'user logged in', 'job processed', 'record created'],
        warn: ['slow query detected', 'retrying upstream call', 'deprecated endpoint used'],
        error: ['upstream timeout', 'unhandled exception', 'failed to connect to db', 'payment declined'],
      };
      const out = [];
      const now = Date.now();
      for (let i = 200; i >= 0; i--) {
        const level = levels[Math.floor(rng() * levels.length)];
        const svc = services[Math.floor(rng() * services.length)];
        const msgs = messages[level];
        const ts = new Date(now - i * 90 * 1000 - Math.floor(rng() * 60000));
        const code = level === 'error' ? [500, 502, 503][Math.floor(rng() * 3)] : level === 'warn' ? 429 : 200;
        out.push({
          Timestamp: isoMinute(ts),
          Level: level,
          Service: svc,
          Message: msgs[Math.floor(rng() * msgs.length)],
          StatusCode: code,
          DurationMs: +(5 + rng() * 600).toFixed(1),
        });
      }
      return out;
    },
  },

  // 4) Numeric / categorical sales data (good for aggregation/group-by demos).
  {
    title: 'Sales',
    columns: [
      { title: 'Date', uidt: 'Date' },
      { title: 'Region', ...sel('SingleSelect', ['North', 'South', 'East', 'West']) },
      { title: 'Product', ...sel('SingleSelect', ['Widget', 'Gadget', 'Gizmo', 'Doohickey']) },
      { title: 'Units', uidt: 'Number' },
      { title: 'Revenue', uidt: 'Currency' },
      { title: 'Returned', uidt: 'Checkbox' },
    ],
    rows() {
      const rng = makeRng(53);
      const regions = ['North', 'South', 'East', 'West'];
      const products = ['Widget', 'Gadget', 'Gizmo', 'Doohickey'];
      const price = { Widget: 9.99, Gadget: 19.99, Gizmo: 49.99, Doohickey: 4.99 };
      const out = [];
      for (let i = 0; i < 90; i++) {
        const d = new Date(Date.UTC(2024, 0, 1));
        d.setUTCDate(d.getUTCDate() + Math.floor(rng() * 180));
        const product = products[Math.floor(rng() * products.length)];
        const units = 1 + Math.floor(rng() * 50);
        out.push({
          Date: dateOnly(d),
          Region: regions[Math.floor(rng() * regions.length)],
          Product: product,
          Units: units,
          Revenue: +(units * price[product]).toFixed(2),
          Returned: rng() > 0.85,
        });
      }
      return out;
    },
  },
];

async function createApiToken(auth) {
  const headers = { 'xc-auth': auth };
  const created = await req('/api/v1/tokens', {
    method: 'POST',
    headers,
    body: { description: 'grafana-local' },
  });
  if (!created.ok || !created.json?.token) {
    throw new Error(`Failed to create API token: ${created.status} ${created.text}`);
  }
  console.log('Minted API token for Grafana.');
  return created.json.token;
}

async function writeDatasourceProvisioning(token, baseId) {
  const yaml = `apiVersion: 1

# This file is generated by scripts/seed.mjs on \`docker compose up\`.
# It is overwritten on each start with a fresh token and base id.
deleteDatasources:
  - name: NocoDB
    orgId: 1

datasources:
  - name: NocoDB
    type: yesoreyeram-nocodb-datasource
    access: proxy
    isDefault: true
    jsonData:
      platform: selfhosted
      baseURL: ${GRAFANA_INTERNAL_NOCODB_URL}
      apiVersion: v2
      baseId: ${baseId}
    secureJsonData:
      apiToken: ${token}
    editable: true
`;
  await mkdir(dirname(DS_OUTPUT), { recursive: true });
  await writeFile(DS_OUTPUT, yaml, 'utf8');
  console.log(`Wrote Grafana datasource provisioning to ${DS_OUTPUT}`);
}

async function main() {
  await waitForNocoDB();
  const auth = await getAuthToken();
  const baseId = await findOrCreateBase(auth);
  const token = await createApiToken(auth);

  for (const spec of TABLES) {
    const { id: tableId } = await findOrCreateTable(auth, baseId, spec);
    await ensureRows(token, tableId, spec);
  }

  await writeDatasourceProvisioning(token, baseId);
  console.log('NocoDB seed complete.');
}

main().catch((err) => {
  console.error('Seed failed:', err.message);
  process.exit(1);
});
