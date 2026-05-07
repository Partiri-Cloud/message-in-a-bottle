# @partiri/message-in-a-bottle-sdk

Framework-agnostic TypeScript SDK for connecting to the Message in a Bottle notification platform.

## Install

```bash
npm install @partiri/message-in-a-bottle-sdk
```

## Quick start

```typescript
import { NotificationClient } from '@partiri/message-in-a-bottle-sdk';

const client = new NotificationClient({
  apiUrl: 'https://api.example.com',
  wsUrl: 'wss://ws.example.com',
  apiKey: 'your-api-key',
  subscriberToken: 'base64payload.hmacSignature',
});

// Connect to receive real-time notifications
client.connect();

// Listen for events
client.on('connected', () => console.log('Connected'));
client.on('notification:new', (notification) => console.log('New:', notification));
client.on('unseen_count:changed', (count) => console.log('Unseen:', count));

// Disconnect when done
client.disconnect();
```

## Configuration

```typescript
interface NotificationClientOptions {
  apiUrl: string;          // REST API base URL (e.g. https://api.example.com)
  wsUrl: string;           // WebSocket server URL (e.g. wss://ws.example.com)
  apiKey: string;          // API key for authentication
  subscriberToken: string; // HMAC-signed subscriber token (generated server-side)
}
```

The `subscriberToken` is generated on your backend using `SUBSCRIBER_HMAC_SECRET`. The SDK extracts the `subscriberId` automatically from the token payload.

## WebSocket authentication

The SDK authenticates over WebSocket using a secure handshake:

1. Opens a connection to `{wsUrl}/ws` (no credentials in the URL)
2. Sends credentials as the first message after the connection opens:
   ```json
   { "apiKey": "...", "subscriberToken": "...", "subscriberId": "..." }
   ```
3. Waits for `{"event": "authenticated"}` from the server
4. Emits `connected` and begins receiving notifications

Credentials are never exposed in URLs, query strings, or browser history.

## Modules

### Notifications

```typescript
// Fetch notification feed (paginated)
const feed = await client.notifications.list({ page: 1, limit: 20 });
const feed = await client.notifications.list({ read: false });
const feed = await client.notifications.list({ seen: false });

// Mark notifications
await client.notifications.markAsSeen(notificationId);
await client.notifications.markAsRead(notificationId);
await client.notifications.archive(notificationId);

// Get unseen count
const { count } = await client.notifications.unseenCount();
```

### Preferences

```typescript
// Get all preferences
const prefs = await client.preferences.list();

// Update global channel preferences
await client.preferences.update({
  channels: { email: true, sms: false, push: true, inApp: true },
});

// Update workflow-specific preferences
await client.preferences.update({
  workflowId: 'workflow-id',
  channels: { email: false },
});
```

## Events

| Event                    | Payload              | Description                          |
|--------------------------|----------------------|--------------------------------------|
| `connected`              | --                   | WebSocket authenticated and ready    |
| `disconnected`           | --                   | WebSocket disconnected               |
| `notification:new`       | `Notification`       | New notification received            |
| `notification:updated`   | `Notification`       | Notification status changed          |
| `unseen_count:changed`   | `number`             | Unseen notification count changed    |
| `error`                  | `Error`              | Connection or protocol error         |

```typescript
// Subscribe
const unsubscribe = client.on('notification:new', handler);

// Unsubscribe
unsubscribe();
// or
client.off('notification:new', handler);
```

## Auto-reconnect

The SDK automatically reconnects on disconnect with exponential backoff (1s, 2s, 4s, ..., up to 10 attempts). Call `disconnect()` to stop reconnection.

## Development

```bash
npm install
npm test          # run tests
npm run build     # build with tsup
npm run dev       # watch mode
```
