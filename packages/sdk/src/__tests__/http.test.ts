import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HttpClient } from '../http';

const mockFetch = vi.fn();
globalThis.fetch = mockFetch;

describe('HttpClient', () => {
  let client: HttpClient;

  beforeEach(() => {
    mockFetch.mockReset();
    client = new HttpClient('https://api.example.com', 'nv_prod_key123', 'sub-token-abc');
  });

  it('get sends correct headers', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    });

    await client.get('/api/v1/subscribers');

    const [url, opts] = mockFetch.mock.calls[0];
    expect(url).toBe('https://api.example.com/api/v1/subscribers');
    expect(opts.headers['Authorization']).toBe('ApiKey nv_prod_key123');
    expect(opts.headers['X-Subscriber-Token']).toBe('sub-token-abc');
  });

  it('get appends query params', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    });

    await client.get('/api/v1/feed', { page: '2', limit: '10' });

    const [url] = mockFetch.mock.calls[0];
    expect(url).toContain('page=2');
    expect(url).toContain('limit=10');
  });

  it('post sends JSON body', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: { id: '123' } }),
    });

    await client.post('/api/v1/subscribers', { subscriberId: 'usr_1' });

    const [, opts] = mockFetch.mock.calls[0];
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(opts.body)).toEqual({ subscriberId: 'usr_1' });
  });

  it('patch sends JSON body', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: {} }),
    });

    await client.patch('/api/v1/preferences', { channels: { email: false } });

    const [, opts] = mockFetch.mock.calls[0];
    expect(opts.method).toBe('PATCH');
    expect(JSON.parse(opts.body)).toEqual({ channels: { email: false } });
  });

  it('handles error responses with JSON body', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: { message: 'forbidden' } }),
    });

    await expect(client.get('/api/v1/secret')).rejects.toThrow('forbidden');
  });

  it('handles non-JSON error responses', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('not json')),
    });

    await expect(client.get('/api/v1/broken')).rejects.toThrow('HTTP 500');
  });

  it('post without body sends undefined body', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: {} }),
    });

    await client.post('/api/v1/subscribers/usr_1/feed/n_1/seen');

    const [, opts] = mockFetch.mock.calls[0];
    expect(opts.body).toBeUndefined();
  });

  it('handles error response with no message field falls back to HTTP status', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 422,
      json: () => Promise.resolve({ details: 'validation failed' }),
    });

    await expect(client.get('/api/v1/feed')).rejects.toThrow('HTTP 422');
  });

  it('strips trailing slash from base URL', async () => {
    const c = new HttpClient('https://api.example.com/', 'key', 'token');
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: {} }),
    });

    await c.get('/test');
    const [url] = mockFetch.mock.calls[0];
    expect(url).toBe('https://api.example.com/test');
  });
});
