# Deployment Guide

Complete guide for deploying Message in a Bottle, configuring providers, and integrating with the SDK.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Variables](#environment-variables)
3. [Infrastructure Setup](#infrastructure-setup)
4. [Deployment Options](#deployment-options)
5. [Bootstrapping Environments and API Keys](#bootstrapping-environments-and-api-keys)
6. [Configuring Notification Providers](#configuring-notification-providers)
7. [WebSocket Setup](#websocket-setup)
8. [SDK Integration](#sdk-integration)
9. [Rate Limiting](#rate-limiting)
10. [Security](#security)
11. [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Dependency | Version | Purpose |
|------------|---------|---------|
| MongoDB    | 7+      | Persistent storage for all data (environments, subscribers, workflows, integrations, notifications) |
| Redis      | 7+      | Task queue (Asynq) and caching |
| Go         | 1.25+   | Only needed if building from source (not required for Docker) |
| Docker     | 24+     | Only needed if deploying with Docker / Docker Compose |

---

## Environment Variables

Copy `.env.example` to `.env` and fill in the required values:

```bash
cp .env.example .env
```

### Required

These must be set before starting any service.

| Variable | Used By | Description | How to Generate |
|----------|---------|-------------|-----------------|
| `ADMIN_SECRET` | api | Secret for authenticating admin API requests (`/admin/*` endpoints). Used to manage environments and API keys. | `openssl rand -hex 32` |
| `CREDENTIALS_ENCRYPTION_KEY` | api, worker | AES-256-GCM key for encrypting provider credentials at rest. Must be exactly 64 hex characters (32 bytes). | `openssl rand -hex 32` |
| `SUBSCRIBER_HMAC_SECRET` | ws | HMAC-SHA256 secret for signing subscriber WebSocket tokens. | `openssl rand -hex 32` |

### Infrastructure

| Variable | Default | Used By | Description |
|----------|---------|---------|-------------|
| `MONGO_URI` | `mongodb://localhost:27017` | all | MongoDB connection string. Supports replica sets and SRV records. |
| `MONGO_DB` | `message_in_a_bottle` | all | MongoDB database name. |
| `REDIS_ADDR` | `localhost:6379` | all | Redis server address (`host:port`). |
| `REDIS_PASSWORD` | *(empty)* | all | Redis password. Leave empty if Redis has no auth. |

### Server

| Variable | Default | Used By | Description |
|----------|---------|---------|-------------|
| `API_PORT` | `3000` | api | HTTP port for the REST API server. |
| `WS_PORT` | `3001` | ws | HTTP port for the WebSocket server. |
| `WS_ALLOWED_ORIGINS` | *(empty = all origins)* | ws | Comma-separated list of allowed WebSocket origins for CORS. **Must be set in production.** Example: `https://app.example.com,https://dashboard.example.com` |
| `MAX_REQUEST_BODY_BYTES` | `2097152` (2 MB) | api | Maximum request body size. |

### Data Retention

| Variable | Default | Used By | Description |
|----------|---------|---------|-------------|
| `NOTIFICATION_RETENTION_DAYS` | `90` | worker | Days before notification records expire (MongoDB TTL). |
| `ACTIVITY_LOG_RETENTION_DAYS` | `30` | worker | Days before activity log entries expire (MongoDB TTL). |

### Rate Limiting

| Variable | Default | Used By | Description |
|----------|---------|---------|-------------|
| `RATE_LIMIT_CONFIG` | *(built-in defaults)* | worker | JSON object to override per-channel rate limits. See [Rate Limiting](#rate-limiting) for format and defaults. |

### Per-service summary

| Service | Required Env Vars | Optional Env Vars |
|---------|-------------------|-------------------|
| **api** | `ADMIN_SECRET`, `CREDENTIALS_ENCRYPTION_KEY`, `MONGO_URI`, `REDIS_ADDR` | `API_PORT`, `MAX_REQUEST_BODY_BYTES` |
| **ws** | `SUBSCRIBER_HMAC_SECRET`, `MONGO_URI`, `REDIS_ADDR` | `WS_PORT`, `WS_ALLOWED_ORIGINS` |
| **worker** | `CREDENTIALS_ENCRYPTION_KEY`, `MONGO_URI`, `REDIS_ADDR` | `NOTIFICATION_RETENTION_DAYS`, `ACTIVITY_LOG_RETENTION_DAYS`, `RATE_LIMIT_CONFIG` |

---

## Infrastructure Setup

### MongoDB

Recommended configuration:
- **Replica set** for production (required for change streams if you use them).
- **WiredTiger** storage engine (default since MongoDB 4.x).
- Create the database manually or let the services create it on first write.
- Indexes are created automatically by the API server on startup.

```bash
# Local development
docker run -d --name mongo -p 27017:27017 mongo:7

# With authentication
docker run -d --name mongo -p 27017:27017 \
  -e MONGO_INITDB_ROOT_USERNAME=admin \
  -e MONGO_INITDB_ROOT_PASSWORD=secret \
  mongo:7
```

If using authentication, update `MONGO_URI`:
```
MONGO_URI=mongodb://admin:secret@localhost:27017/?authSource=admin
```

### Redis

Recommended configuration:
- At least **512 MB** of memory.
- Eviction policy: `allkeys-lru` (the worker uses Redis as a task queue, so eviction of completed tasks is safe).
- **Persistence** is optional (Redis is used for queuing, not as a primary data store).

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine \
  redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
```

### Connecting to an external MongoDB

If you already have MongoDB running — on a VM, a container cluster, or any managed cloud service — set `MONGO_URI` to point at it. The app creates its indexes automatically on startup; no manual schema setup is required.

**Standard connection (no auth):**
```
MONGO_URI=mongodb://<host>:<port>
```

**With authentication:**
```
MONGO_URI=mongodb://<user>:<password>@<host>:<port>/?authSource=admin
```

**SRV record** (the format used by most managed MongoDB services):
```
MONGO_URI=mongodb+srv://<user>:<password>@<host>/?retryWrites=true&w=majority
```

**Replica set:**
```
MONGO_URI=mongodb://<host1>:27017,<host2>:27017,<host3>:27017/?replicaSet=<rs-name>
```

Replace `<host>`, `<user>`, `<password>` with your own values. Use `MONGO_DB` to set the database name (default: `message_in_a_bottle`). A replica set is recommended for production.

### Connecting to an external Redis

Set `REDIS_ADDR` (and optionally `REDIS_PASSWORD`) to point at any Redis instance. Redis is used only as a job queue — it is not primary storage, so it does not need persistence enabled.

**Standard connection:**
```
REDIS_ADDR=<host>:<port>
```

**With password:**
```
REDIS_ADDR=<host>:<port>
REDIS_PASSWORD=<password>
```

Recommended settings: at least 512 MB of memory, eviction policy `allkeys-lru`. These apply whether you are running Redis yourself or using a managed service.

---

## Deploying to the Cloud

The three services are stateless and can be deployed independently on any platform that can run Docker containers. The same Docker image is used for all three; only the entrypoint differs.

| Service | Entrypoint | Port | Needs public ingress? |
|---|---|---|---|
| API server | `/api` | 3000 | Yes |
| WebSocket server | `/ws` | 3001 | Yes |
| Worker | `/worker` | — | No |

**Generic `docker run` examples:**

```bash
# API
docker run -d -p 3000:3000 \
  -e MONGO_URI=<your-mongo-uri> \
  -e REDIS_ADDR=<host>:<port> \
  -e ADMIN_SECRET=<secret> \
  -e CREDENTIALS_ENCRYPTION_KEY=<key> \
  --entrypoint /api ghcr.io/partiri-cloud/message-in-a-bottle

# WebSocket
docker run -d -p 3001:3001 \
  -e MONGO_URI=<your-mongo-uri> \
  -e REDIS_ADDR=<host>:<port> \
  -e SUBSCRIBER_HMAC_SECRET=<secret> \
  -e WS_ALLOWED_ORIGINS=https://your-app.com \
  --entrypoint /ws ghcr.io/partiri-cloud/message-in-a-bottle

# Worker (no port needed)
docker run -d \
  -e MONGO_URI=<your-mongo-uri> \
  -e REDIS_ADDR=<host>:<port> \
  -e CREDENTIALS_ENCRYPTION_KEY=<key> \
  --entrypoint /worker ghcr.io/partiri-cloud/message-in-a-bottle
```

Or use the provided app-only Compose file, which expects `MONGO_URI` and `REDIS_ADDR` to be set in your `.env`:

```bash
docker compose -f docker-compose.app.yml up
```

**Key points:**
- Only the API and WebSocket servers need public ingress. The Worker is a background process with no exposed port.
- Set `WS_ALLOWED_ORIGINS` to the domain(s) of your frontend application.
- All three services connect to the same MongoDB database and Redis instance.
- See [Environment Variables](#environment-variables) for the full list of required and optional settings.

---

## Deployment Options

### Option 1: Docker Compose (recommended for getting started)

Starts MongoDB, Redis, and all three services:

```bash
# 1. Create your .env file
cp .env.example .env
# Edit .env and fill in ADMIN_SECRET, CREDENTIALS_ENCRYPTION_KEY, and SUBSCRIBER_HMAC_SECRET

# 2. Start everything
docker compose up -d

# 3. Check logs
docker compose logs -f api
docker compose logs -f worker
```

The `docker-compose.yml` defines these services:
- `mongo` -- MongoDB 7 on port 27017
- `redis` -- Redis 7 on port 6379
- `api` -- API server on port 3000
- `ws` -- WebSocket server on port 3001
- `worker` -- Background job processor (no port)

### Option 2: Build from source

```bash
# Install dependencies
task deps

# Build all binaries (produces bin/api, bin/ws, bin/worker)
task build

# Source your .env
export $(grep -v '^#' .env | xargs)

# Run all three processes (Ctrl-C stops all)
task run
```

### Option 3: Run services independently

Each service can run on a different host. Build and deploy each binary separately:

```bash
task build:api
task build:ws
task build:worker
```

Run each with only its required env vars:

```bash
# API server (port 3000)
MONGO_URI=mongodb://db:27017 REDIS_ADDR=redis:6379 \
  ADMIN_SECRET=<secret> CREDENTIALS_ENCRYPTION_KEY=<key> \
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

### Option 4: Docker (individual containers)

```bash
docker build -t miab .

# API
docker run -d -p 3000:3000 --env-file .env --entrypoint /api miab

# WebSocket
docker run -d -p 3001:3001 --env-file .env --entrypoint /ws miab

# Worker
docker run -d --env-file .env --entrypoint /worker miab
```

The Docker image uses `gcr.io/distroless/static-debian12` as the base, runs as non-root (UID 1001), and contains all three binaries. Override the entrypoint to select which service to run.

---

## Bootstrapping Environments and API Keys

Before you can use the notification API, you need to create an **environment** with an **API key**. This is done via the admin API endpoints, which are protected by the `ADMIN_SECRET` environment variable.

### Admin API authentication

All `/admin/*` endpoints require the admin secret in the `Authorization` header:

```
Authorization: AdminSecret <value-of-ADMIN_SECRET>
```

The API server **will not start** if `ADMIN_SECRET` is not set.

### Admin API endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/admin/environments` | Create a new environment with an initial API key |
| `GET` | `/admin/environments` | List all environments and their key metadata |
| `POST` | `/admin/environments/:identifier/keys` | Add a new API key to an existing environment |

### Create an environment

```bash
curl -X POST http://localhost:3000/admin/environments \
  -H "Authorization: AdminSecret <your-admin-secret>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production",
    "identifier": "production"
  }'
```

Response (201):
```json
{
  "data": {
    "id": "680123abc...",
    "name": "Production",
    "identifier": "production",
    "apiKey": "mib_a3f8c91d..."
  }
}
```

**Save the `apiKey` immediately.** It is only returned once. The key is stored as a SHA-256 hash in MongoDB and cannot be recovered.

### Add additional API keys

```bash
curl -X POST http://localhost:3000/admin/environments/production/keys \
  -H "Authorization: AdminSecret <your-admin-secret>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ci-deploy"
  }'
```

Response (201):
```json
{
  "data": {
    "name": "ci-deploy",
    "apiKey": "mib_b7e2d41f..."
  }
}
```

### List environments

```bash
curl http://localhost:3000/admin/environments \
  -H "Authorization: AdminSecret <your-admin-secret>"
```

Response (200):
```json
{
  "data": [
    {
      "id": "680123abc...",
      "name": "Production",
      "identifier": "production",
      "apiKeys": [
        {
          "name": "default",
          "isActive": true,
          "createdAt": "2026-04-05T10:30:00Z",
          "expiresAt": null,
          "lastUsedAt": "2026-04-05T12:00:00Z"
        },
        {
          "name": "ci-deploy",
          "isActive": true,
          "createdAt": "2026-04-05T11:00:00Z",
          "expiresAt": null,
          "lastUsedAt": null
        }
      ],
      "createdAt": "2026-04-05T10:30:00Z"
    }
  ]
}
```

Key hashes are never returned in any response.

### Using API keys

All notification API endpoints under `/api/v1` require an API key in the `Authorization` header:

```
Authorization: ApiKey mib_a3f8c91d...
```

API keys carry fine-grained permissions. Keys created via the admin API are granted all permissions.

---

## Configuring Notification Providers

Provider credentials are configured at runtime via the REST API, **not** environment variables. Credentials are encrypted with AES-256-GCM using the `CREDENTIALS_ENCRYPTION_KEY` before storage.

### Integration API endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/integrations` | Create a new integration |
| `GET` | `/api/v1/integrations` | List all integrations |
| `PUT` | `/api/v1/integrations/:id` | Update an integration |
| `DELETE` | `/api/v1/integrations/:id` | Delete an integration |
| `PATCH` | `/api/v1/integrations/:id/primary` | Set as primary for its channel |

### Integration request format

```json
{
  "channel": "<channel>",
  "providerId": "<provider_id>",
  "name": "<display name>",
  "credentials": { ... },
  "isPrimary": true,
  "metadata": {
    "senderName": "My App",
    "senderEmail": "noreply@example.com"
  }
}
```

The `metadata` fields (`senderName`, `senderEmail`) are used by email providers as the "From" address.

### Email Providers

#### SMTP

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "providerId": "smtp",
    "name": "SMTP Server",
    "credentials": {
      "host": "smtp.example.com",
      "port": 587,
      "user": "smtp-username",
      "password": "smtp-password",
      "secure": true
    },
    "isPrimary": true,
    "metadata": {
      "senderName": "My App",
      "senderEmail": "noreply@example.com"
    }
  }'
```

| Credential | Type | Description |
|------------|------|-------------|
| `host` | string | SMTP server hostname |
| `port` | int | SMTP server port (typically 587 for STARTTLS, 465 for SSL) |
| `user` | string | SMTP username |
| `password` | string | SMTP password or app-specific password |
| `secure` | bool | Whether to use TLS |

#### SendGrid

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "providerId": "sendgrid",
    "name": "SendGrid",
    "credentials": {
      "apiKey": "SG.xxxxxxxxxxxx"
    },
    "isPrimary": true,
    "metadata": {
      "senderName": "My App",
      "senderEmail": "noreply@example.com"
    }
  }'
```

| Credential | Type | Description |
|------------|------|-------------|
| `apiKey` | string | SendGrid API key (starts with `SG.`) |

#### AWS SES

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "providerId": "ses",
    "name": "AWS SES",
    "credentials": {
      "accessKeyId": "AKIAIOSFODNN7EXAMPLE",
      "secretAccessKey": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
      "region": "eu-west-1"
    },
    "isPrimary": true,
    "metadata": {
      "senderName": "My App",
      "senderEmail": "noreply@example.com"
    }
  }'
```

| Credential | Type | Description |
|------------|------|-------------|
| `accessKeyId` | string | AWS access key ID |
| `secretAccessKey` | string | AWS secret access key |
| `region` | string | AWS region (e.g., `us-east-1`, `eu-west-1`) |

### SMS Providers

#### Twilio

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "sms",
    "providerId": "twilio",
    "name": "Twilio",
    "credentials": {
      "accountSid": "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
      "authToken": "your_auth_token",
      "fromNumber": "+15551234567"
    },
    "isPrimary": true
  }'
```

| Credential | Type | Description |
|------------|------|-------------|
| `accountSid` | string | Twilio Account SID (starts with `AC`) |
| `authToken` | string | Twilio Auth Token |
| `fromNumber` | string | Twilio phone number in E.164 format |

#### Vonage

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "sms",
    "providerId": "vonage",
    "name": "Vonage",
    "credentials": {
      "apiKey": "your_api_key",
      "apiSecret": "your_api_secret",
      "fromNumber": "+15551234567"
    },
    "isPrimary": true
  }'
```

| Credential | Type | Description |
|------------|------|-------------|
| `apiKey` | string | Vonage API key |
| `apiSecret` | string | Vonage API secret |
| `fromNumber` | string | Sender phone number in E.164 format |

### Push Notification Providers

#### Firebase Cloud Messaging (FCM)

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "push",
    "providerId": "fcm",
    "name": "Firebase",
    "credentials": {
      "serviceAccountJson": "{\"type\":\"service_account\",\"project_id\":\"my-project\",...}"
    },
    "isPrimary": true
  }'
```

| Credential | Type | Description |
|------------|------|-------------|
| `serviceAccountJson` | string | Firebase service account JSON (as a string, not an object). Download from Firebase Console > Project Settings > Service Accounts. |

#### Apple Push Notification Service (APNS)

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "push",
    "providerId": "apns",
    "name": "APNS",
    "credentials": {
      "keyId": "ABC123DEFG",
      "teamId": "ABCDE12345",
      "privateKey": "-----BEGIN PRIVATE KEY-----\nMIGT...\n-----END PRIVATE KEY-----",
      "bundleId": "com.example.myapp"
    },
    "isPrimary": true
  }'
```

| Credential | Type | Description |
|------------|------|-------------|
| `keyId` | string | APNS key ID (10 characters) |
| `teamId` | string | Apple Developer Team ID |
| `privateKey` | string | APNS `.p8` private key contents (PEM format) |
| `bundleId` | string | Your app's bundle identifier |

### Chat Providers

#### Slack (Incoming Webhook)

Slack integrations use webhook URLs. The webhook URL is passed as the subscriber's contact address (the `To` field), not stored as a credential.

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "chat",
    "providerId": "slack_webhook",
    "name": "Slack",
    "credentials": {},
    "isPrimary": true
  }'
```

When triggering a notification, set the subscriber's Slack contact to the webhook URL:
```
https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX
```

#### Microsoft Teams (Incoming Webhook)

Same pattern as Slack -- the webhook URL is the subscriber's contact address.

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "chat",
    "providerId": "ms_teams_webhook",
    "name": "MS Teams",
    "credentials": {},
    "isPrimary": true
  }'
```

### Debug Provider

#### Log (stdout)

Logs all notifications to stdout instead of sending them. Useful for development and testing.

```bash
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "providerId": "log",
    "name": "Debug Logger",
    "credentials": {},
    "isPrimary": true
  }'
```

### Provider reference table

| Provider | Channel | `providerId` | Credentials |
|----------|---------|-------------|-------------|
| SMTP | `email` | `smtp` | `host`, `port`, `user`, `password`, `secure` |
| SendGrid | `email` | `sendgrid` | `apiKey` |
| AWS SES | `email` | `ses` | `accessKeyId`, `secretAccessKey`, `region` |
| Twilio | `sms` | `twilio` | `accountSid`, `authToken`, `fromNumber` |
| Vonage | `sms` | `vonage` | `apiKey`, `apiSecret`, `fromNumber` |
| FCM | `push` | `fcm` | `serviceAccountJson` |
| APNS | `push` | `apns` | `keyId`, `teamId`, `privateKey`, `bundleId` |
| Slack | `chat` | `slack_webhook` | *(empty -- webhook URL is the subscriber contact)* |
| MS Teams | `chat` | `ms_teams_webhook` | *(empty -- webhook URL is the subscriber contact)* |
| Log | any | `log` | *(empty -- logs to stdout)* |

### Primary integrations

Each channel can have one primary integration. The worker uses the primary integration for that channel when sending notifications. Set `"isPrimary": true` when creating, or update later:

```bash
curl -X PATCH http://localhost:3000/api/v1/integrations/<id>/primary \
  -H "Authorization: ApiKey <your-key>"
```

---

## Sending Your First Notification

Once you have an environment, an API key, and at least one provider configured, follow these steps to send a notification end-to-end.

### Step 1: Create a subscriber

Subscribers are the recipients of notifications. Each subscriber needs a unique `subscriberId` (your internal user ID).

```bash
curl -X POST http://localhost:3000/api/v1/subscribers \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "subscriberId": "user-123",
    "email": "jane@example.com",
    "firstName": "Jane",
    "lastName": "Doe"
  }'
```

Response (201):
```json
{
  "data": {
    "subscriberId": "user-123",
    "email": "jane@example.com",
    "firstName": "Jane",
    "lastName": "Doe",
    "createdAt": "2026-04-05T10:45:00Z"
  }
}
```

You can also set phone numbers, push tokens, Slack/Teams webhook URLs, custom data, locale, and timezone. See the full subscriber schema:

| Field | Type | Description |
|-------|------|-------------|
| `subscriberId` | string | **Required.** Your internal user identifier. |
| `email` | string | Email address (used by email providers). |
| `phone` | string | Phone number in E.164 format (used by SMS providers). |
| `firstName` | string | First name (available in templates). |
| `lastName` | string | Last name (available in templates). |
| `locale` | string | Preferred locale (e.g., `en`, `pt-BR`). Used for template localization. |
| `timezone` | string | IANA timezone (e.g., `Europe/Lisbon`). |
| `data` | object | Custom key-value data accessible in templates and conditions. |
| `channels.push.fcmTokens` | string[] | Firebase Cloud Messaging device tokens. |
| `channels.push.apnsTokens` | string[] | Apple Push Notification device tokens. |
| `channels.slack.webhookUrl` | string | Slack incoming webhook URL. |
| `channels.msTeams.webhookUrl` | string | MS Teams incoming webhook URL. |

### Step 2: Create a workflow

Workflows define multi-step notification pipelines. Each step specifies a channel and an inline template.

```bash
curl -X POST http://localhost:3000/api/v1/workflows \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "identifier": "welcome-email",
    "name": "Welcome Email",
    "steps": [
      {
        "type": "email",
        "order": 1,
        "template": {
          "subject": { "en": "Welcome to {{.Payload.appName}}, {{.Subscriber.FirstName}}!" },
          "body": { "en": "<h1>Hello {{.Subscriber.FirstName}}</h1><p>Thanks for signing up for {{.Payload.appName}}.</p>" }
        }
      }
    ]
  }'
```

Response (201):
```json
{
  "data": {
    "id": "...",
    "identifier": "welcome-email",
    "name": "Welcome Email",
    "isActive": true,
    "steps": [ ... ],
    "createdAt": "2026-04-05T10:50:00Z"
  }
}
```

#### Workflow step types

| Type | Description | Config |
|------|-------------|--------|
| `email` | Send an email | `template` with `subject` and `body` |
| `sms` | Send an SMS | `template` with `content` |
| `push` | Send a push notification | `template` with `subject` and `body` |
| `in_app` | Deliver to WebSocket feed | `template` with `subject` and `content` |
| `chat` | Send to Slack/Teams | `template` with `content` |
| `delay` | Pause before the next step | `delayConfig` with `amount` and `unit` (`seconds`, `minutes`, `hours`, `days`) |
| `digest` | Batch notifications over a window | `digestConfig` with `amount`, `unit`, and `digestKey` |

#### Multi-step workflow example

```bash
curl -X POST http://localhost:3000/api/v1/workflows \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "identifier": "order-confirmation",
    "name": "Order Confirmation",
    "steps": [
      {
        "type": "email",
        "order": 1,
        "template": {
          "subject": { "en": "Order #{{.Payload.orderId}} confirmed" },
          "body": { "en": "<p>Hi {{.Subscriber.FirstName}}, your order #{{.Payload.orderId}} for {{.Payload.total}} has been confirmed.</p>" }
        }
      },
      {
        "type": "in_app",
        "order": 2,
        "template": {
          "subject": { "en": "Order confirmed" },
          "content": { "en": "Your order #{{.Payload.orderId}} is confirmed." }
        }
      },
      {
        "type": "delay",
        "order": 3,
        "delayConfig": { "amount": 30, "unit": "minutes" }
      },
      {
        "type": "sms",
        "order": 4,
        "template": {
          "content": { "en": "Your order #{{.Payload.orderId}} is being prepared." }
        },
        "conditions": [
          { "field": "subscriber.phone", "operator": "isNotEmpty", "value": true }
        ]
      }
    ]
  }'
```

This workflow: sends an email, delivers an in-app notification, waits 30 minutes, then sends an SMS only if the subscriber has a phone number.

### Step 3: Trigger a notification

```bash
curl -X POST http://localhost:3000/api/v1/events/trigger \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "workflowIdentifier": "welcome-email",
    "to": [
      { "subscriberId": "user-123" }
    ],
    "payload": {
      "appName": "My App",
      "firstName": "Jane"
    }
  }'
```

The worker picks up the event, resolves the workflow steps, renders templates with the payload, and delivers via the configured providers.

#### Trigger to a topic (group)

You can also send to all subscribers in a topic:

```bash
# First, create a topic
curl -X POST http://localhost:3000/api/v1/topics \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{ "key": "beta-users", "name": "Beta Users" }'

# Add subscribers to the topic
curl -X POST http://localhost:3000/api/v1/topics/beta-users/subscribers \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{ "subscriberIds": ["user-123", "user-456"] }'

# Trigger to the topic
curl -X POST http://localhost:3000/api/v1/events/trigger \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "workflowIdentifier": "welcome-email",
    "to": [
      { "type": "Topic", "topicKey": "beta-users" }
    ],
    "payload": {
      "appName": "My App",
      "firstName": "there"
    }
  }'
