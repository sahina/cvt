import * as path from "path";
import {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
} from "../src/index";
import * as http from "http";
import * as https from "https";
import { EventEmitter } from "events";

// Mock gRPC client
const mockRegisterSchema = jest.fn();
const mockValidateInteraction = jest.fn();
const mockClose = jest.fn();
const mockCompareSchemas = jest.fn();
const mockGenerateFixture = jest.fn();
const mockListEndpoints = jest.fn();
const mockRegisterConsumer = jest.fn();
const mockListConsumers = jest.fn();
const mockDeregisterConsumer = jest.fn();
const mockCanIDeploy = jest.fn();
const mockGetSchema = jest.fn();

jest.mock("@grpc/grpc-js", () => {
  // Define MockMetadata inside the factory to avoid hoisting issues
  class MockMetadata {
    private data: Map<string, string> = new Map();
    set(key: string, value: string) {
      this.data.set(key, value);
    }
    get(key: string) {
      return this.data.get(key);
    }
  }

  return {
    loadPackageDefinition: jest.fn().mockReturnValue({
      cvt: {
        ContractValidator: jest.fn(),
      },
    }),
    credentials: {
      createInsecure: jest.fn(),
      createSsl: jest.fn(),
    },
    Metadata: MockMetadata,
  };
});

jest.mock("@grpc/proto-loader", () => {
  return {
    loadSync: jest.fn(),
  };
});

// Mock http and https
jest.mock("http");
jest.mock("https");

