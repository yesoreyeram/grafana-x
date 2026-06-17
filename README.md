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

This is a [Yarn 4](https://yarnpkg.com) workspaces monorepo.

| Package | Description |
| --- | --- |
| [`@yesoreyeram/grafana-utils`](./packages/utils) | CLI tool for Grafana plugin development and management. |
| [`@yesoreyeram/grafana-plugin-tools`](./packages/plugin-tools) | Shared templates and configuration distributed via the registry below. |

## Plugins

| Plugin | Description |
| --- | --- |
| [`yesoreyeram-nocodb-datasource`](./plugins/grafana-nocodb-datasource) | Grafana data source plugin for [NocoDB](https://nocodb.com) (TypeScript frontend + Go backend). |

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