```

#### Broadcast to all subscribers

```bash
curl -X POST http://localhost:3000/api/v1/events/trigger/broadcast \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "workflowIdentifier": "welcome-email",
    "payload": { "appName": "My App", "firstName": "there" }
  }'
```

### Step 4: Check delivery status

```bash
# List recent notifications
curl "http://localhost:3000/api/v1/notifications?page=1&limit=10" \
  -H "Authorization: ApiKey <your-api-key>"

# Check activity log
curl "http://localhost:3000/api/v1/activity?page=1&limit=10" \
  -H "Authorization: ApiKey <your-api-key>"
```

---

## WebSocket Setup

The WebSocket server provides real-time in-app notification delivery.

### Connection flow

1. **Client opens a WebSocket connection** to `ws://host:3001/ws` (no credentials in the URL).
2. **Client sends a JSON auth message** as the first message within 10 seconds:
   ```json
   {
     "apiKey": "<api-key>",
     "subscriberToken": "<hmac-signed-token>",
     "subscriberId": "<subscriber-external-id>"
   }
   ```
3. **Server validates** the API key, verifies the HMAC subscriber token, and confirms the subscriber exists.
4. **Server responds** with `{"event": "authenticated"}`.
5. **Connection is live** and receives real-time notifications.

### Subscriber tokens

