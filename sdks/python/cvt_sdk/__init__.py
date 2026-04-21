"""CVT Python SDK for validating HTTP interactions against OpenAPI schemas."""

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Optional, TypedDict, TYPE_CHECKING

if TYPE_CHECKING:
    from .adapters.types import CapturedInteraction

from urllib.request import urlopen
from urllib.error import URLError, HTTPError

import grpc

from .proto import (
    ContractValidatorStub,
    RegisterSchemaRequest,
    GetSchemaRequest,
    InteractionRequest,
    RequestData,
    ResponseData,
    ValidationResult as ProtoValidationResult,
    CompareSchemasRequest,
    CompareSchemasResponse,
    GenerateFixtureRequest,
    GenerateFixtureResponse,
    ListEndpointsRequest,
    ListEndpointsResponse,
    OutputType,
    # Consumer Registry
    RegisterConsumerRequest,
    RegisterConsumerResponse,
    ListConsumersRequest,
    ListConsumersResponse,
    DeregisterConsumerRequest,
    DeregisterConsumerResponse,
    ProtoEndpointUsage,
    # Deployment Safety
    CanIDeployRequest,
    CanIDeployResponse,
)


@dataclass
class TLSOptions:
    """TLS configuration options for secure connections."""

    enabled: bool = False
    """Enable TLS for the connection."""

    root_cert_path: Optional[str] = None
    """Path to the root CA certificate file for server verification."""

    cert_path: Optional[str] = None
    """Path to the client certificate file (for mTLS)."""

    key_path: Optional[str] = None
    """Path to the client private key file (for mTLS)."""


@dataclass
class ContractValidatorOptions:
    """Configuration options for the ContractValidator."""

    address: str = "localhost:9550"
    """The address of the CVT gRPC server."""

    tls: Optional[TLSOptions] = None
    """TLS configuration for secure connections."""

    api_key: Optional[str] = None
    """API key for authentication."""


class ValidationRequest(TypedDict, total=False):
    """HTTP request object for validation."""

    method: str
    path: str
    headers: Dict[str, Any]
    body: Any


class ValidationResponse(TypedDict, total=False):
    """HTTP response object for validation."""

    status_code: int
    statusCode: int  # Support both naming conventions
    headers: Dict[str, Any]
    body: Any


class ValidationResult(TypedDict):
    """Validation result object."""

    valid: bool
    errors: list[str]


class BreakingChange(TypedDict):
    """Represents a breaking change detected between schema versions."""

    type: str
    """Type of breaking change (e.g., ENDPOINT_REMOVED, REQUIRED_FIELD_ADDED)."""

    path: str
    """API path affected (e.g., "/pet/{petId}")."""

    method: str
    """HTTP method affected (e.g., "DELETE")."""

    description: str
    """Human-readable description of the breaking change."""

    old_value: str
    """Previous value (for context)."""

    new_value: str
    """New value (for context)."""


class CompareResult(TypedDict):
    """Result of comparing two schema versions."""

    compatible: bool
    """True if no breaking changes were detected."""

    breaking_changes: list[BreakingChange]
    """List of breaking changes detected."""


@dataclass
class GenerateOptions:
    """Options for generating test fixtures."""

    status_code: int = 0
    """Response status code to generate (default: first successful status)."""

    use_examples: bool = True
    """Whether to use schema examples when available."""

    content_type: str = "application/json"
    """Content type for request/response."""


class GeneratedRequest(TypedDict, total=False):
    """Generated HTTP request fixture."""

    method: str
    path: str
    headers: Dict[str, str]
    body: Any


class GeneratedResponse(TypedDict, total=False):
    """Generated HTTP response fixture."""

    status_code: int
    headers: Dict[str, str]
    body: Any


class GeneratedFixture(TypedDict):
    """Complete generated fixture with request and response."""

    request: GeneratedRequest
    response: GeneratedResponse


class EndpointInfo(TypedDict):
    """Information about an API endpoint."""

    method: str
    path: str
    operation_id: Optional[str]
    summary: Optional[str]


# ============================================================================
# Consumer Registry Types
# ============================================================================


