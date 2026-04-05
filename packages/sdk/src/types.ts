/** Configuration options for creating a {@link NotificationClient}. */
export interface NotificationClientOptions {
  /** Base URL of the REST API server (e.g. `https://api.example.com`). */
  apiUrl: string;
  /** Base URL of the WebSocket server (e.g. `wss://ws.example.com`). */
  wsUrl: string;
  /** API key used to authenticate requests. Sent as `Authorization: ApiKey <key>`. */
  apiKey: string;
  /**
   * HMAC-signed subscriber token generated on the server.
   *
   * Format: `base64({"subscriberId":"<id>","timestamp":<unix_ms>}).<hex-hmac-sha256>`
   *
   * The SDK extracts the `subscriberId` from this token automatically.
   * Tokens expire after 24 hours.
   */
  subscriberToken: string;
}

/** A notification record returned by the API. */
export interface Notification {
  /** Unique notification ID. */
  id: string;
  /** Environment this notification belongs to. */
  environmentId: string;
  /** Subscriber who received this notification. */
  subscriberId: string;
  /** Workflow that triggered this notification. */
  workflowId: string;
  /** Client-provided transaction ID for idempotency. */
  transactionId: string;
  /** Arbitrary payload data attached to the notification. */
  payload: Record<string, unknown>;
  /** Delivery status per channel. */
  channels: ChannelDelivery[];
  /** Whether the notification has been seen by the subscriber. */
  seen: boolean;
  /** Whether the notification has been read by the subscriber. */
  read: boolean;
  /** ISO 8601 timestamp when the notification was marked as seen. */
  seenAt?: string;
  /** ISO 8601 timestamp when the notification was marked as read. */
  readAt?: string;
  /** ISO 8601 timestamp when the notification was archived. */
  archivedAt?: string;
  /** ISO 8601 timestamp when the notification was created. */
  createdAt: string;
  /** ISO 8601 timestamp when the notification was last updated. */
  updatedAt: string;
}

/** Delivery status for a single notification channel. */
export interface ChannelDelivery {
  /** Channel type (e.g. `email`, `sms`, `push`, `in_app`, `slack`, `ms_teams`). */
  channel: string;
  /** Delivery status: `pending`, `sent`, `delivered`, `failed`, or `skipped`. */
  status: string;
  /** ID of the provider integration used. */
  providerId: string;
  /** Message ID returned by the external provider. */
  providerMessageId?: string;
  /** Error message if delivery failed. */
  errorMessage?: string;
  /** Number of delivery retry attempts. */
  retryCount: number;
  /** ISO 8601 timestamp when the message was sent. */
  sentAt?: string;
  /** ISO 8601 timestamp when delivery was confirmed. */
  deliveredAt?: string;
  /** ISO 8601 timestamp when delivery failed permanently. */
  failedAt?: string;
}

/** Paginated API response wrapper. */
export interface PaginatedResponse<T> {
  /** The response data. */
  data: T;
  /** Pagination metadata. */
  meta: {
    /** Current page number (1-based). */
    page: number;
    /** Number of items per page. */
    limit: number;
    /** Total number of items across all pages. */
    total: number;
  };
}

/** Options for querying the notification feed. */
export interface FeedOptions {
  /** Page number (1-based). Defaults to 1. */
  page?: number;
  /** Number of items per page. Defaults to 20, max 100. */
  limit?: number;
  /** Filter by read status. `true` = only read, `false` = only unread. */
  read?: boolean;
  /** Filter by seen status. `true` = only seen, `false` = only unseen. */
  seen?: boolean;
}

/** Per-channel notification preferences. Set a channel to `false` to opt out. */
export interface ChannelPreferences {
  /** Whether email notifications are enabled. */
  email?: boolean;
  /** Whether SMS notifications are enabled. */
  sms?: boolean;
  /** Whether push notifications are enabled. */
  push?: boolean;
  /** Whether in-app notifications are enabled. */
  inApp?: boolean;
  /** Whether Slack notifications are enabled. */
  slack?: boolean;
  /** Whether Microsoft Teams notifications are enabled. */
  msTeams?: boolean;
}

/**
 * Payload for updating subscriber notification preferences.
 *
 * If `workflowId` is provided, the preferences apply to that workflow only.
 * Otherwise, they are set as global defaults.
 */
export interface PreferenceUpdate {
  /** Workflow ID to scope the preference to. Omit for global preferences. */
  workflowId?: string;
  /** Channel preferences to set. */
  channels: ChannelPreferences;
}

/** A stored notification preference record. */
export interface Preference {
  /** Workflow ID this preference applies to, or `undefined` for global. */
  workflowId?: string;
  /** Channel preferences. */
  channels: ChannelPreferences;
  /** ISO 8601 timestamp when the preference was last updated. */
  updatedAt: string;
}

/**
 * Events emitted by the {@link NotificationClient}.
 *
 * - `notification:new` -- A new notification was received (payload: {@link Notification}).
 * - `notification:updated` -- A notification was updated (payload: {@link Notification}).
 * - `unseen_count:changed` -- The unseen notification count changed (payload: `number`).
 * - `connected` -- WebSocket connection authenticated and ready.
 * - `disconnected` -- WebSocket connection closed.
 * - `error` -- A connection or protocol error occurred (payload: `Error`).
 */
export type NotificationEvent =
  | 'notification:new'
  | 'notification:updated'
  | 'unseen_count:changed'
  | 'connected'
  | 'disconnected'
  | 'error';

/** Callback function for handling notification events. */
export type EventHandler<T = unknown> = (data: T) => void;
