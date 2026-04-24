# API Server

REST API backend for all CRUD operations and event triggering. Listens on HTTP.

## Endpoints

- `GET /health` -- health check
- `/api/v1/*` -- all resources (subscribers, workflows, integrations, templates, topics, preferences, notifications, events)

All `/api/v1` endpoints require `Authorization: ApiKey <key>` header.

## Environment variables

### Required

| Variable                     | Description                                                      |
|------------------------------|------------------------------------------------------------------|
| `CREDENTIALS_ENCRYPTION_KEY` | AES-256-GCM key (64 hex chars). `openssl rand -hex 32`          |

### Required infrastructure

| Variable         | Default                     | Description              |
|------------------|-----------------------------|--------------------------|
| `MONGO_URI`      | `mongodb://localhost:27017` | MongoDB connection URI   |
| `MONGO_DB`       | `message_in_a_bottle`       | MongoDB database name    |
| `REDIS_ADDR`     | `localhost:6379`            | Redis address            |
| `REDIS_PASSWORD` | *(empty)*                   | Redis password           |

### Optional

| Variable                      | Default          | Description                                      |
|-------------------------------|------------------|--------------------------------------------------|
| `API_PORT`                    | `3000`           | HTTP listen port                                 |
| `MAX_REQUEST_BODY_BYTES`      | `2097152` (2 MB) | Maximum request body size                        |
| `NOTIFICATION_RETENTION_DAYS` | `90`             | TTL for notification records (days)              |
| `ACTIVITY_LOG_RETENTION_DAYS` | `30`             | TTL for activity log entries (days)              |

### Not used by this service

`SUBSCRIBER_HMAC_SECRET`, `WS_ALLOWED_ORIGINS`, `WS_PORT`, `RATE_LIMIT_CONFIG` -- these are used by the WS server or worker only.

## Run

```bash
# Standalone
CREDENTIALS_ENCRYPTION_KEY=<key> ./api

# Docker
docker run -p 3000:3000 --env-file .env --entrypoint /api ghcr.io/partiri-cloud/message-in-a-bottle
```
