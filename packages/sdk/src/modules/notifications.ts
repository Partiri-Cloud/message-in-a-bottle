import { HttpClient } from '../http';
import { WSClient } from '../ws';
import type { FeedOptions, Notification, PaginatedResponse } from '../types';

/**
 * Module for interacting with the subscriber's notification feed.
 *
 * Provides methods to list, mark, archive, and query notifications.
 * Accessed via {@link NotificationClient.notifications}.
 *
 * @example
 * ```typescript
 * // Fetch unread notifications
 * const feed = await client.notifications.list({ read: false });
 *
 * // Mark a notification as read
 * await client.notifications.markAsRead('notif_abc123');
 *
 * // Get unseen count for a badge
 * const { count } = await client.notifications.unseenCount();
 * ```
 */
export class NotificationsModule {
  private http: HttpClient;
  private ws: WSClient;
  private subscriberId: string;

  /** @internal */
  constructor(http: HttpClient, ws: WSClient, subscriberId: string) {
    this.http = http;
    this.ws = ws;
    this.subscriberId = subscriberId;
  }

  /**
   * Fetches the subscriber's notification feed.
   *
   * @param options - Pagination and filter options.
   * @returns Paginated list of notifications.
   */
  async list(options: FeedOptions = {}): Promise<PaginatedResponse<Notification[]>> {
    const params: Record<string, string> = {};
    if (options.page) params.page = String(options.page);
    if (options.limit) params.limit = String(options.limit);
    if (options.read !== undefined) params.read = String(options.read);
    if (options.seen !== undefined) params.seen = String(options.seen);

    return this.http.get(`/api/v1/subscribers/${this.subscriberId}/feed`, params);
  }

  /**
   * Marks a notification as seen.
   *
   * Also notifies the server over WebSocket so other connected clients update in real time.
   *
   * @param notificationId - The notification ID to mark as seen.
   */
  async markAsSeen(notificationId: string): Promise<void> {
    await this.http.post(`/api/v1/subscribers/${this.subscriberId}/feed/${notificationId}/seen`);
    this.ws.send('notification:seen', { notificationId });
  }

  /**
   * Marks a notification as read.
   *
   * Also notifies the server over WebSocket so other connected clients update in real time.
   *
   * @param notificationId - The notification ID to mark as read.
   */
  async markAsRead(notificationId: string): Promise<void> {
    await this.http.post(`/api/v1/subscribers/${this.subscriberId}/feed/${notificationId}/read`);
    this.ws.send('notification:read', { notificationId });
  }

  /**
   * Archives a notification, hiding it from the default feed.
   *
   * Also notifies the server over WebSocket so other connected clients update in real time.
   *
   * @param notificationId - The notification ID to archive.
   */
  async archive(notificationId: string): Promise<void> {
    await this.http.post(`/api/v1/subscribers/${this.subscriberId}/feed/${notificationId}/archive`);
    this.ws.send('notification:archive', { notificationId });
  }

  /**
   * Marks multiple notifications as read in a single request.
   *
   * @param notificationIds - Array of notification IDs to mark as read.
   */
  async bulkMarkAsRead(notificationIds: string[]): Promise<void> {
    await this.http.post(`/api/v1/subscribers/${this.subscriberId}/feed/bulk-action`, {
      action: 'read',
      notificationIds,
    });
  }

  /**
   * Marks multiple notifications as seen in a single request.
   *
   * @param notificationIds - Array of notification IDs to mark as seen.
   */
  async bulkMarkAsSeen(notificationIds: string[]): Promise<void> {
    await this.http.post(`/api/v1/subscribers/${this.subscriberId}/feed/bulk-action`, {
      action: 'seen',
      notificationIds,
    });
  }

  /**
   * Returns the number of unseen notifications for the subscriber.
   *
   * @returns Object with a `count` property.
   *
   * @example
   * ```typescript
   * const { count } = await client.notifications.unseenCount();
   * badge.textContent = count > 0 ? String(count) : '';
   * ```
   */
  async unseenCount(): Promise<{ count: number }> {
    const resp = await this.http.get<{ data: { count: number } }>(
      `/api/v1/subscribers/${this.subscriberId}/feed/unseen-count`,
    );
    return resp.data;
  }
}
