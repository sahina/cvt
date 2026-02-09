import type {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
  GenerateOptions,
  GeneratedResponse,
} from "../index";
import type { CapturedInteraction, PathFilter } from "./types";
import { shouldValidatePath } from "./types";

/**
 * Configuration for the Mock adapter.
 */
export interface MockAdapterConfig {
  /** CVT validator instance with registered schema */
  validator: ContractValidator;

  /**
   * Whether to cache generated responses by method+path.
   * @default false
   */
  cache?: boolean;

  /**
   * Options for response generation.
   */
  generateOptions?: GenerateOptions;

  /**
   * Only mock requests matching these patterns.
   * If empty, all paths are mocked (subject to excludePaths).
   */
  includePaths?: PathFilter[];

  /**
   * Exclude requests matching these patterns from mocking.
   * Evaluated before includePaths.
   */
  excludePaths?: PathFilter[];
}

/**
 * Mock adapter that generates responses from OpenAPI schema instead of making real HTTP calls.
 *
 * This is useful for testing consumers against OpenAPI schemas without
 * requiring the producer API to be running.
 *
 * @example
 * ```typescript
 * import { ContractValidator } from '@sahina/cvt-sdk';
 * import { createMockAdapter } from '@sahina/cvt-sdk/adapters/mock';
 *
 * const validator = new ContractValidator();
 * await validator.registerSchema('petstore', './openapi.json');
 *
 * const mock = createMockAdapter({ validator, cache: true });
 *
 * // Use the mock fetch - no real HTTP call made!
 * const response = await mock.fetch('http://any.host/pet/1');
 * const data = await response.json();
 *
 * // Check captured interactions
 * const interactions = mock.getInteractions();
 * ```
 */
export class MockAdapter {
  private readonly validator: ContractValidator;
  private readonly cacheEnabled: boolean;
  private readonly generateOptions: GenerateOptions;
  private readonly includePaths: PathFilter[];
  private readonly excludePaths: PathFilter[];

  private capturedInteractions: CapturedInteraction[] = [];
  private responseCache: Map<string, GeneratedResponse> = new Map();

  constructor(config: MockAdapterConfig) {
    this.validator = config.validator;
    this.cacheEnabled = config.cache ?? false;
    this.generateOptions = config.generateOptions ?? {};
    this.includePaths = config.includePaths ?? [];
    this.excludePaths = config.excludePaths ?? [];
  }

  /**
   * Mock fetch function that generates responses from schema.
   *
   * @param input - URL string, URL object, or Request object
   * @param init - Optional fetch init options
   * @returns Promise resolving to the mock Response
   *
   * @example
   * ```typescript
   * // Simple GET
   * const response = await mock.fetch('http://api.test/pet/1');
   *
   * // POST with body
   * const response = await mock.fetch('http://api.test/pet', {
   *   method: 'POST',
   *   headers: { 'Content-Type': 'application/json' },
   *   body: JSON.stringify({ name: 'Fluffy', photoUrls: [] }),
   * });
   * ```
   */
  async fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    // Normalize input to get method and URL
    const { method, path, requestBody, headers } = await this.normalizeRequest(
      input,
      init,
    );

    // Check if we should mock this path
    if (!shouldValidatePath(path, this.includePaths, this.excludePaths)) {
      throw new Error(`cvt: path "${path}" is excluded from mocking`);
    }

    // Generate or retrieve cached response
    const generated = await this.getOrGenerateResponse(method, path);

    // Build validation request
    const validationRequest: ValidationRequest = {
      method,
      path,
      headers,
      body: requestBody,
    };

    // Build validation response
    const validationResponse: ValidationResponse = {
      statusCode: generated.statusCode,
      headers: generated.headers,
      body: generated.body,
    };

    // Record interaction
    const interaction: CapturedInteraction = {
      request: validationRequest,
      response: validationResponse,
      timestamp: new Date(),
    };
    this.capturedInteractions.push(interaction);

