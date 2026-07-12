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
task test        # Go tests (starts MongoDB if you don't have one)
task test:all    # Go tests + SDK tests
task test:sdk    # SDK tests only
```

`task test` runs [`scripts/test.sh`](scripts/test.sh), which passes any extra arguments through to `go test`:

```bash
task test -- ./internal/repository/ -run TestSubscriberRepo
```

**About MongoDB.** Roughly a fifth of the Go suite is integration tests that need a real database. The script guarantees one: it reuses whatever is listening on port `27017` (a `task dev` stack, say), and otherwise starts a throwaway `mongo:7` container and removes it when the run ends.

Prefer it over a bare `go test ./...`. Without a database those tests **skip rather than fail**, and `go test` still exits 0 — so the suite looks green while the entire persistence layer went untested. The script sets `MONGO_TEST_REQUIRED=1`, which turns that skip into a failure.

To point the tests at your own database, set `MONGO_TEST_URI` and it is used as-is:

```bash
MONGO_TEST_URI=mongodb://localhost:27017 task test
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
3. Run `task test` and ensure all tests pass. (Not a bare `go test ./...` — it skips every test that needs MongoDB and still exits 0.)
4. Open a pull request against `dev`. Fill in the PR template.

## Reporting Issues

Use the [GitHub issue tracker](https://github.com/partiri-cloud/message-in-a-bottle/issues). Choose the appropriate template (bug report or feature request) and fill it in completely.

## Code Style

- Go: standard `gofmt` formatting. Run `go vet ./...` before submitting.
- TypeScript: the SDK uses the project's existing ESLint/Prettier config.
- No new comments unless the intent is genuinely non-obvious.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
