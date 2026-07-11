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
    // The wire shape the API actually sends: effective channels (all six),
    // an `explicit` flag, and a global row with null identifiers.
    const mockPrefs = [
      {
        workflowId: null,
        workflowIdentifier: null,
        channels: { email: true, sms: false, push: true, inApp: true, slack: true, msTeams: true },
        explicit: true,
        updatedAt: '2026-01-01T00:00:00Z',
      },
      {
        workflowId: '65f0c0ffee0000000000dead',
        workflowIdentifier: 'deploy-started',
        channels: { email: false, sms: false, push: false, inApp: true, slack: false, msTeams: false },
        explicit: false,
        updatedAt: null,
      },
    ];
    (http.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockPrefs });

    const result = await mod.list();

    expect(result).toEqual(mockPrefs);
    expect(result[1].workflowIdentifier).toBe('deploy-started');
    expect(result[1].explicit).toBe(false);
    expect(result[1].channels.inApp).toBe(true);
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

  it('update with workflowIdentifier patches the workflow-scoped endpoint', async () => {
    await mod.update({ workflowIdentifier: 'deploy-started', channels: { email: false } });

    expect(http.patch).toHaveBeenCalledWith(
      '/api/v1/subscribers/usr_abc/preferences/deploy-started',
      { channels: { email: false } },
    );
  });

  it('update prefers workflowIdentifier when both are given', async () => {
    await mod.update({
      workflowIdentifier: 'deploy-started',
      workflowId: '65f0c0ffee0000000000dead',
      channels: { email: false },
    });

    expect(http.patch).toHaveBeenCalledWith(
      '/api/v1/subscribers/usr_abc/preferences/deploy-started',
      { channels: { email: false } },
    );
  });

  it('update escapes the workflow segment', async () => {
    await mod.update({ workflowIdentifier: 'deploy/started', channels: { email: false } });

    expect(http.patch).toHaveBeenCalledWith(
      '/api/v1/subscribers/usr_abc/preferences/deploy%2Fstarted',
      { channels: { email: false } },
    );
  });

  // `??` would pick the empty string over the workflowId (it is not nullish),
  // then read it as falsy and send a workflow-scoped opt-out to the GLOBAL
  // endpoint — disabling the channel everywhere. UI code that writes
  // `row.workflowIdentifier ?? ''` makes this a real path.
  it('update falls back to workflowId when workflowIdentifier is an empty string', async () => {
    await mod.update({
      workflowIdentifier: '',
      workflowId: '65f0c0ffee0000000000dead',
      channels: { sms: false },
    });

    expect(http.patch).toHaveBeenCalledWith(
      '/api/v1/subscribers/usr_abc/preferences/65f0c0ffee0000000000dead',
      { channels: { sms: false } },
    );
  });

  it('update with only an empty workflowIdentifier still targets the global endpoint', async () => {
    await mod.update({ workflowIdentifier: '', channels: { sms: false } });

    expect(http.patch).toHaveBeenCalledWith(
      '/api/v1/subscribers/usr_abc/preferences',
      { channels: { sms: false } },
    );
  });
});
