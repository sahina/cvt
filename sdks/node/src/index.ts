import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import * as fs from "fs";
import * as http from "http";
import * as https from "https";
import * as path from "path";
import { buildConsumerFromInteractions as buildConsumer } from "./auto-register";

// Resolve proto path relative to the package root
// At runtime: dist/src/index.js -> ../../proto/cvt.proto
const PROTO_PATH = path.resolve(__dirname, "..", "..", "proto", "cvt.proto");

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
 * Configuration options for the ContractValidator.
 */
export interface ContractValidatorOptions {
  /** The address of the CVT gRPC server (default: "localhost:9550"). */
  address?: string;
  /** TLS configuration for secure connections. */
  tls?: TLSOptions;
  /** API key for authentication. */
  apiKey?: string;
}

export interface ValidationRequest<TBody = any> {
  method: string;
  path: string;
  headers?: Record<string, any>;
  body?: TBody;
  [key: string]: any;
}

export interface ValidationResponse<TBody = any> {
  statusCode?: number;
  status_code?: number;
  headers?: Record<string, any>;
  body?: TBody;
  [key: string]: any;
}

export interface ValidationResult {
  valid: boolean;
  errors?: string[];
}

/**
 * Represents a breaking change detected between schema versions.
 */
export interface BreakingChange {
  /** Type of breaking change (e.g., ENDPOINT_REMOVED, REQUIRED_FIELD_ADDED) */
  type: string;
  /** API path affected (e.g., "/pet/{petId}") */
  path: string;
  /** HTTP method affected (e.g., "DELETE") */
  method: string;
  /** Human-readable description of the breaking change */
  description: string;
  /** Previous value (for context) */
  oldValue?: string;
  /** New value (for context) */
  newValue?: string;
}

/**
 * Result of comparing two schema versions.
 */
export interface CompareResult {
  /** True if no breaking changes were detected */
  compatible: boolean;
  /** List of breaking changes detected */
  breakingChanges: BreakingChange[];
  /** Metadata about the old schema version */
  oldSchema?: {
    schemaId: string;
    schemaVersion: string;
  };
  /** Metadata about the new schema version */
  newSchema?: {
    schemaId: string;
    schemaVersion: string;
  };
}

/**
 * Options for generating test fixtures.
 */
export interface GenerateOptions {
  /** Response status code to generate (default: first successful status) */
  statusCode?: number;
  /** Whether to use schema examples when available (default: true) */
  useExamples?: boolean;
  /** Content type for request/response (default: "application/json") */
  contentType?: string;
}

/**
 * Generated HTTP request fixture.
 */
export interface GeneratedRequest {
  method: string;
  path: string;
  headers?: Record<string, string>;
  body?: any;
}

/**
 * Generated HTTP response fixture.
 */
export interface GeneratedResponse {
  statusCode: number;
  headers?: Record<string, string>;
  body?: any;
}

/**
 * Complete generated fixture with request and response.
 */
export interface GeneratedFixture {
  request: GeneratedRequest;
  response: GeneratedResponse;
}

/**
 * Information about an API endpoint.
 */
export interface EndpointInfo {
  method: string;
  path: string;
  operationId?: string;
  summary?: string;
}

// ============================================================================
// Consumer Registry Types
// ============================================================================

/**
 * Describes which endpoints and fields a consumer uses.
 */
export interface EndpointUsage {
  /** HTTP method (GET, POST, etc.) */
  method: string;
  /** API path (e.g., "/users/{id}") */
  path: string;
  /** Fields used in response (e.g., ["email", "name"]) */
  usedFields?: string[];
}

/**
 * Information about a registered consumer.
 */
export interface ConsumerInfo {
  /** Unique consumer identifier (e.g., "order-service") */
  consumerId: string;
  /** Consumer's version (e.g., "2.1.0") */
  consumerVersion: string;
  /** Schema this consumer depends on */
  schemaId: string;
  /** Schema version consumer was tested against */
  schemaVersion: string;
  /** Environment (dev, staging, prod) */
  environment: string;
  /** Unix timestamp of registration */
  registeredAt: number;
  /** Unix timestamp of last successful validation */
  lastValidatedAt: number;
  /** Which endpoints the consumer uses */
  usedEndpoints: EndpointUsage[];
}

/**
 * Options for registering a consumer.
 */
