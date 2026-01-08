package com.cvt.examples;

import com.cvt.sdk.ContractValidator;
import com.cvt.sdk.ValidationRequest;
import com.cvt.sdk.ValidationResponse;
import com.cvt.sdk.ValidationResult;

import java.io.IOException;
import java.nio.file.Paths;
import java.util.Arrays;
import java.util.List;

/**
 * Advanced usage example for the CVT Java SDK.
 *
 * This example demonstrates:
 * - Nested object validation
 * - Batch validation (multiple requests)
 * - Enum validation
 * - Invalid request scenarios with detailed error messages
 *
 * Prerequisites: CVT server must be running (make up from repo root)
 */
public class AdvancedUsage {
    public static void main(String[] args) {
        System.out.println("🚀 ContractValidator - Advanced Usage Example\n");

        String schemaPath = getSchemaPath();

        try (ContractValidator validator = new ContractValidator("localhost:50052")) {
            System.out.println("📋 Registering OpenAPI schema...");
            validator.registerSchema("advanced-schema", schemaPath);
            System.out.println("✅ Schema registered successfully\n");

            System.out.println("=".repeat(70));
            System.out.println("ADVANCED VALIDATION SCENARIOS");
            System.out.println("=".repeat(70));
            System.out.println();

            // Example 1: Nested objects
            example1NestedObjects(validator);

            // Example 2: Batch validation
            example2BatchValidation(validator);

            // Example 3: Path parameters
            example3PathParameters(validator);

            System.out.println("=".repeat(70));
            System.out.println("ERROR SCENARIOS - Demonstrating Validation Failures");
            System.out.println("=".repeat(70));
            System.out.println();

            // Example 4: Missing required fields
            example4MissingFields(validator);

            // Example 5: Invalid enum value
            example5InvalidEnum(validator);

            System.out.println("=".repeat(70));
            System.out.println("🎉 All advanced examples completed!");
            System.out.println("=".repeat(70));
            System.out.println();

        } catch (IOException e) {
            System.err.println("❌ Error: " + e.getMessage());
            e.printStackTrace();
        }
    }

    private static void example1NestedObjects(ContractValidator validator) {
        System.out.println("🔍 Example 1: Create pet with nested objects");
        System.out.println("   Demonstrates: Nested object validation\n");

        ValidationRequest request = ValidationRequest.builder()
                .method("POST")
                .path("/pet")
                .header("content-type", "application/json")
                .body("{\"name\":\"Max\",\"photoUrls\":[\"http://example.com/photo1.jpg\"]," +
                      "\"category\":{\"id\":1,\"name\":\"Dogs\"}," +
                      "\"tags\":[{\"id\":1,\"name\":\"friendly\"},{\"id\":2,\"name\":\"trained\"}]}")
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(405)
                .header("content-type", "application/json")
                .build();

        ValidationResult result = validator.validate(request, response);
        printResult("Create Pet with Nested Objects", result);
    }

    private static void example2BatchValidation(ContractValidator validator) {
        System.out.println("🔍 Example 2: Create multiple users (batch validation)");
        System.out.println("   Demonstrates: Validating multiple requests in sequence\n");

        List<String> usernames = Arrays.asList("alice", "bob", "charlie");

        for (String username : usernames) {
            ValidationRequest request = ValidationRequest.builder()
                    .method("POST")
                    .path("/user")
                    .header("content-type", "application/json")
                    .body("{\"username\":\"" + username + "\",\"firstName\":\"User\"," +
                          "\"lastName\":\"Test\",\"email\":\"" + username + "@example.com\"," +
                          "\"password\":\"password123\",\"phone\":\"123-456-7890\"}")
                    .build();

            ValidationResponse response = ValidationResponse.builder()
                    .statusCode(200)
                    .build();

            ValidationResult result = validator.validate(request, response);
            String status = result.isValid() ? "✅" : "❌";
            System.out.println("   User " + username + ": " + status);
        }
        System.out.println();
    }

    private static void example3PathParameters(ContractValidator validator) {
        System.out.println("🔍 Example 3: GET request with path parameter");
        System.out.println("   Demonstrates: Path parameter validation\n");

        ValidationRequest request = ValidationRequest.builder()
                .method("GET")
                .path("/pet/123")
                .header("api_key", "special-key")
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(200)
                .header("content-type", "application/json")
                .body("{\"id\":123,\"name\":\"Max\"," +
                      "\"photoUrls\":[\"http://example.com/photo1.jpg\"]," +
                      "\"status\":\"available\"}")
                .build();

        ValidationResult result = validator.validate(request, response);
        printResult("GET Pet by ID", result);
    }

    private static void example4MissingFields(ContractValidator validator) {
        System.out.println("🔍 Example 4: Invalid pet (missing required fields)");
        System.out.println("   Demonstrates: Required field validation with detailed errors\n");

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

    private static void example5InvalidEnum(ContractValidator validator) {
        System.out.println("🔍 Example 5: Invalid pet (wrong enum value)");
        System.out.println("   Demonstrates: Enum validation\n");

        ValidationRequest request = ValidationRequest.builder()
                .method("POST")
                .path("/pet")
                .header("content-type", "application/json")
                .body("{\"name\":\"Fluffy\"," +
                      "\"photoUrls\":[\"http://example.com/photo.jpg\"]," +
                      "\"status\":\"invalid-status\"}")  // Invalid enum value
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(405)
                .build();

        ValidationResult result = validator.validate(request, response);
        printResult("Invalid Pet (Wrong Enum)", result);
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
        String currentDir = Paths.get("").toAbsolutePath().toString();
        return Paths.get(currentDir, "..", "shared", "openapi.json")
                .toAbsolutePath().normalize().toString();
    }
}
