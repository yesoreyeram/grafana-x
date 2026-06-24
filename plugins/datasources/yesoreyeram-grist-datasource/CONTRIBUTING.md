# Contributing

## Development workflow

1. Make your changes
2. Run `go test ./pkg/...` to verify backend tests pass
3. Run `npm test` to verify frontend tests pass
4. Run `npm run lint` to check frontend linting
5. Run `go vet ./...` to check backend code quality
6. Update golden test data if frame output changed: `UPDATE_GOLDEN=true go test ./pkg/... -run TestGolden`

## Code conventions

- Follow the existing Airtable/NocoDB plugin pattern for structure and naming
- Use `httptest.Server` for HTTP client tests
- Frame construction tests should verify type inference, column ordering, and data plane compliance
- Resource handlers return `[]{id, title}` for docs/tables, `[]{title, type}` for fields
