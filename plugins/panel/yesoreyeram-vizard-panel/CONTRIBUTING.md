# Contributing

This plugin (`yesoreyeram-vizard-panel`) is a workspace in the
[`grafana-x`](../../README.md) Yarn 4 monorepo.

## Setup

```bash
# from the monorepo root
yarn install
```

## Quality gates

Run all of these before opening a PR (this is what CI runs):

```bash
yarn workspace @yesoreyeram/grafana-vizard-panel typecheck
yarn workspace @yesoreyeram/grafana-vizard-panel lint
yarn workspace @yesoreyeram/grafana-vizard-panel test
yarn workspace @yesoreyeram/grafana-vizard-panel build
yarn workspace @yesoreyeram/grafana-vizard-panel spellcheck
```

## Run it locally

```bash
yarn workspace @yesoreyeram/grafana-vizard-panel build
docker compose -f plugins/panel/yesoreyeram-vizard-panel/docker-compose.yaml up
# Grafana at http://localhost:3000 (anonymous admin), with a demo dashboard.
```

## Notes

- Frontend only — there is no Go backend.
- Read [`AGENTS.md`](./AGENTS.md) for the architecture and the invariants that
  must not regress (especially the security guarantees in `spec/sanitizeSpec.ts`
  and `components/VegaView.tsx`).
- Add unit tests for any pure logic you change (`src/**/<name>.test.ts`).
- Keep dependencies pinned to exact versions.
