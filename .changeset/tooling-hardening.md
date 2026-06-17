---
"@yesoreyeram/grafana-utils": patch
---

Harden the CLI: read the version dynamically from package.json, validate the
`--yarn-version` input, report which command failed during setup, and ship only
built output (`dist`) when published.
