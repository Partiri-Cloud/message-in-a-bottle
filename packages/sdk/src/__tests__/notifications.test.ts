import { describe, it, expect, vi, beforeEach } from 'vitest';
import { NotificationsModule } from '../modules/notifications';
import type { HttpClient } from '../http';
import type { WSClient } from '../ws';

function createMockHttp(): HttpClient {
  return {
    get: vi.fn().mockResolvedValue({ data: [], meta: { page: 1, limit: 20, total: 0 } }),
    post: vi.fn().mockResolvedValue({ data: { acknowledged: true } }),
    patch: vi.fn().mockResolvedValue({ data: {} }),
  } as unknown as HttpClient;
}

function createMockWs(): WSClient {
  return {
    send: vi.fn(),
  } as unknown as WSClient;
}

describe('NotificationsModule', () => {
  let http: ReturnType<typeof createMockHttp>;
  let ws: ReturnType<typeof createMockWs>;
  let mod: NotificationsModule;

  beforeEach(() => {
    http = createMockHttp();
    ws = createMockWs();
    mod = new NotificationsModule(http as any, ws as any, 'usr_abc');
  });

  it('list calls correct endpoint with params', async () => {
    await mod.list({ page: 2, limit: 10, read: false });

    expect(http.get).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed', {
      page: '2',
      limit: '10',
      read: 'false',
    });
  });

  it('list uses default params when none provided', async () => {
    await mod.list();
    expect(http.get).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed', {});
  });

  it('markAsSeen sends HTTP and WS', async () => {
    await mod.markAsSeen('notif_123');

    expect(http.post).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed/notif_123/seen');
    expect(ws.send).toHaveBeenCalledWith('notification:seen', { notificationId: 'notif_123' });
  });

  it('markAsRead sends HTTP and WS', async () => {
    await mod.markAsRead('notif_456');

    expect(http.post).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed/notif_456/read');
    expect(ws.send).toHaveBeenCalledWith('notification:read', { notificationId: 'notif_456' });
  });

  it('archive sends HTTP and WS', async () => {
    await mod.archive('notif_789');

    expect(http.post).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed/notif_789/archive');
    expect(ws.send).toHaveBeenCalledWith('notification:archive', { notificationId: 'notif_789' });
  });

  it('bulkMarkAsRead sends correct payload', async () => {
    await mod.bulkMarkAsRead(['id1', 'id2', 'id3']);

    expect(http.post).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed/bulk-action', {
      action: 'read',
      notificationIds: ['id1', 'id2', 'id3'],
    });
  });

  it('bulkMarkAsSeen sends correct payload', async () => {
    await mod.bulkMarkAsSeen(['id1', 'id2']);

    expect(http.post).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed/bulk-action', {
      action: 'seen',
      notificationIds: ['id1', 'id2'],
    });
  });

  it('unseenCount returns count', async () => {
    (http.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: { count: 7 } });

    const result = await mod.unseenCount();
    expect(result).toEqual({ count: 7 });
    expect(http.get).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/feed/unseen-count');
  });
});
