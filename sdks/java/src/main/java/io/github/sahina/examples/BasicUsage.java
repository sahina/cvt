package io.github.sahina.examples;

import io.github.sahina.sdk.ContractValidator;
import io.github.sahina.sdk.ValidationRequest;
import io.github.sahina.sdk.ValidationResponse;
import io.github.sahina.sdk.ValidationResult;

import java.io.IOException;
import java.nio.file.Paths;

/**
 * Basic usage example for the CVT Java SDK.
 *
 * This example demonstrates:
 * - Initializing the validator
 * - Registering a schema from a local file
 * - Validating successful and failed requests
 * - Error handling
 *
 * Prerequisites: CVT server must be running (make up from repo root)
 */
public class BasicUsage {
    public static void main(String[] args) {
        System.out.println("🚀 ContractValidator Basic Usage Example\n");

        // Get schema path
        String schemaPath = getSchemaPath();
        System.out.println("📋 Using schema: " + schemaPath + "\n");

        // Initialize validator with try-with-resources for automatic cleanup
        try (ContractValidator validator = new ContractValidator()) {
            System.out.println("✅ Validator initialized\n");

            // Register the OpenAPI schema
            System.out.println("📋 Registering OpenAPI schema...");
            validator.registerSchema("basic-schema", schemaPath);
            System.out.println("✅ Schema registered successfully\n");

            System.out.println("=" .repeat(70));
            System.out.println("BASIC VALIDATION EXAMPLES");
            System.out.println("=".repeat(70));
            System.out.println();

            // Example 1: Valid pet creation
            example1ValidPetCreation(validator);

            // Example 2: Valid user creation
            example2ValidUserCreation(validator);

            // Example 3: Invalid pet (missing required fields)
            example3InvalidPet(validator);

            // Example 4: GET request
            example4GetRequest(validator);

            System.out.println("🎉 All examples completed!");

        } catch (IOException e) {
            System.err.println("❌ Error: " + e.getMessage());
            e.printStackTrace();
        }
    }

    private static void example1ValidPetCreation(ContractValidator validator) {
        System.out.println("🔍 Example 1: Valid pet creation");
        System.out.println("   Demonstrates: Successful validation\n");

        ValidationRequest request = ValidationRequest.builder()
                .method("POST")
                .path("/pet")
                .header("content-type", "application/json")
                .body("{\"name\":\"Fluffy\",\"photoUrls\":[\"http://example.com/photo1.jpg\"],\"status\":\"available\"}")
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(405)
                .build();

        ValidationResult result = validator.validate(request, response);
        printResult("Valid Pet Creation", result);
    }

    private static void example2ValidUserCreation(ContractValidator validator) {
        System.out.println("🔍 Example 2: Valid user creation");
        System.out.println("   Demonstrates: Multiple field validation\n");

        ValidationRequest request = ValidationRequest.builder()
                .method("POST")
                .path("/user")
                .header("content-type", "application/json")
                .body("{\"username\":\"alice\",\"firstName\":\"Alice\",\"lastName\":\"Smith\"," +
                      "\"email\":\"alice@example.com\",\"password\":\"password123\"," +
                      "\"phone\":\"123-456-7890\"}")
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(200)
                .build();

        ValidationResult result = validator.validate(request, response);
        printResult("Valid User Creation", result);
    }

    private static void example3InvalidPet(ContractValidator validator) {
        System.out.println("🔍 Example 3: Invalid pet (missing required fields)");
        System.out.println("   Demonstrates: Validation failure with detailed errors\n");

        ValidationRequest request = ValidationRequest.builder()
                .method("POST")
                .path("/pet")
                .header("content-type", "application/json")
                .body("{\"status\":\"available\"}")  // Missing 'name' and 'photoUrls'
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(405)
                .build();

        ValidationResult result = validator.validate(request, response);
        printResult("Invalid Pet (Missing Fields)", result);
    }

    private static void example4GetRequest(ContractValidator validator) {
        System.out.println("🔍 Example 4: GET request with response body");
        System.out.println("   Demonstrates: Response validation\n");

        ValidationRequest request = ValidationRequest.builder()
                .method("GET")
                .path("/pet/123")
                .header("api_key", "special-key")
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(200)
                .header("content-type", "application/json")
                .body("{\"id\":123,\"name\":\"Fluffy\"," +
                      "\"photoUrls\":[\"http://example.com/photo1.jpg\"]," +
                      "\"status\":\"available\"}")
                .build();

        ValidationResult result = validator.validate(request, response);
        printResult("GET Pet by ID", result);
    }

    private static void printResult(String testName, ValidationResult result) {
        System.out.println("\n" + testName);
        if (result.isValid()) {
            System.out.println("Result: ✅ Valid");
        } else {
            System.out.println("Result: ❌ Invalid");
            if (!result.getErrors().isEmpty()) {
                System.out.println("Errors: " + result.getErrors());
            }
        }
        System.out.println();
    }

    private static String getSchemaPath() {
        // Schema is located at sdks/shared/openapi.json
        String currentDir = Paths.get("").toAbsolutePath().toString();
        return Paths.get(currentDir, "..", "shared", "openapi.json")
                .toAbsolutePath().normalize().toString();
    }
}
