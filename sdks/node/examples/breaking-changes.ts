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
 * - npx ts-node examples/breaking-changes.ts
 */

import { ContractValidator, BreakingChange, CompareResult } from "../src";
import * as path from "path";

// Schema paths - v1 is the "old" version, v2 has breaking changes
const SCHEMA_V1_PATH = path.resolve(__dirname, "../../shared/openapi-v1.json");
const SCHEMA_V2_PATH = path.resolve(__dirname, "../../shared/openapi-v2-breaking.json");

const SCHEMA_ID = "petstore-api";

/**
 * Formats a breaking change for display.
 */
function formatBreakingChange(change: BreakingChange, index: number): string {
  const lines = [
    `${index + 1}. ${change.type}`,
    `   ${change.description}`,
  ];

  if (change.path) {
    lines.push(`   Path: ${change.method} ${change.path}`);
  }

  if (change.oldValue && change.newValue) {
    lines.push(`   Changed: "${change.oldValue}" -> "${change.newValue}"`);
  }

  return lines.join("\n");
}

/**
 * Logs the comparison result in a formatted way.
 */
function logCompareResult(result: CompareResult): void {
  console.log("\n" + "=".repeat(60));

  if (result.compatible) {
    console.log("RESULT: COMPATIBLE");
    console.log("No breaking changes detected between schema versions.");
  } else {
    console.log("RESULT: INCOMPATIBLE");
    console.log(`\nBreaking changes detected: ${result.breakingChanges.length}`);
    console.log("-".repeat(60));

    result.breakingChanges.forEach((change, index) => {
      console.log(formatBreakingChange(change, index));
      console.log();
    });
  }

  console.log("=".repeat(60));
}

async function main() {
  console.log("=== CVT Breaking Change Detection Example ===\n");

  const validator = new ContractValidator("localhost:50052");

  try {
    // Step 1: Register schema v1.0.0
    console.log("Step 1: Registering schema v1.0.0...");
    await validator.registerSchemaWithVersion(SCHEMA_ID, SCHEMA_V1_PATH, "1.0.0");
    console.log("        Schema v1.0.0 registered successfully.\n");

    // Step 2: Register schema v2.0.0 (with breaking changes)
    console.log("Step 2: Registering schema v2.0.0...");
    await validator.registerSchemaWithVersion(SCHEMA_ID, SCHEMA_V2_PATH, "2.0.0");
    console.log("        Schema v2.0.0 registered successfully.\n");

    // Step 3: Compare the two versions
    console.log("Step 3: Comparing schema versions 1.0.0 and 2.0.0...");
    const result = await validator.compareSchemas(SCHEMA_ID, "1.0.0", "2.0.0");

    // Display the results
    logCompareResult(result);

    // Step 4: Demonstrate CI/CD integration pattern
    console.log("\n--- CI/CD Integration Example ---");
    if (!result.compatible) {
      console.log("In a CI/CD pipeline, you would fail the build here:");
      console.log('  process.exit(1); // Fail build due to breaking changes');
      console.log("\nOr create a report for review:");
      result.breakingChanges.forEach(change => {
        console.log(`  - [${change.type}] ${change.description}`);
      });
    } else {
      console.log("Schema changes are backward compatible. Safe to deploy!");
    }

  } catch (error) {
    console.error("Error:", error);
    process.exit(1);
  } finally {
    validator.close();
    console.log("\nValidator closed.");
  }
}

main();
