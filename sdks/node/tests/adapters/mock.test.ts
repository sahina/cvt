import { ContractValidator, GeneratedResponse } from "../../src/index";
import {
  createMockAdapter,
  createMockFetch,
  MockAdapter,
} from "../../src/adapters/mock";

// Mock the ContractValidator
jest.mock("../../src/index", () => {
  return {
    ContractValidator: jest.fn().mockImplementation(() => ({
      generateResponse: jest.fn().mockResolvedValue({
        statusCode: 200,
        headers: { "content-type": "application/json" },
        body: { id: "123", name: "Test User" },
      } as GeneratedResponse),
      validate: jest.fn().mockResolvedValue({ valid: true, errors: [] }),
      registerSchema: jest.fn().mockResolvedValue(undefined),
      close: jest.fn(),
    })),
  };
});

describe("MockAdapter", () => {
  let mockValidator: jest.Mocked<ContractValidator>;
  let adapter: MockAdapter;

  beforeEach(() => {
    jest.clearAllMocks();
    mockValidator = new ContractValidator() as jest.Mocked<ContractValidator>;
  });

  describe("createMockAdapter", () => {
    it("should create an adapter instance", () => {
      adapter = createMockAdapter({
        validator: mockValidator,
      });

      expect(adapter).toBeInstanceOf(MockAdapter);
    });

    it("should return empty interactions initially", () => {
      adapter = createMockAdapter({
        validator: mockValidator,
      });

      expect(adapter.getInteractions()).toEqual([]);
    });
  });

  describe("fetch", () => {
    beforeEach(() => {
      adapter = createMockAdapter({
        validator: mockValidator,
      });
    });

    it("should return schema-generated response for GET requests", async () => {
      const response = await adapter.fetch("http://mock.api/users/123");

      expect(response.status).toBe(200);
      expect(response.headers.get("content-type")).toBe("application/json");

      const body = await response.json();
      expect(body).toEqual({ id: "123", name: "Test User" });
    });

    it("should call generateResponse with correct method and path", async () => {
      await adapter.fetch("http://mock.api/users/123");

      expect(mockValidator.generateResponse).toHaveBeenCalledWith(
        "GET",
        "/users/123",
        {},
      );
    });

    it("should handle POST requests", async () => {
      await adapter.fetch("http://mock.api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "New User" }),
      });

      expect(mockValidator.generateResponse).toHaveBeenCalledWith(
        "POST",
        "/users",
        {},
      );
    });

    it("should capture request in interactions", async () => {
      await adapter.fetch("http://mock.api/users/123");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.method).toBe("GET");
      expect(interactions[0].request.path).toBe("/users/123");
    });

    it("should capture response in interactions", async () => {
      await adapter.fetch("http://mock.api/users/123");

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].response.statusCode).toBe(200);
      expect(interactions[0].response.body).toEqual({
        id: "123",
        name: "Test User",
      });
    });

    it("should capture request body for POST", async () => {
      const requestBody = { name: "New User", email: "test@example.com" };

      await adapter.fetch("http://mock.api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      });

      const interactions = adapter.getInteractions();
      expect(interactions[0].request.body).toEqual(requestBody);
    });

    it("should include query string in captured interaction path", async () => {
      await adapter.fetch("http://mock.api/users?status=active&limit=10");

      // generateResponse is called with path WITHOUT query string (for OpenAPI route matching)
      expect(mockValidator.generateResponse).toHaveBeenCalledWith(
        "GET",
        "/users",
        {},
      );

      // But the captured interaction should have the full path WITH query string
      const interactions = adapter.getInteractions();
      expect(interactions[0].request.path).toBe("/users?status=active&limit=10");
    });

    it("should handle URL object input", async () => {
      const url = new URL("http://mock.api/users/456");
      await adapter.fetch(url);

      expect(mockValidator.generateResponse).toHaveBeenCalledWith(
        "GET",
        "/users/456",
        {},
      );
    });
  });

  describe("caching", () => {
    it("should not cache by default", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
      });

      await adapter.fetch("http://mock.api/users/123");
      await adapter.fetch("http://mock.api/users/123");

      expect(mockValidator.generateResponse).toHaveBeenCalledTimes(2);
    });

    it("should cache responses when enabled", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
        cache: true,
      });

      await adapter.fetch("http://mock.api/users/123");
      await adapter.fetch("http://mock.api/users/123");

      expect(mockValidator.generateResponse).toHaveBeenCalledTimes(1);
    });

    it("should cache by method+path", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
        cache: true,
      });

      await adapter.fetch("http://mock.api/users/123");
      await adapter.fetch("http://mock.api/users/456");

      expect(mockValidator.generateResponse).toHaveBeenCalledTimes(2);
    });

    it("should clear cache", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
        cache: true,
      });

      await adapter.fetch("http://mock.api/users/123");
      adapter.clearCache();
      await adapter.fetch("http://mock.api/users/123");

      expect(mockValidator.generateResponse).toHaveBeenCalledTimes(2);
    });
  });

  describe("interactions management", () => {
    beforeEach(() => {
      adapter = createMockAdapter({
        validator: mockValidator,
      });
    });

    it("should return copy of interactions", async () => {
      await adapter.fetch("http://mock.api/users/123");

      const interactions1 = adapter.getInteractions();
      const interactions2 = adapter.getInteractions();

      expect(interactions1).not.toBe(interactions2);
      expect(interactions1).toEqual(interactions2);
    });

    it("should clear interactions", async () => {
      await adapter.fetch("http://mock.api/users/123");
      expect(adapter.getInteractions()).toHaveLength(1);

      adapter.clearInteractions();
      expect(adapter.getInteractions()).toHaveLength(0);
    });

    it("should record timestamp", async () => {
      const before = new Date();
      await adapter.fetch("http://mock.api/users/123");
      const after = new Date();

      const interactions = adapter.getInteractions();
      expect(interactions[0].timestamp.getTime()).toBeGreaterThanOrEqual(
        before.getTime(),
      );
      expect(interactions[0].timestamp.getTime()).toBeLessThanOrEqual(
        after.getTime(),
      );
    });
  });

  describe("path filtering", () => {
    it("should exclude paths matching excludePaths", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
        excludePaths: ["/health"],
      });

      await expect(adapter.fetch("http://mock.api/health")).rejects.toThrow(
        'path "/health" is excluded from mocking',
      );
    });

    it("should exclude paths matching excludePaths regex", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
        excludePaths: [/^\/internal/],
      });

      await expect(
        adapter.fetch("http://mock.api/internal/status"),
      ).rejects.toThrow("excluded from mocking");
    });

    it("should only include paths matching includePaths", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
        includePaths: ["/api/"],
      });

      // Included path works
      await adapter.fetch("http://mock.api/api/users/123");
      expect(adapter.getInteractions()).toHaveLength(1);

      // Non-included path fails
      await expect(adapter.fetch("http://mock.api/other/path")).rejects.toThrow(
        "excluded from mocking",
      );
    });
  });

  describe("generateOptions", () => {
    it("should pass generateOptions to generateResponse", async () => {
      adapter = createMockAdapter({
        validator: mockValidator,
        generateOptions: {
          statusCode: 201,
          useExamples: true,
        },
      });

      await adapter.fetch("http://mock.api/users", { method: "POST" });

      expect(mockValidator.generateResponse).toHaveBeenCalledWith(
        "POST",
        "/users",
        {
          statusCode: 201,
          useExamples: true,
        },
      );
    });
  });

  describe("createMockFetch", () => {
    it("should return a fetch-like function", async () => {
      const mockFetch = createMockFetch(mockValidator);

      const response = await mockFetch("http://mock.api/users/123");
      expect(response.status).toBe(200);

      const body = await response.json();
      expect(body).toEqual({ id: "123", name: "Test User" });
    });

    it("should accept options", async () => {
      const mockFetch = createMockFetch(mockValidator, { cache: true });

      await mockFetch("http://mock.api/users/123");
      await mockFetch("http://mock.api/users/123");

      expect(mockValidator.generateResponse).toHaveBeenCalledTimes(1);
    });
  });

  describe("custom status codes", () => {
    it("should handle different status codes", async () => {
      mockValidator.generateResponse = jest.fn().mockResolvedValue({
        statusCode: 201,
        headers: { "content-type": "application/json" },
        body: { id: "new-123" },
      });

      adapter = createMockAdapter({
        validator: mockValidator,
      });

      const response = await adapter.fetch("http://mock.api/users", {
        method: "POST",
      });

      expect(response.status).toBe(201);
      expect(response.statusText).toBe("Created");
    });

    it("should handle 404 status", async () => {
      mockValidator.generateResponse = jest.fn().mockResolvedValue({
        statusCode: 404,
        headers: { "content-type": "application/json" },
        body: { error: "Not Found" },
      });

      adapter = createMockAdapter({
        validator: mockValidator,
      });

      const response = await adapter.fetch("http://mock.api/users/unknown");

      expect(response.status).toBe(404);
      expect(response.statusText).toBe("Not Found");
    });
  });
});
