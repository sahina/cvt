import type {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
  ValidationResult,
} from "../index";

/**
 * Path filter type - can be a string (substring match) or RegExp (pattern match).
 */
export type PathFilter = string | RegExp;

/**
 * A captured HTTP interaction with optional validation result.
 */
export interface CapturedInteraction {
  /** The captured request */
  request: ValidationRequest;
  /** The captured response */
  response: ValidationResponse;
  /** The validation result (if autoValidate was enabled) */
  validationResult?: ValidationResult;
  /** Timestamp when the interaction was captured */
  timestamp: Date;
}

/**
 * Base configuration for CVT adapters.
 */
export interface AdapterConfig {
  /** CVT validator instance with registered schema */
  validator: ContractValidator;

  /**
   * Whether validation runs automatically on each request.
   * @default true
   */
  autoValidate?: boolean;

  /**
   * Callback when validation fails.
   * If not provided, throws an error by default.
   */
  onValidationFailure?: (
    result: ValidationResult,
    request: any,
    response: any,
  ) => void | Promise<void>;

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
  includePaths: PathFilter[],
  excludePaths: PathFilter[],
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
