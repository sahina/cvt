import type {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
  ValidationResult,
} from "../index";
import type { AdapterConfig, CapturedInteraction, PathFilter } from "./types";
import { shouldValidatePath } from "./types";

/**
 * Configuration for the Fetch adapter.
 */
export interface FetchAdapterConfig extends AdapterConfig {
  /**
   * Base URL for relative requests.
   * If not provided, requests must use absolute URLs.
   */
  baseURL?: string;
}

/**
 * Fetch adapter that wraps the native fetch API to capture and validate HTTP interactions.
 *
 * @example
 * ```typescript
 * import { ContractValidator } from '@cvt/cvt-sdk';
 * import { createFetchAdapter } from '@cvt/cvt-sdk/adapters/fetch';
 *
 * const validator = new ContractValidator();
 * await validator.registerSchema('petstore', './openapi.json');
 *
 * const adapter = createFetchAdapter({
 *   validator,
 *   baseURL: 'http://localhost:3000',
 * });
 *
 * // Use the wrapped fetch function
 * const response = await adapter.fetch('/pet/1');
 * const data = await response.json();
 *
 * // Check captured interactions
 * const interactions = adapter.getInteractions();
 * console.log(interactions[0].validationResult?.valid);
 * ```
 */
export class FetchContractAdapter {
  private readonly validator: ContractValidator;
  private readonly baseURL: string;
  private readonly autoValidate: boolean;
  private readonly onValidationFailure: (
    result: ValidationResult,
    request: Request,
    response: Response,
  ) => void | Promise<void>;
  private readonly includePaths: PathFilter[];
  private readonly excludePaths: PathFilter[];

  private capturedInteractions: CapturedInteraction[] = [];

  constructor(config: FetchAdapterConfig) {
    this.validator = config.validator;
    this.baseURL = config.baseURL ?? "";
    this.autoValidate = config.autoValidate ?? true;
    this.onValidationFailure =
      config.onValidationFailure ?? this.defaultFailureHandler;
    this.includePaths = config.includePaths ?? [];
    this.excludePaths = config.excludePaths ?? [];
  }

  /**
   * Wrapped fetch function that captures and validates HTTP interactions.
   *
   * @param input - URL string, URL object, or Request object
   * @param init - Optional fetch init options
   * @returns Promise resolving to the Response
   *
   * @example
   * ```typescript
   * // Simple GET
   * const response = await adapter.fetch('/pet/1');
   *
   * // POST with body
   * const response = await adapter.fetch('/pet', {
   *   method: 'POST',
   *   headers: { 'Content-Type': 'application/json' },
   *   body: JSON.stringify({ name: 'Fluffy', photoUrls: [] }),
   * });
   * ```
   */
  async fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    // Normalize input to Request
    const request = this.normalizeRequest(input, init);

    // Clone request to read body (body can only be read once)
    const requestClone = request.clone();
    const requestBody = await this.readRequestBody(requestClone);

    // Execute the actual fetch
    const response = await fetch(request);

    // Check if we should validate this path
    const url = new URL(request.url);
    const path = url.pathname + url.search;

    if (!shouldValidatePath(path, this.includePaths, this.excludePaths)) {
      return response;
    }

    // Clone response to read body (body can only be read once)
    const responseClone = response.clone();
    const responseBody = await this.readResponseBody(responseClone);

    // Build validation objects
    const validationRequest = this.extractRequest(request, path, requestBody);
    const validationResponse = this.extractResponse(response, responseBody);

    const interaction: CapturedInteraction = {
      request: validationRequest,
      response: validationResponse,
      timestamp: new Date(),
    };

    // Validate if auto-validate is enabled
    if (this.autoValidate) {
      try {
        const result = await this.validator.validate(
          validationRequest,
          validationResponse,
        );
        interaction.validationResult = result;

        if (!result.valid) {
          await this.onValidationFailure(result, request, response);
        }
      } catch (error) {
        // Re-throw validation failures from onValidationFailure callback
        if ((error as Error).message?.includes("Contract validation failed")) {
          throw error;
        }
        console.error("CVT validation error:", error);
      }
    }

