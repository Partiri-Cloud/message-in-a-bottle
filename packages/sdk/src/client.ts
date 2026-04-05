import { EventEmitter } from './events';
import { HttpClient } from './http';
import { NotificationsModule } from './modules/notifications';
import { PreferencesModule } from './modules/preferences';
import type { EventHandler, NotificationClientOptions, NotificationEvent } from './types';
import { WSClient } from './ws';

/**
 * Main entry point for the Message in a Bottle notification SDK.
 *
 * Provides real-time WebSocket notifications and REST API access for
 * managing a subscriber's notification feed and preferences.
 *
 * @example
 * ```typescript
 * const client = new NotificationClient({
 *   apiUrl: 'https://api.example.com',
 *   wsUrl: 'wss://ws.example.com',
 *   apiKey: 'your-api-key',
 *   subscriberToken: 'base64payload.hmacSignature',
 * });
 *
 * client.on('notification:new', (notification) => {
 *   console.log('New notification:', notification);
 * });
 *
 * client.connect();
 * ```
 */
export class NotificationClient {
  private emitter: EventEmitter;
  private wsClient: WSClient;
  private httpClient: HttpClient;

  /** Module for interacting with the notification feed. */
  public readonly notifications: NotificationsModule;
  /** Module for managing subscriber notification preferences. */
  public readonly preferences: PreferencesModule;

  private subscriberId: string;

  /**
   * Creates a new notification client.
   *
   * The `subscriberId` is automatically extracted from the `subscriberToken` payload.
   *
   * @param options - Client configuration including API URLs, key, and subscriber token.
   */
  constructor(options: NotificationClientOptions) {
    this.emitter = new EventEmitter();

    // Extract subscriberId from the token payload (base64 part before the dot)
    const tokenPayload = options.subscriberToken.split('.')[0];
    const decoded = atob(tokenPayload);
    const parsed = JSON.parse(decoded) as { subscriberId: string };
    this.subscriberId = parsed.subscriberId;

    this.httpClient = new HttpClient(options.apiUrl, options.apiKey, options.subscriberToken);
    this.wsClient = new WSClient(options.wsUrl, options.apiKey, options.subscriberToken, this.subscriberId, this.emitter);

    this.notifications = new NotificationsModule(this.httpClient, this.wsClient, this.subscriberId);
    this.preferences = new PreferencesModule(this.httpClient, this.subscriberId);
  }

  /**
   * Opens the WebSocket connection and begins the authentication handshake.
   *
   * Once authenticated, the `connected` event is emitted and real-time
   * notifications will start flowing. Auto-reconnects on disconnect
   * (up to 10 attempts with exponential backoff).
   */
  connect(): void {
    this.wsClient.connect();
  }

  /**
   * Closes the WebSocket connection and stops auto-reconnection.
   *
   * Also removes all registered event listeners.
   */
  disconnect(): void {
    this.wsClient.disconnect();
    this.emitter.removeAllListeners();
  }

  /**
   * Registers an event listener.
   *
   * @param event - The event to listen for.
   * @param handler - Callback invoked when the event fires.
   * @returns A function that removes this listener when called.
   *
   * @example
   * ```typescript
   * const unsubscribe = client.on('notification:new', (notif) => {
   *   console.log(notif);
   * });
   *
   * // Later, stop listening:
   * unsubscribe();
   * ```
   */
  on(event: NotificationEvent, handler: EventHandler): () => void {
    return this.emitter.on(event, handler);
  }

  /**
   * Removes a previously registered event listener.
   *
   * @param event - The event to stop listening for.
   * @param handler - The exact handler function that was registered.
   */
  off(event: NotificationEvent, handler: EventHandler): void {
    this.emitter.off(event, handler);
  }
}
