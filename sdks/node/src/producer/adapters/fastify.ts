import type { FastifyPluginAsync, FastifyRequest, FastifyReply } from "fastify";
import type { ProducerConfig } from "../types";
import { Producer, recordRejection } from "../producer";

/**
 * Fastify-specific producer configuration.
 */
export interface FastifyProducerConfig extends ProducerConfig {
  /**
   * Prefix to add to log messages.
   * @default "cvt"
   */
  logPrefix?: string;
}

/**
 * Creates a Fastify plugin for producer validation.
 *
 * @example
 * ```typescript
 * import Fastify from 'fastify';
 * import { ContractValidator } from '@cvt/node-sdk';
 * import { fastifyProducerPlugin } from '@cvt/node-sdk/producer';
 *
 * const fastify = Fastify({ logger: true });
 *
 * const validator = new ContractValidator();
 * await validator.registerSchema('my-api', './openapi.json');
 *
 * await fastify.register(fastifyProducerPlugin, {
 *   schemaId: 'my-api',
 *   validator,
 *   mode: 'strict',
 * });
 *
 * fastify.get('/users', async (request, reply) => {
 *   return [{ id: 1, name: 'Alice' }];
 * });
 * ```
 */
export const fastifyProducerPlugin: FastifyPluginAsync<
  FastifyProducerConfig
> = async (fastify, config) => {
  const producer = new Producer(config);
  const logPrefix = config.logPrefix || "cvt";

  // Add preHandler hook for request validation
  fastify.addHook(
    "preHandler",
    async (request: FastifyRequest, reply: FastifyReply) => {
      // Check path filters
      const path = request.url;
      if (!producer.shouldValidatePath(path)) {
        return;
      }

      // Skip request validation if disabled
      if (config.validateRequest === false) {
        return;
      }

      const reqHeaders = normalizeHeaders(request.headers);
      const reqBody = request.body;

      // Shadow mode: async validation, don't block
      if (config.mode === "shadow") {
        producer
          .validateRequest(request.method, path, reqHeaders, reqBody)
          .then((result) => {
            if (!result.valid) {
              producer.handleRequestFailure(result, request, reply);
            }
          })
          .catch((error) => {
            request.log.error(
              { err: error },
              `[${logPrefix}] Validation error`,
            );
          });
        return;
      }

      // Strict/Warn mode: validate synchronously
      try {
        const result = await producer.validateRequest(
          request.method,
          path,
          reqHeaders,
          reqBody,
        );

        if (!result.valid) {
          const shouldContinue = await producer.handleRequestFailure(
            result,
            request,
            reply,
          );
          if (!shouldContinue) {
            recordRejection();
            // Send error response
            reply.code(400).send({
              error: "Request validation failed",
              details: result.errors,
            });
            return;
          }
        }
      } catch (error) {
        request.log.error({ err: error }, `[${logPrefix}] Validation error`);
        // Continue on validation errors
      }
    },
  );

  // Add onSend hook for response validation
  fastify.addHook(
    "onSend",
    async (request: FastifyRequest, reply: FastifyReply, payload: any) => {
      // Check path filters
      const path = request.url;
      if (!producer.shouldValidatePath(path)) {
        return payload;
      }

      // Skip response validation if disabled
      if (config.validateResponse === false) {
        return payload;
      }

      const reqHeaders = normalizeHeaders(request.headers);
      const reqBody = request.body;
      const respHeaders = normalizeHeaders(reply.getHeaders());

      // Parse payload
      let respBody: any = payload;
      if (typeof payload === "string") {
        try {
          respBody = JSON.parse(payload);
        } catch {
          // Not JSON, use as-is
        }
      }

      // Shadow mode: async validation
      if (config.mode === "shadow") {
        producer
          .validateResponse(
            request.method,
            path,
            reqHeaders,
            reqBody,
            reply.statusCode,
            respHeaders,
            respBody,
          )
          .then((result) => {
            if (!result.valid) {
              producer.handleResponseFailure(result, request, reply);
            }
          })
          .catch((error) => {
            request.log.error(
              { err: error },
              `[${logPrefix}] Validation error`,
            );
          });
        return payload;
      }

      // Strict/Warn mode: validate (but can't block response)
      try {
        const result = await producer.validateResponse(
          request.method,
          path,
          reqHeaders,
          reqBody,
          reply.statusCode,
          respHeaders,
          respBody,
        );

        if (!result.valid) {
          await producer.handleResponseFailure(result, request, reply);
        }
      } catch (error) {
        request.log.error({ err: error }, `[${logPrefix}] Validation error`);
      }

      return payload;
    },
  );
};

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
