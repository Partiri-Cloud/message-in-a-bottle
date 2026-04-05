# Worker

Background job processor that consumes tasks from Redis (via Asynq) and delivers notifications through configured providers. No exposed ports.

## Task types

| Task              | Queue      | Description                                                |
|-------------------|------------|------------------------------------------------------------|
| `task:trigger`    | `default`  | Evaluate workflow steps and fan out per-subscriber tasks    |
| `task:delivery`   | `critical` | Send notification via provider (email, SMS, push, chat)    |
| `task:delay`      | `low`      | Wait, then enqueue subsequent delivery steps               |
| `task:digest`     | `low`      | Collect notifications into a batch, then deliver            |

Queue concurrency: `critical` 6, `default` 3, `low` 1.

Failed deliveries retry with exponential backoff (30s base, 4x multiplier, max 3 attempts).

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

| Variable           | Default              | Description                                                                |
|--------------------|----------------------|----------------------------------------------------------------------------|
| `RATE_LIMIT_CONFIG`| *(built-in defaults)* | JSON object overriding per-channel rate limits. See `.env.example`.       |

#### Default rate limits

| Channel   | Max per window | Window |
|-----------|---------------|--------|
| email     | 50            | 60 min |
| sms       | 10            | 60 min |
| push      | 100           | 60 min |
| in_app    | 200           | 60 min |
| slack     | 30            | 60 min |
| ms_teams  | 30            | 60 min |

### Not used by this service

`SUBSCRIBER_HMAC_SECRET`, `WS_ALLOWED_ORIGINS`, `API_PORT`, `WS_PORT`, `MAX_REQUEST_BODY_BYTES` -- these are used by the API or WS server only.

## Run

```bash
# Standalone
CREDENTIALS_ENCRYPTION_KEY=<key> ./worker

# Docker
docker run --env-file .env --entrypoint /worker partiri/message-in-a-bottle
```
