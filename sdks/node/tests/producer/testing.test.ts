import {
  ProducerTestKit,
  createProducerTestKit,
  type ProducerTestConfig,
  type ProducerValidationResult,
} from "../../src/producer/testing";

// Mock gRPC and proto-loader
const mockValidateProducerResponse = jest.fn();
const mockClose = jest.fn();

jest.mock("@grpc/grpc-js", () => {
  class MockMetadata {
    private data: Map<string, string> = new Map();
    set(key: string, value: string) {
      this.data.set(key, value);
    }
    get(key: string) {
      return this.data.get(key);
    }
    getMap() {
      return Object.fromEntries(this.data);
    }
  }

  return {
    loadPackageDefinition: jest.fn().mockReturnValue({
      cvt: {
        ContractValidator: jest.fn().mockImplementation(() => ({
          ValidateProducerResponse: mockValidateProducerResponse,
          close: mockClose,
        })),
      },
    }),
    credentials: {
      createInsecure: jest.fn(),
    },
    Metadata: MockMetadata,
  };
});

jest.mock("@grpc/proto-loader", () => ({
  loadSync: jest.fn().mockReturnValue({}),
}));

describe("ProducerTestKit", () => {
  beforeEach(() => {
    mockValidateProducerResponse.mockReset();
    mockClose.mockReset();
    jest.clearAllMocks();
  });

  describe("constructor", () => {
    it("should throw error when schemaId is missing", () => {
      expect(() => {
        new ProducerTestKit({
          schemaId: "",
        });
      }).toThrow("schemaId is required");
    });

    it("should create instance with valid config", () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });
      expect(testKit).toBeDefined();
      testKit.close();
    });

    it("should accept optional schemaVersion", () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
        schemaVersion: "1.0.0",
      });
      expect(testKit).toBeDefined();
      testKit.close();
    });

    it("should accept custom server address", () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
        serverAddress: "custom-host:9999",
      });
      expect(testKit).toBeDefined();
      testKit.close();
    });

    it("should use localhost:9550 as default address when no serverAddress is provided", () => {
      const grpc = jest.requireMock("@grpc/grpc-js");
      const mockContractValidator =
        grpc.loadPackageDefinition().cvt.ContractValidator;
      mockContractValidator.mockClear();

      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });
      expect(mockContractValidator).toHaveBeenCalledTimes(1);
      expect(mockContractValidator.mock.calls[0][0]).toBe("localhost:9550");
      testKit.close();
    });

    it("should accept API key for authentication", () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
        apiKey: "test-api-key",
      });
      expect(testKit).toBeDefined();
      testKit.close();
    });
  });

  describe("createProducerTestKit", () => {
    it("should be an alternative to constructor", () => {
      const testKit = createProducerTestKit({
        schemaId: "test-api",
      });
      expect(testKit).toBeInstanceOf(ProducerTestKit);
      testKit.close();
    });
  });

  describe("validateResponse", () => {
    it("should validate response successfully", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
        schemaVersion: "1.0.0",
      });

      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            valid: true,
            errors: [],
            validated_against_version: "1.0.0",
            validated_against_hash: "abc123",
          });
        },
      );

      const result = await testKit.validateResponse({
        method: "GET",
        path: "/users/123",
        response: {
          statusCode: 200,
          headers: { "Content-Type": "application/json" },
          body: { id: "123", name: "John" },
        },
      });

      expect(result.valid).toBe(true);
      expect(result.errors).toEqual([]);
      expect(result.validatedAgainstVersion).toBe("1.0.0");
      expect(result.validatedAgainstHash).toBe("abc123");
      testKit.close();
    });

    it("should return validation errors", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, {
            valid: false,
            errors: ["Missing required field: name"],
          });
        },
      );

      const result = await testKit.validateResponse({
        method: "GET",
        path: "/users/123",
        response: {
          statusCode: 200,
          body: { id: "123" },
        },
      });

      expect(result.valid).toBe(false);
      expect(result.errors).toContain("Missing required field: name");
      testKit.close();
    });

    it("should reject on gRPC error", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(new Error("gRPC connection failed"));
        },
      );

      await expect(
        testKit.validateResponse({
          method: "GET",
          path: "/users/123",
          response: {
            statusCode: 200,
          },
        }),
      ).rejects.toThrow("gRPC connection failed");

      testKit.close();
    });

    it("should include request context when provided", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      let capturedRequest: any;
      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          capturedRequest = req;
          callback(null, { valid: true, errors: [] });
        },
      );

      await testKit.validateResponse({
        method: "POST",
        path: "/users",
        response: {
          statusCode: 201,
          body: { id: "123" },
        },
        request: {
          method: "POST",
          path: "/users",
          headers: { "Content-Type": "application/json" },
          body: { name: "John" },
        },
      });

      expect(capturedRequest.request).toBeDefined();
      expect(capturedRequest.request.method).toBe("POST");
      expect(capturedRequest.request.headers).toEqual({
        "Content-Type": "application/json",
      });
      testKit.close();
    });

    it("should serialize string body as-is", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      let capturedRequest: any;
      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          capturedRequest = req;
          callback(null, { valid: true, errors: [] });
        },
      );

      await testKit.validateResponse({
        method: "GET",
        path: "/data",
        response: {
          statusCode: 200,
          body: "raw string body",
        },
      });

      expect(capturedRequest.response.body).toBe("raw string body");
      testKit.close();
    });

    it("should serialize null/undefined body as empty string", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      let capturedRequest: any;
      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          capturedRequest = req;
          callback(null, { valid: true, errors: [] });
        },
      );

      await testKit.validateResponse({
        method: "DELETE",
        path: "/users/123",
        response: {
          statusCode: 204,
          body: null,
        },
      });

      expect(capturedRequest.response.body).toBe("");
      testKit.close();
    });

    it("should serialize object body as JSON", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      let capturedRequest: any;
      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          capturedRequest = req;
          callback(null, { valid: true, errors: [] });
        },
      );

      const bodyObj = { id: 123, name: "Test" };
      await testKit.validateResponse({
        method: "GET",
        path: "/data",
        response: {
          statusCode: 200,
          body: bodyObj,
        },
      });

      expect(capturedRequest.response.body).toBe(JSON.stringify(bodyObj));
      testKit.close();
    });

    it("should include API key in metadata when configured", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
        apiKey: "secret-key",
      });

      let capturedMetadata: any;
      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          capturedMetadata = metadata;
          callback(null, { valid: true, errors: [] });
        },
      );

      await testKit.validateResponse({
        method: "GET",
        path: "/users",
        response: { statusCode: 200 },
      });

      expect(capturedMetadata.get("x-api-key")).toBe("secret-key");
      testKit.close();
    });
  });

  describe("validateInteraction", () => {
    it("should validate full interaction", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          callback(null, { valid: true, errors: [] });
        },
      );

      const result = await testKit.validateInteraction({
        request: {
          method: "POST",
          path: "/users",
          headers: { "Content-Type": "application/json" },
          body: { name: "John" },
        },
        response: {
          statusCode: 201,
          body: { id: "123", name: "John" },
        },
      });

      expect(result.valid).toBe(true);
      testKit.close();
    });
  });

  describe("forEndpoint", () => {
    it("should return an endpoint tester", () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      const endpointTester = testKit.forEndpoint("GET", "/users/{id}");
      expect(endpointTester).toBeDefined();
      expect(typeof endpointTester.validateResponse).toBe("function");

      testKit.close();
    });

    it("should substitute path parameters", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      let capturedRequest: any;
      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          capturedRequest = req;
          callback(null, { valid: true, errors: [] });
        },
      );

      const endpointTester = testKit.forEndpoint(
        "GET",
        "/users/{id}/posts/{postId}",
      );
      await endpointTester.validateResponse(
        { statusCode: 200, body: [] },
        { pathValues: { id: "123", postId: "456" } },
      );

      expect(capturedRequest.path).toBe("/users/123/posts/456");
      testKit.close();
    });

    it("should work without path parameter substitution", async () => {
      const testKit = new ProducerTestKit({
        schemaId: "test-api",
      });

      let capturedRequest: any;
      mockValidateProducerResponse.mockImplementation(
        (req: any, metadata: any, callback: any) => {
          capturedRequest = req;
          callback(null, { valid: true, errors: [] });
        },
      );

      const endpointTester = testKit.forEndpoint("GET", "/users");
      await endpointTester.validateResponse({ statusCode: 200, body: [] });

      expect(capturedRequest.path).toBe("/users");
      testKit.close();
    });
  });

  describe("API shape", () => {
    let testKit: ProducerTestKit;

    beforeAll(() => {
      testKit = new ProducerTestKit({
        schemaId: "test-api",
      });
    });

    afterAll(() => {
      testKit.close();
    });

    it("should have validateResponse method", () => {
      expect(typeof testKit.validateResponse).toBe("function");
    });

    it("should have validateInteraction method", () => {
      expect(typeof testKit.validateInteraction).toBe("function");
    });

    it("should have forEndpoint method", () => {
      expect(typeof testKit.forEndpoint).toBe("function");
    });

    it("should have close method", () => {
      expect(typeof testKit.close).toBe("function");
    });
  });
});

