# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **API responses were PascalCase**: handlers serialize the persistence models directly, and those models carried BSON tags but no JSON tags — so Go fell back to the field names and the API answered `{"WorkflowID": …, "Channels": …}`. Every camelCase client (the SDK, the dashboard) read `undefined` and silently rendered empty state. All models now carry explicit JSON tags, pinned by tests in `internal/model/json_test.go`.
- **Preferences could not be saved**: `PATCH /subscribers/:id/preferences/:workflowId` ran the path parameter through `bson.ObjectIDFromHex`, so the workflow identifiers clients actually hold (`deploy-started`, the same slug a trigger carries) were rejected with `400 VALIDATION_ERROR`. The route now resolves an identifier first and falls back to an ObjectID hex; an unknown workflow returns `404`. `GET …/preferences` returns `workflowIdentifier` alongside `workflowId`.
- **Partial preference updates disabled unlisted channels**: `ChannelPrefsDTO` used non-pointer bools, so `{"channels":{"email":true}}` bound `inApp`/`sms`/`push` to `false` and the upsert persisted them — turning email on silently turned in-app off. Channel fields are now pointers, and only the channels a request actually names are written.
- **Concurrent preference updates lost each other**: writing the full six-channel document back meant two simultaneous PATCHes (one disabling email, one disabling SMS) each merged onto the same stale row, and the second silently reverted the first — both answering `200`. `PreferenceRepository.UpdateChannels` now writes one dotted `$set` path per named channel, so the two updates are independent; the channels a request does not name are seeded via `$setOnInsert` and left untouched on an existing row.
- **Preference writes on inactive workflows were invisible**: `GET …/preferences` lists only active workflows, but `PATCH …/preferences/:workflowId` resolved workflows without an `isActive` filter — so a write against a deactivated workflow returned `200` and stored a row that no read could ever surface, silently governing delivery again on reactivation. Inactive workflows now return `404`, matching the read path.
- **Preferences read as "everything on" when nothing was stored**: `GET /subscribers/:id/preferences` returned only the stored rows, so a client had no way to see a workflow's declared defaults and had to guess. A guess of "enabled" is wrong for every workflow whose defaults disable a channel, and the settings page duly showed channels as on that the server had off. It now returns one row per active workflow plus the global preference, with `channels` already resolved (explicit workflow choice, else workflow defaults filtered through the global opt-out mask) and an `explicit` flag saying whether the subscriber ever chose. `engine.ResolveChannelPrefs` is the single source of that precedence, shared by the read path, the update merge base, and delivery, so the three cannot drift.
- **A workflow-scoped preference update reversed a global opt-out**: the merge base was the workflow's defaults, but a workflow row shadows the global row outright at delivery time (`engine.IsChannelEnabled`). A subscriber who globally disabled email and then toggled any single channel on one workflow had email switched back on for it. The base is now the resolved effective state.
- **The global preference replaced workflow defaults wholesale**: a stored global row overrode every workflow's declared defaults outright, so it could *enable* channels a workflow had off — and a first global update of a single channel (merging onto the zero value) silently opted the subscriber out of everything else. The global preference is now a per-channel **opt-out mask** ANDed with the workflow defaults: it can silence a channel, never enable one, and an explicit workflow row still wins outright. A first global write merges onto the all-allowed identity, so `{"sms": false}` stores exactly that one opt-out.

  **Upgrade note.** This reinterprets any global row already in the database. A row written under the old semantics to *enable* a channel over a workflow's defaults (`sms: true` where the workflow has `sms: false`) will now silence it instead. Before deploying, check for such rows:

  ```js
  db.subscriber_preferences.find({ workflowId: null })
  ```

  If the query returns nothing, there is nothing to migrate.
