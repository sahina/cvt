import type { ContractValidator } from "../index";
import type {
  ProducerConfig,
  ProducerValidationResult,
  Interaction,
  Validator,
} from "./types";
import { shouldValidatePath, defaultProducerConfig } from "./types";

/**
 * Producer handles server-side validation of HTTP requests and responses.
 */
export class Producer {
  private readonly config: Required<
    Pick<
      ProducerConfig,
      | "schemaId"
      | "mode"
      | "validateRequest"
      | "validateResponse"
      | "includePaths"
      | "excludePaths"
    >
  > &
    ProducerConfig;

  constructor(config: ProducerConfig) {
    if (!config.schemaId) {
      throw new Error("schemaId is required");
    }
    if (!config.validator) {
      throw new Error("validator is required");
    }

    this.config = {
      ...defaultProducerConfig,
      ...config,
    } as any;
  }

  /**
   * Check if a path should be validated.
   */
  shouldValidatePath(path: string): boolean {
    return shouldValidatePath(
      path,
      this.config.includePaths,
      this.config.excludePaths,
    );
  }

  /**
   * Validates an incoming HTTP request.
   */
  async validateRequest(
    method: string,
    path: string,
    headers: Record<string, string> = {},
    body?: any,
  ): Promise<ProducerValidationResult> {
    if (!this.config.validateRequest) {
      return { valid: true, type: "request" };
    }

    const interaction: Interaction = {
      method,
      path,
      headers,
      body,
      // Minimal valid response for request-only validation
      statusCode: 200,
      responseBody: {},
    };

    const result = await this.validate(interaction);
    return { ...result, type: "request" };
  }

  /**
   * Validates an outgoing HTTP response.
   */
  async validateResponse(
    method: string,
    path: string,
    reqHeaders: Record<string, string> = {},
    reqBody: any,
    statusCode: number,
    respHeaders: Record<string, string> = {},
    respBody: any,
  ): Promise<ProducerValidationResult> {
    if (!this.config.validateResponse) {
      return { valid: true, type: "response" };
    }

    const interaction: Interaction = {
      method,
      path,
      headers: reqHeaders,
      body: reqBody,
      statusCode,
      responseHeaders: respHeaders,
      responseBody: respBody,
    };

    const result = await this.validate(interaction);
    return { ...result, type: "response" };
  }

  /**
   * Performs the actual validation.
   */
  private async validate(
    interaction: Interaction,
  ): Promise<ProducerValidationResult> {
    const validator = this.config.validator;

    // Check if it's a ContractValidator (gRPC client)
    if ("validate" in validator && typeof validator.validate === "function") {
      // Could be either ContractValidator or custom Validator interface
      if (isContractValidator(validator)) {
        // Use ContractValidator's validate method
        const result = await validator.validate(
          {
            method: interaction.method,
            path: interaction.path,
            headers: interaction.headers,
            body: interaction.body,
          },
          {
            statusCode: interaction.statusCode,
            headers: interaction.responseHeaders,
            body: interaction.responseBody,
          },
        );
        return {
          valid: result.valid,
          errors: result.errors,
        };
      } else {
        // Use Validator interface
        return (validator as Validator).validate(
          this.config.schemaId,
          interaction,
        );
      }
    }

    throw new Error("Invalid validator type");
  }

  /**
   * Handles request validation failure based on mode.
   * @returns true if request should continue, false if rejected
   */
  async handleRequestFailure(
    result: ProducerValidationResult,
    req: any,
    res: any,
  ): Promise<boolean> {
    // Call custom handler if configured
    if (this.config.onRequestFailure) {
      const customResponse = await this.config.onRequestFailure(result, req);
      if (customResponse !== undefined) {
        // Custom handler provided a response
        return false;
      }
    }

    switch (this.config.mode) {
      case "strict":
        // Send 400 response
        this.writeErrorResponse(res, result);
        return false;

      case "warn":
        // Log and continue
        this.logValidationFailure("request", req, result);
        return true;

      case "shadow":
        // Metrics only, continue
        recordValidationMetrics("request", result);
        return true;
    }

    return true;
  }

  /**
   * Handles response validation failure based on mode.
   */
  async handleResponseFailure(
    result: ProducerValidationResult,
    req: any,
    res: any,
  ): Promise<void> {
    // Call custom handler if configured
    if (this.config.onResponseFailure) {
      await this.config.onResponseFailure(result, req, res);
      return;
    }

    switch (this.config.mode) {
      case "strict":
      case "warn":
        // Log the failure (can't modify response - already sent)
        this.logValidationFailure("response", req, result);
        break;

      case "shadow":
        // Metrics only
        recordValidationMetrics("response", result);
        break;
    }
  }

  /**
   * Writes a standardized error response.
   */
  private writeErrorResponse(res: any, result: ProducerValidationResult): void {
    if (res.headersSent) {
      return;
    }

    const errorBody = {
      error: "Request validation failed",
      details: result.errors || [],
    };

    // Express-style
    if (typeof res.status === "function" && typeof res.json === "function") {
      res.status(400).json(errorBody);
      return;
    }

    // Node http-style
    res.statusCode = 400;
    res.setHeader("Content-Type", "application/json");
    res.end(JSON.stringify(errorBody));
  }

  /**
   * Logs a validation failure.
   */
  private logValidationFailure(
    type: string,
    req: any,
    result: ProducerValidationResult,
  ): void {
    const method = req.method || "UNKNOWN";
    const path = req.url || req.path || "UNKNOWN";
    console.warn(
      `[CVT] ${type} validation failed for ${method} ${path}:`,
      result.errors,
    );
  }

  /**
   * Gets the validation mode.
   */
  get mode(): string {
    return this.config.mode;
  }
}

/**
 * Type guard to check if validator is a ContractValidator.
 */
function isContractValidator(
  validator: ContractValidator | Validator,
): validator is ContractValidator {
  return (
    "registerSchema" in validator &&
    typeof (validator as any).registerSchema === "function"
  );
}

// Simple metrics tracking
const metrics = {
  requestValidations: 0,
  requestValidationsPassed: 0,
  requestValidationsFailed: 0,
  responseValidations: 0,
  responseValidationsPassed: 0,
  responseValidationsFailed: 0,
  requestsRejected: 0,
};

/**
 * Records validation metrics.
 */
export function recordValidationMetrics(
  type: "request" | "response",
  result: ProducerValidationResult,
): void {
  if (type === "request") {
    metrics.requestValidations++;
    if (result.valid) {
      metrics.requestValidationsPassed++;
    } else {
      metrics.requestValidationsFailed++;
    }
  } else {
    metrics.responseValidations++;
    if (result.valid) {
      metrics.responseValidationsPassed++;
    } else {
      metrics.responseValidationsFailed++;
    }
  }
}

/**
 * Records a request rejection.
 */
export function recordRejection(): void {
  metrics.requestsRejected++;
}

/**
 * Gets the current metrics snapshot.
 */
export function getMetrics(): typeof metrics {
  return { ...metrics };
}

/**
 * Resets all metrics.
 */
export function resetMetrics(): void {
  metrics.requestValidations = 0;
  metrics.requestValidationsPassed = 0;
  metrics.requestValidationsFailed = 0;
  metrics.responseValidations = 0;
  metrics.responseValidationsPassed = 0;
  metrics.responseValidationsFailed = 0;
  metrics.requestsRejected = 0;
}
