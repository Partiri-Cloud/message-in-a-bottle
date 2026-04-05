import type { EventHandler, NotificationEvent } from './types';

/**
 * Simple typed event emitter for SDK events.
 *
 * @internal This class is not part of the public API. Use {@link NotificationClient.on} instead.
 */
export class EventEmitter {
  private listeners = new Map<string, Set<EventHandler>>();

  /**
   * Registers an event listener.
   *
   * @param event - The event name to listen for.
   * @param handler - Callback invoked when the event is emitted.
   * @returns A function that removes this listener when called.
   */
  on(event: NotificationEvent, handler: EventHandler): () => void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(handler);

    return () => {
      this.listeners.get(event)?.delete(handler);
    };
  }

  /**
   * Removes a previously registered event listener.
   *
   * @param event - The event name.
   * @param handler - The exact handler function that was registered.
   */
  off(event: NotificationEvent, handler: EventHandler): void {
    this.listeners.get(event)?.delete(handler);
  }

  /**
   * Emits an event to all registered listeners.
   *
   * @param event - The event name to emit.
   * @param data - Optional data passed to each handler.
   */
  emit(event: NotificationEvent, data?: unknown): void {
    this.listeners.get(event)?.forEach((handler) => {
      try {
        handler(data);
      } catch (err) {
        console.error(`Event handler error for ${event}:`, err);
      }
    });
  }

  /** Removes all registered listeners for all events. */
  removeAllListeners(): void {
    this.listeners.clear();
  }
}