- **Subscriber upserts clobbered stored state**: `$set` carried the whole struct, so re-posting a subscriber without channel data replaced `channels` wholesale — erasing push tokens and Slack/Teams webhooks — reset `locale` to `en`, and flipped `isOnline` to `false` on a subscriber with a live WebSocket. `locale` and `isOnline` are now omitted from `$set` when unsupplied and seeded via `$setOnInsert`. `POST /subscribers/bulk` no longer forces `locale` either, matching `POST /subscribers`.
- **Posting one channel config wiped the others**: `$set` on the nested `channels` document replaced it wholesale, so a payload carrying only a Slack webhook erased the subscriber's stored FCM/APNS push tokens. Channel config is now flattened generically into one dotted path per leaf, so it merges per field and a channel added to the model merges without touching the repository. (Corollary: a profile upsert can no longer clear channel config.)
- **A second device evicted the first device's push token**: the token arrays were `$set` wholesale, so device B registering `[tok-b]` replaced device A's `[tok-a]` and push silently stopped reaching the first device. Token arrays now merge with `$addToSet`, which also stops a re-registered token being stored twice. Since the upsert can now only *add* tokens, removal gets its own route (see Added) — otherwise a token from an uninstalled app would live forever and the worker would keep pushing to a dead device.
- **Concurrent first preference writes returned 500**: an upsert that inserts races the unique index on `{environmentId, subscriberId, workflowId}`, so two simultaneous first-time toggles for the same subscriber left one with a duplicate-key error and a dropped change. `UpdateChannels` now retries once on a duplicate key — the row exists by then, so the retry takes the update path and merges cleanly.
- **An empty preference payload created a row that changed nothing**: `{"channels":{}}` (or a payload whose channels are all `null`) passed validation and still inserted a row, promoting a subscriber who was inheriting into one with an explicit choice they never made — which then shadows their global opt-out mask at delivery time. A payload naming no channel is now `400`, and an unknown channel name (`"emial"`) is `400` rather than a silently ignored key.
- **No CORS**: the browser SDK runs on a different origin, so its preflight `OPTIONS` hit an unrouted method and gin answered `404`, blocking every request before it was made. Added `middleware.CORS`, configured by the new `CORS_ALLOWED_ORIGINS` env var. `Vary: Origin` is sent on every response, including those with no `Origin`, so a shared cache cannot serve a header-less copy to a browser.
- **SDK sent workflow-scoped opt-outs to the global endpoint**: `preferences.update()` picked the workflow scope with `??`, so an empty-string `workflowIdentifier` (as UI code writing `row.workflowIdentifier ?? ''` produces) won over a valid `workflowId` and then read as falsy — disabling the channel across every workflow instead of one. It now uses `||`.
- **Template rendering**: SMS, push, and chat channels were HTML-escaped because the template renderer used `html/template` for all channel types. Email continues to use `html/template`; all other channels now use `text/template`.
- **Multi-subscriber trigger**: Triggering a workflow for multiple subscribers in a single request now works correctly. The notifications unique index previously only covered `{environmentId, transactionId}`, causing every subscriber after the first to fail with a duplicate-key error. The index now covers `{environmentId, transactionId, subscriberId}`.
- **Double-delivery window**: The pre-check `FindByTransactionID → ErrDuplicateTransaction` was non-atomic and could allow two concurrent requests with the same `transactionId` to both enqueue worker tasks. Idempotency is now enforced solely by the MongoDB unique index on insert.
- **Broadcast memory exhaustion**: `POST /events/trigger/broadcast` previously fetched all subscribers into memory before enqueuing. Large environments would exhaust the API process. Broadcast now enqueues an async `task:broadcast` worker task that paginates subscribers in 100-record batches.
- **Digest key collision**: Digest Redis keys did not include the step index, so two digest steps in the same workflow sharing the same `digestKey` string would merge into a single digest window. Step index is now part of the key.
- **Silent errors in worker**: Several `json.Marshal` and `bson.ObjectIDFromHex` calls silently ignored errors (`_, _`), leading to zero-value ObjectIDs or empty task payloads being enqueued. All are now propagated as errors.
- **`json.Marshal` error on credentials update**: `PUT /integrations/:id` silently ignored a potential marshal error on the credentials field; now returns `400 VALIDATION_ERROR`.

### Added

- `CORS_ALLOWED_ORIGINS` — comma-separated allowlist of browser origins. A single `*` allows any origin; an unlisted origin receives no CORS headers and is blocked by the browser.
- `POST /api/v1/subscribers/:subscriberId/channels/push/tokens/remove` — unregister device tokens (`{"fcmTokens": [...], "apnsTokens": [...]}`). Subscriber upserts merge push tokens so one device cannot evict another's, which means they can only add; this is the removal half. Removing a token the subscriber does not have is a no-op, so the call is safe to retry.
- `GET /api/v1/integrations/:id` — fetch a single integration by ID (was missing while `PUT`, `DELETE`, and `PATCH` all existed).
- `broadcast_handler.go` — new async worker task handler for paginated broadcast fan-out.

### Changed

- SDK `0.2.0`: `PreferenceUpdate` and `Preference` gain `workflowIdentifier`; `preferences.update()` prefers it over `workflowId`. Omitted channels are now left untouched rather than disabled. `preferences.list()` returns effective settings for every active workflow (plus the global preference) rather than only the stored rows, and each entry carries `explicit`.
- `POST /subscribers` and `POST /subscribers/bulk` no longer default `locale` to `en` on update — only on insert. Re-posting an existing subscriber without a locale preserves the stored one.
- `engine.IsChannelEnabled` is now a thin wrapper over the new `engine.ResolveChannelPrefs`, so preference precedence is defined in exactly one place.
- The six channels are enumerated once, in `model.channelFields` (`internal/model/channel_prefs.go`), behind `ChannelNames`/`Get`/`Set`/`And`/`AllChannelsEnabled`/`ChannelBSONField`/`ChannelByField`. Masking, dotted-path updates, and delivery lookups all iterate that table, so adding a channel means adding a field and a row rather than editing the hand-written lists that compiled cleanly while leaving the new channel `false`. A reflection test pins the table to `ChannelPrefs`.
- The preference API no longer restates the channel list. A request binds `channels` as a field map resolved through `model.ChannelByField` (so a new channel is accepted with no handler change, and an unknown one is rejected), and `PreferenceResponse.Channels` is `model.ChannelPrefs` itself rather than a copy of its shape — removing the two hand-written conversions that would have silently reported a new channel as `false`.

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
