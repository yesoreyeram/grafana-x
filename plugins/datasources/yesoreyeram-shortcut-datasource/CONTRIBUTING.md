# Contributing

## Prerequisites

- Go 1.26+
- Node.js 24+
- Yarn 4
- Mage

## Development

```bash
# Install dependencies
yarn install

# Build all
yarn build

# Run tests
yarn test
go test ./pkg/...

# Run local stack
SHORTCUT_API_TOKEN=your_token docker compose up
```

## Code style

- Frontend: ESLint + Prettier + TypeScript strict
- Backend: `gofmt` + `go vet`
