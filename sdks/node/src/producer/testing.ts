import * as fs from "fs";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import * as path from "path";

// Resolve proto path relative to the package root
// At runtime: dist/src/producer/testing.js -> ../../../proto/cvt.proto
const PROTO_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "..",
  "proto",
  "cvt.proto",
);

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

const protoDescriptor = grpc.loadPackageDefinition(packageDefinition) as any;
const cvt = protoDescriptor.cvt;

/**
 * TLS configuration options for secure connections.
 */
export interface TLSOptions {
  /** Enable TLS for the connection. */
  enabled: boolean;
  /** Path to the root CA certificate file for server verification. */
  rootCertPath?: string;
  /** Path to the client certificate file (for mTLS). */
  certPath?: string;
  /** Path to the client private key file (for mTLS). */
  keyPath?: string;
}

/**
 * Configuration for the ProducerTestKit.
 */
export interface ProducerTestConfig {
  /** The schema ID to validate against (required). */
  schemaId: string;
  /** Optional schema version to validate against. */
  schemaVersion?: string;
  /** Server address (default: "localhost:50051"). */
  serverAddress?: string;
  /** API key for authentication (optional). */
  apiKey?: string;
  /** TLS configuration for secure connections. */
  tls?: TLSOptions;
}

/**
 * Response data for validation.
 */
export interface ResponseData {
  /** HTTP status code. */
  statusCode: number;
  /** Response headers. */
  headers?: Record<string, string>;
  /** Response body (object or string). */
  body?: any;
}

/**
 * Request context for path parameter extraction (optional).
 */
export interface RequestContext {
  /** HTTP method. */
  method?: string;
  /** Request path. */
  path?: string;
  /** Request headers. */
  headers?: Record<string, string>;
  /** Request body. */
  body?: any;
}

/**
 * Validation result from producer testing.
 */
export interface ProducerValidationResult {
  /** Whether the validation passed. */
  valid: boolean;
  /** Validation error messages (empty if valid). */
  errors: string[];
  /** Schema version used for validation. */
  validatedAgainstVersion?: string;
  /** Schema hash used for validation. */
  validatedAgainstHash?: string;
}

/**
 * Parameters for validating a producer response.
 */
export interface ValidateResponseParams {
  /** HTTP method (GET, POST, etc.). */
  method: string;
  /** API path with actual values (e.g., /users/123). */
  path: string;
  /** Response data to validate. */
  response: ResponseData;
  /** Optional request context for path parameter extraction. */
  request?: RequestContext;
}

/**
 * ProducerTestKit enables schema compliance testing for producers.
 *
 * This allows producers to validate their API responses against their OpenAPI
 * schema without needing real consumers. Use it in your test suite to ensure
 * your handler output matches your contract.
 *
 * @example
 * ```typescript
 * import { ProducerTestKit } from '@cvt/node-sdk/producer';
 *
 * describe('User API', () => {
 *   let testKit: ProducerTestKit;
 *
 *   beforeAll(async () => {
 *     // With TLS and API key
 *     testKit = new ProducerTestKit({
 *       schemaId: 'user-api',
 *       serverAddress: 'localhost:50051',
 *       tls: { enabled: true, rootCertPath: './certs/ca.crt' },
 *       apiKey: 'cvt-dev-key-12345',
 *     });
 *   });
 *
 *   afterAll(() => {
 *     testKit.close();
 *   });
 *
 *   it('should return valid user response', async () => {
 *     // Call your handler
 *     const response = await myHandler.getUser('123');
 *
 *     // Validate against schema
 *     const result = await testKit.validateResponse({
 *       method: 'GET',
 *       path: '/users/123',
 *       response: {
 *         statusCode: 200,
 *         body: response,
 *       },
 *     });
 *
 *     expect(result.valid).toBe(true);
 *   });
 * });
 * ```
 */
export class ProducerTestKit {
  private client: any;
  private schemaId: string;
  private schemaVersion?: string;
  private apiKey?: string;

  /**
   * Creates a new ProducerTestKit instance.
   * @param config - Configuration for the test kit.
   */
  constructor(config: ProducerTestConfig) {
    if (!config.schemaId) {
      throw new Error("schemaId is required");
    }

    this.schemaId = config.schemaId;
    this.schemaVersion = config.schemaVersion;
    this.apiKey = config.apiKey;

    const address = config.serverAddress || "localhost:50051";
    let credentials: grpc.ChannelCredentials;
    if (config.tls?.enabled) {
      credentials = this.createTLSCredentials(config.tls);
    } else {
      credentials = grpc.credentials.createInsecure();
    }
    this.client = new cvt.ContractValidator(address, credentials);
  }

  /**
   * Creates TLS credentials from the provided options.
   */
  private createTLSCredentials(
    tlsOptions: TLSOptions,
  ): grpc.ChannelCredentials {
    let rootCert: Buffer | null = null;
    let clientCert: Buffer | undefined;
    let clientKey: Buffer | undefined;

    if (tlsOptions.rootCertPath) {
      rootCert = fs.readFileSync(tlsOptions.rootCertPath);
    }

    if (tlsOptions.certPath && tlsOptions.keyPath) {
      clientCert = fs.readFileSync(tlsOptions.certPath);
      clientKey = fs.readFileSync(tlsOptions.keyPath);
    }

    return grpc.credentials.createSsl(rootCert, clientKey, clientCert);
  }

  /**
   * Creates gRPC metadata with the API key if configured.
   */
  private createMetadata(): grpc.Metadata {
    const metadata = new grpc.Metadata();
    if (this.apiKey) {
      metadata.set("x-api-key", this.apiKey);
    }
    return metadata;
  }

