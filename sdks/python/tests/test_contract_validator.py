"Tests for the CVT Python SDK ContractValidator class using mocks."

from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest
from cvt_sdk import (
    ContractValidator,
    ValidationRequest,
    ValidationResponse,
    RegisterConsumerOptions,
    SchemaInfo,
    SchemaOwnership,
)
from cvt_sdk.proto import RegisterSchemaResponse, ValidationResult


@pytest.fixture
def mock_grpc_stub():
    """Mock the gRPC stub."""
    with patch("cvt_sdk.ContractValidatorStub") as mock:
        yield mock


@pytest.fixture
def mock_grpc_channel():
    """Mock the gRPC channel."""
    with patch("grpc.insecure_channel") as mock:
        yield mock


@pytest.fixture
def mock_urlopen():
    """Mock urllib.request.urlopen."""
    with patch("cvt_sdk.urlopen") as mock:
        yield mock


@pytest.fixture
def validator(mock_grpc_stub, mock_grpc_channel):
    """Create a ContractValidator instance with mocked dependencies."""
    v = ContractValidator()
    # Mock the client instance methods
    v._client = MagicMock()
    return v


@pytest.fixture
def schema_path():
    """Path to the shared OpenAPI schema for testing."""
    return "mock/path/to/openapi.json"


@pytest.fixture
def validator_with_schema(validator, schema_path):
    """Create a ContractValidator with pre-registered schema."""
    # Setup mock for successful registration
    mock_response = MagicMock(spec=RegisterSchemaResponse)
    mock_response.success = True
    validator._client.RegisterSchema.return_value = mock_response

    # Mock file reading
    with (
        patch.object(Path, "exists", return_value=True),
        patch.object(Path, "read_text", return_value='{"openapi": "3.0.0"}'),
    ):
        validator.register_schema("test-schema", schema_path)
    return validator


class TestSchemaRegistration:
    """Tests for schema registration functionality."""

    def test_register_schema_from_local_file(self, validator, schema_path):
        """Should register schema from local file successfully."""
        mock_response = MagicMock(spec=RegisterSchemaResponse)
        mock_response.success = True
        validator._client.RegisterSchema.return_value = mock_response

        with (
            patch.object(Path, "exists", return_value=True),
            patch.object(Path, "read_text", return_value='{"openapi": "3.0.0"}'),
        ):
            validator.register_schema("test-schema", schema_path)

            validator._client.RegisterSchema.assert_called_once()
            args = validator._client.RegisterSchema.call_args[0][0]
            assert args.schema_id == "test-schema"
            assert args.schema_content == '{"openapi": "3.0.0"}'

    def test_register_schema_from_url(self, validator, mock_urlopen):
        """Should register schema from URL successfully."""
        mock_response = MagicMock(spec=RegisterSchemaResponse)
        mock_response.success = True
        validator._client.RegisterSchema.return_value = mock_response

        # Mock URL response
        mock_url_response = MagicMock()
        mock_url_response.status = 200
        mock_url_response.read.return_value = b'{"openapi": "3.0.0"}'
        mock_urlopen.return_value.__enter__.return_value = mock_url_response

        validator.register_schema("url-test-schema", "http://example.com/openapi.json")

        validator._client.RegisterSchema.assert_called_once()
        args = validator._client.RegisterSchema.call_args[0][0]
        assert args.schema_id == "url-test-schema"
        assert args.schema_content == '{"openapi": "3.0.0"}'

    def test_register_schema_invalid_file(self, validator):
        """Should raise FileNotFoundError for non-existent file."""
        with patch.object(Path, "exists", return_value=False):
            with pytest.raises(FileNotFoundError):
                validator.register_schema("test-schema", "/non/existent.json")

    def test_register_schema_failure(self, validator, schema_path):
        """Should raise ValueError if server returns failure."""
        mock_response = MagicMock(spec=RegisterSchemaResponse)
        mock_response.success = False
        mock_response.message = "Invalid schema"
        validator._client.RegisterSchema.return_value = mock_response

        with (
            patch.object(Path, "exists", return_value=True),
            patch.object(Path, "read_text", return_value="{}"),
        ):
            with pytest.raises(ValueError, match="Schema registration failed"):
                validator.register_schema("test-schema", schema_path)


