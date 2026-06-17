# Contributing

Thanks for your interest in contributing to Grafana X.

## Prerequisites

- Node.js (see [`.nvmrc`](./.nvmrc))
- Yarn 4 (managed via [`corepack`](https://nodejs.org/api/corepack.html))

```bash
corepack enable
yarn install --immutable
```

## Workflow

1. Create a branch from `main`.
2. Make your change.
3. Run the quality gates locally (these mirror CI):

   ```bash
   yarn spellcheck
   yarn lint
   yarn format:check
   yarn typecheck
   yarn build
   yarn test
   ```

4. If you changed a publishable package (for example
   `@yesoreyeram/grafana-utils`), add a changeset:

   ```bash
   yarn changeset
   ```

5. Open a pull request.

## Releasing

Releases are automated with [Changesets](https://github.com/changesets/changesets).
On merge to `main`, a "Version Packages" PR is opened. Merging that PR bumps
versions, updates changelogs, and publishes to npm.
