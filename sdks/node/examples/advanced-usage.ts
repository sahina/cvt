import {
  ContractValidator,
  OPENAPI_SCHEMA_PATH,
  Pet,
  User,
  createSamplePet,
  createSampleUser,
  logValidationResult,
} from "./shared";

/**
 * Advanced Usage Example - Demonstrating Additional Features
 *
 * This example demonstrates advanced validation scenarios:
 * - Nested objects (Category and Tags)
 * - Array responses
 * - Using helper functions from shared.ts
 * - Invalid request scenarios with detailed error messages
 *
 * Note on Swagger 2.0:
 * The SDK also supports Swagger 2.0 specifications. The server automatically
 * converts Swagger 2.0 to OpenAPI 3.0 internally during schema registration.
 *
 * To use Swagger 2.0:
 *   await validator.registerSchema('id', 'https://petstore.swagger.io/v2/swagger.json')
 *
 * Key differences between Swagger 2.0 and OpenAPI 3.0:
 * - Schema Definitions: "definitions" vs "components/schemas"
 * - Security: "securityDefinitions" vs "components/securitySchemes"
 * - Request/Response: "consumes"/"produces" vs "content" with media types
 */
async function advancedUsageExample() {
  console.log("🚀 ContractValidator - Advanced Usage Example\n");

  // Initialize the validator
  const validator = new ContractValidator();

  // Register the OpenAPI schema
  console.log("📋 Registering OpenAPI schema:", OPENAPI_SCHEMA_PATH);
  await validator.registerSchema("advanced-schema", OPENAPI_SCHEMA_PATH);
  console.log("✅ Schema registered successfully\n");

  console.log("=".repeat(70));
  console.log("ADVANCED VALIDATION SCENARIOS");
  console.log("=".repeat(70) + "\n");

  // Example 1: Create pet with nested objects using helper function
  console.log("🔍 Example 1: Create pet with nested objects");
  console.log("   Demonstrates: Nested object validation + helper functions\n");

  const petWithNestedObjects: Pet = createSamplePet({
    name: "Max",
    category: {
      id: 1,
      name: "Dogs",
    },
    tags: [
      { id: 1, name: "friendly" },
      { id: 2, name: "trained" },
    ],
  });

  const nestedPetResult = await validator.validate<Pet, Pet>(
    {
      method: "POST",
      path: "/pet",
      headers: { "content-type": "application/json" },
      body: petWithNestedObjects,
    },
    {
      statusCode: 405,
      headers: { "content-type": "application/json" },
      body: {
        id: 10,
        ...petWithNestedObjects,
      },
    }
  );
  logValidationResult("Create Pet with Nested Objects", nestedPetResult);

  // Example 2: Create multiple users demonstrating code reuse
  console.log("🔍 Example 2: Create multiple users with helper functions");
  console.log("   Demonstrates: Code reuse with helper functions\n");

  const users = [
    createSampleUser({ username: "alice" }),
    createSampleUser({ username: "bob", email: "bob@example.com" }),
    createSampleUser({ username: "charlie", phone: "555-1234" }),
  ];

  for (const user of users) {
    const result = await validator.validate<User>(
      {
        method: "POST",
        path: "/user",
        headers: { "content-type": "application/json" },
        body: user,
      },
      {
        statusCode: 200,
      }
    );
    console.log(`   User ${user.username}: ${result.valid ? "✅" : "❌"}`);
  }
  console.log();

  // Example 3: GET request with path parameters
  console.log("🔍 Example 3: GET request with path parameter");
  console.log("   Demonstrates: Path parameter validation\n");

  const getResult = await validator.validate<void, Pet>(
    {
      method: "GET",
      path: "/pet/123",
      headers: { api_key: "special-key" },
    },
    {
      statusCode: 200,
      headers: { "content-type": "application/json" },
      body: createSamplePet({ id: 123 }),
    }
  );
  logValidationResult("GET Pet by ID", getResult);

  console.log("=".repeat(70));
  console.log("ERROR SCENARIOS - Demonstrating Validation Failures");
  console.log("=".repeat(70) + "\n");

  // Example 4: Invalid pet - missing required fields
  console.log("🔍 Example 4: Invalid pet (missing required fields)");
  console.log(
    "   Demonstrates: Required field validation with detailed errors\n"
  );

  const invalidPetResult = await validator.validate(
    {
      method: "POST",
      path: "/pet",
      headers: { "content-type": "application/json" },
      body: {
        status: "available",
        // Missing required 'name' and 'photoUrls' fields
      },
    },
    {
      statusCode: 405,
    }
  );
  logValidationResult("Invalid Pet (Missing Fields)", invalidPetResult);

  // Example 5: Invalid pet - wrong enum value
  console.log("🔍 Example 5: Invalid pet (wrong enum value)");
  console.log("   Demonstrates: Enum validation\n");

  const invalidEnumResult = await validator.validate(
    {
      method: "POST",
      path: "/pet",
      headers: { "content-type": "application/json" },
      body: {
        name: "Fluffy",
        photoUrls: ["http://example.com/photo.jpg"],
        status: "invalid-status", // Invalid enum value (should be available/pending/sold)
      },
    },
    {
      statusCode: 405,
    }
  );
  logValidationResult("Invalid Pet (Wrong Enum)", invalidEnumResult);

  // Example 6: Invalid user - missing required field
  console.log("🔍 Example 6: Invalid user (missing username)");
  console.log("   Demonstrates: Partial object validation\n");

  const invalidUserResult = await validator.validate(
    {
      method: "POST",
      path: "/user",
      headers: { "content-type": "application/json" },
      body: {
        // Missing required 'username' field
        firstName: "John",
        lastName: "Doe",
      },
    },
    {
      statusCode: 200,
    }
  );
  logValidationResult("Invalid User (Missing Username)", invalidUserResult);

  console.log("=".repeat(70));
  console.log("🎉 All advanced examples completed!");
  console.log("=".repeat(70) + "\n");
}

// Run the example
if (require.main === module) {
  advancedUsageExample().catch((error) => {
    console.error("❌ Example failed:", error);
    process.exit(1);
  });
}