class TestValidation:
    """Tests for validation functionality."""

    def test_validate_without_schema_registration(self, validator):
        """Should raise ValueError when validating without registering schema."""
        request: ValidationRequest = {"method": "GET", "path": "/pet/123"}
        response: ValidationResponse = {"status_code": 200}

        with pytest.raises(ValueError, match="Schema not registered"):
            validator.validate(request, response)

    def test_validate_valid_pet_creation(self, validator_with_schema):
        """Should validate valid pet creation."""
        mock_result = MagicMock(spec=ValidationResult)
        mock_result.valid = True
        mock_result.errors = []
        validator_with_schema._client.ValidateInteraction.return_value = mock_result

        request: ValidationRequest = {
            "method": "POST",
            "path": "/pet",
            "headers": {"content-type": "application/json"},
            "body": {"name": "Fluffy"},
        }

        response: ValidationResponse = {"status_code": 201}

        result = validator_with_schema.validate(request, response)

        assert result["valid"] is True
        validator_with_schema._client.ValidateInteraction.assert_called_once()
        args = validator_with_schema._client.ValidateInteraction.call_args[0][0]
        assert args.schema_id == "test-schema"
        assert args.request.method == "POST"

    def test_validate_invalid_interaction(self, validator_with_schema):
        """Should return validation errors for invalid interaction."""
        mock_result = MagicMock(spec=ValidationResult)
        mock_result.valid = False
        mock_result.errors = ["Missing field"]
        validator_with_schema._client.ValidateInteraction.return_value = mock_result

        request: ValidationRequest = {"method": "POST", "path": "/pet"}
        response: ValidationResponse = {"status_code": 400}

        result = validator_with_schema.validate(request, response)

        assert result["valid"] is False
        assert "Missing field" in result["errors"]

    def test_validate_with_statuscode_alternative_naming(self, validator_with_schema):
        """Should support both status_code and statusCode naming conventions."""
        mock_result = MagicMock(spec=ValidationResult)
        mock_result.valid = True
        mock_result.errors = []
        validator_with_schema._client.ValidateInteraction.return_value = mock_result

        validator_with_schema.validate(
            request={"method": "GET", "path": "/pet/123"}, response={"statusCode": 200}
        )

        args = validator_with_schema._client.ValidateInteraction.call_args[0][0]
        assert args.response.status_code == 200

    def test_close(self, validator, mock_grpc_channel):
        """Should close the gRPC channel."""
        # The channel mock is already set up in the fixture
        validator.close()
        # Access the channel instance that was created
        validator._channel.close.assert_called_once()


class TestCompareSchemas:
    """Tests for schema comparison functionality."""

    def test_compare_schemas_compatible(self, validator):
        """Should compare compatible schemas."""
        mock_response = MagicMock()
        mock_response.compatible = True
        mock_response.breaking_changes = []
        validator._client.CompareSchemas.return_value = mock_response

        result = validator.compare_schemas("test-schema", "1.0.0", "2.0.0")

        assert result["compatible"] is True
        assert result["breaking_changes"] == []
        validator._client.CompareSchemas.assert_called_once()

    def test_compare_schemas_with_breaking_changes(self, validator):
        """Should return breaking changes for incompatible schemas."""
        mock_breaking_change = MagicMock()
        mock_breaking_change.type = "ENDPOINT_REMOVED"
        mock_breaking_change.path = "/users/{id}"
        mock_breaking_change.method = "DELETE"
        mock_breaking_change.description = "Endpoint was removed"
        mock_breaking_change.old_value = "existed"
        mock_breaking_change.new_value = ""

        mock_response = MagicMock()
        mock_response.compatible = False
        mock_response.breaking_changes = [mock_breaking_change]
        validator._client.CompareSchemas.return_value = mock_response

        result = validator.compare_schemas("test-schema")

        assert result["compatible"] is False
        assert len(result["breaking_changes"]) == 1
        assert result["breaking_changes"][0]["path"] == "/users/{id}"


class TestRegisterSchemaWithVersion:
    """Tests for schema registration with version."""

    def test_register_schema_with_version(self, validator):
        """Should register schema with version successfully."""
        mock_response = MagicMock()
        mock_response.success = True
        validator._client.RegisterSchema.return_value = mock_response

        with (
            patch.object(Path, "exists", return_value=True),
            patch.object(Path, "read_text", return_value='{"openapi": "3.0.0"}'),
        ):
            validator.register_schema_with_version(
                "test-schema", "mock/path.json", "1.0.0"
            )

        validator._client.RegisterSchema.assert_called_once()
        args = validator._client.RegisterSchema.call_args[0][0]
        assert args.schema_id == "test-schema"
        assert args.schema_version == "1.0.0"