describe("ContractValidator", () => {
  let validator: ContractValidator;

  beforeEach(() => {
    // Reset mocks
    mockRegisterSchema.mockReset();
    mockValidateInteraction.mockReset();
    mockClose.mockReset();
    mockCompareSchemas.mockReset();
    mockGenerateFixture.mockReset();
    mockListEndpoints.mockReset();
    mockRegisterConsumer.mockReset();
    mockListConsumers.mockReset();
    mockDeregisterConsumer.mockReset();
    mockCanIDeploy.mockReset();
    mockGetSchema.mockReset();
    jest.clearAllMocks();

    // Create validator
    validator = new ContractValidator();

    // Inject mock client into the private 'client' property
    (validator as any).client = {
      RegisterSchema: mockRegisterSchema,
      ValidateInteraction: mockValidateInteraction,
      close: mockClose,
      CompareSchemas: mockCompareSchemas,
      GenerateFixture: mockGenerateFixture,
      ListEndpoints: mockListEndpoints,
      RegisterConsumer: mockRegisterConsumer,
      ListConsumers: mockListConsumers,
      DeregisterConsumer: mockDeregisterConsumer,
      CanIDeploy: mockCanIDeploy,
      GetSchema: mockGetSchema,
    };
  });

  describe("schema registration", () => {
    it("should register schema from local file successfully", async () => {
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true, message: "OK" });
        },
      );

      await expect(
        validator.registerSchema("test-schema", schemaPath),
      ).resolves.toBeUndefined();

      expect(mockRegisterSchema).toHaveBeenCalledTimes(1);
      expect(mockRegisterSchema.mock.calls[0][0]).toMatchObject({
        schema_id: "test-schema",
      });
    });

    it("should register schema from URL successfully", async () => {
      const mockSchemaUrl = "http://example.com/openapi.json";
      const mockSchemaContent = '{"openapi": "3.0.0"}';

      // Mock http.get
      (http.get as jest.Mock).mockImplementation((url, callback) => {
        const response = new EventEmitter();
        (response as any).statusCode = 200;
        callback(response);
        response.emit("data", mockSchemaContent);
        response.emit("end");
        return { on: jest.fn() };
      });

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true, message: "OK" });
        },
      );

      await expect(
        validator.registerSchema("url-test-schema", mockSchemaUrl),
      ).resolves.toBeUndefined();

      expect(mockRegisterSchema).toHaveBeenCalledWith(
        expect.objectContaining({
          schema_id: "url-test-schema",
          schema_content: mockSchemaContent,
        }),
        expect.any(Object), // metadata
        expect.any(Function),
      );
    });

    it("should register schema from HTTPS URL successfully", async () => {
      const mockSchemaUrl = "https://example.com/openapi.json";
      const mockSchemaContent = '{"openapi": "3.0.0"}';

      // Mock https.get
      (https.get as jest.Mock).mockImplementation((url, callback) => {
        const response = new EventEmitter();
        (response as any).statusCode = 200;
        callback(response);
        response.emit("data", mockSchemaContent);
        response.emit("end");
        return { on: jest.fn() };
      });

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true, message: "OK" });
        },
      );

      await expect(
        validator.registerSchema("url-https-schema", mockSchemaUrl),
      ).resolves.toBeUndefined();
    });

    it("should handle URL fetch error (non-200 status)", async () => {
      const mockSchemaUrl = "http://example.com/openapi.json";

      (http.get as jest.Mock).mockImplementation((url, callback) => {
        const response = new EventEmitter();
        (response as any).statusCode = 404;
        callback(response);
        response.emit("end");
        return { on: jest.fn() };
      });

      await expect(
        validator.registerSchema("url-error-schema", mockSchemaUrl),
      ).rejects.toThrow("Failed to fetch schema: HTTP 404");
    });

    it("should handle URL fetch network error", async () => {
      const mockSchemaUrl = "http://example.com/openapi.json";

      (http.get as jest.Mock).mockImplementation((_url, _callback) => {
        const req = new EventEmitter();
        setTimeout(() => {
          req.emit("error", new Error("Network error"));
        }, 10);
        return req;
      });

      await expect(
        validator.registerSchema("url-net-error-schema", mockSchemaUrl),
      ).rejects.toThrow("Failed to fetch schema: Network error");
    });

    it("should handle registration failure", async () => {
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: false, message: "Invalid schema" });
        },
      );

      await expect(
        validator.registerSchema("test-schema", schemaPath),
      ).rejects.toThrow("Invalid schema");
    });

    it("should handle gRPC error during registration", async () => {
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("Connection failed"), null);
        },
      );

      await expect(
        validator.registerSchema("test-schema", schemaPath),
      ).rejects.toThrow("Connection failed");
    });
  });

  describe("validation", () => {
    beforeEach(async () => {
      // Mock successful registration
      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);
    });

    it("should validate valid pet creation", async () => {
      mockValidateInteraction.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { valid: true, errors: [] });
        },
      );

      const request: ValidationRequest = {
        method: "POST",
        path: "/pet",
        headers: { "content-type": "application/json" },
        body: { name: "Fluffy" },
      };

      const response: ValidationResponse = { statusCode: 201 };

      const result = await validator.validate(request, response);

      expect(result.valid).toBe(true);
      expect(mockValidateInteraction).toHaveBeenCalledTimes(1);
      const callArg = mockValidateInteraction.mock.calls[0][0];
      expect(callArg.schema_id).toBe("test-schema");
      expect(callArg.request.method).toBe("POST");
    });

    it("should reject invalid interactions", async () => {
      mockValidateInteraction.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { valid: false, errors: ["Missing field"] });
        },
      );

      const request: ValidationRequest = { method: "POST", path: "/pet" };
      const response: ValidationResponse = { statusCode: 400 };

      const result = await validator.validate(request, response);

      expect(result.valid).toBe(false);
      expect(result.errors).toContain("Missing field");
    });

    it("should throw error if validation fails", async () => {
      mockValidateInteraction.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("RPC failed"), null);
        },
      );

      await expect(
        validator.validate({ method: "GET", path: "/" }, { statusCode: 200 }),
      ).rejects.toThrow("RPC failed");
    });

    it("should throw error if schema not registered", async () => {
      // Create new validator without registration
      const newValidator = new ContractValidator();
      // Inject mocks again for the new instance as beforeEach handles 'validator'
      (newValidator as any).client = {
        RegisterSchema: mockRegisterSchema,
        ValidateInteraction: mockValidateInteraction,
        close: mockClose,
      };

      await expect(
        newValidator.validate(
          { method: "GET", path: "/" },
          { statusCode: 200 },
        ),
      ).rejects.toThrow("Schema not bound");
    });
  });

  describe("close", () => {
    it("should close client", () => {
      validator.close();
      expect(mockClose).toHaveBeenCalled();
    });
  });

  describe("compareSchemas", () => {
    it("should compare schemas successfully", async () => {
      mockCompareSchemas.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            compatible: true,
            breaking_changes: [],
          });
        },
      );

      const result = await validator.compareSchemas(
        "test-schema",
        "1.0.0",
        "2.0.0",
      );

      expect(result.compatible).toBe(true);
      expect(result.breakingChanges).toEqual([]);
      expect(mockCompareSchemas).toHaveBeenCalledWith(
        expect.objectContaining({
          schema_id: "test-schema",
          old_version: "1.0.0",
          new_version: "2.0.0",
        }),
        expect.any(Object),
        expect.any(Function),
      );
    });

    it("should return breaking changes when schemas are incompatible", async () => {
      mockCompareSchemas.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            compatible: false,
            breaking_changes: [
              {
                type: "ENDPOINT_REMOVED",
                path: "/users/{id}",
                method: "DELETE",
                description: "Endpoint was removed",
                old_value: "existed",
                new_value: "",
              },
            ],
          });
        },
      );

      const result = await validator.compareSchemas("test-schema");

      expect(result.compatible).toBe(false);
      expect(result.breakingChanges).toHaveLength(1);
      expect(result.breakingChanges[0].type).toBe("ENDPOINT_REMOVED");
      expect(result.breakingChanges[0].path).toBe("/users/{id}");
    });

    it("should handle gRPC error during comparison", async () => {
      mockCompareSchemas.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("Comparison failed"), null);
        },
      );

      await expect(
        validator.compareSchemas("test-schema", "1.0.0", "2.0.0"),
      ).rejects.toThrow("Comparison failed");
    });
  });

  describe("generateFixture", () => {
    beforeEach(async () => {
      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);
    });

    it("should generate fixture successfully", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: true,
            fixture: {
              request: {
                method: "POST",
                path: "/users",
                body: '{"name":"test"}',
              },
              response: {
                status_code: 201,
                body: '{"id":1,"name":"test"}',
              },
            },
          });
        },
      );

      const result = await validator.generateFixture("POST", "/users");

      expect(result).toBeDefined();
      expect(result?.request?.method).toBe("POST");
      expect(result?.response?.statusCode).toBe(201);
    });

    it("should handle gRPC error during fixture generation", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("Generation failed"), null);
        },
      );

      await expect(validator.generateFixture("POST", "/users")).rejects.toThrow(
        "Generation failed",
      );
    });
  });

  describe("listEndpoints", () => {
    beforeEach(async () => {
      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);
    });

    it("should list endpoints successfully", async () => {
      mockListEndpoints.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            endpoints: [
              { method: "GET", path: "/users", operation_id: "getUsers" },
              { method: "POST", path: "/users", operation_id: "createUser" },
            ],
          });
        },
      );

      const endpoints = await validator.listEndpoints();

      expect(endpoints).toHaveLength(2);
      expect(endpoints[0].method).toBe("GET");
      expect(endpoints[0].path).toBe("/users");
      expect(endpoints[0].operationId).toBe("getUsers");
    });

    it("should handle gRPC error when listing endpoints", async () => {
      mockListEndpoints.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("List failed"), null);
        },
      );

      await expect(validator.listEndpoints()).rejects.toThrow("List failed");
    });
  });

  describe("registerConsumer", () => {
    it("should register consumer successfully", async () => {
      mockRegisterConsumer.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: true,
            consumer: {
              consumer_id: "order-service",
              consumer_version: "1.0.0",
              schema_id: "user-api",
              schema_version: "1.0.0",
              environment: "prod",
            },
          });
        },
      );

      const result = await validator.registerConsumer({
        consumerId: "order-service",
        consumerVersion: "1.0.0",
        schemaId: "user-api",
        schemaVersion: "1.0.0",
        environment: "prod",
        usedEndpoints: [
          { method: "GET", path: "/users/{id}", usedFields: ["email"] },
        ],
      });

      expect(result.consumerId).toBe("order-service");
      expect(result.environment).toBe("prod");
    });

    it("should handle registration failure", async () => {
      mockRegisterConsumer.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: false, message: "Registration failed" });
        },
      );

      await expect(
        validator.registerConsumer({
          consumerId: "order-service",
          consumerVersion: "1.0.0",
          schemaId: "user-api",
          schemaVersion: "1.0.0",
          environment: "prod",
        }),
      ).rejects.toThrow("Registration failed");
    });
  });

  describe("listConsumers", () => {
    it("should list consumers successfully", async () => {
      mockListConsumers.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            consumers: [
              {
                consumer_id: "order-service",
                consumer_version: "1.0.0",
                environment: "prod",
              },
              {
                consumer_id: "billing-service",
                consumer_version: "2.0.0",
                environment: "prod",
              },
            ],
          });
        },
      );

      const consumers = await validator.listConsumers("user-api", "prod");

      expect(consumers).toHaveLength(2);
      expect(consumers[0].consumerId).toBe("order-service");
      expect(consumers[1].consumerId).toBe("billing-service");
    });

    it("should handle gRPC error when listing consumers", async () => {
      mockListConsumers.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("List failed"), null);
        },
      );

      await expect(validator.listConsumers("user-api", "prod")).rejects.toThrow(
        "List failed",
      );
    });
  });

  describe("deregisterConsumer", () => {
    it("should deregister consumer successfully", async () => {
      mockDeregisterConsumer.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );

      await expect(
        validator.deregisterConsumer("order-service", "user-api", "prod"),
      ).resolves.toBeUndefined();

      expect(mockDeregisterConsumer).toHaveBeenCalledWith(
        expect.objectContaining({
          consumer_id: "order-service",
          schema_id: "user-api",
          environment: "prod",
        }),
        expect.any(Object),
        expect.any(Function),
      );
    });

    it("should handle deregistration failure", async () => {
      mockDeregisterConsumer.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: false, message: "Consumer not found" });
        },
      );

      await expect(
        validator.deregisterConsumer("unknown", "user-api", "prod"),
      ).rejects.toThrow("Consumer not found");
    });
  });

  describe("canIDeploy", () => {
    it("should return safe to deploy", async () => {
      mockCanIDeploy.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            safe_to_deploy: true,
            summary: "No breaking changes",
            breaking_changes: [],
            affected_consumers: [],
          });
        },
      );

      const result = await validator.canIDeploy("user-api", "2.0.0", "prod");

      expect(result.safeToDeploy).toBe(true);
      expect(result.summary).toBe("No breaking changes");
    });

    it("should return affected consumers when unsafe", async () => {
      mockCanIDeploy.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            safe_to_deploy: false,
            summary: "Breaking changes affect consumers",
            breaking_changes: [
              { type: "ENDPOINT_REMOVED", path: "/users", method: "DELETE" },
            ],
            affected_consumers: [
              {
                consumer_id: "order-service",
                consumer_version: "1.0.0",
                will_break: true,
              },
            ],
          });
        },
      );

      const result = await validator.canIDeploy("user-api", "2.0.0", "prod");

      expect(result.safeToDeploy).toBe(false);
      expect(result.affectedConsumers).toHaveLength(1);
      expect(result.affectedConsumers[0].consumerId).toBe("order-service");
    });

    it("should handle gRPC error", async () => {
      mockCanIDeploy.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("Check failed"), null);
        },
      );

      await expect(
        validator.canIDeploy("user-api", "2.0.0", "prod"),
      ).rejects.toThrow("Check failed");
    });
  });

  describe("constructor options", () => {
    it("should create validator with API key option", () => {
      const validatorWithApiKey = new ContractValidator({
        address: "localhost:50051",
        apiKey: "test-api-key",
      });
      expect(validatorWithApiKey).toBeDefined();
    });

    it("should create validator with TLS enabled", () => {
      const grpc = jest.requireMock("@grpc/grpc-js");
      const validatorWithTls = new ContractValidator({
        address: "localhost:50051",
        tls: {
          enabled: true,
        },
      });
      expect(validatorWithTls).toBeDefined();
      expect(grpc.credentials.createSsl).toHaveBeenCalled();
    });

    it("should create validator with default address when not provided", () => {
      const validatorDefault = new ContractValidator({});
      expect(validatorDefault).toBeDefined();
    });
  });

  describe("registerSchemaWithVersion", () => {
    it("should register schema with version from local file", async () => {
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          expect(req.schema_version).toBe("2.0.0");
          callback(null, { success: true, message: "OK" });
        },
      );

      await expect(
        validator.registerSchemaWithVersion("test-schema", schemaPath, "2.0.0"),
      ).resolves.toBeUndefined();
    });

    it("should register schema with version from URL", async () => {
      const mockSchemaUrl = "http://example.com/openapi.json";
      const mockSchemaContent = '{"openapi": "3.0.0"}';

      (http.get as jest.Mock).mockImplementation((url, callback) => {
        const response = new EventEmitter();
        (response as any).statusCode = 200;
        callback(response);
        response.emit("data", mockSchemaContent);
        response.emit("end");
        return { on: jest.fn() };
      });

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          expect(req.schema_version).toBe("1.5.0");
          callback(null, { success: true, message: "OK" });
        },
      );

      await expect(
        validator.registerSchemaWithVersion(
          "url-schema",
          mockSchemaUrl,
          "1.5.0",
        ),
      ).resolves.toBeUndefined();
    });

    it("should handle registration failure with version", async () => {
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");

      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: false, message: "Version conflict" });
        },
      );

      await expect(
        validator.registerSchemaWithVersion("test-schema", schemaPath, "1.0.0"),
      ).rejects.toThrow("Version conflict");
    });
  });

  describe("generateFixture edge cases", () => {
    it("should throw error when schema not registered", async () => {
      const newValidator = new ContractValidator();
      (newValidator as any).client = {
        GenerateFixture: mockGenerateFixture,
      };

      await expect(
        newValidator.generateFixture("GET", "/users"),
      ).rejects.toThrow("Schema not bound");
    });

    it("should handle generateFixture failure response", async () => {
      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);

      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: false, message: "No endpoint found" });
        },
      );

      await expect(
        validator.generateFixture("GET", "/nonexistent"),
      ).rejects.toThrow("No endpoint found");
    });

    it("should handle fixture with empty bodies", async () => {
      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);

      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: true,
            fixture: {
              request: {
                method: "DELETE",
                path: "/users/123",
                headers: {},
                body: "",
              },
              response: {
                status_code: 204,
                headers: {},
                body: "",
              },
            },
          });
        },
      );

      const result = await validator.generateFixture("DELETE", "/users/123");

      expect(result.request.body).toBeUndefined();
      expect(result.response.body).toBeUndefined();
    });
  });

  describe("API key in metadata", () => {
    it("should include API key in gRPC metadata", async () => {
      const validatorWithApiKey = new ContractValidator({
        address: "localhost:50051",
        apiKey: "secret-key-123",
      });

      let capturedMetadata: any;
      const mockClient = {
        RegisterSchema: (req: any, metadata: any, callback: any) => {
          capturedMetadata = metadata;
          callback(null, { success: true });
        },
      };
      (validatorWithApiKey as any).client = mockClient;

      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validatorWithApiKey.registerSchema("test", schemaPath);

      expect(capturedMetadata.get("x-api-key")).toBe("secret-key-123");
    });
  });

  describe("compareSchemas with schema metadata", () => {
    it("should return old_schema and new_schema when present", async () => {
      mockCompareSchemas.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            compatible: true,
            breaking_changes: [],
            old_schema: {
              schema_id: "my-api",
              schema_version: "1.0.0",
            },
            new_schema: {
              schema_id: "my-api",
              schema_version: "2.0.0",
            },
          });
        },
      );

      const result = await validator.compareSchemas("my-api", "1.0.0", "2.0.0");

      expect(result.compatible).toBe(true);
      expect(result.oldSchema).toEqual({
        schemaId: "my-api",
        schemaVersion: "1.0.0",
      });
      expect(result.newSchema).toEqual({
        schemaId: "my-api",
        schemaVersion: "2.0.0",
      });
    });

    it("should return undefined for missing old_schema and new_schema", async () => {
      mockCompareSchemas.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            compatible: true,
            breaking_changes: [],
          });
        },
      );

      const result = await validator.compareSchemas("my-api");

      expect(result.oldSchema).toBeUndefined();
      expect(result.newSchema).toBeUndefined();
    });
  });

  describe("generateResponse", () => {
    beforeEach(async () => {
      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);
    });

    it("should generate a response fixture", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: true,
            response: {
              status_code: 200,
              headers: { "content-type": "application/json" },
              body: '{"id": 1, "name": "test"}',
            },
          });
        },
      );

      const result = await validator.generateResponse("GET", "/users/1");

      expect(result.statusCode).toBe(200);
      expect(result.headers).toEqual({ "content-type": "application/json" });
      expect(result.body).toEqual({ id: 1, name: "test" });
    });

    it("should handle response with no body", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: true,
            response: {
              status_code: 204,
              headers: {},
              body: null,
            },
          });
        },
      );

      const result = await validator.generateResponse("DELETE", "/users/1");

      expect(result.statusCode).toBe(204);
      expect(result.body).toBeUndefined();
    });

    it("should throw when schema not registered", async () => {
      const newValidator = new ContractValidator();
      await expect(
        newValidator.generateResponse("GET", "/users"),
      ).rejects.toThrow("Schema not bound");
    });

    it("should reject on failure response", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: false,
            message: "Generation failed",
          });
        },
      );

      await expect(validator.generateResponse("GET", "/users")).rejects.toThrow(
        "Generation failed",
      );
    });
  });

  describe("generateRequestBody", () => {
    beforeEach(async () => {
      mockRegisterSchema.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { success: true });
        },
      );
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);
    });

    it("should generate a request body fixture", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: true,
            request_body: '{"name": "test", "email": "test@example.com"}',
          });
        },
      );

      const result = await validator.generateRequestBody("POST", "/users");

      expect(result).toEqual({ name: "test", email: "test@example.com" });
    });

    it("should return undefined when no request body", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: true,
            request_body: null,
          });
        },
      );

      const result = await validator.generateRequestBody("GET", "/users");

      expect(result).toBeUndefined();
    });

    it("should throw when schema not registered", async () => {
      const newValidator = new ContractValidator();
      await expect(
        newValidator.generateRequestBody("POST", "/users"),
      ).rejects.toThrow("Schema not bound");
    });

    it("should reject on failure response", async () => {
      mockGenerateFixture.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            success: false,
            message: "Generation failed",
          });
        },
      );

      await expect(
        validator.generateRequestBody("POST", "/users"),
      ).rejects.toThrow("Generation failed");
    });
  });

  describe("useSchema", () => {
    const metadataFixture = {
      schema_id: "petstore",
      schema_version: "1.2.0",
      schema_hash: "abc123",
      registered_at: 1714000000,
      updated_at: 1714000500,
      openapi_version: "3.0.0",
      endpoint_count: 7,
      ownership: {
        owner: "jane",
        team: "platform",
        contact_email: "platform@example.com",
        read_only: false,
      },
    };

    it("resolves latest version and returns SchemaInfo", async () => {
      mockGetSchema.mockImplementation((req: any, _md: any, cb: any) => {
        expect(req.schema_id).toBe("petstore");
        expect(req.schema_version).toBe("");
        cb(null, { found: true, metadata: metadataFixture });
      });

      const info = await validator.useSchema("petstore");

      expect(info.schemaId).toBe("petstore");
      expect(info.schemaVersion).toBe("1.2.0");
      expect(info.openapiVersion).toBe("3.0.0");
      expect(info.endpointCount).toBe(7);
      expect(info.ownership).toEqual({
        owner: "jane",
        team: "platform",
        contactEmail: "platform@example.com",
        readOnly: false,
      });
    });

    it("pins to an explicit version", async () => {
      mockGetSchema.mockImplementation((req: any, _md: any, cb: any) => {
        expect(req.schema_version).toBe("1.0.0");
        cb(null, {
          found: true,
          metadata: { ...metadataFixture, schema_version: "1.0.0" },
        });
      });

      const info = await validator.useSchema("petstore", "1.0.0");
      expect(info.schemaVersion).toBe("1.0.0");
    });

    it("throws a clear error when schema is not registered", async () => {
      mockGetSchema.mockImplementation((_req: any, _md: any, cb: any) => {
        cb(null, { found: false });
      });

      await expect(validator.useSchema("nope")).rejects.toThrow(
        /'nope'.*not registered/,
      );
    });

    it("includes the version in the error when pinning", async () => {
      mockGetSchema.mockImplementation((_req: any, _md: any, cb: any) => {
        cb(null, { found: false });
      });

      await expect(validator.useSchema("petstore", "9.9.9")).rejects.toThrow(
        /'petstore@9\.9\.9'/,
      );
    });

    it("propagates gRPC errors from GetSchema", async () => {
      mockGetSchema.mockImplementation((_req: any, _md: any, cb: any) => {
        cb(new Error("Connection refused"), null);
      });

      await expect(validator.useSchema("petstore")).rejects.toThrow(
        "Connection refused",
      );
    });

    it("causes validate() to send schema_version after useSchema", async () => {
      mockGetSchema.mockImplementation((_req: any, _md: any, cb: any) => {
        cb(null, { found: true, metadata: metadataFixture });
      });
      mockValidateInteraction.mockImplementation(
        (_req: any, _md: any, cb: any) => {
          cb(null, { valid: true, errors: [] });
        },
      );

      await validator.useSchema("petstore");
      await validator.validate(
        { method: "GET", path: "/pet/1" },
        { statusCode: 200 },
      );

      const sent = mockValidateInteraction.mock.calls[0][0];
      expect(sent.schema_id).toBe("petstore");
      expect(sent.schema_version).toBe("1.2.0");
    });

    it("clears version pin when registerSchema rebinds after useSchema", async () => {
      mockGetSchema.mockImplementation((_req: any, _md: any, cb: any) => {
        cb(null, { found: true, metadata: metadataFixture });
      });
      mockRegisterSchema.mockImplementation((_req: any, _md: any, cb: any) => {
        cb(null, { success: true });
      });
      mockValidateInteraction.mockImplementation(
        (_req: any, _md: any, cb: any) => {
          cb(null, { valid: true, errors: [] });
        },
      );

      await validator.useSchema("petstore");
      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("other-schema", schemaPath);
      await validator.validate(
        { method: "GET", path: "/pet/1" },
        { statusCode: 200 },
      );

      const sent = mockValidateInteraction.mock.calls[0][0];
      expect(sent.schema_id).toBe("other-schema");
      expect(sent.schema_version).toBeUndefined();
    });

    it("does NOT send schema_version when only registerSchema was called", async () => {
      mockRegisterSchema.mockImplementation((_req: any, _md: any, cb: any) => {
        cb(null, { success: true });
      });
      mockValidateInteraction.mockImplementation(
        (_req: any, _md: any, cb: any) => {
          cb(null, { valid: true, errors: [] });
        },
      );

      const schemaPath = path.resolve(__dirname, "../../shared/openapi.json");
      await validator.registerSchema("test-schema", schemaPath);
      await validator.validate(
        { method: "GET", path: "/pet/1" },
        { statusCode: 200 },
      );

      const sent = mockValidateInteraction.mock.calls[0][0];
      expect(sent.schema_id).toBe("test-schema");
      expect(sent.schema_version).toBeUndefined();
    });
  });
});
