import type { ContractValidator } from "../index";

/**
 * Validation mode determines how validation failures are handled.
 */
export type ValidationMode = "strict" | "warn" | "shadow";

/**
 * Path filter type - can be a string (substring match) or RegExp (pattern match).
 */
export type PathFilter = string | RegExp;

/**
 * Interaction represents an HTTP request/response pair for validation.
 */
export interface Interaction {
  /** Request method (GET, POST, etc.) */
  method: string;
  /** Request path with query string */
  path: string;
  /** Request headers */
  headers?: Record<string, string>;
  /** Request body (as string or object) */
  body?: any;
  /** Response status code */
  statusCode: number;
  /** Response headers */
  responseHeaders?: Record<string, string>;
  /** Response body (as string or object) */
  responseBody?: any;
}

/**
 * Validator interface for producer validation.
 * Can be implemented by gRPC client or embedded validator.
 */
export interface Validator {
  /**
   * Validates an interaction against a schema.
   * @param schemaId - The schema identifier
   * @param interaction - The HTTP interaction to validate
   * @returns Validation result
   */
  validate(
    schemaId: string,
    interaction: Interaction,
  ): Promise<ProducerValidationResult>;
}

/**
 * Validation result from producer validation.
 */
export interface ProducerValidationResult {
  /** Whether validation passed */
  valid: boolean;
  /** Validation error messages */
  errors?: string[];
  /** Type of validation (request or response) */
  type?: "request" | "response";
}

/**
 * Configuration for producer-side validation middleware.
 */
export interface ProducerConfig {
  /**
   * The schema ID to validate against (required).
   */
  schemaId: string;

  /**
   * The validator to use for validation (required).
   * This should be a ContractValidator instance or compatible validator.
   */
  validator: ContractValidator | Validator;

  /**
   * Validation mode determines how failures are handled.
   * - strict: Reject invalid requests with 400/422 errors
   * - warn: Log violations but allow requests to proceed
   * - shadow: Validate asynchronously, record metrics only
   * @default "strict"
   */
  mode?: ValidationMode;

  /**
   * Whether to validate incoming requests.
   * @default true
   */
  validateRequest?: boolean;

  /**
   * Whether to validate outgoing responses.
   * @default true
   */
  validateResponse?: boolean;

  /**
   * Only validate requests matching these patterns.
   * If empty, all paths are validated (subject to excludePaths).
   */
  includePaths?: PathFilter[];

  /**
   * Exclude requests matching these patterns from validation.
   * Evaluated before includePaths.
   */
  excludePaths?: PathFilter[];

  /**
   * Called when request validation fails.
   * For strict mode, return a response object to send, or undefined for default 400.
   * For warn/shadow modes, use for logging/alerting.
   */
  onRequestFailure?: (
    result: ProducerValidationResult,
    req: any,
  ) => any | void | Promise<any | void>;

  /**
   * Called when response validation fails.
   * Cannot modify the response (already sent), but useful for logging/alerting.
   */
  onResponseFailure?: (
    result: ProducerValidationResult,
    req: any,
    res: any,
  ) => void | Promise<void>;
}

/**
 * Check if a path matches a filter pattern.
 */
export function matchesPathFilter(path: string, pattern: PathFilter): boolean {
  if (typeof pattern === "string") {
    return path.includes(pattern);
  }
  return pattern.test(path);
}

/**
 * Determine if a path should be validated based on include/exclude filters.
 */
export function shouldValidatePath(
  path: string,
  includePaths: PathFilter[] = [],
  excludePaths: PathFilter[] = [],
): boolean {
  // Check excludes first
  for (const pattern of excludePaths) {
    if (matchesPathFilter(path, pattern)) {
      return false;
    }
  }

  // If includes specified, must match at least one
  if (includePaths.length > 0) {
    for (const pattern of includePaths) {
      if (matchesPathFilter(path, pattern)) {
        return true;
      }
    }
    return false;
  }

  return true;
}

/**
 * Default configuration values.
 */
export const defaultProducerConfig: Partial<ProducerConfig> = {
  mode: "strict",
  validateRequest: true,
  validateResponse: true,
  includePaths: [],
  excludePaths: [],
};