Tokens are generated server-side and handed to the client application. Format:

```
base64({"subscriberId":"<id>","timestamp":<unix_ms>}).<hex-hmac-sha256-signature>
```

- Signed with `SUBSCRIBER_HMAC_SECRET` using HMAC-SHA256
- Verified with constant-time comparison
- Expire after 24 hours

### CORS

Set `WS_ALLOWED_ORIGINS` to restrict which origins can establish WebSocket connections:

```
WS_ALLOWED_ORIGINS=https://app.example.com,https://dashboard.example.com
```

If empty, **all origins are accepted** -- this is not safe for production.

---

## SDK Integration

A TypeScript client SDK is available in `packages/sdk/`. Install it in your project:

```bash
npm install @partiri-cloud/message-in-a-bottle-sdk
```

See the [SDK README](packages/sdk/README.md) for full API reference.

The SDK provides methods for:
- Triggering notifications
- Managing subscribers
- Managing workflows and templates
- WebSocket real-time feed

---

## Rate Limiting

The worker enforces per-channel rate limits to prevent abuse and respect provider limits.

### Default limits

| Channel | Max per Window | Window |
|---------|---------------|--------|
| `email` | 50 | 60 min |
| `sms` | 10 | 60 min |
| `push` | 100 | 60 min |
| `in_app` | 200 | 60 min |
| `slack` | 30 | 60 min |
| `ms_teams` | 30 | 60 min |

