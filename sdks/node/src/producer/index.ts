/**
 * Producer validation module for CVT.
 *
 * This module provides server-side HTTP middleware for validating
 * incoming requests and outgoing responses against OpenAPI schemas.
 *
 * @example
 * ```typescript
 * import { ContractValidator } from '@cvt/node-sdk';
 * import { createExpressMiddleware } from '@cvt/node-sdk/producer';
 *
 * const validator = new ContractValidator();
 * await validator.registerSchema('my-api', './openapi.json');
 *
 * app.use(createExpressMiddleware({
 *   schemaId: 'my-api',
 *   validator,
 *   mode: 'strict',
 * }));
 * ```
 */

// Types
export type {
  ValidationMode,
  PathFilter,
  Interaction,
  Validator,
  ProducerValidationResult,
  ProducerConfig,
} from "./types";

export {
  matchesPathFilter,
  shouldValidatePath,
  defaultProducerConfig,
} from "./types";

// Core
export {
  Producer,
  recordValidationMetrics,
  recordRejection,
  getMetrics,
  resetMetrics,
} from "./producer";

// Express adapter
export type { ExpressProducerConfig } from "./adapters/express";
export { createExpressMiddleware } from "./adapters/express";

// Fastify adapter
export type { FastifyProducerConfig } from "./adapters/fastify";
export { fastifyProducerPlugin } from "./adapters/fastify";

// Testing
export type {
  ProducerTestConfig,
  ResponseData,
  RequestContext,
  ProducerValidationResult as TestValidationResult,
  ValidateResponseParams,
} from "./testing";
export { ProducerTestKit, createProducerTestKit } from "./testing";
