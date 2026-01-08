import {
  Producer,
  recordValidationMetrics,
  recordRejection,
  getMetrics,
  resetMetrics,
} from "../../src/producer/producer";
import {
  shouldValidatePath,
  matchesPathFilter,
  type Validator,
  type ProducerValidationResult,
  type Interaction,
} from "../../src/producer/types";

/**
 * Mock validator that always returns valid.
 */
function createMockValidator(
  validateFn?: (
    schemaId: string,
    interaction: Interaction,
  ) => Promise<ProducerValidationResult>,
): Validator {
  return {
    validate:
      validateFn ||
      (async () => ({
        valid: true,
        errors: [],
      })),
  };
}

/**
 * Mock validator that always returns invalid.
 */
function createInvalidValidator(
  errors: string[] = ["Validation error"],
): Validator {
  return {
    validate: async () => ({
      valid: false,
      errors,
    }),
  };
}

/**
 * Mock Express-style response object.
 */
function createMockResponse() {
  const res: any = {
    statusCode: 200,
    headersSent: false,
    headers: {} as Record<string, string>,
    body: null as any,
    status(code: number) {
      res.statusCode = code;
      return res;
    },
    json(data: any) {
      res.body = data;
      res.headersSent = true;
      return res;
    },
    setHeader(name: string, value: string) {
      res.headers[name] = value;
    },
    end(data?: any) {
      if (data) res.body = data;
      res.headersSent = true;
    },
  };
  return res;
}

/**
 * Mock Express-style request object.
 */
function createMockRequest(method: string, path: string, body?: any) {
  return {
    method,
    url: path,
    path,
    headers: { "content-type": "application/json" },
    body,
  };
}