### Overriding limits

Set `RATE_LIMIT_CONFIG` as a JSON object:

```bash
RATE_LIMIT_CONFIG='{"email":{"maxPerWindow":100,"windowMinutes":60},"sms":{"maxPerWindow":20,"windowMinutes":60}}'
```

Only specify the channels you want to override -- unspecified channels keep their defaults.

---

## Security

### Credential encryption

All provider credentials (SMTP passwords, API keys, etc.) are encrypted with **AES-256-GCM** before being stored in MongoDB. The encryption uses:
- The `CREDENTIALS_ENCRYPTION_KEY` environment variable (64 hex chars = 32-byte key)
- A random 12-byte nonce per encryption operation
- Authenticated encryption (ciphertext cannot be tampered with)

Credentials are decrypted only in memory by the worker when it needs to send a notification. They are never returned in API responses.

### API key security

- API keys are stored as **SHA-256 hashes** in MongoDB -- the raw key is never stored.
- Keys are only returned once when created via the admin API.
- Each key carries fine-grained permissions.
- Keys support optional expiration dates.
- `lastUsedAt` is updated (debounced to every 5 minutes) for monitoring.

### Docker security

The Docker image runs as **non-root** (UID 1001) using a distroless base image (`gcr.io/distroless/static-debian12`), minimizing the attack surface.

