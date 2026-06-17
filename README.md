<!-- markdownlint-configure-file {
  "MD013": false,
  "MD041": false,
  "MD033": false
} -->

<p align="center">
      <img src="https://us1.discourse-cdn.com/grafana/original/2X/a/a7f38198d3aa26d70bae13c3379e5b93a010e7d7.png" alt="Grafana logo" width=140">
</p>

<h1 align="center">
  Grafana X
</h1>

<p align="center">Collection of grafana plugins, datasources, panels, tools, skills, experiments</p>

## Packages

This is a [Yarn 4](https://yarnpkg.com) workspaces monorepo, with task
orchestration and caching handled by [Turborepo](https://turborepo.com).

| Package | Description |
| --- | --- |
| [`@yesoreyeram/grafana-utils`](./packages/utils) | CLI tool for Grafana plugin development and management. |
| [`@yesoreyeram/grafana-plugin-tools`](./packages/plugin-tools) | Shared templates and configuration distributed via the registry below. |

## Plugins

| Plugin | Description |
| --- | --- |
| [`yesoreyeram-baserow-datasource`](./plugins/grafana-baserow-datasource) | Grafana data source plugin for [Baserow](https://baserow.io) (TypeScript frontend + Go backend). |
| [`yesoreyeram-nocodb-datasource`](./plugins/grafana-nocodb-datasource) | Grafana data source plugin for [NocoDB](https://nocodb.com) (TypeScript frontend + Go backend). |
| [`yesoreyeram-notion-datasource`](./plugins/grafana-notion-datasource) | Grafana data source plugin for [Notion](https://www.notion.so) (TypeScript frontend + Go backend). |
| [`yesoreyeram-linear-datasource`](./plugins/grafana-linear-datasource) | Grafana data source plugin for [Linear](https://linear.app) (TypeScript frontend + Go backend). |
| [`yesoreyeram-plane-datasource`](./plugins/grafana-plane-datasource) | Grafana data source plugin for [Plane](https://plane.so) (TypeScript frontend + Go backend). |

## Registry

[`registry.json`](./registry.json) is a [shadcn-style registry](https://ui.shadcn.com/docs/registry)
that distributes ready-made setup for Grafana data source plugin repositories
(package manager, build, lint, test, e2e, CI workflows, AI agent instructions,
and more). Each item installs files and dependencies into a target repo.

## Development

```bash
# Install dependencies
yarn install --immutable

# Run all quality gates (what CI runs)
yarn spellcheck
yarn lint
yarn format:check
yarn typecheck
yarn build
yarn test
```

### Task running and caching

`build`, `typecheck`, and `test` are run across all workspaces by
[Turborepo](https://turborepo.com) (configured in [`turbo.json`](./turbo.json)).
Turborepo runs tasks in topological order (a plugin's dependencies build first)
and **caches** their results — re-running a task with no relevant changes
replays the cached output instantly (`>>> FULL TURBO`).

```bash
# Run a task across every workspace (cached)
yarn build          # turbo run build
yarn typecheck      # turbo run typecheck
yarn test           # turbo run test

# Run a task for a single workspace
yarn turbo run build --filter=yesoreyeram-notion-datasource

# Watch/dev (not cached)
yarn turbo run dev --filter=yesoreyeram-notion-datasource

# Force a run, ignoring the cache
yarn turbo run build --force
```

The cache lives in `.turbo/` (git-ignored). Cache keys are derived from each
task's inputs (source files, configs) plus global config such as `.nvmrc`,
`eslint.config.mjs`, and `cspell.config.json`, so changing any of these
invalidates the affected tasks.

### Local stack (all plugins)

The root [`docker-compose.yaml`](./docker-compose.yaml) runs a single Grafana
with **every plugin** in this monorepo mounted, plus the backing services they
need. Grafana starts with anonymous admin at http://localhost:3000.

Build each plugin's `dist/` (frontend + backend) first, then start the stack.
`yarn build` builds both the frontend and the Go backend (all platforms) for
every plugin, so it requires Go and [Mage](https://magefile.org) on your PATH.

```bash
# Build every plugin's frontend + backend (-> dist/, incl. linux binaries)
yarn build

# Start Grafana + all plugins (+ backing services)
NOTION_API_TOKEN=secret_... docker compose up
```

The NocoDB datasource is seeded and auto-provisioned; the Notion datasource is
auto-provisioned from `NOTION_API_TOKEN` (omit it to skip Notion). Each plugin
also keeps its own `docker-compose.yaml` for running that plugin in isolation.

### Releasing

Releases are automated with [Changesets](https://github.com/changesets/changesets).
When you change a publishable package, add a changeset:

```bash
yarn changeset
```

On merge to `main`, the release workflow opens a "Version Packages" PR; merging
that PR publishes the affected packages to npm.

## License

[Apache-2.0](./LICENSE)