class TestGenerateFixture:
    """Tests for fixture generation functionality."""

    def test_generate_fixture(self, validator_with_schema):
        """Should generate fixture successfully."""
        mock_fixture = MagicMock()
        mock_fixture.request.method = "POST"
        mock_fixture.request.path = "/users"
        mock_fixture.request.body = '{"name":"test"}'
        mock_fixture.request.headers = {}
        mock_fixture.response.status_code = 201
        mock_fixture.response.body = '{"id":1}'
        mock_fixture.response.headers = {}

        mock_response = MagicMock()
        mock_response.success = True
        mock_response.fixture = mock_fixture
        validator_with_schema._client.GenerateFixture.return_value = mock_response

        result = validator_with_schema.generate_fixture("POST", "/users")

        assert result is not None
        assert result["request"]["method"] == "POST"

    def test_generate_fixture_failure(self, validator_with_schema):
        """Should raise error on generation failure."""
        mock_response = MagicMock()
        mock_response.success = False
        mock_response.message = "Path not found"
        validator_with_schema._client.GenerateFixture.return_value = mock_response

        with pytest.raises(ValueError, match="Fixture generation failed"):
            validator_with_schema.generate_fixture("POST", "/unknown")


class TestListEndpoints:
    """Tests for endpoint listing functionality."""

    def test_list_endpoints(self, validator_with_schema):
        """Should list endpoints successfully."""
        mock_endpoint1 = MagicMock()
        mock_endpoint1.method = "GET"
        mock_endpoint1.path = "/users"
        mock_endpoint1.operation_id = "getUsers"
        mock_endpoint1.summary = "Get all users"

        mock_endpoint2 = MagicMock()
        mock_endpoint2.method = "POST"
        mock_endpoint2.path = "/users"
        mock_endpoint2.operation_id = "createUser"
        mock_endpoint2.summary = "Create user"

        mock_response = MagicMock()
        mock_response.endpoints = [mock_endpoint1, mock_endpoint2]
        validator_with_schema._client.ListEndpoints.return_value = mock_response

        endpoints = validator_with_schema.list_endpoints()

        assert len(endpoints) == 2
        assert endpoints[0]["method"] == "GET"
        assert endpoints[0]["path"] == "/users"


class TestConsumerOperations:
    """Tests for consumer registration and management."""

    def test_register_consumer(self, validator):
        """Should register consumer successfully."""
        mock_consumer = MagicMock()
        mock_consumer.consumer_id = "order-service"
        mock_consumer.consumer_version = "1.0.0"
        mock_consumer.schema_id = "user-api"
        mock_consumer.schema_version = "1.0.0"
        mock_consumer.environment = "prod"
        mock_consumer.used_endpoints = []
        mock_consumer.registered_at = 0
        mock_consumer.last_validated_at = 0

        mock_response = MagicMock()
        mock_response.success = True
        mock_response.consumer = mock_consumer
        validator._client.RegisterConsumer.return_value = mock_response

        options = RegisterConsumerOptions(
            consumer_id="order-service",
            consumer_version="1.0.0",
            schema_id="user-api",
            schema_version="1.0.0",
            environment="prod",
        )
        result = validator.register_consumer(options)

        assert result["consumer_id"] == "order-service"
        assert result["environment"] == "prod"

    def test_register_consumer_failure(self, validator):
        """Should raise error on consumer registration failure."""
        mock_response = MagicMock()
        mock_response.success = False
        mock_response.message = "Registration failed"
        validator._client.RegisterConsumer.return_value = mock_response

        options = RegisterConsumerOptions(
            consumer_id="order-service",
            consumer_version="1.0.0",
            schema_id="user-api",
            schema_version="1.0.0",
            environment="prod",
        )
        with pytest.raises(ValueError, match="Consumer registration failed"):
            validator.register_consumer(options)

    def test_list_consumers(self, validator):
        """Should list consumers successfully."""
        mock_consumer = MagicMock()
        mock_consumer.consumer_id = "order-service"
        mock_consumer.consumer_version = "1.0.0"
        mock_consumer.environment = "prod"
        mock_consumer.used_endpoints = []
        mock_consumer.registered_at = 0
        mock_consumer.last_validated_at = 0

        mock_response = MagicMock()
        mock_response.consumers = [mock_consumer]
        validator._client.ListConsumers.return_value = mock_response

        consumers = validator.list_consumers("user-api", "prod")

        assert len(consumers) == 1
        assert consumers[0]["consumer_id"] == "order-service"

    def test_deregister_consumer(self, validator):
        """Should deregister consumer successfully."""
        mock_response = MagicMock()
        mock_response.success = True
        validator._client.DeregisterConsumer.return_value = mock_response

        validator.deregister_consumer("order-service", "user-api", "prod")

        validator._client.DeregisterConsumer.assert_called_once()

    def test_deregister_consumer_failure(self, validator):
        """Should raise error on deregistration failure."""
        mock_response = MagicMock()
        mock_response.success = False
        mock_response.message = "Consumer not found"
        validator._client.DeregisterConsumer.return_value = mock_response

        with pytest.raises(ValueError, match="Consumer deregistration failed"):
            validator.deregister_consumer("unknown", "user-api", "prod")


