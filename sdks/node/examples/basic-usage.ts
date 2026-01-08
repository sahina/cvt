import {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
  Pet,
  User,
  ApiResponse,
  OPENAPI_SCHEMA_PATH,
} from "./shared";

/**
 * Basic usage example for the ContractValidator SDK
 * This demonstrates how to register a schema and validate API interactions
 *
 * Schema Registration Options:
 * 1. Local file: registerSchema('id', '/path/to/schema.yaml')
 * 2. URL: registerSchema('id', 'https://api.example.com/openapi.json')
 *
 * The SDK automatically detects the source type and handles accordingly.
 */
async function basicUsageExample() {
  console.log("🚀 ContractValidator Basic Usage Example\n");

  // Initialize the validator
  const validator = new ContractValidator();

  // Register the OpenAPI schema from local file
  console.log("📋 Registering schema from local file:", OPENAPI_SCHEMA_PATH);
  await validator.registerSchema("sample-schema", OPENAPI_SCHEMA_PATH);
  console.log("✅ Schema registered successfully\n");

  console.log("📝 Note: You can also register schemas from URLs:");
  console.log("   await validator.registerSchema('id', 'https://api.example.com/openapi.json')\n");

  // Example 1: Valid pet creation
  console.log("🔍 Example 1: Validating successful pet creation");
  const petRequest: ValidationRequest<Pet> = {
    method: "POST",
    path: "/pet",
    headers: { "content-type": "application/json" },
    body: {
      name: "Fluffy",
      photoUrls: ["http://example.com/photo1.jpg"],
      status: "available",
    },
  };

  const petResponse: ValidationResponse<ApiResponse> = {
    statusCode: 405, // Petstore API response for successful POST
  };

  const validResult = await validator.validate<Pet, ApiResponse>(
    petRequest,
    petResponse
  );

  console.log("Result:", validResult.valid ? "✅ Valid" : "❌ Invalid");
  if (!validResult.valid) {
    console.log("Errors:", validResult.errors);
  }
  console.log();

  // Example 2: Valid user creation
  console.log("🔍 Example 2: Validating user creation");
  const userRequest: ValidationRequest<User> = {
    method: "POST",
    path: "/user",
    headers: { "content-type": "application/json" },
    body: {
      username: "alice",
      firstName: "Alice",
      lastName: "Smith",
      email: "alice@example.com",
      password: "password123",
      phone: "123-456-7890",
    },
  };

  const userResponse: ValidationResponse = {
    statusCode: 200, // Default response for user creation
  };

  const userResult = await validator.validate<User>(userRequest, userResponse);

  console.log("Result:", userResult.valid ? "✅ Valid" : "❌ Invalid");
  if (!userResult.valid) {
    console.log("Errors:", userResult.errors);
  }
  console.log();

  // Example 3: Invalid pet creation (missing required fields)
  // Note: You can also use the validator without explicit types (generics default to 'any')
  console.log(
    "🔍 Example 3: Validating invalid pet creation (missing required fields) - Untyped Usage"
  );
  const invalidResult = await validator.validate(
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

  console.log("Result:", invalidResult.valid ? "✅ Valid" : "❌ Invalid");
  if (!invalidResult.valid) {
    console.log("Errors:", invalidResult.errors);
  }
  console.log();

  // Example 4: GET pet by ID
  console.log("🔍 Example 4: Validating GET pet by ID");
  const getResult = await validator.validate(
    {
      method: "GET",
      path: "/pet/123",
      headers: { api_key: "special-key" },
    },
    {
      statusCode: 200,
      headers: { "content-type": "application/json" },
      body: {
        id: 123,
        name: "Fluffy",
        photoUrls: ["http://example.com/photo1.jpg"],
        status: "available",
      },
    }
  );

  console.log("Result:", getResult.valid ? "✅ Valid" : "❌ Invalid");
  console.log();

  console.log("🎉 All examples completed!");
}

// Run the example
if (require.main === module) {
  basicUsageExample().catch((error) => {
    console.error("❌ Example failed:", error);
    process.exit(1);
  });
}
