import type {
  AxiosInstance,
  AxiosResponse,
  InternalAxiosRequestConfig,
} from "axios";
import type {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
  ValidationResult,
} from "../index";
import type { AdapterConfig, CapturedInteraction, PathFilter } from "./types";
import { shouldValidatePath } from "./types";

/**
 * Configuration for the Axios adapter.
 */
export interface AxiosAdapterConfig extends AdapterConfig {
  /** The Axios instance to attach interceptors to */
  axios: AxiosInstance;
}

/**
 * Axios adapter that automatically captures and validates HTTP interactions.
 *
 * @example
 * ```typescript
 * import axios from 'axios';
 * import { ContractValidator } from '@sahina/cvt-sdk';
 * import { createAxiosAdapter } from '@sahina/cvt-sdk/adapters';
 *
 * const validator = new ContractValidator();
 * await validator.registerSchema('petstore', './openapi.json');
 *
 * const api = axios.create({ baseURL: 'http://localhost:3000' });
 * const adapter = createAxiosAdapter({ axios: api, validator });
 *
 * // All requests are now automatically validated
 * const response = await api.post('/pet', { name: 'Fluffy' });
 *
 * // Check captured interactions
 * const interactions = adapter.getInteractions();
 * console.log(interactions[0].validationResult?.valid);
 *
 * // Clean up when done
 * adapter.detach();
 * ```
 */
export class AxiosContractAdapter {
  private readonly axios: AxiosInstance;
  private readonly validator: ContractValidator;
  private readonly autoValidate: boolean;
  private readonly onValidationFailure: (
    result: ValidationResult,
    request: any,
    response: any,
  ) => void | Promise<void>;
  private readonly includePaths: PathFilter[];
  private readonly excludePaths: PathFilter[];

  private pendingRequests: Map<
    string,
    { config: InternalAxiosRequestConfig; body: any }
  > = new Map();
  private capturedInteractions: CapturedInteraction[] = [];
  private requestInterceptorId: number;
  private responseInterceptorId: number;

  constructor(config: AxiosAdapterConfig) {
    this.axios = config.axios;
    this.validator = config.validator;
    this.autoValidate = config.autoValidate ?? true;
    this.onValidationFailure =
      config.onValidationFailure ?? this.defaultFailureHandler;
    this.includePaths = config.includePaths ?? [];
    this.excludePaths = config.excludePaths ?? [];

    this.requestInterceptorId = this.attachRequestInterceptor();
    this.responseInterceptorId = this.attachResponseInterceptor();
  }

  /**
   * Attach interceptor to capture outgoing requests.
   */
  private attachRequestInterceptor(): number {
    return this.axios.interceptors.request.use((config) => {
      const requestId = this.generateRequestId();
      (config as any).__cvtRequestId = requestId;

      // Store the request config and body for later
      this.pendingRequests.set(requestId, {
        config: config,
        body: config.data,
      });

      return config;
    });
  }

  /**
   * Attach interceptor to capture and validate responses.
   */
  private attachResponseInterceptor(): number {
    return this.axios.interceptors.response.use(
      async (response) => {
        await this.handleResponse(response);
        return response;
      },
      async (error) => {
        // Also validate error responses (4xx, 5xx)
        if (error.response) {
          await this.handleResponse(error.response);
        }
        throw error;
      },
    );
  }

