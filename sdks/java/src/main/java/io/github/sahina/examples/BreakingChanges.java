package io.github.sahina.examples;

import io.github.sahina.sdk.BreakingChange;
import io.github.sahina.sdk.CompareResult;
import io.github.sahina.sdk.ContractValidator;

import java.io.IOException;
import java.nio.file.Path;
import java.nio.file.Paths;

/**
 * Breaking Change Detection Example
 *
 * This example demonstrates how to use the CVT SDK to detect breaking changes
 * between two versions of an OpenAPI schema. This is useful for:
 * - CI/CD pipelines to prevent breaking API changes from being deployed
 * - API governance to ensure backward compatibility
 * - Schema evolution tracking
 *
 * Prerequisites:
 * - CVT server running on localhost:50052
 * - Run: make up (from project root)
 *
 * Usage:
 * - mvn compile exec:java -Dexec.mainClass="io.github.sahina.examples.BreakingChanges"
 */
public class BreakingChanges {
    private static final String SCHEMA_ID = "petstore-api";

    public static void main(String[] args) {
        System.out.println("=== CVT Breaking Change Detection Example ===\n");

        // Resolve schema paths relative to project root
        Path projectRoot = Paths.get(System.getProperty("user.dir"));
        Path sharedDir = projectRoot.resolve("../shared");
        String schemaV1Path = sharedDir.resolve("openapi-v1.json").toString();
        String schemaV2Path = sharedDir.resolve("openapi-v2-breaking.json").toString();

        try (ContractValidator validator = new ContractValidator("localhost:50052")) {
            // Step 1: Register schema v1.0.0
            System.out.println("Step 1: Registering schema v1.0.0...");
            validator.registerSchemaWithVersion(SCHEMA_ID, schemaV1Path, "1.0.0");
            System.out.println("        Schema v1.0.0 registered successfully.\n");

            // Step 2: Register schema v2.0.0 (with breaking changes)
            System.out.println("Step 2: Registering schema v2.0.0...");
            validator.registerSchemaWithVersion(SCHEMA_ID, schemaV2Path, "2.0.0");
            System.out.println("        Schema v2.0.0 registered successfully.\n");

            // Step 3: Compare the two versions
            System.out.println("Step 3: Comparing schema versions 1.0.0 and 2.0.0...");
            CompareResult result = validator.compareSchemas(SCHEMA_ID, "1.0.0", "2.0.0");

            // Display the results
            logCompareResult(result);

            // Step 4: Demonstrate CI/CD integration pattern
            System.out.println("\n--- CI/CD Integration Example ---");
            if (!result.isCompatible()) {
                System.out.println("In a CI/CD pipeline, you would fail the build here:");
                System.out.println("  System.exit(1); // Fail build due to breaking changes");
                System.out.println("\nOr create a report for review:");
                for (BreakingChange change : result.getBreakingChanges()) {
                    System.out.printf("  - [%s] %s%n", change.getType(), change.getDescription());
                }
            } else {
                System.out.println("Schema changes are backward compatible. Safe to deploy!");
            }

        } catch (IOException e) {
            System.err.println("Error: " + e.getMessage());
            System.exit(1);
        }

        System.out.println("\nValidator closed.");
    }

    private static void logCompareResult(CompareResult result) {
        System.out.println();
        System.out.println("=".repeat(60));

        if (result.isCompatible()) {
            System.out.println("RESULT: COMPATIBLE");
            System.out.println("No breaking changes detected between schema versions.");
        } else {
            System.out.println("RESULT: INCOMPATIBLE");
            System.out.printf("%nBreaking changes detected: %d%n", result.getBreakingChangeCount());
            System.out.println("-".repeat(60));

            int index = 0;
            for (BreakingChange change : result.getBreakingChanges()) {
                System.out.println(formatBreakingChange(change, index++));
                System.out.println();
            }
        }

        System.out.println("=".repeat(60));
    }

    private static String formatBreakingChange(BreakingChange change, int index) {
        StringBuilder sb = new StringBuilder();
        sb.append(String.format("%d. %s%n", index + 1, change.getType()));
        sb.append(String.format("   %s", change.getDescription()));

        if (change.getPath() != null && !change.getPath().isEmpty()) {
            sb.append(String.format("%n   Path: %s %s", change.getMethod(), change.getPath()));
        }

        if (change.getOldValue() != null && !change.getOldValue().isEmpty()
                && change.getNewValue() != null && !change.getNewValue().isEmpty()) {
            sb.append(String.format("%n   Changed: \"%s\" -> \"%s\"", change.getOldValue(), change.getNewValue()));
        }

        return sb.toString();
    }
}
