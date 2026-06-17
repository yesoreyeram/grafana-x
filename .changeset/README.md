# Changesets

This directory is managed by [Changesets](https://github.com/changesets/changesets).
It is used to track changes to the publishable packages in this repository and to
automate versioning and changelog generation.

## Adding a changeset

When you make a change to a publishable package (for example
`@yesoreyeram/grafana-utils`), run:

```bash
yarn changeset
```

Select the affected packages, choose a semver bump (patch / minor / major), and
write a short summary. This creates a markdown file in this directory that should
be committed alongside your change.

## Releasing

On merge to `main`, the release workflow runs `changeset version` to consume the
pending changesets (bumping versions and updating changelogs) and opens a
"Version Packages" pull request. Merging that PR publishes the updated packages
to npm via `changeset publish`.
