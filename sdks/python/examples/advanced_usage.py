#!/usr/bin/env python3
"""
Advanced Usage Example - Demonstrating Additional Features

This example demonstrates advanced validation scenarios:
- Nested objects (Category and Tags)
- Array responses
- Using helper functions from shared.py
- Invalid request scenarios with detailed error messages

Note on Swagger 2.0:
The SDK also supports Swagger 2.0 specifications. The server automatically
converts Swagger 2.0 to OpenAPI 3.0 internally during schema registration.

To use Swagger 2.0:
    validator.register_schema('id', 'https://petstore.swagger.io/v2/swagger.json')

Key differences between Swagger 2.0 and OpenAPI 3.0:
- Schema Definitions: "definitions" vs "components/schemas"
- Security: "securityDefinitions" vs "components/securitySchemes"
- Request/Response: "consumes"/"produces" vs "content" with media types
"""

from shared import (
    ContractValidator,
    OPENAPI_SCHEMA_PATH,
    create_sample_pet,
    create_sample_user,
    log_validation_result,
)


def main():
    """Run advanced usage examples."""
    print("🚀 ContractValidator - Advanced Usage Example\n")

    # Initialize the validator
    validator = ContractValidator()

    try:
        # Register the OpenAPI schema
        print(f"📋 Registering OpenAPI schema: {OPENAPI_SCHEMA_PATH}")
        validator.register_schema("advanced-schema", OPENAPI_SCHEMA_PATH)
        print("✅ Schema registered successfully\n")

        print("=" * 70)
        print("ADVANCED VALIDATION SCENARIOS")
        print("=" * 70 + "\n")

        # Example 1: Create pet with nested objects using helper function
        print("🔍 Example 1: Create pet with nested objects")
        print("   Demonstrates: Nested object validation + helper functions\n")

        pet_with_nested_objects = create_sample_pet(
            name="Max",
            category={"id": 1, "name": "Dogs"},
            tags=[
                {"id": 1, "name": "friendly"},
                {"id": 2, "name": "trained"},
            ],
        )

        nested_pet_result = validator.validate(
            request={
                "method": "POST",
                "path": "/pet",
                "headers": {"content-type": "application/json"},
                "body": pet_with_nested_objects,
            },
            response={
                "status_code": 405,
                "headers": {"content-type": "application/json"},
                "body": {
                    "id": 10,
                    **pet_with_nested_objects,
                },
            },
        )
        log_validation_result("Create Pet with Nested Objects", nested_pet_result)

        # Example 2: Create multiple users demonstrating code reuse
        print("🔍 Example 2: Create multiple users with helper functions")
        print("   Demonstrates: Code reuse with helper functions\n")

        users = [
            create_sample_user(username="alice"),
            create_sample_user(username="bob", email="bob@example.com"),
            create_sample_user(username="charlie", phone="555-1234"),
        ]

        for user in users:
            result = validator.validate(
                request={
                    "method": "POST",
                    "path": "/user",
                    "headers": {"content-type": "application/json"},
                    "body": user,
                },
                response={
                    "status_code": 200,
                },
            )
            print(f"   User {user['username']}: {'✅' if result['valid'] else '❌'}")
        print()

        # Example 3: GET request with path parameters
        print("🔍 Example 3: GET request with path parameter")
        print("   Demonstrates: Path parameter validation\n")

        get_result = validator.validate(
            request={
                "method": "GET",
                "path": "/pet/123",
                "headers": {"api_key": "special-key"},
            },
            response={
                "status_code": 200,
                "headers": {"content-type": "application/json"},
                "body": create_sample_pet(id=123),
            },
        )
        log_validation_result("GET Pet by ID", get_result)

        print("=" * 70)
        print("ERROR SCENARIOS - Demonstrating Validation Failures")
        print("=" * 70 + "\n")

        # Example 4: Invalid pet - missing required fields
        print("🔍 Example 4: Invalid pet (missing required fields)")
        print("   Demonstrates: Required field validation with detailed errors\n")

        invalid_pet_result = validator.validate(
            request={
                "method": "POST",
                "path": "/pet",
                "headers": {"content-type": "application/json"},
                "body": {
                    "status": "available",
                    # Missing required 'name' and 'photoUrls' fields
                },
            },
            response={
                "status_code": 405,
            },
        )
        log_validation_result("Invalid Pet (Missing Fields)", invalid_pet_result)

        # Example 5: Invalid pet - wrong enum value
        print("🔍 Example 5: Invalid pet (wrong enum value)")
        print("   Demonstrates: Enum validation\n")

        invalid_enum_result = validator.validate(
            request={
                "method": "POST",
                "path": "/pet",
                "headers": {"content-type": "application/json"},
                "body": {
                    "name": "Fluffy",
                    "photoUrls": ["http://example.com/photo.jpg"],
                    "status": "invalid-status",  # Invalid enum value (should be available/pending/sold)
                },
            },
            response={
                "status_code": 405,
            },
        )
        log_validation_result("Invalid Pet (Wrong Enum)", invalid_enum_result)

        # Example 6: Invalid user - missing required field
        print("🔍 Example 6: Invalid user (missing username)")
        print("   Demonstrates: Partial object validation\n")

        invalid_user_result = validator.validate(
            request={
                "method": "POST",
                "path": "/user",
                "headers": {"content-type": "application/json"},
                "body": {
                    # Missing required 'username' field
                    "firstName": "John",
                    "lastName": "Doe",
                },
            },
            response={
                "status_code": 200,
            },
        )
        log_validation_result("Invalid User (Missing Username)", invalid_user_result)

        print("=" * 70)
        print("🎉 All advanced examples completed!")
        print("=" * 70 + "\n")

    finally:
        # Clean up
        validator.close()


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"❌ Example failed: {error}")
        exit(1)
