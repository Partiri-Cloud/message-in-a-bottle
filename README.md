# Message in a Bottle

Multi-channel notification platform with workflow orchestration, subscriber management, and real-time delivery.

## Architecture

The platform runs as three separate binaries backed by MongoDB and Redis:

```
Clients (SDK / HTTP)
        |
   +---------+       +----------+
   | API     |       | WS       |
   | :3000   |       | :3001    |
   +----+----+       +-----+----+
        |                   |
   +----v---+          +----v----+
   | MongoDB |<------->| Redis   |
   +----+----+         +----+----+
        |                   |
   +----v------------------v----+
   |         Worker             |
   +---+-----+-----+------+----+
       |     |     |      |
    Email   SMS  Push   Chat
```

| Binary   | Port | Purpose                                    |
|----------|------|--------------------------------------------|
| `api`    | 3000 | REST API for all CRUD and event triggers    |
| `ws`     | 3001 | WebSocket server for real-time in-app feed  |
| `worker` | --   | Background job processor (Asynq)           |

### Notification channels

Email (SendGrid, SES, SMTP), SMS (Twilio, Vonage), Push (FCM, APNS), Chat (Slack, MS Teams), In-App (WebSocket).

### Workflow engine

Workflows define multi-step notification pipelines with:

- **Channel steps** -- deliver via a specific channel
- **Delay steps** -- pause before continuing
- **Digest steps** -- batch notifications over a time window
- **Conditions** -- skip steps based on subscriber data or payload fields

Subscriber preferences cascade: workflow-specific > global > workflow defaults.

## Quick start

```bash
cp .env.example .env
# Fill in at least CREDENTIALS_ENCRYPTION_KEY and SUBSCRIBER_HMAC_SECRET:
#   openssl rand -hex 32  (run twice, one for each)
```

### Run everything together (docker compose)

Starts MongoDB, Redis, and all three services:

```bash
docker compose up -d
```

### Run everything together (without Docker)

You need Go 1.25+, a running MongoDB instance, and a running Redis instance.

```bash
# 1. Install dependencies
task deps

# 2. Build all binaries
task build
# produces bin/api, bin/ws, bin/worker

# 3. Source your env file (or export vars manually)
export $(grep -v '^#' .env | xargs)

# 4. Run all three processes (Ctrl-C stops all)
task run
```

Or without Taskfile:

```bash
go mod download
go build -o bin/api ./cmd/api
go build -o bin/ws ./cmd/ws
go build -o bin/worker ./cmd/worker

export $(grep -v '^#' .env | xargs)
./bin/worker & ./bin/ws & ./bin/api
```

If you don't have MongoDB/Redis installed locally, you can start just those with Docker:

```bash
task dev    # or: docker compose up -d mongo redis
```

If you already have MongoDB and Redis running — locally or via any cloud service — skip this step and set `MONGO_URI` and `REDIS_ADDR` in your `.env` to point at your instances. The app creates its own indexes on first start; no manual schema setup is needed.

To run only the app containers against external databases, use the app-only Compose file:

```bash
docker compose -f docker-compose.app.yml up
```

### Run services separately (without Docker)

Build individual binaries and run them independently. Useful when each service runs on a different host or you only need one during development.

```bash
# Build all three at once (parallel)
task build

# Or build one at a time
task build:api
task build:ws
task build:worker
```

Without Taskfile:

```bash
go build -o bin/api ./cmd/api
go build -o bin/ws ./cmd/ws
go build -o bin/worker ./cmd/worker
```

Run each binary with its required env vars:

```bash
# API server (port 3000)
MONGO_URI=mongodb://db:27017 REDIS_ADDR=redis:6379 \
  CREDENTIALS_ENCRYPTION_KEY=<key> \
  ./bin/api

# WebSocket server (port 3001)
MONGO_URI=mongodb://db:27017 REDIS_ADDR=redis:6379 \
  SUBSCRIBER_HMAC_SECRET=<secret> \
  ./bin/ws

# Worker (no port)
MONGO_URI=mongodb://db:27017 REDIS_ADDR=redis:6379 \
  CREDENTIALS_ENCRYPTION_KEY=<key> \
  ./bin/worker
```

Or with Taskfile (reads from `.env` via your shell):

```bash
task run:api
task run:ws
task run:worker
```

### Run with Docker

All three binaries are in the same image. Pick the entrypoint:

```bash
docker build -t miab .

# Together
docker compose up -d

# Separately
docker run -p 3000:3000 --env-file .env --entrypoint /api miab
docker run -p 3001:3001 --env-file .env --entrypoint /ws miab
docker run --env-file .env --entrypoint /worker miab
```

### Per-service env var reference

Each service documents its own required and optional env vars:

- [`cmd/api/README.md`](cmd/api/README.md) -- needs `CREDENTIALS_ENCRYPTION_KEY`
- [`cmd/ws/README.md`](cmd/ws/README.md) -- needs `SUBSCRIBER_HMAC_SECRET`
- [`cmd/worker/README.md`](cmd/worker/README.md) -- needs `CREDENTIALS_ENCRYPTION_KEY`

