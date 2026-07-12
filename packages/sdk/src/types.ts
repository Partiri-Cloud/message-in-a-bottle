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
  /** Rendered subject of the in-app step. Absent until the in_app channel is delivered. */
  subject?: string;
  /** Rendered body of the in-app step. Absent until the in_app channel is delivered. */
  content?: string;
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
 * Scope the update to a single workflow with `workflowIdentifier` (the slug a
 * trigger carries, e.g. `deploy-started`) or `workflowId` (the raw ID).
 *
 * Omit both to update the subscriber's **global opt-out mask**. The mask can only
 * silence a channel, never enable one: setting `sms: false` globally turns SMS
 * off for every workflow the subscriber has no explicit preference on (an
 * explicit workflow preference always wins), and setting `email: true` globally
 * will not switch email on for a workflow whose defaults have it off. Use a
 * workflow-scoped update for that.
 *
 * Only the channels you name are changed — omitted channels keep their current
 * value, so `{ channels: { email: false } }` will not disturb in-app delivery.
 *
 * `channels` must name at least one channel: an empty object is rejected with
 * `400`, because a preference row that changes nothing would still promote the
 * subscriber from inheriting their settings to having chosen them.
 */
export interface PreferenceUpdate {
  /** Workflow identifier to scope the preference to. Preferred over `workflowId`. */
  workflowIdentifier?: string;
  /** Workflow ID to scope the preference to. Omit for global preferences. */
  workflowId?: string;
  /** Channel preferences to set. Unlisted channels are left untouched, and at least one must be named. */
  channels: ChannelPreferences;
}

/**
 * The effective state of every channel. Unlike {@link ChannelPreferences}, whose
 * fields are optional because a *request* may name only the channels it changes,
 * a response always carries all six — the server resolves them.
 */
export interface ResolvedChannels {
  email: boolean;
  sms: boolean;
  push: boolean;
  inApp: boolean;
  slack: boolean;
  msTeams: boolean;
}

/**
 * A subscriber's notification settings for one workflow.
 *
 * `channels` holds the **effective** values — what will actually govern
 * delivery, after the subscriber's workflow-specific choice, their global
 * opt-out mask, and the workflow's own defaults have been resolved server-side.
 * Render them as-is. A workflow whose defaults disable email reports
 * `email: false` here even though the subscriber never touched it.
 */
export interface Preference {
  /** Workflow ID this preference applies to, or `null` for the global preference. */
  workflowId?: string | null;
  /** Workflow identifier (e.g. `deploy-started`), or `null` for the global preference. */
  workflowIdentifier?: string | null;
  /** The effective channel settings for this workflow. Always all six. */
  channels: ResolvedChannels;
  /**
   * Whether the subscriber has explicitly saved a choice here, as opposed to
   * inheriting the workflow's defaults or their global preference.
   */
  explicit: boolean;
  /** ISO 8601 timestamp of the subscriber's last change, or `null` if they never made one. */
  updatedAt?: string | null;
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