### Key rotation

To rotate the `CREDENTIALS_ENCRYPTION_KEY`:
1. All existing encrypted credentials would need to be re-encrypted with the new key.
2. This is not currently automated -- plan for it if you need regular rotation.
3. The `SUBSCRIBER_HMAC_SECRET` can be rotated freely, but all existing subscriber tokens will become invalid (they expire after 24 hours anyway).

---

## Troubleshooting

### "ADMIN_SECRET is required"

The API server requires `ADMIN_SECRET` to be set. Generate one with:
```bash
openssl rand -hex 32
```

### "failed to load config" or "invalid CREDENTIALS_ENCRYPTION_KEY"

The `CREDENTIALS_ENCRYPTION_KEY` must be exactly 64 hex characters (32 bytes). Generate with:
```bash
openssl rand -hex 32
```

### "failed to connect to mongodb" / "context deadline exceeded"

- Check that MongoDB is running and reachable at the address in `MONGO_URI`.
- If using Docker Compose, ensure the `mongo` service is healthy before starting app services.
- Verify network connectivity: `mongosh mongodb://localhost:27017`

### API returns 401 Unauthorized

- For `/api/v1/*` endpoints: check the header format `Authorization: ApiKey <raw-key>`.
- For `/admin/*` endpoints: check the header format `Authorization: AdminSecret <secret>`.
- Verify the API key was created via the admin API and is still active.
- Ensure the key has the required permissions for the endpoint.