@dataclass
class EndpointUsage:
    """Describes which endpoints and fields a consumer uses."""

    method: str
    """HTTP method (GET, POST, etc.)."""

    path: str
    """API path (e.g., "/users/{id}")."""

    used_fields: Optional[list[str]] = None
    """Fields used in response (e.g., ["email", "name"])."""


class ConsumerInfo(TypedDict):
    """Information about a registered consumer."""

    consumer_id: str
    """Unique consumer identifier (e.g., "order-service")."""

    consumer_version: str
    """Consumer's version (e.g., "2.1.0")."""

    schema_id: str
    """Schema this consumer depends on."""

    schema_version: str
    """Schema version consumer was tested against."""

    environment: str
    """Environment (dev, staging, prod)."""

    registered_at: int
    """Unix timestamp of registration."""

    last_validated_at: int
    """Unix timestamp of last successful validation."""

    used_endpoints: list[EndpointUsage]
    """Which endpoints the consumer uses."""


@dataclass
class RegisterConsumerOptions:
    """Options for registering a consumer."""

    consumer_id: str
    """Unique consumer identifier (e.g., "order-service")."""

    consumer_version: str
    """Consumer's version (e.g., "2.1.0")."""

    schema_id: str
    """Schema this consumer depends on."""

    schema_version: str
    """Schema version consumer was tested against."""

    environment: str
    """Environment (dev, staging, prod)."""

    used_endpoints: Optional[list[EndpointUsage]] = None
    """Which endpoints the consumer uses."""


class ConsumerImpact(TypedDict):
    """Impact of schema changes on a specific consumer."""

    consumer_id: str
    """Consumer identifier."""

    consumer_version: str
    """Consumer version."""

    current_schema_version: str
    """Schema version consumer was tested against."""

    environment: str
    """Environment."""

    will_break: bool
    """True if consumer will be affected."""

    relevant_changes: list[BreakingChange]
    """Breaking changes that affect this consumer."""


class CanIDeployResult(TypedDict):
    """Result of can-i-deploy check."""

    safe_to_deploy: bool
    """True if safe to deploy."""

    summary: str
    """Human-readable summary."""

    breaking_changes: list[BreakingChange]
    """All breaking changes in the new version."""

    affected_consumers: list[ConsumerImpact]
    """Impact on each affected consumer."""


@dataclass
class SchemaOwnership:
    """Ownership information for a registered schema."""

    owner: str = ""
    team: str = ""
    contact_email: str = ""
    read_only: bool = False


@dataclass
class SchemaInfo:
    """Metadata describing a schema resolved via use_schema()."""

    schema_id: str
    schema_version: str
    schema_hash: str = ""
    registered_at: int = 0
    updated_at: int = 0
    openapi_version: str = ""
    endpoint_count: int = 0
    ownership: Optional[SchemaOwnership] = None


