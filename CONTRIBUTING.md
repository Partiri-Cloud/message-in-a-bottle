# Contributing to Message in a Bottle

Thank you for your interest in contributing! This document covers how to set up a development environment, run tests, and submit changes.

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [Node.js 18+](https://nodejs.org/) (for SDK development)
- [Task](https://taskfile.dev/) (optional but recommended — all commands have a plain equivalent)

## Local Development

**1. Clone and install dependencies**

```bash
git clone https://github.com/partiri-cloud/message-in-a-bottle.git
cd message-in-a-bottle
task deps        # go mod download + npm install in packages/sdk
```

Without Taskfile:
```bash
go mod download
cd packages/sdk && npm install && cd ../..
```

**2. Start the database dependencies**

```bash
task dev         # starts MongoDB and Redis in Docker
# or:
docker compose up -d mongo redis
```

**3. Configure environment**

```bash
cp .env.example .env
```

Edit `.env` and fill in the required secrets (`ADMIN_SECRET`, `CREDENTIALS_ENCRYPTION_KEY`, `SUBSCRIBER_HMAC_SECRET`). The file has generation instructions for each.

**4. Run the services**

```bash
task run:api     # port 3000
task run:ws      # port 3001
task run:worker  # background
```

Or all at once with Docker Compose:
```bash
docker compose up
```

## Running Tests

```bash
task test:all    # Go tests + SDK tests
# or separately:
go test ./...
cd packages/sdk && npx vitest run
```

## Building

```bash
task build       # all three Go binaries → bin/
task build:all   # binaries + SDK
task docker      # Docker image
```

## Submitting Changes

1. Fork the repository and create a branch from `master`.
2. Make your changes. Add tests for new behaviour.
3. Run `go test ./...` and ensure all tests pass.
4. Open a pull request against `dev`. Fill in the PR template.

## Reporting Issues

Use the [GitHub issue tracker](https://github.com/partiri-cloud/message-in-a-bottle/issues). Choose the appropriate template (bug report or feature request) and fill it in completely.

## Code Style

- Go: standard `gofmt` formatting. Run `go vet ./...` before submitting.
- TypeScript: the SDK uses the project's existing ESLint/Prettier config.
- No new comments unless the intent is genuinely non-obvious.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