describe("Producer", () => {
  describe("constructor", () => {
    it("should create producer with valid config", () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
      });
      expect(producer).toBeDefined();
      expect(producer.mode).toBe("strict");
    });

    it("should throw error when schemaId is missing", () => {
      expect(() => {
        new Producer({
          schemaId: "",
          validator: createMockValidator(),
        });
      }).toThrow("schemaId is required");
    });

    it("should throw error when validator is missing", () => {
      expect(() => {
        new Producer({
          schemaId: "test-schema",
          validator: undefined as any,
        });
      }).toThrow("validator is required");
    });

    it("should apply default config values", () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
      });
      expect(producer.mode).toBe("strict");
    });

    it("should use custom mode when provided", () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        mode: "warn",
      });
      expect(producer.mode).toBe("warn");
    });
  });

  describe("validateRequest", () => {
    it("should validate a valid request", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
      });

      const result = await producer.validateRequest(
        "POST",
        "/users",
        { "content-type": "application/json" },
        { name: "test" },
      );

      expect(result.valid).toBe(true);
      expect(result.type).toBe("request");
    });

    it("should return invalid for invalid request", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createInvalidValidator(["missing required field: email"]),
      });

      const result = await producer.validateRequest(
        "POST",
        "/users",
        { "content-type": "application/json" },
        { name: "test" },
      );

      expect(result.valid).toBe(false);
      expect(result.errors).toContain("missing required field: email");
      expect(result.type).toBe("request");
    });

    it("should skip validation when validateRequest is false", async () => {
      const validateFn = jest.fn();
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(validateFn),
        validateRequest: false,
      });

      const result = await producer.validateRequest("POST", "/users", {}, {});

      expect(result.valid).toBe(true);
      expect(validateFn).not.toHaveBeenCalled();
    });

    it("should pass correct interaction to validator", async () => {
      let capturedInteraction: Interaction | null = null;
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(async (schemaId, interaction) => {
          capturedInteraction = interaction;
          return { valid: true };
        }),
      });

      await producer.validateRequest(
        "POST",
        "/users?page=1",
        { "content-type": "application/json" },
        { name: "test" },
      );

      expect(capturedInteraction).not.toBeNull();
      expect(capturedInteraction!.method).toBe("POST");
      expect(capturedInteraction!.path).toBe("/users?page=1");
      expect(capturedInteraction!.body).toEqual({ name: "test" });
    });
  });

  describe("validateResponse", () => {
    it("should validate a valid response", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
      });

      const result = await producer.validateResponse(
        "GET",
        "/users/123",
        {},
        null,
        200,
        { "content-type": "application/json" },
        { id: 123, name: "test" },
      );

      expect(result.valid).toBe(true);
      expect(result.type).toBe("response");
    });

    it("should return invalid for invalid response", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createInvalidValidator(["response body type mismatch"]),
      });

      const result = await producer.validateResponse(
        "GET",
        "/users/123",
        {},
        null,
        200,
        {},
        { id: "not-a-number" },
      );

      expect(result.valid).toBe(false);
      expect(result.errors).toContain("response body type mismatch");
      expect(result.type).toBe("response");
    });

    it("should skip validation when validateResponse is false", async () => {
      const validateFn = jest.fn();
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(validateFn),
        validateResponse: false,
      });

      const result = await producer.validateResponse(
        "GET",
        "/users/123",
        {},
        null,
        200,
        {},
        {},
      );

      expect(result.valid).toBe(true);
      expect(validateFn).not.toHaveBeenCalled();
    });
  });

  describe("handleRequestFailure", () => {
    it("should reject request in strict mode", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        mode: "strict",
      });

      const req = createMockRequest("POST", "/users");
      const res = createMockResponse();
      const result: ProducerValidationResult = {
        valid: false,
        errors: ["test error"],
      };

      const shouldContinue = await producer.handleRequestFailure(
        result,
        req,
        res,
      );

      expect(shouldContinue).toBe(false);
      expect(res.statusCode).toBe(400);
      expect(res.body).toEqual({
        error: "Request validation failed",
        details: ["test error"],
      });
    });

    it("should continue in warn mode", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        mode: "warn",
      });

      const req = createMockRequest("POST", "/users");
      const res = createMockResponse();
      const result: ProducerValidationResult = {
        valid: false,
        errors: ["test error"],
      };

      // Suppress console.warn
      const warnSpy = jest.spyOn(console, "warn").mockImplementation();

      const shouldContinue = await producer.handleRequestFailure(
        result,
        req,
        res,
      );

      expect(shouldContinue).toBe(true);
      expect(res.body).toBeNull(); // Response not modified
      expect(warnSpy).toHaveBeenCalled();

      warnSpy.mockRestore();
    });

    it("should continue in shadow mode", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        mode: "shadow",
      });

      const req = createMockRequest("POST", "/users");
      const res = createMockResponse();
      const result: ProducerValidationResult = {
        valid: false,
        errors: ["test error"],
      };

      const shouldContinue = await producer.handleRequestFailure(
        result,
        req,
        res,
      );

      expect(shouldContinue).toBe(true);
      expect(res.body).toBeNull(); // Response not modified
    });

    it("should call custom onRequestFailure handler", async () => {
      const customHandler = jest.fn().mockReturnValue({ custom: "response" });

      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        mode: "strict",
        onRequestFailure: customHandler,
      });

      const req = createMockRequest("POST", "/users");
      const res = createMockResponse();
      const result: ProducerValidationResult = {
        valid: false,
        errors: ["test error"],
      };

      const shouldContinue = await producer.handleRequestFailure(
        result,
        req,
        res,
      );

      expect(customHandler).toHaveBeenCalledWith(result, req);
      expect(shouldContinue).toBe(false);
    });
  });

  describe("handleResponseFailure", () => {
    it("should log in strict mode", async () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        mode: "strict",
      });

      const req = createMockRequest("GET", "/users/123");
      const res = createMockResponse();
      const result: ProducerValidationResult = {
        valid: false,
        errors: ["response error"],
      };

      const warnSpy = jest.spyOn(console, "warn").mockImplementation();

      await producer.handleResponseFailure(result, req, res);

      expect(warnSpy).toHaveBeenCalled();
      warnSpy.mockRestore();
    });

    it("should call custom onResponseFailure handler", async () => {
      const customHandler = jest.fn();

      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        mode: "strict",
        onResponseFailure: customHandler,
      });

      const req = createMockRequest("GET", "/users/123");
      const res = createMockResponse();
      const result: ProducerValidationResult = {
        valid: false,
        errors: ["response error"],
      };

      await producer.handleResponseFailure(result, req, res);

      expect(customHandler).toHaveBeenCalledWith(result, req, res);
    });
  });

  describe("shouldValidatePath", () => {
    it("should validate all paths by default", () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
      });

      expect(producer.shouldValidatePath("/api/users")).toBe(true);
      expect(producer.shouldValidatePath("/health")).toBe(true);
      expect(producer.shouldValidatePath("/any/path")).toBe(true);
    });

    it("should exclude paths matching excludePaths", () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        excludePaths: ["/health", "/metrics"],
      });

      expect(producer.shouldValidatePath("/api/users")).toBe(true);
      expect(producer.shouldValidatePath("/health")).toBe(false);
      expect(producer.shouldValidatePath("/metrics")).toBe(false);
    });

    it("should only include paths matching includePaths", () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        includePaths: ["/api/"],
      });

      expect(producer.shouldValidatePath("/api/users")).toBe(true);
      expect(producer.shouldValidatePath("/health")).toBe(false);
    });

    it("should give precedence to excludePaths", () => {
      const producer = new Producer({
        schemaId: "test-schema",
        validator: createMockValidator(),
        includePaths: ["/api/"],
        excludePaths: ["/api/internal"],
      });

      expect(producer.shouldValidatePath("/api/users")).toBe(true);
      expect(producer.shouldValidatePath("/api/internal/debug")).toBe(false);
    });
  });
});