### Notifications not being sent

- Check that the **worker** process is running.
- Verify an integration exists for the channel and is set as primary.
- Check worker logs for delivery errors.
- Ensure Redis is reachable (the worker uses Redis/Asynq for task queuing).

### WebSocket connection drops immediately

- Check that `WS_ALLOWED_ORIGINS` includes your client's origin.
- Verify the auth message is sent within 10 seconds of connecting.
- Check that the subscriber token has not expired (24-hour TTL).
- Ensure the subscriber exists in the database.

### Rate limit errors

- Check current rate limits with `RATE_LIMIT_CONFIG`.
- Rate limits are per-environment, per-channel. If you're hitting limits, either increase them or batch your notifications using digest workflows.

---

## Quick Reference: Complete Setup Checklist

```bash
# 1. Clone and enter the project
cd message-in-a-bottle

# 2. Create .env from template
cp .env.example .env

# 3. Generate secrets (run each openssl command and paste into .env)
openssl rand -hex 32  # -> ADMIN_SECRET
openssl rand -hex 32  # -> CREDENTIALS_ENCRYPTION_KEY
openssl rand -hex 32  # -> SUBSCRIBER_HMAC_SECRET

# 4. Start infrastructure + services
docker compose up -d
# Or: task dev && task build && task run

# 5. Create an environment via the admin API
curl -X POST http://localhost:3000/admin/environments \
  -H "Authorization: AdminSecret <your-admin-secret>" \
  -H "Content-Type: application/json" \
  -d '{"name": "Production", "identifier": "production"}'
# Save the returned apiKey!

# 6. Configure an email provider (e.g., SMTP)
curl -X POST http://localhost:3000/api/v1/integrations \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "providerId": "smtp",
    "name": "SMTP",
    "credentials": {
      "host": "smtp.example.com",
      "port": 587,
      "user": "user",
      "password": "pass",
      "secure": true
    },
    "isPrimary": true,
    "metadata": {
      "senderName": "My App",
      "senderEmail": "noreply@example.com"
    }
  }'

# 7. Create a subscriber
curl -X POST http://localhost:3000/api/v1/subscribers \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{"subscriberId":"user-1","email":"you@example.com","firstName":"Your Name"}'

# 8. Create a workflow
curl -X POST http://localhost:3000/api/v1/workflows \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "identifier": "test-email",
    "name": "Test Email",
    "steps": [{
      "type": "email", "order": 1,
      "template": {
        "subject": {"en": "Hello {{.Payload.firstName}}"},
        "body": {"en": "<p>This is a test notification for {{.Payload.firstName}}.</p>"}
      }
    }]
  }'

# 9. Send a notification
curl -X POST http://localhost:3000/api/v1/events/trigger \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "workflowIdentifier": "test-email",
    "to": [{"subscriberId": "user-1"}],
    "payload": {"firstName": "Your Name"}
  }'
# Check your inbox!

# 10. Set WS_ALLOWED_ORIGINS for production
# Edit .env and restart the ws service
```
