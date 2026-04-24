# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **Template rendering**: SMS, push, and chat channels were HTML-escaped because the template renderer used `html/template` for all channel types. Email continues to use `html/template`; all other channels now use `text/template`.
- **Multi-subscriber trigger**: Triggering a workflow for multiple subscribers in a single request now works correctly. The notifications unique index previously only covered `{environmentId, transactionId}`, causing every subscriber after the first to fail with a duplicate-key error. The index now covers `{environmentId, transactionId, subscriberId}`.
- **Double-delivery window**: The pre-check `FindByTransactionID → ErrDuplicateTransaction` was non-atomic and could allow two concurrent requests with the same `transactionId` to both enqueue worker tasks. Idempotency is now enforced solely by the MongoDB unique index on insert.
- **Broadcast memory exhaustion**: `POST /events/trigger/broadcast` previously fetched all subscribers into memory before enqueuing. Large environments would exhaust the API process. Broadcast now enqueues an async `task:broadcast` worker task that paginates subscribers in 100-record batches.
- **Digest key collision**: Digest Redis keys did not include the step index, so two digest steps in the same workflow sharing the same `digestKey` string would merge into a single digest window. Step index is now part of the key.
- **Silent errors in worker**: Several `json.Marshal` and `bson.ObjectIDFromHex` calls silently ignored errors (`_, _`), leading to zero-value ObjectIDs or empty task payloads being enqueued. All are now propagated as errors.
- **`json.Marshal` error on credentials update**: `PUT /integrations/:id` silently ignored a potential marshal error on the credentials field; now returns `400 VALIDATION_ERROR`.

### Added

- `GET /api/v1/integrations/:id` — fetch a single integration by ID (was missing while `PUT`, `DELETE`, and `PATCH` all existed).
- `broadcast_handler.go` — new async worker task handler for paginated broadcast fan-out.

### Changed

- `TriggerPayload.SubscriberIDs` changed from `[]string` to `map[string]string` (subscriberID → notificationID). The trigger worker now looks up each subscriber's own notification record directly by ID instead of using a shared `transactionId` query.
- `BroadcastTaskPayload` added to both `service` and `worker` packages for the new async broadcast flow.
- Condition evaluator `eq` / `ne` operators now use typed comparison: `nil` vs non-nil is always false, booleans are compared as booleans (not strings), and numbers are compared numerically. Only unrecognised types fall back to `fmt.Sprintf`.
- SDK `publishConfig` registry changed from GitHub Packages to npmjs.com (`"access": "public"`), removing the authentication requirement for installation.

## [0.1.0] - 2026-04-24

### Added

- Initial release: multi-channel notification platform with workflow orchestration.
- Channels: Email (SendGrid, SES, SMTP), SMS (Twilio, Vonage), Push (FCM, APNS), Chat (Slack, MS Teams), In-App (WebSocket).
- Workflow engine with delay, digest, and conditional steps.
- Subscriber management, topic fan-out, and preference overrides.
- TypeScript SDK (`@partiri-cloud/message-in-a-bottle-sdk`).
- Docker Compose single-command local development setup.