    this.capturedInteractions.push(interaction);
    return response;
  }

  /**
   * Normalize input to a Request object.
   */
  private normalizeRequest(
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Request {
    if (input instanceof Request) {
      return init ? new Request(input, init) : input;
    }

    let url: string;
    if (input instanceof URL) {
      url = input.toString();
    } else if (typeof input === "string") {
      // Handle relative URLs with baseURL
      if (
        this.baseURL &&
        !input.startsWith("http://") &&
        !input.startsWith("https://")
      ) {
        url = this.baseURL.replace(/\/$/, "") + "/" + input.replace(/^\//, "");
      } else {
        url = input;
      }
    } else {
      url = String(input);
    }

    return new Request(url, init);
  }

  /**
   * Read request body and parse as JSON if possible.
   */
  private async readRequestBody(request: Request): Promise<any> {
    if (!request.body) {
      return undefined;
    }

    try {
      const text = await request.text();
      if (!text) {
        return undefined;
      }

      try {
        return JSON.parse(text);
      } catch {
        return text;
      }
    } catch {
      return undefined;
    }
  }

  /**
   * Read response body and parse as JSON if possible.
   */
  private async readResponseBody(response: Response): Promise<any> {
    try {
      const text = await response.text();
      if (!text) {
        return undefined;
      }

      try {
        return JSON.parse(text);
      } catch {
        return text;
      }
    } catch {
      return undefined;
    }
  }

  /**
   * Extract ValidationRequest from fetch Request.
   */
  private extractRequest(
    request: Request,
    path: string,
    body: any,
  ): ValidationRequest {
    const headers = this.normalizeHeaders(request.headers);

    return {
      method: request.method.toUpperCase(),
      path,
      headers,
      body,
    };
  }

  /**
   * Extract ValidationResponse from fetch Response.
   */
  private extractResponse(response: Response, body: any): ValidationResponse {
    const headers = this.normalizeHeaders(response.headers);

    return {
      statusCode: response.status,
      headers,
      body,
    };
  }

  /**
   * Normalize Headers to Record<string, string>.
   */
  private normalizeHeaders(headers: Headers): Record<string, string> {
    const normalized: Record<string, string> = {};
    headers.forEach((value, key) => {
      normalized[key.toLowerCase()] = value;
    });
    return normalized;
  }

  /**
   * Default handler for validation failures - throws an error.
   */
  private defaultFailureHandler(result: ValidationResult): void {
    const errors = result.errors?.join(", ") || "Unknown validation error";
    throw new Error(`Contract validation failed: ${errors}`);
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
   * Manually validate a captured interaction.
   */
  async validateInteraction(
    interaction: CapturedInteraction,
  ): Promise<ValidationResult> {
    return this.validator.validate(interaction.request, interaction.response);
  }
}

/**
 * Factory function to create a Fetch contract adapter.
 *
 * @param config - Adapter configuration
 * @returns A new FetchContractAdapter instance
 *
 * @example
 * ```typescript
 * const adapter = createFetchAdapter({
 *   validator,
 *   baseURL: 'http://localhost:3000',
 *   autoValidate: true,
 *   excludePaths: ['/health', '/metrics'],
 * });
 *
 * // Use the wrapped fetch
 * const response = await adapter.fetch('/pet/1');
 * ```
 */
export function createFetchAdapter(
  config: FetchAdapterConfig,
): FetchContractAdapter {
  return new FetchContractAdapter(config);
}

/**
 * Create a global fetch wrapper that validates all requests.
 *
 * @param config - Adapter configuration
 * @returns An object with the wrapped fetch function and adapter
 *
 * @example
 * ```typescript
 * const { fetch: validatedFetch, adapter } = createValidatingFetch({
 *   validator,
 *   baseURL: 'http://localhost:3000',
 * });
 *
 * // Use like native fetch
 * const response = await validatedFetch('/pet/1');
 *
 * // Check interactions
 * console.log(adapter.getInteractions());
 * ```
 */
export function createValidatingFetch(config: FetchAdapterConfig): {
  fetch: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
  adapter: FetchContractAdapter;
} {
  const adapter = new FetchContractAdapter(config);
  return {
    fetch: (input: RequestInfo | URL, init?: RequestInit) =>
      adapter.fetch(input, init),
    adapter,
  };
}
