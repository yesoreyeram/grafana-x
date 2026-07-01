# Contributing

## Development Setup

1. Clone the monorepo and install dependencies:
   ```bash
   yarn install
   ```

2. Build the plugin:
   ```bash
   yarn build
   ```

3. Run tests:
   ```bash
   go test ./pkg/...
   yarn test
   ```

4. Run with local Grafana:
   ```bash
   PIPEDRIVE_COMPANY_DOMAIN=mycompany PIPEDRIVE_API_TOKEN=xxx docker compose up
   ```

## Code Style

- TypeScript: Follow existing patterns. Use stable `@grafana/ui` components.
- Go: `gofmt` formatting, `testify` for tests, `httptest` for HTTP testing.
- No comments on implementation code unless explaining non-obvious logic.

## Pull Requests

- Keep changes focused. One feature/fix per PR.
- Ensure `yarn typecheck`, `yarn lint`, and `go test ./pkg/...` pass.
- Update CHANGELOG.md for user-facing changes.