    // Build and return Response
    return this.buildResponse(generated);
  }

  /**
   * Normalize input to extract method, URL, and body.
   */
  private async normalizeRequest(
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<{
    method: string;
    url: string;
    path: string;
    requestBody: any;
    headers: Record<string, string>;
  }> {
    let method = "GET";
    let url: string;
    let body: any = undefined;
    let headers: Record<string, string> = {};

    if (input instanceof Request) {
      method = input.method.toUpperCase();
      url = input.url;
      headers = this.headersToRecord(input.headers);
      if (input.body) {
        const clone = input.clone();
        try {
          const text = await clone.text();
          body = text ? JSON.parse(text) : undefined;
        } catch {
          body = undefined;
        }
      }
    } else if (input instanceof URL) {
      url = input.toString();
    } else {
      url = input;
    }

    // Override with init if provided
    if (init) {
      if (init.method) {
        method = init.method.toUpperCase();
      }
      if (init.headers) {
        headers = this.normalizeHeaders(init.headers);
      }
      if (init.body) {
        if (typeof init.body === "string") {
          try {
            body = JSON.parse(init.body);
          } catch {
            body = init.body;
          }
        } else {
          body = init.body;
        }
      }
    }

    // Extract path from URL
    let path: string;
    try {
      const urlObj = new URL(url);
      path = urlObj.pathname + urlObj.search;
    } catch {
      // If URL parsing fails, assume it's already a path
      path = url;
    }

    return { method, url, path, requestBody: body, headers };
  }

  /**
   * Convert Headers object to Record.
   */
  private headersToRecord(headers: Headers): Record<string, string> {
    const result: Record<string, string> = {};
    headers.forEach((value, key) => {
      result[key.toLowerCase()] = value;
    });
    return result;
  }

  /**
   * Normalize headers from various formats.
   */
  private normalizeHeaders(headers: HeadersInit): Record<string, string> {
    const result: Record<string, string> = {};

    if (headers instanceof Headers) {
      headers.forEach((value, key) => {
        result[key.toLowerCase()] = value;
      });
    } else if (Array.isArray(headers)) {
      for (const [key, value] of headers) {
        result[key.toLowerCase()] = value;
      }
    } else {
      for (const [key, value] of Object.entries(headers)) {
        result[key.toLowerCase()] = value;
      }
    }

    return result;
  }

  /**
   * Get cached response or generate a new one.
   */
  private async getOrGenerateResponse(
    method: string,
    path: string,
  ): Promise<GeneratedResponse> {
    // Strip query params for route matching - OpenAPI paths don't include query strings
    const pathWithoutQuery = path.split("?")[0];
    const cacheKey = `${method}:${pathWithoutQuery}`;

    // Check cache
    if (this.cacheEnabled) {
      const cached = this.responseCache.get(cacheKey);
      if (cached) {
        return cached;
      }
    }

    // Generate new response using path without query params
    const generated = await this.validator.generateResponse(
      method,
      pathWithoutQuery,
      this.generateOptions,
    );

    // Cache if enabled
    if (this.cacheEnabled) {
      this.responseCache.set(cacheKey, generated);
    }

    return generated;
  }

  /**
   * Build a Response object from GeneratedResponse.
   */
  private buildResponse(generated: GeneratedResponse): Response {
    const body = generated.body ? JSON.stringify(generated.body) : null;

    const headers = new Headers();
    if (generated.headers) {
      for (const [key, value] of Object.entries(generated.headers)) {
        headers.set(key, value);
      }
    }

    // Ensure Content-Type is set
    if (!headers.has("content-type") && body) {
      headers.set("content-type", "application/json");
    }

    return new Response(body, {
      status: generated.statusCode,
      statusText: this.getStatusText(generated.statusCode),
      headers,
    });
  }

  /**
   * Get status text for a status code.
   */
  private getStatusText(statusCode: number): string {
    const statusTexts: Record<number, string> = {
      200: "OK",
      201: "Created",
      204: "No Content",
      400: "Bad Request",
      401: "Unauthorized",
      403: "Forbidden",
      404: "Not Found",
      500: "Internal Server Error",
    };
    return statusTexts[statusCode] ?? "";
  }

  /**
   * Get all captured interactions.
   */
  getInteractions(): CapturedInteraction[] {
    return [...this.capturedInteractions];
  }

  /**
   * Clear all captured interactions.
   */
  clearInteractions(): void {
    this.capturedInteractions = [];
  }

  /**
   * Clear the response cache.
   */
  clearCache(): void {
    this.responseCache.clear();
  }
}

/**
 * Factory function to create a Mock adapter.
 *
 * @param config - Adapter configuration
 * @returns A new MockAdapter instance
 *
 * @example
 * ```typescript
 * const mock = createMockAdapter({
 *   validator,
 *   cache: true,
 * });
 *
 * const response = await mock.fetch('http://any.host/pet/1');
 * ```
 */
export function createMockAdapter(config: MockAdapterConfig): MockAdapter {
  return new MockAdapter(config);
}

/**
 * Create a mock fetch function that generates responses from schema.
 *
 * This is the simplest way to create a mock client when you don't need
 * to track interactions.
 *
 * @param validator - CVT validator with registered schema
 * @param options - Optional configuration
 * @returns A fetch-like function
 *
 * @example
 * ```typescript
 * const mockFetch = createMockFetch(validator);
 *
 * // Use like native fetch
 * const response = await mockFetch('http://any.host/pet/1');
 * const data = await response.json();
 * ```
 */
export function createMockFetch(
  validator: ContractValidator,
  options: Omit<MockAdapterConfig, "validator"> = {},
): (input: RequestInfo | URL, init?: RequestInit) => Promise<Response> {
  const adapter = new MockAdapter({ validator, ...options });
  return (input: RequestInfo | URL, init?: RequestInit) =>
    adapter.fetch(input, init);
}
