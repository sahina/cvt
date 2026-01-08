#!/usr/bin/env python3
"""
Breaking Change Detection Example

This example demonstrates how to use the CVT SDK to detect breaking changes
between two versions of an OpenAPI schema. This is useful for:
- CI/CD pipelines to prevent breaking API changes from being deployed
- API governance to ensure backward compatibility
- Schema evolution tracking

Prerequisites:
- CVT server running on localhost:50052
- Run: make up (from project root)

Usage:
- uv run python examples/breaking_changes.py
"""

from pathlib import Path

from cvt_sdk import ContractValidator, BreakingChange, CompareResult


# Schema paths - v1 is the "old" version, v2 has breaking changes
SHARED_DIR = Path(__file__).parent.parent.parent / "shared"
SCHEMA_V1_PATH = SHARED_DIR / "openapi-v1.json"
SCHEMA_V2_PATH = SHARED_DIR / "openapi-v2-breaking.json"

SCHEMA_ID = "petstore-api"


def format_breaking_change(change: BreakingChange, index: int) -> str:
    """Formats a breaking change for display."""
    lines = [
        f"{index + 1}. {change['type']}",
        f"   {change['description']}",
    ]

    if change.get("path"):
        lines.append(f"   Path: {change['method']} {change['path']}")

    if change.get("old_value") and change.get("new_value"):
        lines.append(f'   Changed: "{change["old_value"]}" -> "{change["new_value"]}"')

    return "\n".join(lines)


def log_compare_result(result: CompareResult) -> None:
    """Logs the comparison result in a formatted way."""
    print("\n" + "=" * 60)

    if result["compatible"]:
        print("RESULT: COMPATIBLE")
        print("No breaking changes detected between schema versions.")
    else:
        print("RESULT: INCOMPATIBLE")
        print(f"\nBreaking changes detected: {len(result['breaking_changes'])}")
        print("-" * 60)

        for index, change in enumerate(result["breaking_changes"]):
            print(format_breaking_change(change, index))
            print()

    print("=" * 60)


def main() -> None:
    print("=== CVT Breaking Change Detection Example ===\n")

    validator = ContractValidator("localhost:50052")

    try:
        # Step 1: Register schema v1.0.0
        print("Step 1: Registering schema v1.0.0...")
        validator.register_schema_with_version(SCHEMA_ID, str(SCHEMA_V1_PATH), "1.0.0")
        print("        Schema v1.0.0 registered successfully.\n")

        # Step 2: Register schema v2.0.0 (with breaking changes)
        print("Step 2: Registering schema v2.0.0...")
        validator.register_schema_with_version(SCHEMA_ID, str(SCHEMA_V2_PATH), "2.0.0")
        print("        Schema v2.0.0 registered successfully.\n")

        # Step 3: Compare the two versions
        print("Step 3: Comparing schema versions 1.0.0 and 2.0.0...")
        result = validator.compare_schemas(SCHEMA_ID, "1.0.0", "2.0.0")

        # Display the results
        log_compare_result(result)

        # Step 4: Demonstrate CI/CD integration pattern
        print("\n--- CI/CD Integration Example ---")
        if not result["compatible"]:
            print("In a CI/CD pipeline, you would fail the build here:")
            print("  sys.exit(1)  # Fail build due to breaking changes")
            print("\nOr create a report for review:")
            for change in result["breaking_changes"]:
                print(f"  - [{change['type']}] {change['description']}")
        else:
            print("Schema changes are backward compatible. Safe to deploy!")

    except Exception as e:
        print(f"Error: {e}")
        raise
    finally:
        validator.close()
        print("\nValidator closed.")


if __name__ == "__main__":
    main()
