#!/usr/bin/env node
/**
 * Idempotent PocketBase seeder for local development.
 *
 * On `docker compose up` this:
 *   1. waits for PocketBase to be ready (GET /api/health),
 *   2. authenticates as the superuser created by the pocketbase service,
 *   3. ensures two sample base collections exist:
 *        - `demo`    (mixed-type rows: text/select/number/bool/date)
 *        - `metrics` (time series: date + numeric measures across services)
 *   4. inserts sample records when the collections are empty.
 *
 * Re-running is safe: existing collections/records are detected and reused.
 *
 * The Grafana datasource itself is provisioned statically from env vars (see
 * provisioning/datasources/pocketbase.yaml), so this script only seeds data.
 */

const PB_URL = process.env.POCKETBASE_INTERNAL_URL ?? 'http://pocketbase:8090';
const IDENTITY = process.env.POCKETBASE_IDENTITY ?? 'admin@example.com';
const PASSWORD = process.env.POCKETBASE_PASSWORD ?? 'Password123!';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function req(path, { method = 'GET', token, body } = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) {
    headers.Authorization = token;
  }
  const res = await fetch(`${PB_URL}${path}`, {
    method,
    headers,
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

async function waitForPocketBase() {
  process.stdout.write('Waiting for PocketBase');
  for (let i = 0; i < 120; i++) {
    try {
      const res = await fetch(`${PB_URL}/api/health`).catch(() => null);
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
  throw new Error('PocketBase did not become ready in time');
}

async function authSuperuser() {
  const res = await req('/api/collections/_superusers/auth-with-password', {
    method: 'POST',
    body: { identity: IDENTITY, password: PASSWORD },
  });
  if (!res.ok || !res.json?.token) {
    throw new Error(`Unable to authenticate as superuser: ${res.status} ${res.text}`);
  }
  console.log('Authenticated as superuser.');
  return res.json.token;
}

async function ensureCollection(token, spec) {
  const existing = await req(`/api/collections/${spec.name}`, { token });
  if (existing.ok) {
    console.log(`Reusing collection "${spec.name}".`);
    return;
  }
  const created = await req('/api/collections', { method: 'POST', token, body: spec });
  if (!created.ok) {
    throw new Error(`Failed to create collection "${spec.name}": ${created.status} ${created.text}`);
  }
  console.log(`Created collection "${spec.name}".`);
}

async function ensureRecords(token, name, rows) {
  const existing = await req(`/api/collections/${name}/records?perPage=1`, { token });
  const total = existing.json?.totalItems ?? 0;
  if (total > 0) {
    console.log(`Collection "${name}" already has ${total} record(s); skipping insert.`);
    return;
  }
  let inserted = 0;
  for (const row of rows) {
    const res = await req(`/api/collections/${name}/records`, { method: 'POST', token, body: row });
    if (!res.ok) {
      throw new Error(`Failed to insert into "${name}": ${res.status} ${res.text}`);
    }
    inserted++;
  }
  console.log(`Inserted ${inserted} records into "${name}".`);
}

// --------------------------------------------------------------------------
// Collection schemas
// --------------------------------------------------------------------------

const createdField = { name: 'created', type: 'autodate', onCreate: true, onUpdate: false };
const updatedField = { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true };

const DEMO = {
  name: 'demo',
  type: 'base',
  fields: [
    { name: 'title', type: 'text', required: true },
    { name: 'category', type: 'select', maxSelect: 1, values: ['bug', 'feature', 'chore'] },
    { name: 'score', type: 'number' },
    { name: 'active', type: 'bool' },
    { name: 'due', type: 'date' },
    createdField,
    updatedField,
  ],
};

const METRICS = {
  name: 'metrics',
  type: 'base',
  fields: [
    { name: 'ts', type: 'date', required: true },
    { name: 'service', type: 'select', maxSelect: 1, values: ['api', 'web', 'worker', 'db'] },
    { name: 'cpu', type: 'number' },
    { name: 'requests', type: 'number' },
    createdField,
  ],
};

// Deterministic pseudo-random so seeds are stable across runs.
function makeRng(seedValue) {
  let s = seedValue >>> 0;
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0;
    return s / 0xffffffff;
  };
}

function demoRows() {
  const rng = makeRng(7);
  const categories = ['bug', 'feature', 'chore'];
  const titles = ['Login flow', 'Export CSV', 'Dark mode', 'Rate limiting', 'Webhook retries', 'Audit log', 'SSO', 'Billing'];
  const out = [];
  for (let i = 0; i < 40; i++) {
    const d = new Date(Date.UTC(2024, 0, 1));
    d.setUTCDate(d.getUTCDate() + Math.floor(rng() * 300));
    out.push({
      title: `${titles[i % titles.length]} #${i + 1}`,
      category: categories[Math.floor(rng() * categories.length)],
      score: Math.floor(rng() * 100),
      active: rng() > 0.4,
      due: d.toISOString(),
    });
  }
  return out;
}

function metricsRows() {
  const rng = makeRng(19);
  const services = ['api', 'web', 'worker', 'db'];
  const out = [];
  const now = Date.now();
  for (const service of services) {
    let cpu = 30 + rng() * 20;
    for (let i = 23; i >= 0; i--) {
      const ts = new Date(now - i * 3600 * 1000);
      cpu = Math.max(2, Math.min(98, cpu + (rng() - 0.5) * 18));
      out.push({
        ts: ts.toISOString(),
        service,
        cpu: +cpu.toFixed(2),
        requests: Math.floor(50 + rng() * 950 + (service === 'api' ? 500 : 0)),
      });
    }
  }
  return out;
}

async function main() {
  await waitForPocketBase();
  const token = await authSuperuser();

  await ensureCollection(token, DEMO);
  await ensureCollection(token, METRICS);

  await ensureRecords(token, 'demo', demoRows());
  await ensureRecords(token, 'metrics', metricsRows());

  console.log('PocketBase seed complete.');
}

main().catch((err) => {
  console.error('Seed failed:', err.message);
  process.exit(1);
});