class ContractValidator:
    """
    Client for the Contract Validator Toolkit (CVT).
    Allows validating HTTP interactions against OpenAPI schemas via a gRPC service.

    Example:
        >>> validator = ContractValidator()
        >>> validator.register_schema("my-api", "./openapi.yaml")
        >>> result = validator.validate(
        ...     request={"method": "GET", "path": "/users"},
        ...     response={"status_code": 200, "body": []}
        ... )
        >>> assert result["valid"]

    Example with TLS and API key:
        >>> options = ContractValidatorOptions(
        ...     address="localhost:9550",
        ...     tls=TLSOptions(enabled=True, root_cert_path="./certs/ca.crt"),
        ...     api_key="cvt-dev-key-12345"
        ... )
        >>> validator = ContractValidator(options)
    """

    def __init__(
        self, address_or_options: str | ContractValidatorOptions = "localhost:9550"
    ):
        """
        Creates a new ContractValidator instance.

        Args:
            address_or_options: Either a server address string or a ContractValidatorOptions object.
        """
        if isinstance(address_or_options, str):
            # Legacy mode: just an address string
            address = address_or_options
            self._api_key: Optional[str] = None
            self._channel = grpc.insecure_channel(address)
        else:
            # New options mode
            options = address_or_options
            address = options.address
            self._api_key = options.api_key

            if options.tls and options.tls.enabled:
                credentials = self._create_tls_credentials(options.tls)
                self._channel = grpc.secure_channel(address, credentials)
            else:
                self._channel = grpc.insecure_channel(address)

        self._client = ContractValidatorStub(self._channel)
        self._schema_id: Optional[str] = None
        self._schema_version: Optional[str] = None

    def _create_tls_credentials(
        self, tls_options: TLSOptions
    ) -> grpc.ChannelCredentials:
        """Creates TLS credentials from the provided options."""
        root_cert = None
        private_key = None
        certificate_chain = None

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

    def register_schema(self, schema_id: str, schema_path: str) -> None:
        """
        Registers a schema for validation.

        Args:
            schema_id: A unique identifier for the schema.
            schema_path: The path to the schema file (local file path or HTTP/HTTPS URL).

        Raises:
            FileNotFoundError: If the schema file doesn't exist.
            ValueError: If schema registration fails.
            URLError: If fetching remote schema fails.
        """
        # Fetch schema content from file or URL
        if schema_path.startswith("http://") or schema_path.startswith("https://"):
            schema_content = self._fetch_schema_from_url(schema_path)
        else:
            schema_content = self._read_schema_from_file(schema_path)

        # Register schema with server
        request = RegisterSchemaRequest(
            schema_id=schema_id, schema_content=schema_content
        )
        response = self._client.RegisterSchema(
            request, metadata=self._create_metadata()
        )

        if not response.success:
            raise ValueError(f"Schema registration failed: {response.message}")

        self._schema_id = schema_id
        self._schema_version = None

    def use_schema(
        self, schema_id: str, schema_version: Optional[str] = None
    ) -> SchemaInfo:
        """
        Binds the validator to a schema already registered on the server, without
        requiring a local OpenAPI file. The schema's concrete version is resolved
        at call time; subsequent validate() calls are pinned to that version.

        Args:
            schema_id: The schema identifier to bind.
            schema_version: Optional version to pin to (default: latest registered).

        Returns:
            A SchemaInfo describing the resolved schema.

        Raises:
            ValueError: If the schema (or specific version) is not registered.

        Example:
            >>> validator = ContractValidator("localhost:9550")
            >>> info = validator.use_schema("petstore")
            >>> print(f"Bound to {info.schema_id}@{info.schema_version}")
            >>> validator.validate(request, response)
        """
        request = GetSchemaRequest(
            schema_id=schema_id,
            schema_version=schema_version or "",
        )
        response = self._client.GetSchema(request, metadata=self._create_metadata())

        if not response.found:
            suffix = f"@{schema_version}" if schema_version else ""
            raise ValueError(f"Schema '{schema_id}{suffix}' not registered on server")

        metadata = response.metadata
        resolved_version = metadata.schema_version
        self._schema_id = schema_id
        self._schema_version = resolved_version

        ownership: Optional[SchemaOwnership] = None
        if metadata.HasField("ownership"):
            ownership = SchemaOwnership(
                owner=metadata.ownership.owner,
                team=metadata.ownership.team,
                contact_email=metadata.ownership.contact_email,
                read_only=metadata.ownership.read_only,
            )

        return SchemaInfo(
            schema_id=metadata.schema_id or schema_id,
            schema_version=resolved_version,
            schema_hash=metadata.schema_hash,
            registered_at=metadata.registered_at,
            updated_at=metadata.updated_at,
            openapi_version=metadata.openapi_version,
            endpoint_count=metadata.endpoint_count,
            ownership=ownership,
        )

    def validate(
        self, request: ValidationRequest, response: ValidationResponse
    ) -> ValidationResult:
        """
        Validates an HTTP interaction (request and response) against the registered schema.

        Args:
            request: The HTTP request object containing method, path, headers, and body.
            response: The HTTP response object containing status code, headers, and body.

        Returns:
            A validation result containing 'valid' (bool) and 'errors' (list of strings).

        Raises:
            ValueError: If no schema has been registered.
        """
        if not self._schema_id:
            raise ValueError("Schema not registered. Call register_schema first.")

        # Build request data
        request_data = RequestData(
            method=request["method"],
            path=request["path"],
            headers=request.get("headers", {}),
            body=json.dumps(request["body"]) if "body" in request else "",
        )

        # Build response data - support both status_code and statusCode
        status_code = response.get("status_code") or response.get("statusCode", 0)
        response_data = ResponseData(
            status_code=status_code,
            headers=response.get("headers", {}),
            body=json.dumps(response["body"]) if "body" in response else "",
        )

        # Validate interaction
        interaction_kwargs: Dict[str, Any] = {
            "schema_id": self._schema_id,
            "request": request_data,
            "response": response_data,
        }
        if self._schema_version:
            interaction_kwargs["schema_version"] = self._schema_version
        interaction_request = InteractionRequest(**interaction_kwargs)
        result: ProtoValidationResult = self._client.ValidateInteraction(
            interaction_request, metadata=self._create_metadata()
        )

        return ValidationResult(valid=result.valid, errors=list(result.errors))

    def register_schema_with_version(
        self, schema_id: str, schema_path: str, version: str
    ) -> None:
        """
        Registers a schema with version information for comparison.

        Args:
            schema_id: A unique identifier for the schema.
            schema_path: The path to the schema file (local file path or HTTP/HTTPS URL).
            version: The semantic version of the schema (e.g., "1.0.0").

        Raises:
            FileNotFoundError: If the schema file doesn't exist.
            ValueError: If schema registration fails.
        """
        if schema_path.startswith("http://") or schema_path.startswith("https://"):
            schema_content = self._fetch_schema_from_url(schema_path)
        else:
            schema_content = self._read_schema_from_file(schema_path)

        request = RegisterSchemaRequest(
            schema_id=schema_id,
            schema_content=schema_content,
            schema_version=version,
        )
        response = self._client.RegisterSchema(
            request, metadata=self._create_metadata()
        )

        if not response.success:
            raise ValueError(f"Schema registration failed: {response.message}")

        self._schema_id = schema_id
        self._schema_version = None

    def compare_schemas(
        self, schema_id: str, old_version: str = "", new_version: str = ""
    ) -> CompareResult:
        """
        Compares two schema versions to detect breaking changes.

        Args:
            schema_id: The schema identifier to compare versions for.
            old_version: The old version to compare from (optional, defaults to previous).
            new_version: The new version to compare to (optional, defaults to latest).

        Returns:
            A CompareResult containing compatibility status and list of breaking changes.

        Example:
            >>> result = validator.compare_schemas("petstore-api", "1.0.0", "2.0.0")
            >>> if not result["compatible"]:
            ...     for change in result["breaking_changes"]:
            ...         print(f"- {change['type']}: {change['description']}")
        """
        request = CompareSchemasRequest(
            schema_id=schema_id,
            old_version=old_version,
            new_version=new_version,
        )
        response: CompareSchemasResponse = self._client.CompareSchemas(
            request, metadata=self._create_metadata()
        )

        breaking_changes: list[BreakingChange] = []
        for bc in response.breaking_changes:
            breaking_changes.append(
                BreakingChange(
                    type=str(bc.type) if bc.type else "UNKNOWN",
                    path=bc.path or "",
                    method=bc.method or "",
                    description=bc.description or "",
                    old_value=bc.old_value or "",
                    new_value=bc.new_value or "",
                )
            )

        return CompareResult(
            compatible=response.compatible,
            breaking_changes=breaking_changes,
        )

    def generate_fixture(
        self, method: str, path: str, options: Optional[GenerateOptions] = None
    ) -> GeneratedFixture:
        """
        Generates a complete test fixture (request and response) for an endpoint.

        Args:
            method: HTTP method (GET, POST, etc.)
            path: API path (e.g., /users/{id})
            options: Generation options

        Returns:
            A GeneratedFixture containing request and response data.

        Raises:
            ValueError: If no schema has been registered.

        Example:
            >>> fixture = validator.generate_fixture("POST", "/users")
            >>> print(fixture["request"]["body"])
            >>> print(fixture["response"]["body"])
        """
        if not self._schema_id:
            raise ValueError("Schema not registered. Call register_schema first.")

        opts = options or GenerateOptions()
        request = GenerateFixtureRequest(
            schema_id=self._schema_id,
            method=method.upper(),
            path=path,
            status_code=opts.status_code,
            use_examples=opts.use_examples,
            content_type=opts.content_type,
            output_type=OutputType.OUTPUT_FIXTURE,
        )
        response: GenerateFixtureResponse = self._client.GenerateFixture(
            request, metadata=self._create_metadata()
        )

        if not response.success:
            raise ValueError(f"Fixture generation failed: {response.message}")

        fixture = response.fixture
        return GeneratedFixture(
            request=GeneratedRequest(
                method=fixture.request.method,
                path=fixture.request.path,
                headers=dict(fixture.request.headers),
                body=json.loads(fixture.request.body) if fixture.request.body else None,
            ),
            response=GeneratedResponse(
                status_code=fixture.response.status_code,
                headers=dict(fixture.response.headers),
                body=json.loads(fixture.response.body)
                if fixture.response.body
                else None,
            ),
        )

    def generate_response(
        self, method: str, path: str, options: Optional[GenerateOptions] = None
    ) -> GeneratedResponse:
        """
        Generates a response fixture for an endpoint.

        Args:
            method: HTTP method (GET, POST, etc.)
            path: API path (e.g., /users/{id})
            options: Generation options

        Returns:
            A GeneratedResponse containing status code, headers, and body.

        Raises:
            ValueError: If no schema has been registered.

        Example:
            >>> response = validator.generate_response("GET", "/users/123")
            >>> expected = {"status_code": response["status_code"], "body": response["body"]}
        """
        if not self._schema_id:
            raise ValueError("Schema not registered. Call register_schema first.")

        opts = options or GenerateOptions()
        request = GenerateFixtureRequest(
            schema_id=self._schema_id,
            method=method.upper(),
            path=path,
            status_code=opts.status_code,
            use_examples=opts.use_examples,
            content_type=opts.content_type,
            output_type=OutputType.OUTPUT_RESPONSE,
        )
        response: GenerateFixtureResponse = self._client.GenerateFixture(
            request, metadata=self._create_metadata()
        )

        if not response.success:
            raise ValueError(f"Response generation failed: {response.message}")

        resp = response.response
        return GeneratedResponse(
            status_code=resp.status_code,
            headers=dict(resp.headers),
            body=json.loads(resp.body) if resp.body else None,
        )

    def generate_request_body(
        self, method: str, path: str, options: Optional[GenerateOptions] = None
    ) -> Any:
        """
        Generates a request body fixture for an endpoint.

        Args:
            method: HTTP method (typically POST, PUT, PATCH)
            path: API path (e.g., /users)
            options: Generation options

        Returns:
            The generated request body as a Python object.

        Raises:
            ValueError: If no schema has been registered.

        Example:
            >>> body = validator.generate_request_body("POST", "/users")
            >>> request = {"method": "POST", "path": "/users", "body": body}
        """
        if not self._schema_id:
            raise ValueError("Schema not registered. Call register_schema first.")

        opts = options or GenerateOptions()
        request = GenerateFixtureRequest(
            schema_id=self._schema_id,
            method=method.upper(),
            path=path,
            use_examples=opts.use_examples,
            content_type=opts.content_type,
            output_type=OutputType.OUTPUT_REQUEST,
        )
        response: GenerateFixtureResponse = self._client.GenerateFixture(
            request, metadata=self._create_metadata()
        )

        if not response.success:
            raise ValueError(f"Request body generation failed: {response.message}")

        return json.loads(response.request_body) if response.request_body else None

    def list_endpoints(self) -> list[EndpointInfo]:
        """
        Lists all endpoints available in the registered schema.

        Returns:
            A list of EndpointInfo objects.

        Raises:
            ValueError: If no schema has been registered.

        Example:
            >>> endpoints = validator.list_endpoints()
            >>> for ep in endpoints:
            ...     print(f"{ep['method']} {ep['path']}")
        """
        if not self._schema_id:
            raise ValueError("Schema not registered. Call register_schema first.")

        request = ListEndpointsRequest(schema_id=self._schema_id)
        response: ListEndpointsResponse = self._client.ListEndpoints(
            request, metadata=self._create_metadata()
        )

        return [
            EndpointInfo(
                method=ep.method,
                path=ep.path,
                operation_id=ep.operation_id if ep.operation_id else None,
                summary=ep.summary if ep.summary else None,
            )
            for ep in response.endpoints
        ]

    def close(self) -> None:
        """
        Closes the gRPC client connection.
        Should be called when the validator is no longer needed to free resources.
        """
        if self._channel:
            self._channel.close()

    # =========================================================================
    # Consumer Registry Methods
    # =========================================================================

    def register_consumer(self, options: RegisterConsumerOptions) -> ConsumerInfo:
        """
        Registers a consumer's dependency on a schema.
        This tracks which consumers use which schemas for deployment safety analysis.

        Args:
            options: Consumer registration options

        Returns:
            ConsumerInfo with the registered consumer details.

        Raises:
            ValueError: If registration fails.

        Example:
            >>> consumer = validator.register_consumer(RegisterConsumerOptions(
            ...     consumer_id="order-service",
            ...     consumer_version="2.1.0",
            ...     schema_id="user-api",
            ...     schema_version="1.0.0",
            ...     environment="prod",
            ...     used_endpoints=[
            ...         EndpointUsage(method="GET", path="/users/{id}", used_fields=["email", "name"])
            ...     ]
            ... ))
        """
        # Convert used endpoints to proto format
        used_endpoints = []
        if options.used_endpoints:
            for ep in options.used_endpoints:
                used_endpoints.append(
                    ProtoEndpointUsage(
                        method=ep.method,
                        path=ep.path,
                        used_fields=ep.used_fields or [],
                    )
                )

        request = RegisterConsumerRequest(
            consumer_id=options.consumer_id,
            consumer_version=options.consumer_version,
            schema_id=options.schema_id,
            schema_version=options.schema_version,
            environment=options.environment,
            used_endpoints=used_endpoints,
        )
        response: RegisterConsumerResponse = self._client.RegisterConsumer(
            request, metadata=self._create_metadata()
        )

        if not response.success:
            raise ValueError(
                f"Consumer registration failed: {response.message or 'Unknown error'}"
            )

        return self._map_consumer_info(response.consumer)

    def list_consumers(
        self, schema_id: str, environment: Optional[str] = None
    ) -> list[ConsumerInfo]:
        """
        Lists all consumers that depend on a schema.

        Args:
            schema_id: The schema identifier
            environment: Optional environment filter (dev, staging, prod)

        Returns:
            List of ConsumerInfo objects.

        Example:
            >>> consumers = validator.list_consumers("user-api", "prod")
            >>> for c in consumers:
            ...     print(f"{c['consumer_id']} v{c['consumer_version']}")
        """
        request = ListConsumersRequest(
            schema_id=schema_id,
            environment=environment or "",
        )
        response: ListConsumersResponse = self._client.ListConsumers(
            request, metadata=self._create_metadata()
        )

        return [self._map_consumer_info(c) for c in response.consumers]

    def deregister_consumer(
        self, consumer_id: str, schema_id: str, environment: str
    ) -> None:
        """
        Removes a consumer registration for a specific schema.

        Args:
            consumer_id: The consumer identifier
            schema_id: The schema identifier
            environment: The environment (dev, staging, prod)

        Raises:
            ValueError: If deregistration fails.

        Example:
            >>> validator.deregister_consumer("order-service", "user-api", "prod")
        """
        request = DeregisterConsumerRequest(
            consumer_id=consumer_id,
            schema_id=schema_id,
            environment=environment,
        )
        response: DeregisterConsumerResponse = self._client.DeregisterConsumer(
            request, metadata=self._create_metadata()
        )

        if not response.success:
            raise ValueError(
                f"Consumer deregistration failed: {response.message or 'Unknown error'}"
            )

    def can_i_deploy(
        self, schema_id: str, new_version: str, environment: str
    ) -> CanIDeployResult:
        """
        Checks if a schema version can be safely deployed without breaking consumers.

        Args:
            schema_id: The schema identifier
            new_version: The new version to deploy
            environment: Target environment (dev, staging, prod)

        Returns:
            CanIDeployResult with deployment safety analysis.

        Example:
            >>> result = validator.can_i_deploy("user-api", "2.0.0", "prod")
            >>> if not result["safe_to_deploy"]:
            ...     print("Unsafe to deploy:")
            ...     for c in result["affected_consumers"]:
            ...         print(f"- {c['consumer_id']} will break")
        """
        request = CanIDeployRequest(
            schema_id=schema_id,
            new_version=new_version,
            environment=environment,
        )
        response: CanIDeployResponse = self._client.CanIDeploy(
            request, metadata=self._create_metadata()
        )

        # Convert breaking changes
        breaking_changes: list[BreakingChange] = []
        for bc in response.breaking_changes:
            breaking_changes.append(
                BreakingChange(
                    type=str(bc.type) if bc.type else "UNKNOWN",
                    path=bc.path or "",
                    method=bc.method or "",
                    description=bc.description or "",
                    old_value=bc.old_value or "",
                    new_value=bc.new_value or "",
                )
            )

        # Convert affected consumers
        affected_consumers: list[ConsumerImpact] = []
        for c in response.affected_consumers:
            relevant_changes: list[BreakingChange] = []
            for bc in c.relevant_changes:
                relevant_changes.append(
                    BreakingChange(
                        type=str(bc.type) if bc.type else "UNKNOWN",
                        path=bc.path or "",
                        method=bc.method or "",
                        description=bc.description or "",
                        old_value=bc.old_value or "",
                        new_value=bc.new_value or "",
                    )
                )

            affected_consumers.append(
                ConsumerImpact(
                    consumer_id=c.consumer_id or "",
                    consumer_version=c.consumer_version or "",
                    current_schema_version=c.current_schema_version or "",
                    environment=c.environment or "",
                    will_break=c.will_break,
                    relevant_changes=relevant_changes,
                )
            )

        return CanIDeployResult(
            safe_to_deploy=response.safe_to_deploy,
            summary=response.summary or "",
            breaking_changes=breaking_changes,
            affected_consumers=affected_consumers,
        )

    def _map_consumer_info(self, c: Any) -> ConsumerInfo:
        """Maps protobuf ConsumerInfo to Python ConsumerInfo."""
        used_endpoints = []
        for ep in c.used_endpoints:
            used_endpoints.append(
                EndpointUsage(
                    method=ep.method or "",
                    path=ep.path or "",
                    used_fields=list(ep.used_fields) if ep.used_fields else None,
                )
            )

        return ConsumerInfo(
            consumer_id=c.consumer_id or "",
            consumer_version=c.consumer_version or "",
            schema_id=c.schema_id or "",
            schema_version=c.schema_version or "",
            environment=c.environment or "",
            registered_at=c.registered_at,
            last_validated_at=c.last_validated_at,
            used_endpoints=used_endpoints,
        )

    # =========================================================================
    # Auto-Registration Methods
    # =========================================================================

    def build_consumer_from_interactions(
        self,
        interactions: list["CapturedInteraction"],
        config: "AutoRegisterConfig",
    ) -> RegisterConsumerOptions:
        """
        Builds consumer registration options from captured interactions.
        Useful for preview/dry-run scenarios.

        Args:
            interactions: List of captured interactions from mock adapter
            config: Auto-registration configuration

        Returns:
            RegisterConsumerOptions that can be passed to register_consumer

        Example:
            >>> from cvt_sdk.adapters import MockSession
            >>> from cvt_sdk.auto_register import AutoRegisterConfig
            >>> session = MockSession(validator)
            >>> # ... run tests ...
            >>> opts = validator.build_consumer_from_interactions(
            ...     session.get_interactions(),
            ...     AutoRegisterConfig(
            ...         consumer_id="order-service",
            ...         consumer_version="2.1.0",
            ...         environment="dev",
            ...         schema_version="1.0.0",
            ...     )
            ... )
            >>> print(f"Would register {len(opts.used_endpoints)} endpoints")
        """
        from .auto_register import build_consumer_from_interactions

        options, error = build_consumer_from_interactions(interactions, config)
        if error:
            raise ValueError(error)
        return options  # type: ignore

    def register_consumer_from_interactions(
        self,
        interactions: list["CapturedInteraction"],
        config: "AutoRegisterConfig",
    ) -> ConsumerInfo:
        """
        Registers a consumer based on captured mock interactions.
        This combines build_consumer_from_interactions + register_consumer.

        Args:
            interactions: List of captured interactions from mock adapter
            config: Auto-registration configuration

        Returns:
            ConsumerInfo with the registered consumer details.

        Example:
            >>> from cvt_sdk.adapters import MockSession
            >>> from cvt_sdk.auto_register import AutoRegisterConfig
            >>> session = MockSession(validator, cache=True)
            >>>
            >>> # Run tests
            >>> session.get("http://mock.user-api/users/123")
            >>> session.post("http://mock.user-api/users", json={"name": "John"})
            >>>
            >>> # Auto-register consumer from captured interactions
            >>> info = validator.register_consumer_from_interactions(
            ...     session.get_interactions(),
            ...     AutoRegisterConfig(
            ...         consumer_id="order-service",
            ...         consumer_version="2.1.0",
            ...         environment="dev",
            ...         schema_version="1.0.0",
            ...         # schema_id auto-extracted from URL: "user-api"
            ...     )
            ... )
            >>> print(f"Registered {info['consumer_id']} with {len(info['used_endpoints'])} endpoints")
        """
        opts = self.build_consumer_from_interactions(interactions, config)
        return self.register_consumer(opts)

    def _read_schema_from_file(self, file_path: str) -> str:
        """Reads schema content from a local file."""
        path = Path(file_path)
        if not path.exists():
            raise FileNotFoundError(f"Schema file not found: {file_path}")
        return path.read_text(encoding="utf-8")

    def _fetch_schema_from_url(self, url: str) -> str:
        """Fetches schema content from a URL."""
        try:
            with urlopen(url) as response:
                if response.status >= 200 and response.status < 300:
                    return response.read().decode("utf-8")
                else:
                    raise ValueError(f"Failed to fetch schema: HTTP {response.status}")
        except HTTPError as e:
            raise ValueError(f"Failed to fetch schema: HTTP {e.code}") from e
        except URLError as e:
            raise ValueError(f"Failed to fetch schema: {e.reason}") from e


__all__ = [
    "ContractValidator",
    "ContractValidatorOptions",
    "TLSOptions",
    "ValidationRequest",
    "ValidationResponse",
    "ValidationResult",
    "BreakingChange",
    "CompareResult",
    "GenerateOptions",
    "GeneratedRequest",
    "GeneratedResponse",
    "GeneratedFixture",
    "EndpointInfo",
    # Consumer Registry
    "EndpointUsage",
    "ConsumerInfo",
    "RegisterConsumerOptions",
    "ConsumerImpact",
    "CanIDeployResult",
    "SchemaOwnership",
    "SchemaInfo",
]

# Re-export auto-register types (must be at end to avoid circular imports)
from .auto_register import (  # noqa: E402
    AutoRegisterConfig,
    extract_schema_id_from_url,
    extract_schema_id_from_interactions,
    extract_fields_from_body,
    normalize_path_for_endpoint,
    merge_interactions_to_endpoints,
    build_consumer_from_interactions,
)

__all__ += [
    "AutoRegisterConfig",
    "extract_schema_id_from_url",
    "extract_schema_id_from_interactions",
    "extract_fields_from_body",
    "normalize_path_for_endpoint",
    "merge_interactions_to_endpoints",
    "build_consumer_from_interactions",
]
