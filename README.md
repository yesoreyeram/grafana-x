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

| Package                                                        | Description                                                            |
| -------------------------------------------------------------- | ---------------------------------------------------------------------- |
| [`@yesoreyeram/grafana-utils`](./packages/utils)               | CLI tool for Grafana plugin development and management.                |
| [`@yesoreyeram/grafana-plugin-tools`](./packages/plugin-tools) | Shared templates and configuration distributed via the registry below. |

## Plugins

| Plugin                                                                                     | Description                                                                                                                                              |
| ------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [`yesoreyeram-baserow-datasource`](./plugins/datasources/yesoreyeram-baserow-datasource)       | Grafana data source plugin for [Baserow](https://baserow.io) (TypeScript frontend + Go backend).                                                         |
| [`yesoreyeram-nocodb-datasource`](./plugins/datasources/yesoreyeram-nocodb-datasource)         | Grafana data source plugin for [NocoDB](https://nocodb.com) (TypeScript frontend + Go backend).                                                          |
| [`yesoreyeram-notion-datasource`](./plugins/datasources/yesoreyeram-notion-datasource)         | Grafana data source plugin for [Notion](https://www.notion.so) (TypeScript frontend + Go backend).                                                       |
| [`yesoreyeram-linear-datasource`](./plugins/datasources/yesoreyeram-linear-datasource)         | Grafana data source plugin for [Linear](https://linear.app) (TypeScript frontend + Go backend).                                                          |
| [`yesoreyeram-plane-datasource`](./plugins/datasources/yesoreyeram-plane-datasource)           | Grafana data source plugin for [Plane](https://plane.so) (TypeScript frontend + Go backend).                                                             |
| [`yesoreyeram-monday-datasource`](./plugins/datasources/yesoreyeram-monday-datasource)         | Grafana data source plugin for [monday.com](https://monday.com) (TypeScript frontend + Go backend).                                                      |
| [`yesoreyeram-hubspot-datasource`](./plugins/datasources/yesoreyeram-hubspot-datasource)       | Grafana data source plugin for [hubspot.com](https://hubspot.com) (TypeScript frontend + Go backend).                                                    |
| [`yesoreyeram-pocketbase-datasource`](./plugins/datasources/yesoreyeram-pocketbase-datasource) | Grafana data source plugin for [PocketBase](https://pocketbase.io) (TypeScript frontend + Go backend).                                                   |
| [`yesoreyeram-asana-datasource`](./plugins/datasources/yesoreyeram-asana-datasource)           | Grafana data source plugin for [Asana](https://asana.com) (TypeScript frontend + Go backend).                                                            |
| [`yesoreyeram-airtable-datasource`](./plugins/datasources/yesoreyeram-airtable-datasource)     | Grafana data source plugin for [Airtable](https://airtable.com) (TypeScript frontend + Go backend).                                                      |
| [`yesoreyeram-appwrite-datasource`](./plugins/datasources/yesoreyeram-appwrite-datasource)     | Grafana data source plugin for [Appwrite](https://appwrite.io) (TypeScript frontend + Go backend).                                                       |
| [`yesoreyeram-clickup-datasource`](./plugins/datasources/yesoreyeram-clickup-datasource)       | Grafana data source plugin for [ClickUp](https://clickup.com) (TypeScript frontend + Go backend).                                                        |
| [`yesoreyeram-directus-datasource`](./plugins/datasources/yesoreyeram-directus-datasource)     | Grafana data source plugin for [Directus](https://directus.io) (TypeScript frontend + Go backend).                                                       |
| [`yesoreyeram-grist-datasource`](./plugins/datasources/yesoreyeram-grist-datasource)           | Grafana data source plugin for [Grist](https://www.getgrist.com) (TypeScript frontend + Go backend).                                                     |
| [`yesoreyeram-seatable-datasource`](./plugins/datasources/yesoreyeram-seatable-datasource)     | Grafana data source plugin for [SeaTable](https://seatable.io) (TypeScript frontend + Go backend).                                                       |
| [`yesoreyeram-strapi-datasource`](./plugins/datasources/yesoreyeram-strapi-datasource)         | Grafana data source plugin for [Strapi](https://strapi.io) (TypeScript frontend + Go backend).                                                           |
| [`yesoreyeram-teable-datasource`](./plugins/datasources/yesoreyeram-teable-datasource)         | Grafana data source plugin for [Teable](https://teable.io) (TypeScript frontend + Go backend).                                                           |
| [`yesoreyeram-supabase-datasource`](./plugins/datasources/yesoreyeram-supabase-datasource)     | Grafana data source plugin for [Supabase](https://supabase.com) (TypeScript frontend + Go backend).                                                      |
| [`yesoreyeram-trello-datasource`](./plugins/datasources/yesoreyeram-trello-datasource)         | Grafana data source plugin for [Trello](https://trello.com) (TypeScript frontend + Go backend).                                                          |
| [`yesoreyeram-todoist-datasource`](./plugins/datasources/yesoreyeram-todoist-datasource)       | Grafana data source plugin for [Todoist](https://todoist.com) (TypeScript frontend + Go backend).                                                        |
| [`yesoreyeram-shortcut-datasource`](./plugins/datasources/yesoreyeram-shortcut-datasource)     | Grafana data source plugin for [Shortcut](https://www.shortcut.com) (TypeScript frontend + Go backend).                                                  |
| [`yesoreyeram-coda-datasource`](./plugins/datasources/yesoreyeram-coda-datasource)             | Grafana data source plugin for [Coda](https://coda.io) (TypeScript frontend + Go backend).                                                               |
| [`yesoreyeram-confluence-datasource`](./plugins/datasources/yesoreyeram-confluence-datasource) | Grafana data source plugin for [Confluence](https://www.atlassian.com/software/confluence) (TypeScript frontend + Go backend).                           |
| [`yesoreyeram-pipedrive-datasource`](./plugins/datasources/yesoreyeram-pipedrive-datasource)   | Grafana data source plugin for [Pipedrive](https://www.pipedrive.com) (TypeScript frontend + Go backend).                                                |
| [`yesoreyeram-attio-datasource`](./plugins/datasources/yesoreyeram-attio-datasource)           | Grafana data source plugin for [Attio](https://attio.com) (TypeScript frontend + Go backend).                                                            |
| [`yesoreyeram-intercom-datasource`](./plugins/datasources/yesoreyeram-intercom-datasource)     | Grafana data source plugin for [Intercom](https://www.intercom.com) (TypeScript frontend + Go backend).                                                  |
| [`yesoreyeram-vizard-panel`](./plugins/panel/yesoreyeram-vizard-panel)                         | Grafana **panel** plugin: a [Vega-Lite](https://vega.github.io/vega-lite/) visual builder that renders any data frame (TypeScript frontend, no backend). |

See [`ROADMAP.md`](./ROADMAP.md) for the list of plugins planned next, each verified
as not yet published on [grafana.com/api/plugins](https://grafana.com/api/plugins).

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