class TestCanIDeploy:
    """Tests for deployment safety check."""

    def test_can_i_deploy_safe(self, validator):
        """Should return safe to deploy."""
        mock_response = MagicMock()
        mock_response.safe_to_deploy = True
        mock_response.summary = "No breaking changes"
        mock_response.breaking_changes = []
        mock_response.affected_consumers = []
        validator._client.CanIDeploy.return_value = mock_response

        result = validator.can_i_deploy("user-api", "2.0.0", "prod")

        assert result["safe_to_deploy"] is True
        assert result["summary"] == "No breaking changes"

    def test_can_i_deploy_unsafe(self, validator):
        """Should return affected consumers when unsafe."""
        mock_breaking = MagicMock()
        mock_breaking.type = "ENDPOINT_REMOVED"
        mock_breaking.path = "/users"
        mock_breaking.method = "DELETE"
        mock_breaking.description = "Endpoint removed"
        mock_breaking.old_value = ""
        mock_breaking.new_value = ""

        mock_consumer = MagicMock()
        mock_consumer.consumer_id = "order-service"
        mock_consumer.consumer_version = "1.0.0"
        mock_consumer.will_break = True
        mock_consumer.relevant_changes = []
        mock_consumer.current_schema_version = "1.0.0"
        mock_consumer.environment = "prod"

        mock_response = MagicMock()
        mock_response.safe_to_deploy = False
        mock_response.summary = "Breaking changes affect consumers"
        mock_response.breaking_changes = [mock_breaking]
        mock_response.affected_consumers = [mock_consumer]
        validator._client.CanIDeploy.return_value = mock_response

        result = validator.can_i_deploy("user-api", "2.0.0", "prod")

        assert result["safe_to_deploy"] is False
        assert len(result["affected_consumers"]) == 1
        assert result["affected_consumers"][0]["consumer_id"] == "order-service"


class TestSchemaInfoExports:
    """Regression: SchemaInfo / SchemaOwnership must be in the public API."""

    def test_schema_info_importable(self):
        info = SchemaInfo(schema_id="x", schema_version="1.0")
        assert info.schema_id == "x"

    def test_schema_ownership_importable(self):
        o = SchemaOwnership(owner="jane", team="platform")
        assert o.owner == "jane"


