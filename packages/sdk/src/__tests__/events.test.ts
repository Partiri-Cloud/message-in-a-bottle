import { describe, it, expect, vi } from 'vitest';
import { EventEmitter } from '../events';

describe('EventEmitter', () => {
  it('emits events to registered handlers', () => {
    const emitter = new EventEmitter();
    const handler = vi.fn();
    emitter.on('notification:new', handler);
    emitter.emit('notification:new', { id: '123' });
    expect(handler).toHaveBeenCalledWith({ id: '123' });
  });

  it('supports multiple handlers per event', () => {
    const emitter = new EventEmitter();
    const handler1 = vi.fn();
    const handler2 = vi.fn();
    emitter.on('notification:new', handler1);
    emitter.on('notification:new', handler2);
    emitter.emit('notification:new', 'data');
    expect(handler1).toHaveBeenCalledWith('data');
    expect(handler2).toHaveBeenCalledWith('data');
  });

  it('unsubscribe function works', () => {
    const emitter = new EventEmitter();
    const handler = vi.fn();
    const unsub = emitter.on('notification:new', handler);
    unsub();
    emitter.emit('notification:new', 'data');
    expect(handler).not.toHaveBeenCalled();
  });

  it('off removes specific handler', () => {
    const emitter = new EventEmitter();
    const handler1 = vi.fn();
    const handler2 = vi.fn();
    emitter.on('notification:new', handler1);
    emitter.on('notification:new', handler2);
    emitter.off('notification:new', handler1);
    emitter.emit('notification:new', 'data');
    expect(handler1).not.toHaveBeenCalled();
    expect(handler2).toHaveBeenCalledWith('data');
  });

  it('removeAllListeners clears everything', () => {
    const emitter = new EventEmitter();
    const handler = vi.fn();
    emitter.on('notification:new', handler);
    emitter.on('connected', handler);
    emitter.removeAllListeners();
    emitter.emit('notification:new', 'data');
    emitter.emit('connected', 'data');
    expect(handler).not.toHaveBeenCalled();
  });

  it('handler errors are caught', () => {
    const emitter = new EventEmitter();
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const badHandler = () => {
      throw new Error('handler boom');
    };
    const goodHandler = vi.fn();
    emitter.on('notification:new', badHandler);
    emitter.on('notification:new', goodHandler);
    emitter.emit('notification:new', 'data');
    expect(goodHandler).toHaveBeenCalledWith('data');
    expect(errorSpy).toHaveBeenCalled();
    errorSpy.mockRestore();
  });

  it('emitting unregistered event does nothing', () => {
    const emitter = new EventEmitter();
    expect(() => emitter.emit('notification:new')).not.toThrow();
  });
});