export interface RegisterConsumerOptions {
  /** Unique consumer identifier (e.g., "order-service") */
  consumerId: string;
  /** Consumer's version (e.g., "2.1.0") */
  consumerVersion: string;
  /** Schema this consumer depends on */
  schemaId: string;
  /** Schema version consumer was tested against */
  schemaVersion: string;
  /** Environment (dev, staging, prod) */
  environment: string;
  /** Which endpoints the consumer uses */
  usedEndpoints?: EndpointUsage[];
}

/**
 * Impact of schema changes on a specific consumer.
 */
export interface ConsumerImpact {
  /** Consumer identifier */
  consumerId: string;
  /** Consumer version */
  consumerVersion: string;
  /** Schema version consumer was tested against */
  currentSchemaVersion: string;
  /** Environment */
  environment: string;
  /** True if consumer will be affected */
  willBreak: boolean;
  /** Breaking changes that affect this consumer */
  relevantChanges: BreakingChange[];
}

/**
 * Result of can-i-deploy check.
 */
export interface CanIDeployResult {
  /** True if safe to deploy */
  safeToDeploy: boolean;
  /** Human-readable summary */
  summary: string;
  /** All breaking changes in the new version */
  breakingChanges: BreakingChange[];
  /** Impact on each affected consumer */
  affectedConsumers: ConsumerImpact[];
}

/**
 * Ownership information for a registered schema.
 */
export interface SchemaOwnership {
  owner: string;
  team: string;
  contactEmail: string;
  readOnly: boolean;
}

/**
 * Metadata describing a schema resolved via useSchema.
 */
export interface SchemaInfo {
  /** Unique identifier for the schema. */
  schemaId: string;
  /** Concrete semantic version the validator is bound to. */
  schemaVersion: string;
  /** SHA256 hash of the schema content. */
  schemaHash: string;
  /** Unix timestamp of initial registration. */
  registeredAt: number;
  /** Unix timestamp of last update. */
  updatedAt: number;
  /** Optional ownership information. */
  ownership?: SchemaOwnership;
  /** OpenAPI version detected on the server (e.g., "3.0.0"). */
  openapiVersion: string;
  /** Number of endpoints exposed by the schema. */
  endpointCount: number;
}

/**
 * Client for the Contract Validator Toolkit (CVT).
 * Allows validating HTTP interactions against OpenAPI schemas via a gRPC service.
 */
export class ContractValidator {
  private client: any;
  private schemaId: string | null = null;
  private schemaVersion: string | null = null;
  private apiKey: string | undefined;

  /**
   * Creates a new ContractValidator instance.
   * @param addressOrOptions - Either a server address string or a ContractValidatorOptions object.
   *
   * @example
   * // Simple usage (insecure connection)
   * const validator = new ContractValidator("localhost:9550");
   *
   * @example
   * // With TLS and API key
   * const validator = new ContractValidator({
   *   address: "localhost:9550",
   *   tls: { enabled: true, rootCertPath: "./certs/ca.crt" },
   *   apiKey: "cvt-dev-key-12345"
   * });
   */
  constructor(
    addressOrOptions: string | ContractValidatorOptions = "localhost:9550",
  ) {
    let address: string;
    let credentials: grpc.ChannelCredentials;
    let options: grpc.ClientOptions = {};

    if (typeof addressOrOptions === "string") {
      // Legacy mode: just an address string
      address = addressOrOptions;
      credentials = grpc.credentials.createInsecure();
    } else {
      // New options mode
      address = addressOrOptions.address || "localhost:9550";
      this.apiKey = addressOrOptions.apiKey;

      if (addressOrOptions.tls?.enabled) {
        credentials = this.createTLSCredentials(addressOrOptions.tls);
      } else {
        credentials = grpc.credentials.createInsecure();
      }
    }

    this.client = new cvt.ContractValidator(address, credentials, options);
  }

  /**
   * Creates TLS credentials from the provided options.
   */
  private createTLSCredentials(
    tlsOptions: TLSOptions,
  ): grpc.ChannelCredentials {
    let rootCert: Buffer | undefined;
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
   * Registers a schema for validation.
   * @param schemaId - A unique identifier for the schema.
   * @param schemaPath - The path to the schema file (local file path or HTTP/HTTPS URL).
   * @returns A promise that resolves when the schema is successfully registered.
   */
  async registerSchema(schemaId: string, schemaPath: string): Promise<void> {
    let schemaContent: string;

    if (schemaPath.startsWith("http://") || schemaPath.startsWith("https://")) {
      // Fetch schema from URL
      schemaContent = await this.fetchSchemaFromUrl(schemaPath);
    } else {
      // Read schema from local file
      schemaContent = fs.readFileSync(schemaPath, "utf-8");
    }

    return new Promise((resolve, reject) => {
      this.client.RegisterSchema(
        { schema_id: schemaId, schema_content: schemaContent },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else if (!response.success) {
            reject(new Error(response.message));
          } else {
            this.schemaId = schemaId;
            resolve();
          }
        },
      );
    });
  }

