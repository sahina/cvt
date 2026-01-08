import * as path from "path";

// Re-export SDK types for convenience
export {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
  BreakingChange,
  CompareResult,
} from "../src/index";

/**
 * Common interfaces for Petstore API
 * These match the schemas in both OpenAPI 3.0 and Swagger 2.0 specs
 */

export interface Pet {
  id?: number;
  name: string;
  photoUrls: string[];
  status?: "available" | "pending" | "sold";
  category?: Category;
  tags?: Tag[];
}

export interface User {
  id?: number;
  username: string;
  firstName?: string;
  lastName?: string;
  email?: string;
  password?: string;
  phone?: string;
  userStatus?: number;
}

export interface ApiResponse {
  code?: number;
  type?: string;
  message?: string;
}

export interface Category {
  id?: number;
  name?: string;
}

export interface Tag {
  id?: number;
  name?: string;
}

export interface Order {
  id?: number;
  petId?: number;
  quantity?: number;
  shipDate?: string;
  status?: "placed" | "approved" | "delivered";
  complete?: boolean;
}

/**
 * Schema path constants
 */
export const OPENAPI_SCHEMA_PATH = path.resolve(
  __dirname,
  "../../shared/openapi.json"
);
export const SWAGGER_V2_SCHEMA_PATH = path.resolve(
  __dirname,
  "../../shared/swagger.json"
);
export const OPENAPI_V1_SCHEMA_PATH = path.resolve(
  __dirname,
  "../../shared/openapi-v1.json"
);
export const OPENAPI_V2_BREAKING_SCHEMA_PATH = path.resolve(
  __dirname,
  "../../shared/openapi-v2-breaking.json"
);

/**
 * Helper function to log validation results in a consistent format
 */
export function logValidationResult(
  testName: string,
  result: { valid: boolean; errors?: any[] }
): void {
  console.log(`Result: ${result.valid ? "✅ Valid" : "❌ Invalid"}`);
  if (!result.valid && result.errors) {
    console.log("Errors:", result.errors);
  }
  console.log();
}

/**
 * Test data factory functions
 */

export function createSamplePet(overrides?: Partial<Pet>): Pet {
  return {
    name: "Fluffy",
    photoUrls: ["http://example.com/photo1.jpg"],
    status: "available",
    ...overrides,
  };
}

export function createSampleUser(overrides?: Partial<User>): User {
  return {
    username: "alice",
    firstName: "Alice",
    lastName: "Smith",
    email: "alice@example.com",
    password: "password123",
    phone: "123-456-7890",
    ...overrides,
  };
}

export function createSampleOrder(overrides?: Partial<Order>): Order {
  return {
    petId: 1,
    quantity: 1,
    shipDate: new Date().toISOString(),
    status: "placed",
    complete: false,
    ...overrides,
  };
}

/**
 * Helper function to log breaking changes in a consistent format
 */
export function logBreakingChanges(result: CompareResult): void {
  if (result.compatible) {
    console.log("No breaking changes detected.");
    return;
  }

  console.log(`Breaking changes detected: ${result.breakingChanges.length}`);
  console.log("-".repeat(50));

  result.breakingChanges.forEach((change: BreakingChange, index: number) => {
    console.log(`${index + 1}. [${change.type}]`);
    console.log(`   ${change.description}`);
    if (change.path) {
      console.log(`   Path: ${change.method} ${change.path}`);
    }
    console.log();
  });
}
