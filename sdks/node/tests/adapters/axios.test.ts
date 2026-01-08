import axios from "axios";
import { ContractValidator, ValidationResult } from "../../src/index";
import {
  createAxiosAdapter,
  AxiosContractAdapter,
} from "../../src/adapters/axios";
import {
  shouldValidatePath,
  matchesPathFilter,
} from "../../src/adapters/types";

// Mock axios
jest.mock("axios", () => {
  const actualAxios = jest.requireActual("axios");
  return {
    ...actualAxios,
    create: jest.fn(() => {
      const instance = {
        interceptors: {
          request: {
            use: jest.fn((onFulfilled) => {
              instance._requestInterceptor = onFulfilled;
              return 0;
            }),
            eject: jest.fn(),
          },
          response: {
            use: jest.fn((onFulfilled, onRejected) => {
              instance._responseInterceptor = onFulfilled;
              instance._responseErrorInterceptor = onRejected;
              return 0;
            }),
            eject: jest.fn(),
          },
        },
        get: jest.fn(),
        post: jest.fn(),
        put: jest.fn(),
        delete: jest.fn(),
        request: jest.fn(),
        _requestInterceptor: null as any,
        _responseInterceptor: null as any,
        _responseErrorInterceptor: null as any,
        // Helper to simulate request/response flow through interceptors
        async _simulateRequest(
          config: any,
          response: any,
          shouldThrow = false,
        ) {
          // Run request interceptor
          const processedConfig = instance._requestInterceptor
            ? await instance._requestInterceptor(config)
            : config;

          // Simulate response
          const fullResponse = {
            ...response,
            config: processedConfig,
          };

          if (shouldThrow) {
            const error = new Error("Request failed") as any;
            error.response = fullResponse;
            if (instance._responseErrorInterceptor) {
              await instance._responseErrorInterceptor(error);
            }
            throw error;
          }

          // Run response interceptor
          if (instance._responseInterceptor) {
            return await instance._responseInterceptor(fullResponse);
          }
          return fullResponse;
        },
      };
      return instance;
    }),
  };
});

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