describe("ProducerTestConfig type", () => {
  it("should require schemaId", () => {
    const config: ProducerTestConfig = {
      schemaId: "required-field",
    };
    expect(config.schemaId).toBe("required-field");
  });

  it("should make schemaVersion optional", () => {
    const withVersion: ProducerTestConfig = {
      schemaId: "test",
      schemaVersion: "1.0.0",
    };
    const withoutVersion: ProducerTestConfig = {
      schemaId: "test",
    };
    expect(withVersion.schemaVersion).toBe("1.0.0");
    expect(withoutVersion.schemaVersion).toBeUndefined();
  });

  it("should make serverAddress optional", () => {
    const withAddress: ProducerTestConfig = {
      schemaId: "test",
      serverAddress: "localhost:50051",
    };
    const withoutAddress: ProducerTestConfig = {
      schemaId: "test",
    };
    expect(withAddress.serverAddress).toBe("localhost:50051");
    expect(withoutAddress.serverAddress).toBeUndefined();
  });
});

describe("ProducerValidationResult type", () => {
  it("should have valid boolean property", () => {
    const result: ProducerValidationResult = {
      valid: true,
      errors: [],
    };
    expect(result.valid).toBe(true);
  });

  it("should have errors array", () => {
    const result: ProducerValidationResult = {
      valid: false,
      errors: ["Error 1", "Error 2"],
    };
    expect(result.errors).toHaveLength(2);
  });

  it("should have optional version info", () => {
    const result: ProducerValidationResult = {
      valid: true,
      errors: [],
      validatedAgainstVersion: "1.0.0",
      validatedAgainstHash: "abc123",
    };
    expect(result.validatedAgainstVersion).toBe("1.0.0");
    expect(result.validatedAgainstHash).toBe("abc123");
  });
});
