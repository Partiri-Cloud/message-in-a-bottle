import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { EventEmitter } from '../events';
import { WSClient } from '../ws';

// Mock WebSocket
class MockWebSocket {
  static OPEN = 1;
  static CLOSED = 3;
  readyState = MockWebSocket.OPEN;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: ((event: unknown) => void) | null = null;
  send = vi.fn();
  close = vi.fn();

  constructor(public url: string) {
    // Auto-trigger onopen on next tick
    setTimeout(() => this.onopen?.(), 0);
  }
}

/** Simulate the server confirming auth after the client sends its auth message */
async function authenticateClient(client: WSClient) {
  await vi.advanceTimersByTimeAsync(10);
  const ws = (client as any).ws as MockWebSocket;
  // Client should have sent the auth message on open
  expect(ws.send).toHaveBeenCalledTimes(1);
  const authPayload = JSON.parse(ws.send.mock.calls[0][0]);
  expect(authPayload).toEqual({
    apiKey: expect.any(String),
    subscriberToken: expect.any(String),
    subscriberId: expect.any(String),
  });
  // Server responds with authenticated event
  ws.onmessage?.({ data: JSON.stringify({ event: 'authenticated' }) });
}

describe('WSClient', () => {
  let emitter: EventEmitter;
  let originalWebSocket: typeof globalThis.WebSocket;

  beforeEach(() => {
    vi.useFakeTimers();
    emitter = new EventEmitter();
    originalWebSocket = globalThis.WebSocket;
    (globalThis as any).WebSocket = MockWebSocket;
  });

  afterEach(() => {
    vi.useRealTimers();
    globalThis.WebSocket = originalWebSocket;
  });

  it('connects to clean URL without query params', () => {
    const client = new WSClient(
      'wss://ws.example.com',
      'nv_prod_key',
      'sub-token',
      'usr_123',
      emitter,
    );
    client.connect();

    const ws = (client as any).ws as MockWebSocket;
    expect(ws.url).toBe('wss://ws.example.com/ws');
    expect(ws.url).not.toContain('apiKey');
    client.disconnect();
  });

  it('sends auth as first message and emits connected after server confirms', async () => {
    const handler = vi.fn();
    emitter.on('connected', handler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();

    // connected should NOT fire until server confirms auth
    await vi.advanceTimersByTimeAsync(10);
    expect(handler).not.toHaveBeenCalled();

    // Simulate server auth response
    await authenticateClient(client);
    expect(handler).toHaveBeenCalled();
    client.disconnect();
  });

  it('emits disconnected on close', async () => {
    const handler = vi.fn();
    emitter.on('disconnected', handler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await authenticateClient(client);

    const ws = (client as any).ws as MockWebSocket;
    ws.readyState = MockWebSocket.CLOSED;
    ws.onclose?.();

    expect(handler).toHaveBeenCalled();
    client.disconnect();
  });

  it('routes notification:new messages after auth', async () => {
    const handler = vi.fn();
    emitter.on('notification:new', handler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await authenticateClient(client);

    const ws = (client as any).ws as MockWebSocket;
    ws.onmessage?.({
      data: JSON.stringify({ event: 'notification:new', data: { id: 'n_123' } }),
    });

    expect(handler).toHaveBeenCalledWith({ id: 'n_123' });
    client.disconnect();
  });

  it('ignores data messages before authentication', async () => {
    const handler = vi.fn();
    emitter.on('notification:new', handler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    // Send a notification message BEFORE auth confirmation
    const ws = (client as any).ws as MockWebSocket;
    ws.onmessage?.({
      data: JSON.stringify({ event: 'notification:new', data: { id: 'n_123' } }),
    });

    expect(handler).not.toHaveBeenCalled();
    client.disconnect();
  });

  it('routes unseen_count messages', async () => {
    const handler = vi.fn();
    emitter.on('unseen_count:changed', handler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await authenticateClient(client);

    const ws = (client as any).ws as MockWebSocket;
    ws.onmessage?.({
      data: JSON.stringify({ event: 'notification:unseen_count', data: { count: 5 } }),
    });

    expect(handler).toHaveBeenCalledWith(5);
    client.disconnect();
  });

  it('disconnect prevents reconnection', async () => {
    const connHandler = vi.fn();
    emitter.on('connected', connHandler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await authenticateClient(client);

    client.disconnect();

    await vi.advanceTimersByTimeAsync(60000);
    expect(connHandler).toHaveBeenCalledTimes(1);
  });

  it('send only works when socket is open and authenticated', async () => {
    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await authenticateClient(client);

    const ws = (client as any).ws as MockWebSocket;
    // Reset send count (auth message was already sent)
    ws.send.mockClear();

    // Send when open and authenticated
    client.send('notification:seen', { notificationId: '123' });
    expect(ws.send).toHaveBeenCalledTimes(1);

    // Send when closed
    ws.readyState = MockWebSocket.CLOSED;
    client.send('notification:read', { notificationId: '456' });
    expect(ws.send).toHaveBeenCalledTimes(1);

    client.disconnect();
  });

  it('send is blocked when socket is open but not yet authenticated', async () => {
    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await vi.advanceTimersByTimeAsync(10); // onopen fires, auth message sent

    const ws = (client as any).ws as MockWebSocket;
    ws.send.mockClear(); // discard the initial auth handshake send

    // Socket is OPEN (readyState=1) but server has not confirmed auth yet
    client.send('notification:seen', { notificationId: '123' });
    expect(ws.send).not.toHaveBeenCalled();

    client.disconnect();
  });

  it('routes notification:updated messages after auth', async () => {
    const handler = vi.fn();
    emitter.on('notification:updated', handler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await authenticateClient(client);

    const ws = (client as any).ws as MockWebSocket;
    ws.onmessage?.({
      data: JSON.stringify({ event: 'notification:updated', data: { id: 'n_456', read: true } }),
    });

    expect(handler).toHaveBeenCalledWith({ id: 'n_456', read: true });
    client.disconnect();
  });

  it('emits error event on WebSocket error', async () => {
    const errorHandler = vi.fn();
    emitter.on('error', errorHandler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const ws = (client as any).ws as MockWebSocket;
    const fakeError = new Event('error');
    ws.onerror?.(fakeError);

    expect(errorHandler).toHaveBeenCalledWith(fakeError);
    client.disconnect();
  });

  it('reconnects after unexpected disconnect', async () => {
    const connHandler = vi.fn();
    emitter.on('connected', connHandler);

    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    client.connect();
    await authenticateClient(client);
    expect(connHandler).toHaveBeenCalledTimes(1);

    // Simulate unexpected close (not via client.disconnect())
    const ws = (client as any).ws as MockWebSocket;
    ws.readyState = MockWebSocket.CLOSED;
    ws.onclose?.();

    // Advance past the first reconnect delay (1000ms) + time for onopen (0ms)
    await vi.advanceTimersByTimeAsync(1100);

    // A new WebSocket was created and its onopen fired (auth message sent)
    const newWs = (client as any).ws as MockWebSocket;
    expect(newWs).not.toBe(ws);

    // Server confirms auth on the new connection
    newWs.onmessage?.({ data: JSON.stringify({ event: 'authenticated' }) });
    expect(connHandler).toHaveBeenCalledTimes(2);

    client.disconnect();
  });

  it('stops reconnecting after max attempts', async () => {
    const client = new WSClient('wss://ws.example.com', 'key', 'token', 'usr_1', emitter);
    // Start at attempt 9 so the next close pushes reconnectAttempts to 10 (the limit)
    (client as any).reconnectAttempts = 9;
    client.connect();

    await vi.advanceTimersByTimeAsync(10); // onopen fires
    const ws1 = (client as any).ws as MockWebSocket;

    // Close: attempts=9 → delay=512000ms, attempts becomes 10
    ws1.readyState = MockWebSocket.CLOSED;
    ws1.onclose?.();
    await vi.advanceTimersByTimeAsync(512001); // last reconnect timer fires

    const ws2 = (client as any).ws as MockWebSocket;
    expect(ws2).not.toBe(ws1); // one more connection was made

    // Close the last connection: attempts=10 >= maxReconnectAttempts → no timer scheduled
    ws2.readyState = MockWebSocket.CLOSED;
    ws2.onclose?.();

    await vi.advanceTimersByTimeAsync(10_000_000);
    expect((client as any).ws).toBe(ws2); // ws unchanged — no reconnect happened
  });
});
