import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { NotificationClient } from '../client';
import type { NotificationClientOptions } from '../types';

const mockFetch = vi.fn();
globalThis.fetch = mockFetch;

class MockWebSocket {
  static OPEN = 1;
  readyState = MockWebSocket.OPEN;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: ((event: unknown) => void) | null = null;
  send = vi.fn();
  close = vi.fn();
  constructor(public url: string) {
    setTimeout(() => this.onopen?.(), 0);
  }
}

function makeToken(subscriberId: string): string {
  return btoa(JSON.stringify({ subscriberId })) + '.fakesig';
}

const defaultOptions: NotificationClientOptions = {
  apiUrl: 'https://api.example.com',
  wsUrl: 'wss://ws.example.com',
  apiKey: 'test-key',
  subscriberToken: makeToken('usr_default'),
};

describe('NotificationClient', () => {
  let originalWebSocket: typeof globalThis.WebSocket;

  beforeEach(() => {
    vi.useFakeTimers();
    mockFetch.mockReset();
    originalWebSocket = globalThis.WebSocket;
    (globalThis as any).WebSocket = MockWebSocket;
  });

  afterEach(() => {
    vi.useRealTimers();
    globalThis.WebSocket = originalWebSocket;
  });

  it('extracts subscriberId from the subscriber token', () => {
    const client = new NotificationClient({
      ...defaultOptions,
      subscriberToken: makeToken('usr_specific'),
    });
    expect((client as any).subscriberId).toBe('usr_specific');
  });

  it('exposes notifications and preferences modules', () => {
    const client = new NotificationClient(defaultOptions);
    expect(client.notifications).toBeDefined();
    expect(client.preferences).toBeDefined();
  });

  it('connect creates a WebSocket connection', () => {
    const client = new NotificationClient(defaultOptions);
    expect((client as any).wsClient.ws).toBeNull();
    client.connect();
    expect((client as any).wsClient.ws).toBeInstanceOf(MockWebSocket);
    client.disconnect();
  });

  it('on registers a listener and the returned function unsubscribes it', () => {
    const client = new NotificationClient(defaultOptions);
    const handler = vi.fn();
    const unsub = client.on('notification:new', handler);

    (client as any).emitter.emit('notification:new', { id: 'n_1' });
    expect(handler).toHaveBeenCalledWith({ id: 'n_1' });

    unsub();
    (client as any).emitter.emit('notification:new', { id: 'n_2' });
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it('off removes a listener', () => {
    const client = new NotificationClient(defaultOptions);
    const handler = vi.fn();
    client.on('notification:new', handler);
    client.off('notification:new', handler);

    (client as any).emitter.emit('notification:new', { id: 'n_1' });
    expect(handler).not.toHaveBeenCalled();
  });

  it('disconnect clears all event listeners', () => {
    const client = new NotificationClient(defaultOptions);
    const handler = vi.fn();
    client.on('notification:new', handler);

    (client as any).emitter.emit('notification:new', { id: 'n_1' });
    expect(handler).toHaveBeenCalledTimes(1);

    client.disconnect();

    (client as any).emitter.emit('notification:new', { id: 'n_2' });
    expect(handler).toHaveBeenCalledTimes(1);
  });
});
