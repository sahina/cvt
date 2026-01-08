import {
  extractSchemaIdFromUrl,
  extractSchemaIdFromInteractions,
  extractFieldsFromBody,
  normalizePathForEndpoint,
  mergeInteractionsToEndpoints,
  buildConsumerFromInteractions,
  AutoRegisterConfig,
} from "../src/auto-register";
import type { CapturedInteraction } from "../src/adapters/types";

describe("extractSchemaIdFromUrl", () => {
  it("should extract schemaId from mock URL", () => {
    expect(extractSchemaIdFromUrl("http://mock.user-api/users/123")).toBe(
      "user-api",
    );
  });

  it("should extract schemaId from mock URL with subdomain", () => {
    expect(extractSchemaIdFromUrl("http://mock.my-service/api/v1/items")).toBe(
      "my-service",
    );
  });

  it("should return full hostname if no mock prefix", () => {
    expect(extractSchemaIdFromUrl("http://api.example.com/users")).toBe(
      "api.example.com",
    );
  });

  it("should handle https URLs", () => {
    expect(extractSchemaIdFromUrl("https://mock.secure-api/data")).toBe(
      "secure-api",
    );
  });

  it("should handle URLs with ports", () => {
    expect(extractSchemaIdFromUrl("http://mock.test-api:8080/endpoint")).toBe(
      "test-api",
    );
  });

  it("should return null for invalid URLs", () => {
    expect(extractSchemaIdFromUrl("not a url")).toBe(null);
  });
});

describe("extractFieldsFromBody", () => {
  it("should return empty array for null body", () => {
    expect(extractFieldsFromBody(null)).toEqual([]);
  });

  it("should return empty array for undefined body", () => {
    expect(extractFieldsFromBody(undefined)).toEqual([]);
  });

  it("should extract fields from flat object", () => {
    const body = { id: "123", name: "John", email: "john@example.com" };
    const fields = extractFieldsFromBody(body);
    expect(fields).toContain("id");
    expect(fields).toContain("name");
    expect(fields).toContain("email");
  });

  it("should extract nested fields with dot notation", () => {
    const body = {
      id: "123",
      address: {
        city: "NYC",
        zip: "10001",
      },
    };
    const fields = extractFieldsFromBody(body);
    expect(fields).toContain("id");
    expect(fields).toContain("address");
    expect(fields).toContain("address.city");
    expect(fields).toContain("address.zip");
  });

  it("should handle deeply nested objects", () => {
    const body = {
      user: {
        profile: {
          name: "John",
        },
      },
    };
    const fields = extractFieldsFromBody(body);
    expect(fields).toContain("user");
    expect(fields).toContain("user.profile");
    expect(fields).toContain("user.profile.name");
  });

  it("should extract fields from first array element", () => {
    const body = [{ id: "1", name: "Item 1" }];
    const fields = extractFieldsFromBody(body);
    expect(fields).toContain("id");
    expect(fields).toContain("name");
  });

  it("should handle empty array", () => {
    expect(extractFieldsFromBody([])).toEqual([]);
  });

  it("should handle empty object", () => {
    expect(extractFieldsFromBody({})).toEqual([]);
  });

  it("should use prefix when provided", () => {
    const body = { city: "NYC" };
    const fields = extractFieldsFromBody(body, "address");
    expect(fields).toContain("address.city");
  });
});

describe("normalizePathForEndpoint", () => {
  it("should return simple path unchanged", () => {
    expect(normalizePathForEndpoint("/users/123")).toBe("/users/123");
  });

  it("should remove query string", () => {
    expect(normalizePathForEndpoint("/users?page=1&limit=10")).toBe("/users");
  });

  it("should extract path from full URL", () => {
    expect(normalizePathForEndpoint("http://mock.user-api/users/123")).toBe(
      "/users/123",
    );
  });

  it("should extract path from full URL with query", () => {
    expect(
      normalizePathForEndpoint("http://mock.user-api/users?active=true"),
    ).toBe("/users");
  });

  it("should handle https URL", () => {
    expect(normalizePathForEndpoint("https://mock.api/data/items")).toBe(
      "/data/items",
    );
  });
});

describe("extractSchemaIdFromInteractions", () => {
  it("should extract single schemaId", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "http://mock.user-api/users/123" },
        response: { statusCode: 200 },
        timestamp: new Date(),
      },
    ];

    const result = extractSchemaIdFromInteractions(interactions);
    expect(result.error).toBeNull();
    expect(result.schemaId).toBe("user-api");
  });

  it("should handle multiple interactions with same schema", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "http://mock.user-api/users/123" },
        response: { statusCode: 200 },
        timestamp: new Date(),
      },
      {
        request: { method: "POST", path: "http://mock.user-api/users" },
        response: { statusCode: 201 },
        timestamp: new Date(),
      },
    ];

    const result = extractSchemaIdFromInteractions(interactions);
    expect(result.error).toBeNull();
    expect(result.schemaId).toBe("user-api");
  });

  it("should error on multiple different schemas", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "http://mock.user-api/users/123" },
        response: { statusCode: 200 },
        timestamp: new Date(),
      },
      {
        request: { method: "GET", path: "http://mock.order-api/orders/456" },
        response: { statusCode: 200 },
        timestamp: new Date(),
      },
    ];

    const result = extractSchemaIdFromInteractions(interactions);
    expect(result.error).toContain("multiple schemas detected");
    expect(result.schemaId).toBeNull();
  });

  it("should error when no URLs in paths", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "/users/123" },
        response: { statusCode: 200 },
        timestamp: new Date(),
      },
    ];

    const result = extractSchemaIdFromInteractions(interactions);
    expect(result.error).toContain("could not extract schemaId");
    expect(result.schemaId).toBeNull();
  });
});

