"""
Producer testing module for CVT.

Provides ProducerTestKit for schema compliance testing - allows producers
to validate their API responses against their OpenAPI schema without
needing real consumers.

Example with TLS and API key:
    >>> from cvt_sdk.producer import ProducerTestKit, ProducerTestConfig, TLSOptions
    >>>
    >>> # Create test kit
    >>> test_kit = ProducerTestKit(ProducerTestConfig(
    ...     schema_id="user-api",
    ...     server_address="localhost:50051",
    ...     tls=TLSOptions(enabled=True, root_cert_path="./certs/ca.crt"),
    ...     api_key="cvt-dev-key-12345",
    ... ))
    >>>
    >>> # Validate a response
    >>> result = test_kit.validate_response(
    ...     method="GET",
    ...     path="/users/123",
    ...     status_code=200,
    ...     body={"id": "123", "name": "John Doe"},
    ... )
    >>>
    >>> assert result.valid, f"Validation failed: {result.errors}"
"""

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Dict, List, Optional

import grpc

from ..proto import (
    ContractValidatorStub,
    RequestData,
    ResponseData,
    ValidateProducerRequest,
    ValidationResult as ProtoValidationResult,
)


@dataclass
class TLSOptions:
    """TLS configuration options for secure connections."""

    __test__ = False  # Prevent pytest from collecting this class

    enabled: bool = False
    """Enable TLS for the connection."""

    root_cert_path: Optional[str] = None
    """Path to the root CA certificate file for server verification."""

    cert_path: Optional[str] = None
    """Path to the client certificate file (for mTLS)."""

    key_path: Optional[str] = None
    """Path to the client private key file (for mTLS)."""


@dataclass
class ProducerTestConfig:
    """Configuration for the ProducerTestKit."""

    __test__ = False  # Prevent pytest from collecting this class

    schema_id: str
    """The schema ID to validate against (required)."""

    schema_version: Optional[str] = None
    """Optional schema version to validate against."""

    server_address: str = "localhost:50051"
    """Server address (default: "localhost:50051")."""

    api_key: Optional[str] = None
    """API key for authentication (optional)."""

    tls: Optional[TLSOptions] = None
    """TLS configuration for secure connections."""


# Backwards compatibility alias
TestConfig = ProducerTestConfig


@dataclass
class MockResponse:
    """Response data for validation."""

    __test__ = False  # Prevent pytest from collecting this class

    status_code: int
    """HTTP status code."""

    body: Any = None
    """Response body (can be any JSON-serializable type)."""

    headers: Dict[str, str] = field(default_factory=dict)
    """Response headers."""


# Backwards compatibility alias
TestResponseData = MockResponse


@dataclass
class MockRequest:
    """Request context for path parameter extraction (optional)."""

    __test__ = False  # Prevent pytest from collecting this class

    method: str = ""
    """HTTP method."""

    path: str = ""
    """Request path."""

    body: Any = None
    """Request body (can be any JSON-serializable type)."""

    headers: Dict[str, str] = field(default_factory=dict)
    """Request headers."""


# Backwards compatibility alias
TestRequestContext = MockRequest


@dataclass
class ProducerValidationResult:
    """Validation result from producer testing."""

    __test__ = False  # Prevent pytest from collecting this class

    valid: bool
    """Whether the validation passed."""

    errors: List[str]
    """Validation error messages (empty if valid)."""

    validated_against_version: Optional[str] = None
    """Schema version used for validation."""

    validated_against_hash: Optional[str] = None
    """Schema hash used for validation."""


# Backwards compatibility alias
TestValidationResult = ProducerValidationResult