describe("AxiosContractAdapter", () => {
  let mockValidator: jest.Mocked<ContractValidator>;
  let axiosInstance: any;
  let adapter: AxiosContractAdapter;

  beforeEach(() => {
    jest.clearAllMocks();
    mockValidator = new ContractValidator() as jest.Mocked<ContractValidator>;
    axiosInstance = axios.create();
  });

  afterEach(() => {
    if (adapter) {
      adapter.detach();
    }
  });

  describe("createAxiosAdapter", () => {
    it("should create an adapter instance", () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
      });

      expect(adapter).toBeInstanceOf(AxiosContractAdapter);
    });

    it("should attach request and response interceptors", () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
      });

      expect(axiosInstance.interceptors.request.use).toHaveBeenCalledTimes(1);
      expect(axiosInstance.interceptors.response.use).toHaveBeenCalledTimes(1);
    });

    it("should use default configuration values", () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
      });

      expect(adapter.getInteractions()).toEqual([]);
    });
  });

  describe("request/response capture", () => {
    beforeEach(() => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
      });
    });

    it("should capture successful requests", async () => {
      const config = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: { id: 1, name: "Fluffy" },
        headers: { "content-type": "application/json" },
        statusText: "OK",
      };

      await axiosInstance._simulateRequest(config, response);

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.method).toBe("GET");
      expect(interactions[0].request.path).toBe("/pet/1");
      expect(interactions[0].response.statusCode).toBe(200);
    });

    it("should capture request body", async () => {
      const requestBody = {
        name: "Fluffy",
        photoUrls: ["http://example.com/photo.jpg"],
      };
      const config = {
        method: "post",
        url: "/pet",
        baseURL: "http://api.test",
        data: requestBody,
        headers: { "content-type": "application/json" },
      };
      const response = {
        status: 201,
        data: { id: 1, ...requestBody },
        headers: { "content-type": "application/json" },
        statusText: "Created",
      };

      await axiosInstance._simulateRequest(config, response);

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.body).toEqual(requestBody);
    });

    it("should capture response body", async () => {
      const responseBody = { id: 1, name: "Fluffy" };
      const config = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: responseBody,
        headers: { "content-type": "application/json" },
        statusText: "OK",
      };

      await axiosInstance._simulateRequest(config, response);

      const interactions = adapter.getInteractions();
      expect(interactions[0].response.body).toEqual(responseBody);
    });

    it("should include timestamp in captured interactions", async () => {
      const config = {
        method: "get",
        url: "/test",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: {},
        headers: {},
        statusText: "OK",
      };

      const beforeTime = new Date();
      await axiosInstance._simulateRequest(config, response);
      const afterTime = new Date();

      const interactions = adapter.getInteractions();
      expect(interactions[0].timestamp.getTime()).toBeGreaterThanOrEqual(
        beforeTime.getTime(),
      );
      expect(interactions[0].timestamp.getTime()).toBeLessThanOrEqual(
        afterTime.getTime(),
      );
    });
  });

  describe("automatic validation", () => {
    it("should validate requests when autoValidate is true", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: true,
        onValidationFailure: () => {},
      });

      const config = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: { id: 1 },
        headers: {},
        statusText: "OK",
      };

      await axiosInstance._simulateRequest(config, response);

      expect(mockValidator.validate).toHaveBeenCalledTimes(1);
      expect(mockValidator.validate).toHaveBeenCalledWith(
        expect.objectContaining({ method: "GET", path: "/pet/1" }),
        expect.objectContaining({ statusCode: 200 }),
      );
    });

    it("should store validation result in captured interaction", async () => {
      const validationResult: ValidationResult = { valid: true, errors: [] };
      mockValidator.validate = jest.fn().mockResolvedValue(validationResult);

      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: true,
      });

      const config = {
        method: "get",
        url: "/test",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: {},
        headers: {},
        statusText: "OK",
      };

      await axiosInstance._simulateRequest(config, response);

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
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: true,
        onValidationFailure: onFailure,
      });

      const config = {
        method: "get",
        url: "/test",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: {},
        headers: {},
        statusText: "OK",
      };

      await axiosInstance._simulateRequest(config, response);

      expect(onFailure).toHaveBeenCalledTimes(1);
      expect(onFailure).toHaveBeenCalledWith(
        failedResult,
        expect.anything(),
        expect.anything(),
      );
    });

    it("should throw by default when validation fails", async () => {
      const failedResult: ValidationResult = {
        valid: false,
        errors: ["Missing required field"],
      };
      mockValidator.validate = jest.fn().mockResolvedValue(failedResult);

      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: true,
      });

      const config = {
        method: "get",
        url: "/test",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: {},
        headers: {},
        statusText: "OK",
      };

      await expect(
        axiosInstance._simulateRequest(config, response),
      ).rejects.toThrow("Contract validation failed");
    });

    it("should not validate when autoValidate is false", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
      });

      const config = {
        method: "get",
        url: "/test",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: {},
        headers: {},
        statusText: "OK",
      };

      await axiosInstance._simulateRequest(config, response);

      expect(mockValidator.validate).not.toHaveBeenCalled();
      expect(adapter.getInteractions()).toHaveLength(1);
    });
  });

  describe("path filtering", () => {
    it("should exclude paths matching excludePaths string", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
        excludePaths: ["/health"],
      });

      const healthConfig = {
        method: "get",
        url: "/health",
        baseURL: "http://api.test",
        headers: {},
      };
      const petConfig = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = { status: 200, data: {}, headers: {}, statusText: "OK" };

      await axiosInstance._simulateRequest(healthConfig, response);
      await axiosInstance._simulateRequest(petConfig, response);

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should exclude paths matching excludePaths regex", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
        excludePaths: [/^\/health/, /^\/metrics/],
      });

      const healthConfig = {
        method: "get",
        url: "/health/check",
        baseURL: "http://api.test",
        headers: {},
      };
      const petConfig = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = { status: 200, data: {}, headers: {}, statusText: "OK" };

      await axiosInstance._simulateRequest(healthConfig, response);
      await axiosInstance._simulateRequest(petConfig, response);

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should only validate paths matching includePaths", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
        includePaths: [/^\/pet/],
      });

      const userConfig = {
        method: "get",
        url: "/user/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const petConfig = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = { status: 200, data: {}, headers: {}, statusText: "OK" };

      await axiosInstance._simulateRequest(userConfig, response);
      await axiosInstance._simulateRequest(petConfig, response);

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });

    it("should apply excludePaths before includePaths", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
        includePaths: [/^\/pet/],
        excludePaths: ["/pet/health"],
      });

      const petHealthConfig = {
        method: "get",
        url: "/pet/health",
        baseURL: "http://api.test",
        headers: {},
      };
      const petConfig = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = { status: 200, data: {}, headers: {}, statusText: "OK" };

      await axiosInstance._simulateRequest(petHealthConfig, response);
      await axiosInstance._simulateRequest(petConfig, response);

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].request.path).toBe("/pet/1");
    });
  });

  describe("manual validation", () => {
    it("should allow manual validation of captured interactions", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
      });

      const config = {
        method: "get",
        url: "/pet/1",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 200,
        data: { id: 1 },
        headers: {},
        statusText: "OK",
      };

      await axiosInstance._simulateRequest(config, response);

      expect(mockValidator.validate).not.toHaveBeenCalled();

      const interactions = adapter.getInteractions();
      await adapter.validateInteraction(interactions[0]);

      expect(mockValidator.validate).toHaveBeenCalledTimes(1);
    });
  });

  describe("interaction management", () => {
    beforeEach(() => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
      });
    });

    it("should return a copy of interactions", async () => {
      const config = {
        method: "get",
        url: "/test",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = { status: 200, data: {}, headers: {}, statusText: "OK" };

      await axiosInstance._simulateRequest(config, response);

      const interactions1 = adapter.getInteractions();
      const interactions2 = adapter.getInteractions();

      expect(interactions1).not.toBe(interactions2);
      expect(interactions1).toEqual(interactions2);
    });

    it("should clear interactions", async () => {
      const config = {
        method: "get",
        url: "/test",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = { status: 200, data: {}, headers: {}, statusText: "OK" };

      await axiosInstance._simulateRequest(config, response);
      expect(adapter.getInteractions()).toHaveLength(1);

      adapter.clearInteractions();
      expect(adapter.getInteractions()).toHaveLength(0);
    });
  });

  describe("error responses", () => {
    it("should capture error responses (4xx/5xx)", async () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
        autoValidate: false,
      });

      const config = {
        method: "get",
        url: "/pet/999",
        baseURL: "http://api.test",
        headers: {},
      };
      const response = {
        status: 404,
        data: { error: "Not found" },
        headers: {},
        statusText: "Not Found",
      };

      await expect(
        axiosInstance._simulateRequest(config, response, true),
      ).rejects.toThrow();

      const interactions = adapter.getInteractions();
      expect(interactions).toHaveLength(1);
      expect(interactions[0].response.statusCode).toBe(404);
    });
  });

  describe("detach", () => {
    it("should remove interceptors when detached", () => {
      adapter = createAxiosAdapter({
        axios: axiosInstance,
        validator: mockValidator,
      });

      adapter.detach();

      expect(axiosInstance.interceptors.request.eject).toHaveBeenCalled();
      expect(axiosInstance.interceptors.response.eject).toHaveBeenCalled();
    });
  });
});

describe("path filtering utilities", () => {
  describe("matchesPathFilter", () => {
    it("should match string patterns as substrings", () => {
      expect(matchesPathFilter("/api/pet/1", "/pet")).toBe(true);
      expect(matchesPathFilter("/api/user/1", "/pet")).toBe(false);
    });

    it("should match regex patterns", () => {
      expect(matchesPathFilter("/pet/1", /^\/pet/)).toBe(true);
      expect(matchesPathFilter("/user/1", /^\/pet/)).toBe(false);
    });
  });

  describe("shouldValidatePath", () => {
    it("should return true for empty filters", () => {
      expect(shouldValidatePath("/any/path", [], [])).toBe(true);
    });

    it("should return false for excluded paths", () => {
      expect(shouldValidatePath("/health", [], ["/health"])).toBe(false);
    });

    it("should return true only for included paths when includePaths is set", () => {
      expect(shouldValidatePath("/pet/1", ["/pet"], [])).toBe(true);
      expect(shouldValidatePath("/user/1", ["/pet"], [])).toBe(false);
    });

    it("should prioritize excludePaths over includePaths", () => {
      expect(shouldValidatePath("/pet/health", ["/pet"], ["/health"])).toBe(
        false,
      );
    });
  });
});
