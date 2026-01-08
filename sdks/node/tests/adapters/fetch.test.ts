import { ContractValidator, ValidationResult } from "../../src/index";
import {
  createFetchAdapter,
  createValidatingFetch,
  FetchContractAdapter,
} from "../../src/adapters/fetch";

// Store original fetch
const originalFetch = global.fetch;

// Mock the ContractValidator
jest.mock("../../src/index", () => {
  return {
    ContractValidator: jest.fn().mockImplementation(() => ({
      validate: jest.fn().mockResolvedValue({ valid: true, errors: [] }),
      registerSchema: jest.fn().mockResolvedValue(undefined),
      close: jest.fn(),
    })),
  };
});

describe("FetchContractAdapter", () => {
  let mockValidator: jest.Mocked<ContractValidator>;
  let adapter: FetchContractAdapter;
  let mockFetch: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mockValidator = new ContractValidator() as jest.Mocked<ContractValidator>;

    // Mock global fetch
    mockFetch = jest.fn();
    global.fetch = mockFetch;
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  describe("createFetchAdapter", () => {
    it("should create an adapter instance", () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
      });

      expect(adapter).toBeInstanceOf(FetchContractAdapter);
    });

    it("should use default configuration values", () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
      });

      expect(adapter.getInteractions()).toEqual([]);
    });
  });

  describe("request/response capture", () => {
    beforeEach(() => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });
    });

    it("should capture successful GET requests", async () => {
      const responseBody = { id: 1, name: "Fluffy" };
      mockFetch.mockResolvedValue(
        new Response(JSON.stringify(responseBody), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.method).toBe("GET");
      expect(interactions[0].request.path).toBe("/pet/1");
      expect(interactions[0].response.statusCode).toBe(200);
    });

    it("should capture request body for POST requests", async () => {
      const requestBody = {
        name: "Fluffy",
        photoUrls: ["http://example.com/photo.jpg"],
      };
      mockFetch.mockResolvedValue(
        new Response(JSON.stringify({ id: 1, ...requestBody }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
      );

      await adapter.fetch("/pet", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      });

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.method).toBe("POST");
      expect(interactions[0].request.body).toEqual(requestBody);
    });

    it("should capture response body", async () => {
      const responseBody = { id: 1, name: "Fluffy" };
      mockFetch.mockResolvedValue(
        new Response(JSON.stringify(responseBody), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions[0].response.body).toEqual(responseBody);
    });

    it("should capture request headers", async () => {
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/pet/1", {
        headers: {
          Authorization: "Bearer token123",
          "X-Custom-Header": "custom-value",
        },
      });

      const interactions = adapter.getInteractions();
      expect(interactions[0]?.request.headers?.["authorization"]).toBe(
        "Bearer token123",
      );
      expect(interactions[0]?.request.headers?.["x-custom-header"]).toBe(
        "custom-value",
      );
    });

    it("should capture response headers", async () => {
      mockFetch.mockResolvedValue(
        new Response("{}", {
          status: 200,
          headers: {
            "Content-Type": "application/json",
            "X-Request-Id": "req-123",
          },
        }),
      );

      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions[0]?.response.headers?.["content-type"]).toBe(
        "application/json",
      );
      expect(interactions[0]?.response.headers?.["x-request-id"]).toBe(
        "req-123",
      );
    });

    it("should include timestamp in captured interactions", async () => {
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      const beforeTime = new Date();
      await adapter.fetch("/test");
      const afterTime = new Date();

      const interactions = adapter.getInteractions();
      expect(interactions[0].timestamp.getTime()).toBeGreaterThanOrEqual(
        beforeTime.getTime(),
      );
      expect(interactions[0].timestamp.getTime()).toBeLessThanOrEqual(
        afterTime.getTime(),
      );
    });

    it("should handle path with query string", async () => {
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/pet?status=available&limit=10");

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.path).toBe(
        "/pet?status=available&limit=10",
      );
    });

    it("should handle URL object input", async () => {
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch(new URL("http://api.test/pet/1"));

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should handle Request object input", async () => {
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      const request = new Request("http://api.test/pet/1", {
        method: "DELETE",
      });
      await adapter.fetch(request);

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.method).toBe("DELETE");
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should handle non-JSON response body", async () => {
      mockFetch.mockResolvedValue(
        new Response("Plain text response", {
          status: 200,
          headers: { "Content-Type": "text/plain" },
        }),
      );

      await adapter.fetch("/text");

      const interactions = adapter.getInteractions();
      expect(interactions[0].response.body).toBe("Plain text response");
    });

    it("should handle empty response body", async () => {
      mockFetch.mockResolvedValue(new Response(null, { status: 204 }));

      await adapter.fetch("/pet/1", { method: "DELETE" });

      const interactions = adapter.getInteractions();
      expect(interactions[0].response.statusCode).toBe(204);
      expect(interactions[0].response.body).toBeUndefined();
    });
  });

  describe("automatic validation", () => {
    it("should validate requests when autoValidate is true", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: true,
        onValidationFailure: () => {},
      });

      mockFetch.mockResolvedValue(
        new Response(JSON.stringify({ id: 1 }), { status: 200 }),
      );

      await adapter.fetch("/pet/1");

      expect(mockValidator.validate).toHaveBeenCalledTimes(1);
      expect(mockValidator.validate).toHaveBeenCalledWith(
        expect.objectContaining({ method: "GET", path: "/pet/1" }),
        expect.objectContaining({ statusCode: 200 }),
      );
    });

    it("should store validation result in captured interaction", async () => {
      const validationResult: ValidationResult = { valid: true, errors: [] };
      mockValidator.validate = jest.fn().mockResolvedValue(validationResult);

      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: true,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/test");

      const interactions = adapter.getInteractions();
      expect(interactions[0].validationResult).toEqual(validationResult);
    });

    it("should call onValidationFailure when validation fails", async () => {
      const failedResult: ValidationResult = {
        valid: false,
        errors: ["Missing required field"],
      };
      mockValidator.validate = jest.fn().mockResolvedValue(failedResult);

      const onFailure = jest.fn();
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: true,
        onValidationFailure: onFailure,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/test");

      expect(onFailure).toHaveBeenCalledTimes(1);
      expect(onFailure).toHaveBeenCalledWith(
        failedResult,
        expect.any(Request),
        expect.any(Response),
      );
    });

    it("should throw by default when validation fails", async () => {
      const failedResult: ValidationResult = {
        valid: false,
        errors: ["Missing required field"],
      };
      mockValidator.validate = jest.fn().mockResolvedValue(failedResult);

      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: true,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await expect(adapter.fetch("/test")).rejects.toThrow(
        "Contract validation failed",
      );
    });

    it("should not validate when autoValidate is false", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/test");

      expect(mockValidator.validate).not.toHaveBeenCalled();
      expect(adapter.getInteractions()).toHaveLength(1);
    });
  });

  describe("path filtering", () => {
    it("should exclude paths matching excludePaths string", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
        excludePaths: ["/health"],
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/health");
      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should exclude paths matching excludePaths regex", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
        excludePaths: [/^\/health/, /^\/metrics/],
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/health/check");
      await adapter.fetch("/metrics/prometheus");
      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should only validate paths matching includePaths", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
        includePaths: [/^\/pet/],
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/user/1");
      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should apply excludePaths before includePaths", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
        includePaths: [/^\/pet/],
        excludePaths: ["/pet/health"],
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/pet/health");
      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });
  });

  describe("manual validation", () => {
    it("should allow manual validation of captured interactions", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });

      mockFetch.mockResolvedValue(
        new Response(JSON.stringify({ id: 1 }), { status: 200 }),
      );

      await adapter.fetch("/pet/1");

      expect(mockValidator.validate).not.toHaveBeenCalled();

      const interactions = adapter.getInteractions();
      await adapter.validateInteraction(interactions[0]);

      expect(mockValidator.validate).toHaveBeenCalledTimes(1);
    });
  });

  describe("interaction management", () => {
    beforeEach(() => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });
    });

    it("should return a copy of interactions", async () => {
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/test");

      const interactions1 = adapter.getInteractions();
      const interactions2 = adapter.getInteractions();

      expect(interactions1).not.toBe(interactions2);
      expect(interactions1).toEqual(interactions2);
    });

    it("should clear interactions", async () => {
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/test");
      expect(adapter.getInteractions()).toHaveLength(1);

      adapter.clearInteractions();
      expect(adapter.getInteractions()).toHaveLength(0);
    });
  });

  describe("error responses", () => {
    beforeEach(() => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });
    });

    it("should capture 4xx error responses", async () => {
      mockFetch.mockResolvedValue(
        new Response(JSON.stringify({ error: "Not found" }), { status: 404 }),
      );

      await adapter.fetch("/pet/999");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].response.statusCode).toBe(404);
      expect(interactions[0].response.body).toEqual({ error: "Not found" });
    });

    it("should capture 5xx error responses", async () => {
      mockFetch.mockResolvedValue(
        new Response(JSON.stringify({ error: "Internal server error" }), {
          status: 500,
        }),
      );

      await adapter.fetch("/pet/1");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].response.statusCode).toBe(500);
    });
  });

  describe("HTTP methods", () => {
    beforeEach(() => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });
      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));
    });

    it("should capture GET requests", async () => {
      await adapter.fetch("/pet/1", { method: "GET" });

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.method).toBe("GET");
    });

    it("should capture POST requests", async () => {
      await adapter.fetch("/pet", {
        method: "POST",
        body: JSON.stringify({ name: "Fluffy" }),
      });

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.method).toBe("POST");
    });

    it("should capture PUT requests", async () => {
      await adapter.fetch("/pet/1", {
        method: "PUT",
        body: JSON.stringify({ name: "Buddy" }),
      });

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.method).toBe("PUT");
    });

    it("should capture DELETE requests", async () => {
      await adapter.fetch("/pet/1", { method: "DELETE" });

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.method).toBe("DELETE");
    });

    it("should capture PATCH requests", async () => {
      await adapter.fetch("/pet/1", {
        method: "PATCH",
        body: JSON.stringify({ name: "Buddy" }),
      });

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.method).toBe("PATCH");
    });
  });

  describe("baseURL handling", () => {
    it("should prepend baseURL to relative paths", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/pet/1");

      expect(mockFetch).toHaveBeenCalledWith(expect.any(Request));
      const request = mockFetch.mock.calls[0][0] as Request;
      expect(request.url).toBe("http://api.test/pet/1");
    });

    it("should not prepend baseURL to absolute URLs", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("http://other.test/pet/1");

      const request = mockFetch.mock.calls[0][0] as Request;
      expect(request.url).toBe("http://other.test/pet/1");
    });

    it("should handle baseURL with trailing slash", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test/",
        autoValidate: false,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("/pet/1");

      const request = mockFetch.mock.calls[0][0] as Request;
      expect(request.url).toBe("http://api.test/pet/1");
    });

    it("should handle path without leading slash", async () => {
      adapter = createFetchAdapter({
        validator: mockValidator,
        baseURL: "http://api.test",
        autoValidate: false,
      });

      mockFetch.mockResolvedValue(new Response("{}", { status: 200 }));

      await adapter.fetch("pet/1");

      const request = mockFetch.mock.calls[0][0] as Request;
      expect(request.url).toBe("http://api.test/pet/1");
    });
  });
});

describe("createValidatingFetch", () => {
  let mockValidator: jest.Mocked<ContractValidator>;
  let mockFetch: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mockValidator = new ContractValidator() as jest.Mocked<ContractValidator>;

    mockFetch = jest.fn();
    global.fetch = mockFetch;
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("should return wrapped fetch function and adapter", () => {
    const { fetch, adapter } = createValidatingFetch({
      validator: mockValidator,
      baseURL: "http://api.test",
    });

    expect(typeof fetch).toBe("function");
    expect(adapter).toBeInstanceOf(FetchContractAdapter);
  });

  it("should capture interactions via wrapped fetch", async () => {
    const { fetch, adapter } = createValidatingFetch({
      validator: mockValidator,
      baseURL: "http://api.test",
      autoValidate: false,
    });

    mockFetch.mockResolvedValue(
      new Response(JSON.stringify({ id: 1 }), { status: 200 }),
    );

    await fetch("/pet/1");

    const interactions = adapter.getInteractions();
    expect(interactions).toHaveLength(1);
    expect(interactions[0].request.path).toBe("/pet/1");
  });
});