describe("mergeInteractionsToEndpoints", () => {
  it("should merge interactions to endpoints", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "http://mock.user-api/users/123" },
        response: {
          statusCode: 200,
          body: { id: "123", name: "John" },
        },
        timestamp: new Date(),
      },
      {
        request: { method: "POST", path: "http://mock.user-api/users" },
        response: {
          statusCode: 201,
          body: { id: "789" },
        },
        timestamp: new Date(),
      },
    ];

    const endpoints = mergeInteractionsToEndpoints(interactions);
    expect(endpoints).toHaveLength(2);

    const getEndpoint = endpoints.find(
      (ep) => ep.method === "GET" && ep.path === "/users/123",
    );
    expect(getEndpoint).toBeDefined();
    expect(getEndpoint!.usedFields).toContain("id");
    expect(getEndpoint!.usedFields).toContain("name");

    const postEndpoint = endpoints.find(
      (ep) => ep.method === "POST" && ep.path === "/users",
    );
    expect(postEndpoint).toBeDefined();
    expect(postEndpoint!.usedFields).toContain("id");
  });

  it("should merge fields from duplicate endpoints", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "http://mock.user-api/users/123" },
        response: {
          statusCode: 200,
          body: { id: "123", name: "John" },
        },
        timestamp: new Date(),
      },
      {
        request: { method: "GET", path: "http://mock.user-api/users/123" },
        response: {
          statusCode: 200,
          body: { id: "123", email: "john@example.com" },
        },
        timestamp: new Date(),
      },
    ];

    const endpoints = mergeInteractionsToEndpoints(interactions);
    expect(endpoints).toHaveLength(1);

    const endpoint = endpoints[0];
    expect(endpoint.usedFields).toContain("id");
    expect(endpoint.usedFields).toContain("name");
    expect(endpoint.usedFields).toContain("email");
  });
});

describe("buildConsumerFromInteractions", () => {
  const validConfig: AutoRegisterConfig = {
    consumerId: "test-service",
    consumerVersion: "1.0.0",
    environment: "dev",
    schemaVersion: "1.0.0",
  };

  const validInteractions: CapturedInteraction[] = [
    {
      request: { method: "GET", path: "http://mock.user-api/users/123" },
      response: {
        statusCode: 200,
        body: { id: "123", name: "John", email: "john@example.com" },
      },
      timestamp: new Date(),
    },
  ];

  it("should build options from valid interactions", () => {
    const result = buildConsumerFromInteractions(
      validInteractions,
      validConfig,
    );

    expect(result.error).toBeNull();
    expect(result.options).not.toBeNull();
    expect(result.options!.consumerId).toBe("test-service");
    expect(result.options!.consumerVersion).toBe("1.0.0");
    expect(result.options!.schemaId).toBe("user-api");
    expect(result.options!.schemaVersion).toBe("1.0.0");
    expect(result.options!.environment).toBe("dev");
    expect(result.options!.usedEndpoints).toHaveLength(1);
  });

  it("should error on missing consumerId", () => {
    const config = { ...validConfig, consumerId: "" };
    const result = buildConsumerFromInteractions(validInteractions, config);
    expect(result.error).toBe("consumerId is required");
    expect(result.options).toBeNull();
  });

  it("should error on missing consumerVersion", () => {
    const config = { ...validConfig, consumerVersion: "" };
    const result = buildConsumerFromInteractions(validInteractions, config);
    expect(result.error).toBe("consumerVersion is required");
    expect(result.options).toBeNull();
  });

  it("should error on missing environment", () => {
    const config = { ...validConfig, environment: "" };
    const result = buildConsumerFromInteractions(validInteractions, config);
    expect(result.error).toBe("environment is required");
    expect(result.options).toBeNull();
  });

  it("should error on missing schemaVersion", () => {
    const config = { ...validConfig, schemaVersion: "" };
    const result = buildConsumerFromInteractions(validInteractions, config);
    expect(result.error).toBe("schemaVersion is required");
    expect(result.options).toBeNull();
  });

  it("should error on empty interactions", () => {
    const result = buildConsumerFromInteractions([], validConfig);
    expect(result.error).toBe("no interactions to register");
    expect(result.options).toBeNull();
  });

  it("should use explicit schemaId override", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "/users/123" },
        response: { statusCode: 200, body: { id: "123" } },
        timestamp: new Date(),
      },
    ];

    const config: AutoRegisterConfig = {
      ...validConfig,
      schemaId: "my-custom-schema",
    };

    const result = buildConsumerFromInteractions(interactions, config);
    expect(result.error).toBeNull();
    expect(result.options!.schemaId).toBe("my-custom-schema");
  });

  it("should extract nested fields from response body", () => {
    const interactions: CapturedInteraction[] = [
      {
        request: { method: "GET", path: "http://mock.user-api/users/123" },
        response: {
          statusCode: 200,
          body: {
            id: "123",
            address: { city: "NYC", zip: "10001" },
          },
        },
        timestamp: new Date(),
      },
    ];

    const result = buildConsumerFromInteractions(interactions, validConfig);
    expect(result.error).toBeNull();

    const fields = result.options!.usedEndpoints![0].usedFields;
    expect(fields).toContain("id");
    expect(fields).toContain("address");
    expect(fields).toContain("address.city");
    expect(fields).toContain("address.zip");
  });
});