class TestUseSchema:
    """Tests for the use_schema() method."""

    @staticmethod
    def _build_metadata_mock(
        schema_id: str = "petstore",
        schema_version: str = "1.2.0",
        with_ownership: bool = True,
    ):
        metadata = MagicMock()
        metadata.schema_id = schema_id
        metadata.schema_version = schema_version
        metadata.schema_hash = "abc123"
        metadata.registered_at = 1714000000
        metadata.updated_at = 1714000500
        metadata.openapi_version = "3.0.0"
        metadata.endpoint_count = 7
        metadata.HasField = lambda name: name == "ownership" and with_ownership
        metadata.ownership.owner = "jane"
        metadata.ownership.team = "platform"
        metadata.ownership.contact_email = "platform@example.com"
        metadata.ownership.read_only = False
        return metadata

    def test_resolves_latest_version(self, validator):
        """use_schema(id) returns resolved SchemaInfo and binds latest version."""
        metadata = self._build_metadata_mock()
        response = MagicMock()
        response.found = True
        response.metadata = metadata
        validator._client.GetSchema.return_value = response

        info = validator.use_schema("petstore")

        args = validator._client.GetSchema.call_args[0][0]
        assert args.schema_id == "petstore"
        assert args.schema_version == ""
        assert info.schema_id == "petstore"
        assert info.schema_version == "1.2.0"
        assert info.endpoint_count == 7
        assert info.ownership is not None
        assert info.ownership.owner == "jane"
        assert info.ownership.team == "platform"

    def test_pins_explicit_version(self, validator):
        """use_schema(id, version) passes version through to GetSchema."""
        metadata = self._build_metadata_mock(schema_version="1.0.0")
        response = MagicMock()
        response.found = True
        response.metadata = metadata
        validator._client.GetSchema.return_value = response

        info = validator.use_schema("petstore", "1.0.0")

        args = validator._client.GetSchema.call_args[0][0]
        assert args.schema_version == "1.0.0"
        assert info.schema_version == "1.0.0"

    def test_raises_when_schema_not_found(self, validator):
        """use_schema raises ValueError when server returns found=False."""
        response = MagicMock()
        response.found = False
        validator._client.GetSchema.return_value = response

        with pytest.raises(ValueError, match="'nope'.*not registered"):
            validator.use_schema("nope")

    def test_error_message_includes_version_when_pinning(self, validator):
        """Error message mentions the pinned version."""
        response = MagicMock()
        response.found = False
        validator._client.GetSchema.return_value = response

        with pytest.raises(ValueError, match=r"'petstore@9\.9\.9'"):
            validator.use_schema("petstore", "9.9.9")

    def test_validate_sends_schema_version_after_use_schema(self, validator):
        """validate() sends schema_version on the wire after use_schema()."""
        metadata = self._build_metadata_mock()
        get_response = MagicMock()
        get_response.found = True
        get_response.metadata = metadata
        validator._client.GetSchema.return_value = get_response

        validation_response = MagicMock(spec=ValidationResult)
        validation_response.valid = True
        validation_response.errors = []
        validator._client.ValidateInteraction.return_value = validation_response

        validator.use_schema("petstore")
        validator.validate(
            request=ValidationRequest(method="GET", path="/pet/1"),
            response=ValidationResponse(status_code=200),
        )

        sent = validator._client.ValidateInteraction.call_args[0][0]
        assert sent.schema_id == "petstore"
        assert sent.schema_version == "1.2.0"

    def test_register_schema_clears_pin_after_use_schema(self, validator, schema_path):
        """register_schema after use_schema must clear the version pin."""
        metadata = self._build_metadata_mock()
        get_response = MagicMock()
        get_response.found = True
        get_response.metadata = metadata
        validator._client.GetSchema.return_value = get_response

        mock_reg = MagicMock(spec=RegisterSchemaResponse)
        mock_reg.success = True
        validator._client.RegisterSchema.return_value = mock_reg

        validation_response = MagicMock(spec=ValidationResult)
        validation_response.valid = True
        validation_response.errors = []
        validator._client.ValidateInteraction.return_value = validation_response

        validator.use_schema("petstore")
        with (
            patch.object(Path, "exists", return_value=True),
            patch.object(Path, "read_text", return_value='{"openapi": "3.0.0"}'),
        ):
            validator.register_schema("other-schema", schema_path)

        validator.validate(
            request=ValidationRequest(method="GET", path="/pet/1"),
            response=ValidationResponse(status_code=200),
        )

        sent = validator._client.ValidateInteraction.call_args[0][0]
        assert sent.schema_id == "other-schema"
        assert sent.schema_version == ""

    def test_validate_does_not_send_version_after_register_schema(
        self, validator, schema_path
    ):
        """Existing register_schema path must not start sending schema_version."""
        mock_reg = MagicMock(spec=RegisterSchemaResponse)
        mock_reg.success = True
        validator._client.RegisterSchema.return_value = mock_reg

        validation_response = MagicMock(spec=ValidationResult)
        validation_response.valid = True
        validation_response.errors = []
        validator._client.ValidateInteraction.return_value = validation_response

        with (
            patch.object(Path, "exists", return_value=True),
            patch.object(Path, "read_text", return_value='{"openapi": "3.0.0"}'),
        ):
            validator.register_schema("test-schema", schema_path)

        validator.validate(
            request=ValidationRequest(method="GET", path="/pet/1"),
            response=ValidationResponse(status_code=200),
        )

        sent = validator._client.ValidateInteraction.call_args[0][0]
        assert sent.schema_id == "test-schema"
        assert sent.schema_version == ""
