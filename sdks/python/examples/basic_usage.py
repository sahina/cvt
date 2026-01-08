#!/usr/bin/env python3
"""
Basic usage example for the ContractValidator SDK

This demonstrates how to register a schema and validate API interactions

Schema Registration Options:
1. Local file: register_schema('id', '/path/to/schema.yaml')
2. URL: register_schema('id', 'https://api.example.com/openapi.json')

The SDK automatically detects the source type and handles accordingly.
"""

from shared import (
    ContractValidator,
    OPENAPI_SCHEMA_PATH,
    log_validation_result,
)


def main():
    """Run basic usage examples."""
    print("🚀 ContractValidator Basic Usage Example\n")

    # Initialize the validator
    validator = ContractValidator()

    try:
        # Register the OpenAPI schema from local file
        print(f"📋 Registering schema from local file: {OPENAPI_SCHEMA_PATH}")
        validator.register_schema("sample-schema", OPENAPI_SCHEMA_PATH)
        print("✅ Schema registered successfully\n")

        print("📝 Note: You can also register schemas from URLs:")
        print(
            "   validator.register_schema('id', 'https://api.example.com/openapi.json')\n"
        )

        # Example 1: Valid pet creation
        print("🔍 Example 1: Validating successful pet creation")
        pet_request = {
            "method": "POST",
            "path": "/pet",
            "headers": {"content-type": "application/json"},
            "body": {
                "name": "Fluffy",
                "photoUrls": ["http://example.com/photo1.jpg"],
                "status": "available",
            },
        }

        pet_response = {
            "status_code": 405,  # Petstore API response for successful POST
        }

        valid_result = validator.validate(pet_request, pet_response)
        log_validation_result("Pet Creation", valid_result)

        # Example 2: Valid user creation
        print("🔍 Example 2: Validating user creation")
        user_request = {
            "method": "POST",
            "path": "/user",
            "headers": {"content-type": "application/json"},
            "body": {
                "username": "alice",
                "firstName": "Alice",
                "lastName": "Smith",
                "email": "alice@example.com",
                "password": "password123",
                "phone": "123-456-7890",
            },
        }

        user_response = {
            "status_code": 200,  # Default response for user creation
        }

        user_result = validator.validate(user_request, user_response)
        log_validation_result("User Creation", user_result)

        # Example 3: Invalid pet creation (missing required fields)
        print("🔍 Example 3: Validating invalid pet creation (missing required fields)")
        invalid_result = validator.validate(
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

        log_validation_result("Invalid Pet Creation", invalid_result)

        # Example 4: GET pet by ID
        print("🔍 Example 4: Validating GET pet by ID")
        get_result = validator.validate(
            request={
                "method": "GET",
                "path": "/pet/123",
                "headers": {"api_key": "special-key"},
            },
            response={
                "status_code": 200,
                "headers": {"content-type": "application/json"},
                "body": {
                    "id": 123,
                    "name": "Fluffy",
                    "photoUrls": ["http://example.com/photo1.jpg"],
                    "status": "available",
                },
            },
        )

        log_validation_result("GET Pet by ID", get_result)

        print("🎉 All examples completed!")

    finally:
        # Clean up
        validator.close()


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"❌ Example failed: {error}")
        exit(1)