All three need `MONGO_URI`, `MONGO_DB`, `REDIS_ADDR`, and `REDIS_PASSWORD` to reach shared infrastructure.

## Environment variables

### Required

| Variable                     | Description                                                                 |
|------------------------------|-----------------------------------------------------------------------------|
| `ADMIN_SECRET`               | Secret for `/admin/*` endpoints (environment and API key management). Generate with `openssl rand -hex 32`. |
| `CREDENTIALS_ENCRYPTION_KEY` | AES-256-GCM key for encrypting integration credentials. 64 hex chars (32 bytes). Generate with `openssl rand -hex 32`. |
| `SUBSCRIBER_HMAC_SECRET`     | HMAC-SHA256 secret for signing subscriber tokens. Required by the WS server. Generate with `openssl rand -hex 32`. |

### Optional

| Variable                     | Default                  | Description                                                                     |
|------------------------------|--------------------------|---------------------------------------------------------------------------------|
| `MONGO_URI`                  | `mongodb://localhost:27017` | MongoDB connection string.                                                   |
| `MONGO_DB`                   | `message_in_a_bottle`    | MongoDB database name.                                                          |
| `REDIS_ADDR`                 | `localhost:6379`         | Redis address.                                                                  |
| `REDIS_PASSWORD`             | *(empty)*                | Redis password.                                                                 |
| `API_PORT`                   | `3000`                   | HTTP port for the API server.                                                   |
| `WS_PORT`                    | `3001`                   | HTTP port for the WebSocket server.                                             |
| `WS_ALLOWED_ORIGINS`         | *(empty -- all origins)* | Comma-separated list of allowed WebSocket origins. **Set this in production** (e.g. `https://app.example.com,https://dashboard.example.com`). |
| `MAX_REQUEST_BODY_BYTES`     | `2097152` (2 MB)         | Maximum request body size for the API server.                                   |
| `NOTIFICATION_RETENTION_DAYS`| `90`                     | TTL in days for notification records.                                           |
| `ACTIVITY_LOG_RETENTION_DAYS`| `30`                     | TTL in days for activity log entries.                                           |
| `RATE_LIMIT_CONFIG`          | *(built-in defaults)*    | JSON object overriding per-channel rate limits. See `.env.example` for format.  |

### Default rate limits

| Channel   | Max per window | Window   |
|-----------|---------------|----------|
| email     | 50            | 60 min   |
| sms       | 10            | 60 min   |
| push      | 100           | 60 min   |
| in_app    | 200           | 60 min   |
| slack     | 30            | 60 min   |
| ms_teams  | 30            | 60 min   |

## API authentication

All API endpoints under `/api/v1` require an API key:

```
Authorization: ApiKey <your-key>
```

Keys are stored as SHA-256 hashes. Each key carries a set of fine-grained permissions (e.g. `subscribers:write`, `notifications:trigger`).

## WebSocket authentication

The WebSocket server at `/ws` uses a two-step auth handshake:

1. Client opens a plain WebSocket connection (no credentials in URL).
2. Client sends a JSON auth message as the **first message** within 10 seconds:

```json
{
  "apiKey": "<api-key>",
  "subscriberToken": "<hmac-signed-token>",
  "subscriberId": "<subscriber-external-id>"
}
```

3. Server validates the API key, verifies the HMAC subscriber token, and confirms the subscriber exists.
4. Server responds with `{"event": "authenticated"}`.
5. The connection is now live and will receive real-time notifications.

### Subscriber token format

Tokens are generated server-side and handed to the client. Format:

```
base64({"subscriberId":"<id>","timestamp":<unix_ms>}).<hex-hmac-sha256-signature>
```

- Signed with `SUBSCRIBER_HMAC_SECRET` using HMAC-SHA256
- Verified with constant-time comparison
- Expires after 24 hours

## SDK

A TypeScript client SDK is available in [`packages/sdk/`](packages/sdk/). See the [SDK README](packages/sdk/README.md) for usage.

## Build & test

Using Taskfile:

```bash
task deps         # download Go modules + install SDK npm packages
task build        # build all Go binaries (parallel)
task build:all    # build Go binaries + SDK
task test:all     # run Go + SDK tests
task docker       # build Docker image
```

Or manually:

| Component | Install          | Test                  | Build                  |
|-----------|------------------|-----------------------|------------------------|
| Go (all)  | `go mod download` | `go test ./...`       | `go build ./cmd/...`   |
| SDK       | `npm install`    | `npx vitest run`      | `npx tsup`             |

Run `task` with no arguments to see all available commands.

## Docker image

The Dockerfile produces a single image (`gcr.io/distroless/static-debian12`, non-root UID 1001) containing all three binaries. The default entrypoint is `/api` -- override it to run a different service. See [Run services separately](#run-services-separately) above.
