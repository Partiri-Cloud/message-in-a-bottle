/**
 * Low-level HTTP client for making authenticated API requests.
 *
 * Attaches the API key and subscriber token headers to every request.
 * Automatically parses JSON responses and converts non-2xx responses into errors.
 *
 * @internal This class is not part of the public API. Use {@link NotificationClient} instead.
 */
export class HttpClient {
  private baseUrl: string;
  private headers: Record<string, string>;

  /**
   * @param baseUrl - API server base URL (e.g. `https://api.example.com`).
   * @param apiKey - API key sent as `Authorization: ApiKey <key>`.
   * @param subscriberToken - Subscriber token sent as `X-Subscriber-Token` header.
   */
  constructor(baseUrl: string, apiKey: string, subscriberToken: string) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.headers = {
      'Content-Type': 'application/json',
      Authorization: `ApiKey ${apiKey}`,
      'X-Subscriber-Token': subscriberToken,
    };
  }

  /**
   * Sends a GET request.
   *
   * @typeParam T - Expected response type.
   * @param path - API path (e.g. `/api/v1/subscribers/123/feed`).
   * @param params - Optional query parameters.
   * @returns Parsed JSON response.
   * @throws {Error} If the response status is not 2xx.
   */
  async get<T>(path: string, params?: Record<string, string>): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (params) {
      Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v));
    }
    const resp = await fetch(url.toString(), { headers: this.headers });
    if (!resp.ok) {
      throw await this.handleError(resp);
    }
    return resp.json();
  }

  /**
   * Sends a POST request.
   *
   * @typeParam T - Expected response type.
   * @param path - API path.
   * @param body - Optional request body (serialized as JSON).
   * @returns Parsed JSON response.
   * @throws {Error} If the response status is not 2xx.
   */
  async post<T>(path: string, body?: unknown): Promise<T> {
    const resp = await fetch(`${this.baseUrl}${path}`, {
      method: 'POST',
      headers: this.headers,
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!resp.ok) {
      throw await this.handleError(resp);
    }
    return resp.json();
  }

  /**
   * Sends a PATCH request.
   *
   * @typeParam T - Expected response type.
   * @param path - API path.
   * @param body - Request body (serialized as JSON).
   * @returns Parsed JSON response.
   * @throws {Error} If the response status is not 2xx.
   */
  async patch<T>(path: string, body: unknown): Promise<T> {
    const resp = await fetch(`${this.baseUrl}${path}`, {
      method: 'PATCH',
      headers: this.headers,
      body: JSON.stringify(body),
    });
    if (!resp.ok) {
      throw await this.handleError(resp);
    }
    return resp.json();
  }

  private async handleError(resp: Response): Promise<Error> {
    try {
      const body = await resp.json();
      return new Error(body.error?.message || `HTTP ${resp.status}`);
    } catch {
      return new Error(`HTTP ${resp.status}`);
    }
  }
}
