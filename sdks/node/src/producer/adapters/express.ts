import type { Request, Response, NextFunction, RequestHandler } from "express";
import type { ProducerConfig } from "../types";
import { Producer, recordRejection } from "../producer";

/**
 * Express-specific producer configuration.
 */
export interface ExpressProducerConfig extends ProducerConfig {
  /**
   * Whether to capture the raw request body.
   * Required if using body-parser or similar middleware.
   * @default true
   */
  captureBody?: boolean;
}

/**
 * Creates Express middleware for producer validation.
 *
 * @example
 * ```typescript
 * import express from 'express';
 * import { ContractValidator } from '@cvt/node-sdk';
 * import { createExpressMiddleware } from '@cvt/node-sdk/producer';
 *
 * const app = express();
 * app.use(express.json());
 *
 * const validator = new ContractValidator();
 * await validator.registerSchema('my-api', './openapi.json');
 *
 * app.use(createExpressMiddleware({
 *   schemaId: 'my-api',
 *   validator,
 *   mode: 'strict',
 * }));
 *
 * app.get('/users', (req, res) => {
 *   res.json([{ id: 1, name: 'Alice' }]);
 * });
 * ```
 */
export function createExpressMiddleware(
  config: ExpressProducerConfig,
): RequestHandler {
  const producer = new Producer(config);
  const captureBody = config.captureBody !== false;

  return async (req: Request, res: Response, next: NextFunction) => {
    // Check path filters
    const path = req.originalUrl || req.url;
    if (!producer.shouldValidatePath(path)) {
      return next();
    }

    // Capture request data
    const reqHeaders = normalizeHeaders(req.headers);
    const reqBody = captureBody ? req.body : undefined;

    // Validate request (if not shadow mode, do it synchronously)
    if (config.validateRequest !== false && config.mode !== "shadow") {
      try {
        const result = await producer.validateRequest(
          req.method,
          path,
          reqHeaders,
          reqBody,
        );

        if (!result.valid) {
          const shouldContinue = await producer.handleRequestFailure(
            result,
            req,
            res,
          );
          if (!shouldContinue) {
            recordRejection();
            return;
          }
        }
      } catch (error) {
        console.error("[CVT] Request validation error:", error);
        // Continue on validation errors to not block the request
      }
    }

    // Shadow mode: async validation
    if (config.mode === "shadow" && config.validateRequest !== false) {
      producer
        .validateRequest(req.method, path, reqHeaders, reqBody)
        .then((result) => {
          if (!result.valid) {
            producer.handleRequestFailure(result, req, res);
          }
        })
        .catch((error) => {
          console.error("[CVT] Shadow request validation error:", error);
        });
    }

    // Capture response
    if (config.validateResponse !== false) {
      const originalJson = res.json.bind(res);
      const originalSend = res.send.bind(res);
      let responseBody: any;
      let captured = false;

      const captureAndValidate = async (body: any) => {
        if (captured) return;
        captured = true;

        responseBody = body;
        const respHeaders = normalizeHeaders(res.getHeaders());

        try {
          const result = await producer.validateResponse(
            req.method,
            path,
            reqHeaders,
            reqBody,
            res.statusCode,
            respHeaders,
            responseBody,
          );

          if (!result.valid) {
            await producer.handleResponseFailure(result, req, res);
          }
        } catch (error) {
          console.error("[CVT] Response validation error:", error);
        }
      };

      res.json = function (body: any) {
        if (config.mode === "shadow") {
          // Async validation
          captureAndValidate(body).catch(() => {});
        } else {
          // We can't block here since response is being sent
          // Just validate and log
          captureAndValidate(body).catch(() => {});
        }
        return originalJson(body);
      };

      res.send = function (body: any) {
        // Try to parse JSON body
        let parsedBody = body;
        if (typeof body === "string") {
          try {
            parsedBody = JSON.parse(body);
          } catch {
            // Not JSON, use as-is
          }
        }

        if (config.mode === "shadow") {
          captureAndValidate(parsedBody).catch(() => {});
        } else {
          captureAndValidate(parsedBody).catch(() => {});
        }
        return originalSend(body);
      };
    }

    next();
  };
}

/**
 * Normalizes headers to a simple string record.
 */
function normalizeHeaders(
  headers: Record<string, any>,
): Record<string, string> {
  const normalized: Record<string, string> = {};
  for (const [key, value] of Object.entries(headers)) {
    if (value !== undefined) {
      normalized[key.toLowerCase()] = Array.isArray(value)
        ? value[0]
        : String(value);
    }
  }
  return normalized;
}