  /**
   * Binds the validator to a schema already registered on the server, without
   * requiring a local OpenAPI file. Resolves the schema's concrete version at
   * call time; subsequent validate() calls are pinned to that version.
   *
   * @param schemaId - The schema identifier to bind.
   * @param schemaVersion - Optional version to pin to (default: latest registered).
   * @returns Resolved schema metadata from the server.
   * @throws Error if the schema (or specific version) is not registered.
   *
   * @example
   * const validator = new ContractValidator("localhost:9550");
   * const info = await validator.useSchema("petstore");
   * console.log(`Bound to ${info.schemaId}@${info.schemaVersion}`);
   * await validator.validate(req, resp);
   */
  async useSchema(
    schemaId: string,
    schemaVersion?: string,
  ): Promise<SchemaInfo> {
    return new Promise((resolve, reject) => {
      this.client.GetSchema(
        {
          schema_id: schemaId,
          schema_version: schemaVersion || "",
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
            return;
          }
          if (!response.found) {
            const suffix = schemaVersion ? `@${schemaVersion}` : "";
            reject(
              new Error(
                `Schema '${schemaId}${suffix}' not registered on server`,
              ),
            );
            return;
          }

          const metadata = response.metadata || {};
          const resolvedVersion: string = metadata.schema_version || "";
          this.schemaId = schemaId;
          this.schemaVersion = resolvedVersion;

          const info: SchemaInfo = {
            schemaId: metadata.schema_id || schemaId,
            schemaVersion: resolvedVersion,
            schemaHash: metadata.schema_hash || "",
            registeredAt: Number(metadata.registered_at) || 0,
            updatedAt: Number(metadata.updated_at) || 0,
            openapiVersion: metadata.openapi_version || "",
            endpointCount: Number(metadata.endpoint_count) || 0,
          };
          if (metadata.ownership) {
            info.ownership = {
              owner: metadata.ownership.owner || "",
              team: metadata.ownership.team || "",
              contactEmail: metadata.ownership.contact_email || "",
              readOnly: Boolean(metadata.ownership.read_only),
            };
          }
          resolve(info);
        },
      );
    });
  }

  private async fetchSchemaFromUrl(url: string): Promise<string> {
    return new Promise((resolve, reject) => {
      const client = url.startsWith("https://") ? https : http;

      client
        .get(url, (res) => {
          let data = "";

          res.on("data", (chunk) => {
            data += chunk;
          });

          res.on("end", () => {
            if (
              res.statusCode &&
              res.statusCode >= 200 &&
              res.statusCode < 300
            ) {
              resolve(data);
            } else {
              reject(
                new Error(`Failed to fetch schema: HTTP ${res.statusCode}`),
              );
            }
          });
        })
        .on("error", (err) => {
          reject(new Error(`Failed to fetch schema: ${err.message}`));
        });
    });
  }

  /**
   * Validates an HTTP interaction (request and response) against the registered schema.
   * @param request - The HTTP request object containing method, path, headers, and body.
   * @param response - The HTTP response object containing status code, headers, and body.
   * @returns A promise that resolves with the validation result.
   * @throws Error if no schema has been registered.
   */
  async validate<ReqBody = any, ResBody = any>(
    request: ValidationRequest<ReqBody>,
    response: ValidationResponse<ResBody>,
  ): Promise<ValidationResult> {
    if (!this.schemaId) {
      throw new Error("Schema not registered. Call registerSchema first.");
    }

    const interactionRequest: any = {
      schema_id: this.schemaId,
      request: {
        method: request.method,
        path: request.path,
        headers: request.headers || {},
        body: request.body ? JSON.stringify(request.body) : "",
      },
      response: {
        status_code: response.statusCode || response.status_code,
        headers: response.headers || {},
        body: response.body ? JSON.stringify(response.body) : "",
      },
    };
    if (this.schemaVersion) {
      interactionRequest.schema_version = this.schemaVersion;
    }

    return new Promise((resolve, reject) => {
      this.client.ValidateInteraction(
        interactionRequest,
        this.createMetadata(),
        (err: any, result: any) => {
          if (err) {
            reject(err);
          } else {
            resolve(result);
          }
        },
      );
    });
  }

  /**
   * Registers a schema with version information for comparison.
   * @param schemaId - A unique identifier for the schema.
   * @param schemaPath - The path to the schema file (local file path or HTTP/HTTPS URL).
   * @param version - The semantic version of the schema (e.g., "1.0.0").
   * @returns A promise that resolves when the schema is successfully registered.
   */
  async registerSchemaWithVersion(
    schemaId: string,
    schemaPath: string,
    version: string,
  ): Promise<void> {
    let schemaContent: string;

    if (schemaPath.startsWith("http://") || schemaPath.startsWith("https://")) {
      schemaContent = await this.fetchSchemaFromUrl(schemaPath);
    } else {
      schemaContent = fs.readFileSync(schemaPath, "utf-8");
    }

    return new Promise((resolve, reject) => {
      this.client.RegisterSchema(
        {
          schema_id: schemaId,
          schema_content: schemaContent,
          schema_version: version,
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else if (!response.success) {
            reject(new Error(response.message));
          } else {
            this.schemaId = schemaId;
            resolve();
          }
        },
      );
    });
  }

  /**
   * Compares two schema versions to detect breaking changes.
   * @param schemaId - The schema identifier to compare versions for.
   * @param oldVersion - The old version to compare from (optional, defaults to previous version).
   * @param newVersion - The new version to compare to (optional, defaults to latest).
   * @returns A promise that resolves with the comparison result.
   *
   * @example
   * const result = await validator.compareSchemas("petstore-api", "1.0.0", "2.0.0");
   * if (!result.compatible) {
   *   console.log("Breaking changes detected:");
   *   result.breakingChanges.forEach(change => {
   *     console.log(`- ${change.type}: ${change.description}`);
   *   });
   * }
   */
  async compareSchemas(
    schemaId: string,
    oldVersion?: string,
    newVersion?: string,
  ): Promise<CompareResult> {
    return new Promise((resolve, reject) => {
      this.client.CompareSchemas(
        {
          schema_id: schemaId,
          old_version: oldVersion || "",
          new_version: newVersion || "",
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else {
            const breakingChanges: BreakingChange[] = (
              response.breaking_changes || []
            ).map((bc: any) => ({
              type: bc.type || "UNKNOWN",
              path: bc.path || "",
              method: bc.method || "",
              description: bc.description || "",
              oldValue: bc.old_value,
              newValue: bc.new_value,
            }));

            resolve({
              compatible: response.compatible,
              breakingChanges,
              oldSchema: response.old_schema
                ? {
                    schemaId: response.old_schema.schema_id,
                    schemaVersion: response.old_schema.schema_version,
                  }
                : undefined,
              newSchema: response.new_schema
                ? {
                    schemaId: response.new_schema.schema_id,
                    schemaVersion: response.new_schema.schema_version,
                  }
                : undefined,
            });
          }
        },
      );
    });
  }

  /**
   * Generates a complete test fixture (request and response) for an endpoint.
   * @param method - HTTP method (GET, POST, etc.)
   * @param path - API path (e.g., /users/{id})
   * @param options - Generation options
   * @returns A promise that resolves with the generated fixture.
   * @throws Error if no schema has been registered.
   *
   * @example
   * const fixture = await validator.generateFixture("POST", "/users", { useExamples: true });
   * console.log(fixture.request.body);  // Generated request body
   * console.log(fixture.response.body); // Generated response body
   */
  async generateFixture(
    method: string,
    path: string,
    options: GenerateOptions = {},
  ): Promise<GeneratedFixture> {
    if (!this.schemaId) {
      throw new Error("Schema not registered. Call registerSchema first.");
    }

    return new Promise((resolve, reject) => {
      this.client.GenerateFixture(
        {
          schema_id: this.schemaId,
          method: method.toUpperCase(),
          path,
          status_code: options.statusCode || 0,
          use_examples: options.useExamples !== false,
          content_type: options.contentType || "application/json",
          output_type: 0, // OUTPUT_FIXTURE
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else if (!response.success) {
            reject(new Error(response.message));
          } else {
            const fixture = response.fixture;
            resolve({
              request: {
                method: fixture.request.method,
                path: fixture.request.path,
                headers: fixture.request.headers || {},
                body: fixture.request.body
                  ? JSON.parse(fixture.request.body)
                  : undefined,
              },
              response: {
                statusCode: fixture.response.status_code,
                headers: fixture.response.headers || {},
                body: fixture.response.body
                  ? JSON.parse(fixture.response.body)
                  : undefined,
              },
            });
          }
        },
      );
    });
  }

  /**
   * Generates a response fixture for an endpoint.
   * @param method - HTTP method (GET, POST, etc.)
   * @param path - API path (e.g., /users/{id})
   * @param options - Generation options
   * @returns A promise that resolves with the generated response.
   * @throws Error if no schema has been registered.
   *
   * @example
   * const response = await validator.generateResponse("GET", "/users/123");
   * const expectedResponse = { statusCode: response.statusCode, body: JSON.stringify(response.body) };
   */
  async generateResponse(
    method: string,
    path: string,
    options: GenerateOptions = {},
  ): Promise<GeneratedResponse> {
    if (!this.schemaId) {
      throw new Error("Schema not registered. Call registerSchema first.");
    }

    return new Promise((resolve, reject) => {
      this.client.GenerateFixture(
        {
          schema_id: this.schemaId,
          method: method.toUpperCase(),
          path,
          status_code: options.statusCode || 0,
          use_examples: options.useExamples !== false,
          content_type: options.contentType || "application/json",
          output_type: 2, // OUTPUT_RESPONSE
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else if (!response.success) {
            reject(new Error(response.message));
          } else {
            const resp = response.response;
            resolve({
              statusCode: resp.status_code,
              headers: resp.headers || {},
              body: resp.body ? JSON.parse(resp.body) : undefined,
            });
          }
        },
      );
    });
  }

  /**
   * Generates a request body fixture for an endpoint.
   * @param method - HTTP method (typically POST, PUT, PATCH)
   * @param path - API path (e.g., /users)
   * @param options - Generation options
   * @returns A promise that resolves with the generated request body.
   * @throws Error if no schema has been registered.
   *
   * @example
   * const body = await validator.generateRequestBody("POST", "/users");
   * const request = { method: "POST", path: "/users", body };
   */
  async generateRequestBody(
    method: string,
    path: string,
    options: GenerateOptions = {},
  ): Promise<any> {
    if (!this.schemaId) {
      throw new Error("Schema not registered. Call registerSchema first.");
    }

    return new Promise((resolve, reject) => {
      this.client.GenerateFixture(
        {
          schema_id: this.schemaId,
          method: method.toUpperCase(),
          path,
          use_examples: options.useExamples !== false,
          content_type: options.contentType || "application/json",
          output_type: 1, // OUTPUT_REQUEST
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else if (!response.success) {
            reject(new Error(response.message));
          } else {
            resolve(
              response.request_body
                ? JSON.parse(response.request_body)
                : undefined,
            );
          }
        },
      );
    });
  }

  /**
   * Lists all endpoints available in the registered schema.
   * @returns A promise that resolves with the list of endpoints.
   * @throws Error if no schema has been registered.
   *
   * @example
   * const endpoints = await validator.listEndpoints();
   * endpoints.forEach(ep => console.log(`${ep.method} ${ep.path}`));
   */
  async listEndpoints(): Promise<EndpointInfo[]> {
    if (!this.schemaId) {
      throw new Error("Schema not registered. Call registerSchema first.");
    }

    return new Promise((resolve, reject) => {
      this.client.ListEndpoints(
        { schema_id: this.schemaId },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else {
            const endpoints: EndpointInfo[] = (response.endpoints || []).map(
              (ep: any) => ({
                method: ep.method,
                path: ep.path,
                operationId: ep.operation_id || undefined,
                summary: ep.summary || undefined,
              }),
            );
            resolve(endpoints);
          }
        },
      );
    });
  }

  /**
   * Closes the gRPC client connection.
   * Should be called when the validator is no longer needed to free resources.
   */
  close(): void {
    if (this.client) {
      this.client.close();
    }
  }

  // ==========================================================================
  // Consumer Registry Methods
  // ==========================================================================

  /**
   * Registers a consumer's dependency on a schema.
   * This tracks which consumers use which schemas for deployment safety analysis.
   *
   * @param options - Consumer registration options
   * @returns A promise that resolves with the registered consumer info
   *
   * @example
   * const consumer = await validator.registerConsumer({
   *   consumerId: "order-service",
   *   consumerVersion: "2.1.0",
   *   schemaId: "user-api",
   *   schemaVersion: "1.0.0",
   *   environment: "prod",
   *   usedEndpoints: [
   *     { method: "GET", path: "/users/{id}", usedFields: ["email", "name"] }
   *   ]
   * });
   */
  async registerConsumer(
    options: RegisterConsumerOptions,
  ): Promise<ConsumerInfo> {
    const usedEndpoints = (options.usedEndpoints || []).map((ep) => ({
      method: ep.method,
      path: ep.path,
      used_fields: ep.usedFields || [],
    }));

    return new Promise((resolve, reject) => {
      this.client.RegisterConsumer(
        {
          consumer_id: options.consumerId,
          consumer_version: options.consumerVersion,
          schema_id: options.schemaId,
          schema_version: options.schemaVersion,
          environment: options.environment,
          used_endpoints: usedEndpoints,
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else if (!response.success) {
            reject(
              new Error(response.message || "Failed to register consumer"),
            );
          } else {
            resolve(this.mapConsumerInfo(response.consumer));
          }
        },
      );
    });
  }

  /**
   * Lists all consumers that depend on a schema.
   *
   * @param schemaId - The schema identifier
   * @param environment - Optional environment filter (dev, staging, prod)
   * @returns A promise that resolves with the list of consumers
   *
   * @example
   * const consumers = await validator.listConsumers("user-api", "prod");
   * consumers.forEach(c => console.log(`${c.consumerId} v${c.consumerVersion}`));
   */
  async listConsumers(
    schemaId: string,
    environment?: string,
  ): Promise<ConsumerInfo[]> {
    return new Promise((resolve, reject) => {
      this.client.ListConsumers(
        {
          schema_id: schemaId,
          environment: environment || "",
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else {
            const consumers = (response.consumers || []).map((c: any) =>
              this.mapConsumerInfo(c),
            );
            resolve(consumers);
          }
        },
      );
    });
  }

  /**
   * Removes a consumer registration for a specific schema.
   *
   * @param consumerId - The consumer identifier
   * @param schemaId - The schema identifier
   * @param environment - The environment (dev, staging, prod)
   * @returns A promise that resolves when the consumer is deregistered
   *
   * @example
   * await validator.deregisterConsumer("order-service", "user-api", "prod");
   */
  async deregisterConsumer(
    consumerId: string,
    schemaId: string,
    environment: string,
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      this.client.DeregisterConsumer(
        {
          consumer_id: consumerId,
          schema_id: schemaId,
          environment: environment,
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else if (!response.success) {
            reject(
              new Error(response.message || "Failed to deregister consumer"),
            );
          } else {
            resolve();
          }
        },
      );
    });
  }

  /**
   * Checks if a schema version can be safely deployed without breaking consumers.
   *
   * @param schemaId - The schema identifier
   * @param newVersion - The new version to deploy
   * @param environment - Target environment (dev, staging, prod)
   * @returns A promise that resolves with the deployment safety analysis
   *
   * @example
   * const result = await validator.canIDeploy("user-api", "2.0.0", "prod");
   * if (!result.safeToDeploy) {
   *   console.log("Unsafe to deploy:");
   *   result.affectedConsumers.forEach(c => {
   *     console.log(`- ${c.consumerId} will break`);
   *   });
   * }
   */
  async canIDeploy(
    schemaId: string,
    newVersion: string,
    environment: string,
  ): Promise<CanIDeployResult> {
    return new Promise((resolve, reject) => {
      this.client.CanIDeploy(
        {
          schema_id: schemaId,
          new_version: newVersion,
          environment: environment,
        },
        this.createMetadata(),
        (err: any, response: any) => {
          if (err) {
            reject(err);
          } else {
            const breakingChanges: BreakingChange[] = (
              response.breaking_changes || []
            ).map((bc: any) => ({
              type: bc.type || "UNKNOWN",
              path: bc.path || "",
              method: bc.method || "",
              description: bc.description || "",
              oldValue: bc.old_value,
              newValue: bc.new_value,
            }));

            const affectedConsumers: ConsumerImpact[] = (
              response.affected_consumers || []
            ).map((c: any) => ({
              consumerId: c.consumer_id || "",
              consumerVersion: c.consumer_version || "",
              currentSchemaVersion: c.current_schema_version || "",
              environment: c.environment || "",
              willBreak: c.will_break || false,
              relevantChanges: (c.relevant_changes || []).map((bc: any) => ({
                type: bc.type || "UNKNOWN",
                path: bc.path || "",
                method: bc.method || "",
                description: bc.description || "",
                oldValue: bc.old_value,
                newValue: bc.new_value,
              })),
            }));

            resolve({
              safeToDeploy: response.safe_to_deploy || false,
              summary: response.summary || "",
              breakingChanges,
              affectedConsumers,
            });
          }
        },
      );
    });
  }

  /**
   * Maps raw proto consumer info to TypeScript interface.
   */
  private mapConsumerInfo(c: any): ConsumerInfo {
    return {
      consumerId: c.consumer_id || "",
      consumerVersion: c.consumer_version || "",
      schemaId: c.schema_id || "",
      schemaVersion: c.schema_version || "",
      environment: c.environment || "",
      registeredAt: Number(c.registered_at) || 0,
      lastValidatedAt: Number(c.last_validated_at) || 0,
      usedEndpoints: (c.used_endpoints || []).map((ep: any) => ({
        method: ep.method || "",
        path: ep.path || "",
        usedFields: ep.used_fields || [],
      })),
    };
  }

  // ==========================================================================
  // Auto-Registration Methods
  // ==========================================================================

  /**
   * Builds consumer registration options from captured interactions.
   * Useful for preview/dry-run scenarios.
   *
   * @param interactions - Array of captured interactions from mock adapter
   * @param config - Auto-registration configuration
   * @returns Registration options that can be passed to registerConsumer
   *
   * @example
   * const mock = createMockAdapter({ validator });
   * // ... run tests ...
   *
   * const opts = validator.buildConsumerFromInteractions(
   *   mock.getInteractions(),
   *   {
   *     consumerId: "order-service",
   *     consumerVersion: "2.1.0",
   *     environment: "dev",
   *     schemaVersion: "1.0.0",
   *   }
   * );
   * console.log(`Would register ${opts.usedEndpoints?.length} endpoints`);
   */
  buildConsumerFromInteractions(
    interactions: import("./adapters/types").CapturedInteraction[],
    config: import("./auto-register").AutoRegisterConfig,
  ): RegisterConsumerOptions {
    const result = buildConsumer(interactions, config);
    if (result.error || !result.options) {
      throw new Error(result.error ?? "Failed to build consumer options");
    }
    return result.options;
  }

  /**
   * Registers a consumer based on captured mock interactions.
   * This combines buildConsumerFromInteractions + registerConsumer.
   *
   * @param interactions - Array of captured interactions from mock adapter
   * @param config - Auto-registration configuration
   * @returns Promise resolving to registered consumer info
   *
   * @example
   * const mock = createMockAdapter({ validator, cache: true });
   * const mockFetch = mock.fetch.bind(mock);
   *
   * // Run tests using mockFetch
   * await mockFetch("http://mock.user-api/users/123");
   * await mockFetch("http://mock.user-api/users", { method: "POST", body: "{}" });
   *
   * // Auto-register consumer from captured interactions
   * const info = await validator.registerConsumerFromInteractions(
   *   mock.getInteractions(),
   *   {
   *     consumerId: "order-service",
   *     consumerVersion: "2.1.0",
   *     environment: "dev",
   *     schemaVersion: "1.0.0",
   *     // schemaId auto-extracted from URL: "user-api"
   *   }
   * );
   * console.log(`Registered ${info.consumerId} with ${info.usedEndpoints.length} endpoints`);
   */
  async registerConsumerFromInteractions(
    interactions: import("./adapters/types").CapturedInteraction[],
    config: import("./auto-register").AutoRegisterConfig,
  ): Promise<ConsumerInfo> {
    const opts = this.buildConsumerFromInteractions(interactions, config);
    return this.registerConsumer(opts);
  }
}

// Re-export auto-register types and utilities
export type { AutoRegisterConfig } from "./auto-register";
export {
  extractSchemaIdFromUrl,
  extractSchemaIdFromInteractions,
  extractFieldsFromBody,
  normalizePathForEndpoint,
  mergeInteractionsToEndpoints,
  buildConsumerFromInteractions,
} from "./auto-register";
