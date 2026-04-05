# WebSocket Server

Real-time notification delivery to connected subscribers via WebSocket.

## Endpoints

- `GET /health` -- health check
- `GET /ws` -- WebSocket upgrade

## Authentication

Clients connect to `/ws` with no credentials in the URL, then authenticate via the first message:

```json
{
  "apiKey": "<api-key>",
  "subscriberToken": "<hmac-signed-token>",
  "subscriberId": "<subscriber-id>"
}
```

The server validates the API key, verifies the HMAC subscriber token against `SUBSCRIBER_HMAC_SECRET`, and responds with `{"event": "authenticated"}`. Clients must authenticate within 10 seconds or the connection is closed.

## Environment variables

### Required

| Variable                 | Description                                                                                |
|--------------------------|--------------------------------------------------------------------------------------------|
| `SUBSCRIBER_HMAC_SECRET` | HMAC-SHA256 secret for verifying subscriber tokens. `openssl rand -hex 32`                 |

### Required infrastructure

| Variable         | Default                     | Description              |
|------------------|-----------------------------|--------------------------|
| `MONGO_URI`      | `mongodb://localhost:27017` | MongoDB connection URI   |
| `MONGO_DB`       | `message_in_a_bottle`       | MongoDB database name    |
| `REDIS_ADDR`     | `localhost:6379`            | Redis address            |
| `REDIS_PASSWORD` | *(empty)*                   | Redis password           |

### Optional

| Variable             | Default        | Description                                                                                          |
|----------------------|----------------|------------------------------------------------------------------------------------------------------|
| `WS_PORT`            | `3001`         | HTTP listen port                                                                                     |
| `WS_ALLOWED_ORIGINS` | *(all origins)* | Comma-separated allowed origins (e.g. `https://app.example.com`). **Set this in production.**       |

### Not used by this service

`CREDENTIALS_ENCRYPTION_KEY`, `API_PORT`, `MAX_REQUEST_BODY_BYTES`, `NOTIFICATION_RETENTION_DAYS`, `ACTIVITY_LOG_RETENTION_DAYS`, `RATE_LIMIT_CONFIG` -- these are used by the API server or worker only.

## Run

```bash
# Standalone
SUBSCRIBER_HMAC_SECRET=<secret> ./ws

# Docker
docker run -p 3001:3001 --env-file .env --entrypoint /ws partiri/message-in-a-bottle
```
