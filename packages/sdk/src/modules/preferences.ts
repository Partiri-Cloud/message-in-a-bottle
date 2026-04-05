import { HttpClient } from '../http';
import type { Preference, PreferenceUpdate } from '../types';

/**
 * Module for managing subscriber notification preferences.
 *
 * Preferences control which channels a subscriber receives notifications on.
 * They can be set globally or scoped to a specific workflow.
 *
 * Priority order: workflow-specific > global > workflow defaults.
 *
 * Accessed via {@link NotificationClient.preferences}.
 *
 * @example
 * ```typescript
 * // Disable SMS globally
 * await client.preferences.update({
 *   channels: { sms: false },
 * });
 *
 * // Enable only email for a specific workflow
 * await client.preferences.update({
 *   workflowId: 'wf_order_updates',
 *   channels: { email: true, sms: false, push: false, inApp: false },
 * });
 * ```
 */
export class PreferencesModule {
  private http: HttpClient;
  private subscriberId: string;

  /** @internal */
  constructor(http: HttpClient, subscriberId: string) {
    this.http = http;
    this.subscriberId = subscriberId;
  }

  /**
   * Fetches all notification preferences for the subscriber.
   *
   * Returns both global preferences and any workflow-specific overrides.
   *
   * @returns Array of preference records.
   */
  async list(): Promise<Preference[]> {
    const resp = await this.http.get<{ data: Preference[] }>(
      `/api/v1/subscribers/${this.subscriberId}/preferences`,
    );
    return resp.data;
  }

  /**
   * Updates notification preferences.
   *
   * If `workflowId` is provided, the preferences apply to that workflow only.
   * Otherwise, they are saved as global defaults.
   *
   * @param prefs - The preference update payload.
   */
  async update(prefs: PreferenceUpdate): Promise<void> {
    if (prefs.workflowId) {
      await this.http.patch(
        `/api/v1/subscribers/${this.subscriberId}/preferences/${prefs.workflowId}`,
        { channels: prefs.channels },
      );
    } else {
      await this.http.patch(
        `/api/v1/subscribers/${this.subscriberId}/preferences`,
        { channels: prefs.channels },
      );
    }
  }
}