class ProducerTestKit:
    """
    Enables schema compliance testing for producers.

    Allows producers to validate their API responses against their OpenAPI
    schema without needing real consumers. Use it in your test suite to ensure
    your handler output matches your contract.

    Example:
        >>> import pytest
        >>> from cvt_sdk.producer import ProducerTestKit, ProducerTestConfig
        >>>
        >>> @pytest.fixture
        ... def test_kit():
        ...     kit = ProducerTestKit(ProducerTestConfig(
        ...         schema_id="user-api",
        ...         server_address="localhost:50051",
        ...     ))
        ...     yield kit
        ...     kit.close()
        >>>
        >>> def test_get_user(test_kit):
        ...     # Call your handler
        ...     response = my_handler.get_user("123")
        ...
        ...     # Validate against schema
        ...     result = test_kit.validate_response(
        ...         method="GET",
        ...         path="/users/123",
        ...         status_code=200,
        ...         body=response,
        ...     )
        ...
        ...     assert result.valid, f"Errors: {result.errors}"
    """

    def __init__(self, config: ProducerTestConfig):
        """
        Creates a new ProducerTestKit instance.

        Args:
            config: Configuration for the test kit.

        Raises:
            ValueError: If schema_id is not provided.
        """
        if not config.schema_id:
            raise ValueError("schema_id is required")

        self._schema_id = config.schema_id
        self._schema_version = config.schema_version
        self._api_key = config.api_key

        if config.tls and config.tls.enabled:
            credentials = self._create_tls_credentials(config.tls)
            self._channel = grpc.secure_channel(config.server_address, credentials)
        else:
            self._channel = grpc.insecure_channel(config.server_address)
        self._client = ContractValidatorStub(self._channel)

    @staticmethod
    def _create_tls_credentials(tls_options: TLSOptions) -> grpc.ChannelCredentials:
        """Creates TLS credentials from the provided options."""
        root_cert: Optional[bytes] = None
        certificate_chain: Optional[bytes] = None
        private_key: Optional[bytes] = None

        if tls_options.root_cert_path:
            root_cert = Path(tls_options.root_cert_path).read_bytes()

        if tls_options.cert_path and tls_options.key_path:
            certificate_chain = Path(tls_options.cert_path).read_bytes()
            private_key = Path(tls_options.key_path).read_bytes()

        return grpc.ssl_channel_credentials(
            root_certificates=root_cert,
            private_key=private_key,
            certificate_chain=certificate_chain,
        )

    def _create_metadata(self) -> list[tuple[str, str]]:
        """Creates gRPC metadata with the API key if configured."""
        metadata = []
        if self._api_key:
            metadata.append(("x-api-key", self._api_key))
        return metadata

    @staticmethod
    def _serialize_body(body: Any) -> str:
        """Converts a body value to a JSON string."""
        if body is None:
            return ""
        if isinstance(body, str):
            return body
        return json.dumps(body)

    def validate_response(
        self,
        method: str,
        path: str,
        status_code: int,
        body: Any = None,
        headers: Optional[Dict[str, str]] = None,
        request: Optional[MockRequest] = None,
    ) -> ProducerValidationResult:
        """
        Validates a producer's response against the registered schema.

        Args:
            method: HTTP method (GET, POST, etc.).
            path: API path with actual values (e.g., /users/123).
            status_code: HTTP status code.
            body: Response body (any JSON-serializable value).
            headers: Response headers (optional).
            request: Optional request context for path parameter extraction.

        Returns:
            A ProducerValidationResult with valid=True if validation succeeds,
            or valid=False with error details if validation fails.

        Example:
            >>> result = test_kit.validate_response(
            ...     method="GET",
            ...     path="/users/123",
            ...     status_code=200,
            ...     headers={"Content-Type": "application/json"},
            ...     body={"id": "123", "name": "John Doe", "email": "john@example.com"},
            ... )
            >>>
            >>> if not result.valid:
            ...     print(f"Validation errors: {result.errors}")
        """
        # Build response data
        response_data = ResponseData(
            status_code=status_code,
            headers=headers or {},
            body=self._serialize_body(body),
        )

        # Build gRPC request
        req = ValidateProducerRequest(
            schema_id=self._schema_id,
            schema_version=self._schema_version or "",
            method=method.upper(),
            path=path,
            response=response_data,
        )

        # Add optional request context if provided
        if request:
            req.request.CopyFrom(
                RequestData(
                    method=request.method or method,
                    path=request.path or path,
                    headers=request.headers or {},
                    body=self._serialize_body(request.body),
                )
            )

        # Call server
        result: ProtoValidationResult = self._client.ValidateProducerResponse(
            req, metadata=self._create_metadata()
        )

        return ProducerValidationResult(
            valid=result.valid,
            errors=list(result.errors),
            validated_against_version=result.validated_against_version or None,
            validated_against_hash=result.validated_against_hash or None,
        )

    def validate_response_data(
        self,
        method: str,
        path: str,
        response: MockResponse,
        request: Optional[MockRequest] = None,
    ) -> ProducerValidationResult:
        """
        Validates a producer's response using a MockResponse object.

        Args:
            method: HTTP method (GET, POST, etc.).
            path: API path with actual values (e.g., /users/123).
            response: Response data to validate.
            request: Optional request context for path parameter extraction.

        Returns:
            A ProducerValidationResult.
        """
        return self.validate_response(
            method=method,
            path=path,
            status_code=response.status_code,
            body=response.body,
            headers=response.headers,
            request=request,
        )

    def validate_interaction(
        self,
        request: MockRequest,
        response: MockResponse,
    ) -> ProducerValidationResult:
        """
        Validates a full interaction (request + response) against the schema.

        This is useful when you want to validate both the request and response
        together, ensuring the complete contract is honored.

        Args:
            request: Request context with method, path, and body.
            response: Response data to validate.

        Returns:
            A ProducerValidationResult.

        Example:
            >>> result = test_kit.validate_interaction(
            ...     request=MockRequest(
            ...         method="POST",
            ...         path="/users",
            ...         body={"name": "John Doe", "email": "john@example.com"},
            ...     ),
            ...     response=MockResponse(
            ...         status_code=201,
            ...         body={"id": "123", "name": "John Doe", "email": "john@example.com"},
            ...     ),
            ... )
        """
        return self.validate_response(
            method=request.method,
            path=request.path,
            status_code=response.status_code,
            body=response.body,
            headers=response.headers,
            request=request,
        )

    def for_endpoint(self, method: str, path_pattern: str) -> "EndpointTester":
        """
        Creates a helper for testing a specific endpoint.

        This is useful when testing multiple scenarios for the same endpoint.

        Args:
            method: HTTP method (GET, POST, etc.).
            path_pattern: API path pattern (e.g., /users/{id}).

        Returns:
            An EndpointTester with a simplified validation interface.

        Example:
            >>> get_user_endpoint = test_kit.for_endpoint("GET", "/users/{id}")
            >>>
            >>> # Test valid response
            >>> result1 = get_user_endpoint.validate_response(
            ...     status_code=200,
            ...     body={"id": "123", "name": "John"},
            ...     path_values={"id": "123"},
            ... )
            >>>
            >>> # Test not found
            >>> result2 = get_user_endpoint.validate_response(
            ...     status_code=404,
            ...     body={"error": "User not found"},
            ...     path_values={"id": "999"},
            ... )
        """
        return EndpointTester(self, method, path_pattern)

    def close(self) -> None:
        """
        Closes the gRPC connection.
        Should be called when the test kit is no longer needed.
        """
        if self._channel:
            self._channel.close()


class EndpointTester:
    """Helper for testing a specific endpoint."""

    def __init__(self, test_kit: ProducerTestKit, method: str, path_pattern: str):
        """Creates an EndpointTester."""
        self._test_kit = test_kit
        self._method = method
        self._path_pattern = path_pattern

    def validate_response(
        self,
        status_code: int,
        body: Any = None,
        headers: Optional[Dict[str, str]] = None,
        path_values: Optional[Dict[str, str]] = None,
    ) -> ProducerValidationResult:
        """
        Validates a response for this endpoint.

        Args:
            status_code: HTTP status code.
            body: Response body (any JSON-serializable value).
            headers: Response headers (optional).
            path_values: Values to substitute in path parameters (optional).

        Returns:
            A ProducerValidationResult.
        """
        actual_path = self._path_pattern

        # Substitute path parameters
        if path_values:
            for key, value in path_values.items():
                actual_path = actual_path.replace(f"{{{key}}}", value)

        return self._test_kit.validate_response(
            method=self._method,
            path=actual_path,
            status_code=status_code,
            body=body,
            headers=headers,
        )
