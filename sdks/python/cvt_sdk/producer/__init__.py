"""
Producer validation module for CVT.

This module provides server-side HTTP middleware for validating
incoming requests and outgoing responses against OpenAPI schemas.

Example:
    >>> from cvt_sdk import ContractValidator
    >>> from cvt_sdk.producer import ProducerConfig, ValidationMode
    >>> from cvt_sdk.producer.adapters import ASGIMiddleware
    >>>
    >>> # FastAPI example
    >>> from fastapi import FastAPI
    >>> app = FastAPI()
    >>>
    >>> validator = ContractValidator()
    >>> validator.register_schema("my-api", "./openapi.json")
    >>>
    >>> config = ProducerConfig(
    ...     schema_id="my-api",
    ...     validator=validator,
    ...     mode=ValidationMode.STRICT,
    ... )
    >>> app.add_middleware(ASGIMiddleware, config=config)

Example (Flask):
    >>> from flask import Flask
    >>> from cvt_sdk import ContractValidator
    >>> from cvt_sdk.producer import ProducerConfig, ValidationMode
    >>> from cvt_sdk.producer.adapters import WSGIMiddleware
    >>>
    >>> app = Flask(__name__)
    >>>
    >>> validator = ContractValidator()
    >>> validator.register_schema("my-api", "./openapi.json")
    >>>
    >>> config = ProducerConfig(
    ...     schema_id="my-api",
    ...     validator=validator,
    ...     mode=ValidationMode.WARN,
    ... )
    >>> app.wsgi_app = WSGIMiddleware(app.wsgi_app, config)
"""

from .config import (
    Interaction,
    PathFilter,
    ProducerConfig,
    ValidationMode,
    ValidationResult,
    Validator,
    matches_path_filter,
    should_validate_path,
)
from .producer import (
    Producer,
    get_metrics,
    record_rejection,
    record_validation_metrics,
    reset_metrics,
)
from .testing import (
    EndpointTester,
    MockRequest,
    MockResponse,
    ProducerTestConfig,
    ProducerTestKit,
    ProducerValidationResult,
    TLSOptions,
    # Backwards compatibility aliases
    TestConfig,
    TestRequestContext,
    TestResponseData,
    TestValidationResult,
)

__all__ = [
    # Types
    "Interaction",
    "PathFilter",
    "ProducerConfig",
    "ValidationMode",
    "ValidationResult",
    "Validator",
    # Functions
    "matches_path_filter",
    "should_validate_path",
    # Core
    "Producer",
    "get_metrics",
    "record_rejection",
    "record_validation_metrics",
    "reset_metrics",
    # Testing
    "EndpointTester",
    "MockRequest",
    "MockResponse",
    "ProducerTestConfig",
    "ProducerTestKit",
    "ProducerValidationResult",
    "TLSOptions",
    # Backwards compatibility aliases
    "TestConfig",
    "TestRequestContext",
    "TestResponseData",
    "TestValidationResult",
]
