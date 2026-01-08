"""
Common utilities and test data for CVT Python SDK examples.

This module provides:
- Type definitions for Petstore API entities
- Schema path constants
- Helper functions for creating sample data
- Logging utilities for validation results
"""

from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Literal, Optional

# Re-export SDK types for convenience
from cvt_sdk import (
    ContractValidator,
    ValidationRequest,
    ValidationResponse,
    ValidationResult,
    BreakingChange,
    CompareResult,
)

__all__ = [
    "ContractValidator",
    "ValidationRequest",
    "ValidationResponse",
    "ValidationResult",
    "BreakingChange",
    "CompareResult",
    "Pet",
    "User",
    "Order",
    "Category",
    "Tag",
    "ApiResponse",
    "OPENAPI_SCHEMA_PATH",
    "SWAGGER_V2_SCHEMA_PATH",
    "OPENAPI_V1_SCHEMA_PATH",
    "OPENAPI_V2_BREAKING_SCHEMA_PATH",
    "create_sample_pet",
    "create_sample_user",
    "create_sample_order",
    "log_validation_result",
    "log_breaking_changes",
]


# Common interfaces for Petstore API
# These match the schemas in both OpenAPI 3.0 and Swagger 2.0 specs


@dataclass
class Category:
    """Pet category."""

    id: Optional[int] = None
    name: Optional[str] = None


@dataclass
class Tag:
    """Pet tag."""

    id: Optional[int] = None
    name: Optional[str] = None


@dataclass
class Pet:
    """Pet entity."""

    name: str
    photoUrls: list[str]
    id: Optional[int] = None
    status: Optional[Literal["available", "pending", "sold"]] = None
    category: Optional[Category] = None
    tags: Optional[list[Tag]] = None


@dataclass
class User:
    """User entity."""

    username: str
    id: Optional[int] = None
    firstName: Optional[str] = None
    lastName: Optional[str] = None
    email: Optional[str] = None
    password: Optional[str] = None
    phone: Optional[str] = None
    userStatus: Optional[int] = None


@dataclass
class Order:
    """Store order entity."""

    petId: Optional[int] = None
    quantity: Optional[int] = None
    shipDate: Optional[str] = None
    status: Optional[Literal["placed", "approved", "delivered"]] = None
    complete: Optional[bool] = None
    id: Optional[int] = None


@dataclass
class ApiResponse:
    """API response entity."""

    code: Optional[int] = None
    type: Optional[str] = None
    message: Optional[str] = None


# Schema path constants
EXAMPLES_DIR = Path(__file__).parent
SHARED_DIR = EXAMPLES_DIR.parent.parent / "shared"
OPENAPI_SCHEMA_PATH = str(SHARED_DIR / "openapi.json")
SWAGGER_V2_SCHEMA_PATH = str(SHARED_DIR / "swagger.json")
OPENAPI_V1_SCHEMA_PATH = str(SHARED_DIR / "openapi-v1.json")
OPENAPI_V2_BREAKING_SCHEMA_PATH = str(SHARED_DIR / "openapi-v2-breaking.json")


def log_validation_result(test_name: str, result: ValidationResult) -> None:
    """
    Helper function to log validation results in a consistent format.

    Args:
        test_name: Name of the test case
        result: Validation result from CVT
    """
    status = "✅ Valid" if result["valid"] else "❌ Invalid"
    print(f"\n{test_name}")
    print(f"Result: {status}")
    if not result["valid"] and result.get("errors"):
        print(f"Errors: {result['errors']}")
    print()


# Test data factory functions


def create_sample_pet(**overrides) -> dict:
    """
    Create a sample pet with optional field overrides.

    Args:
        **overrides: Fields to override in the sample pet

    Returns:
        A dictionary representing a pet entity
    """
    pet = {
        "name": "Fluffy",
        "photoUrls": ["http://example.com/photo1.jpg"],
        "status": "available",
    }
    pet.update(overrides)
    return pet


def create_sample_user(**overrides) -> dict:
    """
    Create a sample user with optional field overrides.

    Args:
        **overrides: Fields to override in the sample user

    Returns:
        A dictionary representing a user entity
    """
    user = {
        "username": "alice",
        "firstName": "Alice",
        "lastName": "Smith",
        "email": "alice@example.com",
        "password": "password123",
        "phone": "123-456-7890",
    }
    user.update(overrides)
    return user


def create_sample_order(**overrides) -> dict:
    """
    Create a sample order with optional field overrides.

    Args:
        **overrides: Fields to override in the sample order

    Returns:
        A dictionary representing an order entity
    """
    order = {
        "petId": 1,
        "quantity": 1,
        "shipDate": datetime.utcnow().isoformat() + "Z",
        "status": "placed",
        "complete": False,
    }
    order.update(overrides)
    return order


def log_breaking_changes(result: CompareResult) -> None:
    """
    Helper function to log breaking changes in a consistent format.

    Args:
        result: Compare result from CVT
    """
    if result["compatible"]:
        print("No breaking changes detected.")
        return

    print(f"Breaking changes detected: {len(result['breaking_changes'])}")
    print("-" * 50)

    for index, change in enumerate(result["breaking_changes"]):
        print(f"{index + 1}. [{change['type']}]")
        print(f"   {change['description']}")
        if change.get("path"):
            print(f"   Path: {change['method']} {change['path']}")
        print()
