import { describe, it, expect, vi, beforeEach } from 'vitest';
import { PreferencesModule } from '../modules/preferences';
import type { HttpClient } from '../http';

function createMockHttp() {
  return {
    get: vi.fn().mockResolvedValue({ data: [] }),
    patch: vi.fn().mockResolvedValue({ data: {} }),
  } as unknown as HttpClient;
}

describe('PreferencesModule', () => {
  let http: ReturnType<typeof createMockHttp>;
  let mod: PreferencesModule;

  beforeEach(() => {
    http = createMockHttp();
    mod = new PreferencesModule(http as any, 'usr_abc');
  });

  it('list calls the correct endpoint', async () => {
    await mod.list();
    expect(http.get).toHaveBeenCalledWith('/api/v1/subscribers/usr_abc/preferences');
  });

  it('list returns the data array from the response', async () => {
    const mockPrefs = [{ workflowId: 'wf_1', channels: { email: true }, updatedAt: '2026-01-01T00:00:00Z' }];
    (http.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockPrefs });

    const result = await mod.list();
    expect(result).toEqual(mockPrefs);
  });

  it('update without workflowId patches the global preferences endpoint', async () => {
    await mod.update({ channels: { email: false, sms: true } });

    expect(http.patch).toHaveBeenCalledWith(
      '/api/v1/subscribers/usr_abc/preferences',
      { channels: { email: false, sms: true } },
    );
  });

  it('update with workflowId patches the workflow-scoped endpoint', async () => {
    await mod.update({ workflowId: 'wf_order_updates', channels: { email: true, sms: false } });

    expect(http.patch).toHaveBeenCalledWith(
      '/api/v1/subscribers/usr_abc/preferences/wf_order_updates',
      { channels: { email: true, sms: false } },
    );
  });
});
