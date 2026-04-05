import { EventEmitter } from './events';

/**
 * WebSocket client that handles real-time notification delivery.
 *
 * Authentication uses a first-message handshake: after the socket opens,
 * the client sends `{apiKey, subscriberToken, subscriberId}` as JSON.
 * The server responds with `{"event":"authenticated"}` on success.
 *
 * Auto-reconnects on disconnect with exponential backoff (1s, 2s, 4s, ...)
 * up to {@link maxReconnectAttempts} attempts.
 *
 * @internal This class is not part of the public API. Use {@link NotificationClient} instead.
 */
export class WSClient {
  private ws: WebSocket | null = null;
  private wsUrl: string;
  private apiKey: string;
  private subscriberToken: string;
  private subscriberId: string;
  private emitter: EventEmitter;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectDelay = 1000;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private shouldReconnect = false;
  private authenticated = false;

  /**
   * @param wsUrl - WebSocket server base URL (e.g. `wss://ws.example.com`).
   * @param apiKey - API key for authentication.
   * @param subscriberToken - HMAC-signed subscriber token.
   * @param subscriberId - External subscriber ID (must match token payload).
   * @param emitter - Shared event emitter for dispatching SDK events.
   */
  constructor(wsUrl: string, apiKey: string, subscriberToken: string, subscriberId: string, emitter: EventEmitter) {
    this.wsUrl = wsUrl.replace(/\/$/, '') + '/ws';
    this.apiKey = apiKey;
    this.subscriberToken = subscriberToken;
    this.subscriberId = subscriberId;
    this.emitter = emitter;
  }

  /** Opens the WebSocket connection and starts the authentication handshake. */
  connect(): void {
    this.shouldReconnect = true;
    this.doConnect();
  }

  /** Closes the connection and stops auto-reconnection. */
  disconnect(): void {
    this.shouldReconnect = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close(1000, 'client disconnect');
      this.ws = null;
    }
    this.authenticated = false;
  }

  /**
   * Sends a message over the WebSocket. Only works when connected and authenticated.
   *
   * @param action - The action type (e.g. `notification:seen`).
   * @param payload - Action payload data.
   */
  send(action: string, payload: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN && this.authenticated) {
      this.ws.send(JSON.stringify({ action, payload }));
    }
  }

  private doConnect(): void {
    this.authenticated = false;
    try {
      this.ws = new WebSocket(this.wsUrl);
    } catch (err) {
      this.emitter.emit('error', err);
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      // Send auth credentials as first message
      this.ws?.send(
        JSON.stringify({
          apiKey: this.apiKey,
          subscriberToken: this.subscriberToken,
          subscriberId: this.subscriberId,
        }),
      );
    };

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data as string);

        // Handle auth confirmation
        if (!this.authenticated) {
          if (msg.event === 'authenticated') {
            this.authenticated = true;
            this.reconnectAttempts = 0;
            this.emitter.emit('connected');
          }
          return;
        }

        if (msg.event === 'notification:new') {
          this.emitter.emit('notification:new', msg.data);
        } else if (msg.event === 'notification:updated') {
          this.emitter.emit('notification:updated', msg.data);
        } else if (msg.event === 'notification:unseen_count') {
          this.emitter.emit('unseen_count:changed', msg.data?.count);
        }
      } catch {
        // ignore parse errors
      }
    };

    this.ws.onclose = () => {
      this.authenticated = false;
      this.emitter.emit('disconnected');
      if (this.shouldReconnect) {
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = (err) => {
      this.emitter.emit('error', err);
    };
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      return;
    }
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts);
    this.reconnectAttempts++;
    this.reconnectTimer = setTimeout(() => this.doConnect(), delay);
  }
}
