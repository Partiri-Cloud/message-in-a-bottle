import { HttpClient } from '../http';
import type {
  Preference,
  PreferenceUpdate,
  ResolvedChannels,
} from '../types';

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
 * // Disable SMS globally. Every other channel is left as it was.
 * await client.preferences.update({
 *   channels: { sms: false },
 * });
 *
 * // Turn email off for one workflow, addressed by its identifier.
 * await client.preferences.update({
 *   workflowIdentifier: 'deploy-started',
 *   channels: { email: false },
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
   * Fetches the subscriber's effective notification settings.
   *
   * Returns one entry per active workflow, plus one for the subscriber's global
   * preference (identified by a null `workflowId`/`workflowIdentifier`). Each
   * entry's `channels` are already resolved — workflow choice over global choice
   * over the workflow's declared defaults — so a UI can render them directly.
   *
   * Entries the subscriber has never touched are included with `explicit: false`
   * and the inherited values. Do not treat a missing entry as "everything on":
   * a workflow's defaults may well disable a channel.
   *
   * @returns One entry per active workflow, plus the global preference.
   */
  async list(): Promise<Preference[]> {
    const resp = await this.http.get<{ data: Preference[] }>(
      `/api/v1/subscribers/${this.subscriberId}/preferences`,
    );
    return resp.data;
  }

  /**
   * Reports which channels this environment can actually deliver on.
   *
   * Render only these. A channel with no configured integration is not rejected
   * when the subscriber enables it, nor when a notification is triggered for it:
   * the worker marks that channel `failed — no integration configured` and moves
   * on. So a toggle for an unconfigured channel looks like it works, is saved
   * like it works, and delivers nothing — forever, and silently.
   *
   * `inApp` is always true: it needs no integration.
   *
   * @returns One boolean per channel.
   */
  async availableChannels(): Promise<ResolvedChannels> {
    const resp = await this.http.get<{ data: ResolvedChannels }>(
      '/api/v1/channels',
    );
    return resp.data;
  }

  /**
   * Updates notification preferences.
   *
   * Scoped to a single workflow when `workflowIdentifier` or `workflowId` is
   * given, otherwise saved as the subscriber's global defaults. Channels you
   * omit keep their current value.
   *
   * @param prefs - The preference update payload.
   */
  async update(prefs: PreferenceUpdate): Promise<void> {
    // The API accepts either form on this path; the identifier is what callers
    // actually hold, since it is what a trigger carries.
    //
    // `||`, not `??`: an empty string is not a scope, and it must fall through to
    // workflowId rather than win. With `??` it would win (it is not nullish) and
    // then read as falsy below, silently sending a workflow-scoped update to the
    // GLOBAL endpoint — turning one workflow's opt-out into one that applies
    // everywhere. UI code writing `row.workflowIdentifier ?? ''` makes that real.
    const workflow = prefs.workflowIdentifier || prefs.workflowId;

    const path = workflow
      ? `/api/v1/subscribers/${this.subscriberId}/preferences/${encodeURIComponent(workflow)}`
      : `/api/v1/subscribers/${this.subscriberId}/preferences`;

    await this.http.patch(path, { channels: prefs.channels });
  }
}