  /**
   * Process a response and optionally validate it.
   */
  private async handleResponse(response: AxiosResponse): Promise<void> {
    const requestId = (response.config as any).__cvtRequestId;
    const pendingRequest = this.pendingRequests.get(requestId);
    this.pendingRequests.delete(requestId);

    if (!pendingRequest) {
      return;
    }

    // Extract path from URL, including query params from config.params
    const url = new URL(response.config.url || "", response.config.baseURL);
    const searchParams = new URLSearchParams(url.search);

    // Add params from config.params (axios stores query params separately)
    if (response.config.params) {
      for (const [key, value] of Object.entries(response.config.params)) {
        if (value !== undefined && value !== null) {
          searchParams.set(key, String(value));
        }
      }
    }

    const queryString = searchParams.toString();
    const path = url.pathname + (queryString ? "?" + queryString : "");

    if (!shouldValidatePath(path, this.includePaths, this.excludePaths)) {
      return;
    }

    const validationRequest = this.extractRequest(
      pendingRequest.config,
      pendingRequest.body,
    );
    const validationResponse = this.extractResponse(response);

    const interaction: CapturedInteraction = {
      request: validationRequest,
      response: validationResponse,
      timestamp: new Date(),
    };

    if (this.autoValidate) {
      try {
        const result = await this.validator.validate(
          validationRequest,
          validationResponse,
        );
        interaction.validationResult = result;

        if (!result.valid) {
          await this.onValidationFailure(
            result,
            pendingRequest.config,
            response,
          );
        }
      } catch (error) {
        // Re-throw validation failures from onValidationFailure callback
        // Only swallow errors from the validator.validate call itself
        if ((error as Error).message?.includes("Contract validation failed")) {
          throw error;
        }
        console.error("CVT validation error:", error);
      }
    }

    this.capturedInteractions.push(interaction);
  }

  /**
   * Extract ValidationRequest from Axios request config.
   */
  private extractRequest(
    config: InternalAxiosRequestConfig,
    body: any,
  ): ValidationRequest {
    const url = new URL(config.url || "", config.baseURL);
    let path = url.pathname;

    // Build query string from both URL and config.params
    const searchParams = new URLSearchParams(url.search);

    // Add params from config.params (axios stores query params separately)
    if (config.params) {
      for (const [key, value] of Object.entries(config.params)) {
        if (value !== undefined && value !== null) {
          searchParams.set(key, String(value));
        }
      }
    }

    const queryString = searchParams.toString();
    if (queryString) {
      path += "?" + queryString;
    }

    return {
      method: (config.method || "GET").toUpperCase(),
      path,
      headers: this.normalizeHeaders(config.headers),
      body: this.parseBody(body),
    };
  }

  /**
   * Extract ValidationResponse from Axios response.
   */
  private extractResponse(response: AxiosResponse): ValidationResponse {
    return {
      statusCode: response.status,
      headers: this.normalizeHeaders(response.headers),
      body: response.data,
    };
  }

  /**
   * Normalize headers to Record<string, string>.
   */
  private normalizeHeaders(headers: any): Record<string, string> {
    const normalized: Record<string, string> = {};
    if (headers) {
      // Handle AxiosHeaders or plain objects
      let entries: [string, any][];
      if (typeof headers.entries === "function") {
        entries = Array.from(headers.entries() as Iterable<[string, any]>);
      } else {
        entries = Object.entries(headers);
      }

      for (const [key, value] of entries) {
        if (typeof value === "string") {
          normalized[key.toLowerCase()] = value;
        } else if (Array.isArray(value)) {
          normalized[key.toLowerCase()] = value.join(", ");
        }
      }
    }
    return normalized;
  }

  /**
   * Parse request body, handling string and object types.
   */
  private parseBody(body: any): any {
    if (body === undefined || body === null) {
      return undefined;
    }
    if (typeof body === "string") {
      try {
        return JSON.parse(body);
      } catch {
        return body;
      }
    }
    return body;
  }

  /**
   * Generate a unique request ID.
   */
  private generateRequestId(): string {
    return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
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

  /**
   * Detach interceptors and clean up resources.
   * Call this when you're done using the adapter.
   */
  detach(): void {
    this.axios.interceptors.request.eject(this.requestInterceptorId);
    this.axios.interceptors.response.eject(this.responseInterceptorId);
    this.pendingRequests.clear();
  }
}

/**
 * Factory function to create an Axios contract adapter.
 *
 * @param config - Adapter configuration
 * @returns A new AxiosContractAdapter instance
 *
 * @example
 * ```typescript
 * const adapter = createAxiosAdapter({
 *   axios: api,
 *   validator,
 *   autoValidate: true,
 *   excludePaths: ['/health', '/metrics'],
 * });
 * ```
 */
export function createAxiosAdapter(
  config: AxiosAdapterConfig,
): AxiosContractAdapter {
  return new AxiosContractAdapter(config);
}