  /**
   * Serializes a body value to a JSON string.
   */
  private serializeBody(body: any): string {
    if (body === undefined || body === null) {
      return "";
    }
    if (typeof body === "string") {
      return body;
    }
    return JSON.stringify(body);
  }

  /**
   * Validates a producer's response against the registered schema.
   *
   * @param params - Validation parameters including method, path, and response data.
   * @returns A promise that resolves with the validation result.
   *
   * @example
   * ```typescript
   * const result = await testKit.validateResponse({
   *   method: 'GET',
   *   path: '/users/123',
   *   response: {
   *     statusCode: 200,
   *     headers: { 'Content-Type': 'application/json' },
   *     body: { id: '123', name: 'John Doe', email: 'john@example.com' },
   *   },
   * });
   *
   * if (!result.valid) {
   *   console.error('Validation errors:', result.errors);
   * }
   * ```
   */
  async validateResponse(
    params: ValidateResponseParams,
  ): Promise<ProducerValidationResult> {
    const { method, path, response, request } = params;

    // Build the gRPC request
    const grpcRequest: any = {
      schema_id: this.schemaId,
      schema_version: this.schemaVersion || "",
      method: method.toUpperCase(),
      path,
      response: {
        status_code: response.statusCode,
        headers: response.headers || {},
        body: this.serializeBody(response.body),
      },
    };

    // Add optional request context if provided
    if (request) {
      grpcRequest.request = {
        method: request.method || method,
        path: request.path || path,
        headers: request.headers || {},
        body: this.serializeBody(request.body),
      };
    }

    return new Promise((resolve, reject) => {
      this.client.ValidateProducerResponse(
        grpcRequest,
        this.createMetadata(),
        (err: any, result: any) => {
          if (err) {
            reject(err);
          } else {
            resolve({
              valid: result.valid,
              errors: result.errors || [],
              validatedAgainstVersion:
                result.validated_against_version || undefined,
              validatedAgainstHash: result.validated_against_hash || undefined,
            });
          }
        },
      );
    });
  }

  /**
   * Validates a full interaction (request + response) against the schema.
   * This is useful when you want to validate both the request and response
   * together, ensuring the complete contract is honored.
   *
   * @param params - Parameters including request and response data.
   * @returns A promise that resolves with the validation result.
   *
   * @example
   * ```typescript
   * const result = await testKit.validateInteraction({
   *   request: {
   *     method: 'POST',
   *     path: '/users',
   *     body: { name: 'John Doe', email: 'john@example.com' },
   *   },
   *   response: {
   *     statusCode: 201,
   *     body: { id: '123', name: 'John Doe', email: 'john@example.com' },
   *   },
   * });
   * ```
   */
  async validateInteraction(params: {
    request: {
      method: string;
      path: string;
      headers?: Record<string, string>;
      body?: any;
    };
    response: ResponseData;
  }): Promise<ProducerValidationResult> {
    return this.validateResponse({
      method: params.request.method,
      path: params.request.path,
      response: params.response,
      request: params.request,
    });
  }

  /**
   * Creates a helper for testing a specific endpoint.
   * This is useful when testing multiple scenarios for the same endpoint.
   *
   * @param method - HTTP method (GET, POST, etc.).
   * @param path - API path pattern (e.g., /users/{id}).
   * @returns An endpoint tester with a simplified validation interface.
   *
   * @example
   * ```typescript
   * const getUserEndpoint = testKit.forEndpoint('GET', '/users/{id}');
   *
   * // Test valid response
   * const result1 = await getUserEndpoint.validateResponse({
   *   statusCode: 200,
   *   body: { id: '123', name: 'John' },
   * }, { pathValues: { id: '123' } });
   *
   * // Test not found
   * const result2 = await getUserEndpoint.validateResponse({
   *   statusCode: 404,
   *   body: { error: 'User not found' },
   * }, { pathValues: { id: '999' } });
   * ```
   */
  forEndpoint(method: string, pathPattern: string) {
    return {
      /**
       * Validates a response for this endpoint.
       * @param response - Response data to validate.
       * @param options - Optional path values to substitute.
       */
      validateResponse: async (
        response: ResponseData,
        options?: { pathValues?: Record<string, string> },
      ): Promise<ProducerValidationResult> => {
        let actualPath = pathPattern;

        // Substitute path parameters if provided
        if (options?.pathValues) {
          for (const [key, value] of Object.entries(options.pathValues)) {
            actualPath = actualPath.replace(`{${key}}`, value);
          }
        }

        return this.validateResponse({
          method,
          path: actualPath,
          response,
        });
      },
    };
  }

  /**
   * Closes the gRPC client connection.
   * Should be called when the test kit is no longer needed.
   */
  close(): void {
    if (this.client) {
      this.client.close();
    }
  }
}

/**
 * Creates a ProducerTestKit with the given configuration.
 * This is an alternative to using `new ProducerTestKit(config)`.
 *
 * @param config - Configuration for the test kit.
 * @returns A new ProducerTestKit instance.
 *
 * @example
 * ```typescript
 * import { createProducerTestKit } from '@cvt/node-sdk/producer';
 *
 * const testKit = createProducerTestKit({
 *   schemaId: 'my-api',
 *   serverAddress: 'localhost:50051',
 * });
 * ```
 */
export function createProducerTestKit(
  config: ProducerTestConfig,
): ProducerTestKit {
  return new ProducerTestKit(config);
}