describe("Path filtering utilities", () => {
  describe("matchesPathFilter", () => {
    it("should match string patterns (substring)", () => {
      expect(matchesPathFilter("/api/users", "/api/")).toBe(true);
      expect(matchesPathFilter("/api/users", "/users")).toBe(true);
      expect(matchesPathFilter("/api/users", "/other")).toBe(false);
    });

    it("should match regex patterns", () => {
      expect(matchesPathFilter("/api/users/123", /^\/api\//)).toBe(true);
      expect(matchesPathFilter("/health", /^\/api\//)).toBe(false);
      expect(matchesPathFilter("/users/123", /\/users\/\d+/)).toBe(true);
    });
  });

  describe("shouldValidatePath", () => {
    it("should validate all paths when no filters", () => {
      expect(shouldValidatePath("/any/path")).toBe(true);
      expect(shouldValidatePath("/another/path")).toBe(true);
    });

    it("should apply excludePaths first", () => {
      expect(shouldValidatePath("/health", [], ["/health"])).toBe(false);
      expect(shouldValidatePath("/api/users", [], ["/health"])).toBe(true);
    });

    it("should apply includePaths after excludePaths", () => {
      expect(shouldValidatePath("/api/users", ["/api/"], [])).toBe(true);
      expect(shouldValidatePath("/health", ["/api/"], [])).toBe(false);
    });
  });
});

describe("Metrics", () => {
  beforeEach(() => {
    resetMetrics();
  });

  describe("recordValidationMetrics", () => {
    it("should record request validations", () => {
      recordValidationMetrics("request", { valid: true });
      recordValidationMetrics("request", { valid: false, errors: ["error"] });

      const metrics = getMetrics();
      expect(metrics.requestValidations).toBe(2);
      expect(metrics.requestValidationsPassed).toBe(1);
      expect(metrics.requestValidationsFailed).toBe(1);
    });

    it("should record response validations", () => {
      recordValidationMetrics("response", { valid: true });
      recordValidationMetrics("response", { valid: true });
      recordValidationMetrics("response", { valid: false, errors: ["error"] });

      const metrics = getMetrics();
      expect(metrics.responseValidations).toBe(3);
      expect(metrics.responseValidationsPassed).toBe(2);
      expect(metrics.responseValidationsFailed).toBe(1);
    });
  });

  describe("recordRejection", () => {
    it("should record rejections", () => {
      recordRejection();
      recordRejection();

      const metrics = getMetrics();
      expect(metrics.requestsRejected).toBe(2);
    });
  });

  describe("resetMetrics", () => {
    it("should reset all metrics to zero", () => {
      recordValidationMetrics("request", { valid: true });
      recordValidationMetrics("response", { valid: false, errors: [] });
      recordRejection();

      resetMetrics();

      const metrics = getMetrics();
      expect(metrics.requestValidations).toBe(0);
      expect(metrics.responseValidations).toBe(0);
      expect(metrics.requestsRejected).toBe(0);
    });
  });
});
