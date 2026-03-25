"""Tests for the ProducerTestKit."""

import pytest

from cvt_sdk.producer import (
    ProducerTestKit,
    TestConfig,
    TestRequestContext,
    TestResponseData,
    TestValidationResult,
    EndpointTester,
)


class TestTestConfig:
    """Tests for TestConfig dataclass."""

    def test_required_fields(self):
        """Test that schema_id is required."""
        config = TestConfig(schema_id="test-api")
        assert config.schema_id == "test-api"

    def test_default_server_address(self):
        """Test default server address."""
        config = TestConfig(schema_id="test-api")
        assert config.server_address == "localhost:9550"

    def test_custom_server_address(self):
        """Test custom server address."""
        config = TestConfig(schema_id="test-api", server_address="custom:9999")
        assert config.server_address == "custom:9999"

    def test_optional_schema_version(self):
        """Test optional schema version."""
        config = TestConfig(schema_id="test-api", schema_version="1.0.0")
        assert config.schema_version == "1.0.0"

    def test_optional_api_key(self):
        """Test optional API key."""
        config = TestConfig(schema_id="test-api", api_key="test-key")
        assert config.api_key == "test-key"


class TestTestResponseData:
    """Tests for TestResponseData dataclass."""

    def test_status_code(self):
        """Test status code field."""
        response = TestResponseData(status_code=200)
        assert response.status_code == 200

    def test_body(self):
        """Test body field."""
        response = TestResponseData(status_code=200, body={"id": "123"})
        assert response.body == {"id": "123"}

    def test_headers(self):
        """Test headers field."""
        response = TestResponseData(
            status_code=200, headers={"Content-Type": "application/json"}
        )
        assert response.headers == {"Content-Type": "application/json"}

    def test_default_headers(self):
        """Test default empty headers."""
        response = TestResponseData(status_code=200)
        assert response.headers == {}


class TestTestRequestContext:
    """Tests for TestRequestContext dataclass."""

    def test_method_and_path(self):
        """Test method and path fields."""
        request = TestRequestContext(method="POST", path="/users")
        assert request.method == "POST"
        assert request.path == "/users"

    def test_body(self):
        """Test body field."""
        request = TestRequestContext(
            method="POST", path="/users", body={"name": "John"}
        )
        assert request.body == {"name": "John"}

    def test_headers(self):
        """Test headers field."""
        request = TestRequestContext(
            method="POST",
            path="/users",
            headers={"Content-Type": "application/json"},
        )
        assert request.headers == {"Content-Type": "application/json"}


class TestTestValidationResult:
    """Tests for TestValidationResult dataclass."""

    def test_valid_result(self):
        """Test valid result."""
        result = TestValidationResult(valid=True, errors=[])
        assert result.valid is True
        assert result.errors == []

    def test_invalid_result(self):
        """Test invalid result with errors."""
        result = TestValidationResult(
            valid=False, errors=["Missing required field: name"]
        )
        assert result.valid is False
        assert len(result.errors) == 1

    def test_version_info(self):
        """Test version info fields."""
        result = TestValidationResult(
            valid=True,
            errors=[],
            validated_against_version="1.0.0",
            validated_against_hash="abc123",
        )
        assert result.validated_against_version == "1.0.0"
        assert result.validated_against_hash == "abc123"


class TestProducerTestKit:
    """Tests for ProducerTestKit."""

    def test_requires_schema_id(self):
        """Test that schema_id is required."""
        with pytest.raises(ValueError) as exc_info:
            ProducerTestKit(TestConfig(schema_id=""))
        assert "schema_id is required" in str(exc_info.value)

    def test_creates_with_valid_config(self):
        """Test creation with valid config."""
        test_kit = ProducerTestKit(TestConfig(schema_id="test-api"))
        assert test_kit is not None
        test_kit.close()

    def test_creates_with_schema_version(self):
        """Test creation with schema version."""
        test_kit = ProducerTestKit(
            TestConfig(schema_id="test-api", schema_version="1.0.0")
        )
        assert test_kit._schema_version == "1.0.0"
        test_kit.close()

    def test_creates_with_api_key(self):
        """Test creation with API key."""
        test_kit = ProducerTestKit(TestConfig(schema_id="test-api", api_key="test-key"))
        assert test_kit._api_key == "test-key"
        test_kit.close()

    def test_for_endpoint_returns_tester(self):
        """Test for_endpoint returns EndpointTester."""
        test_kit = ProducerTestKit(TestConfig(schema_id="test-api"))
        endpoint_tester = test_kit.for_endpoint("GET", "/users/{id}")
        assert isinstance(endpoint_tester, EndpointTester)
        assert endpoint_tester._method == "GET"
        assert endpoint_tester._path_pattern == "/users/{id}"
        test_kit.close()

    def test_serialize_body_none(self):
        """Test serializing None body."""
        result = ProducerTestKit._serialize_body(None)
        assert result == ""

    def test_serialize_body_string(self):
        """Test serializing string body."""
        result = ProducerTestKit._serialize_body('{"key": "value"}')
        assert result == '{"key": "value"}'

    def test_serialize_body_dict(self):
        """Test serializing dict body."""
        result = ProducerTestKit._serialize_body({"key": "value"})
        assert result == '{"key": "value"}'

    def test_serialize_body_list(self):
        """Test serializing list body."""
        result = ProducerTestKit._serialize_body([1, 2, 3])
        assert result == "[1, 2, 3]"


class TestEndpointTester:
    """Tests for EndpointTester."""

    def test_stores_method_and_path(self):
        """Test that method and path are stored."""
        test_kit = ProducerTestKit(TestConfig(schema_id="test-api"))
        endpoint_tester = EndpointTester(test_kit, "POST", "/users")
        assert endpoint_tester._method == "POST"
        assert endpoint_tester._path_pattern == "/users"
        test_kit.close()

    def test_path_substitution(self):
        """Test path parameter substitution logic."""
        # Test the substitution logic directly
        path_pattern = "/users/{id}/posts/{post_id}"
        path_values = {"id": "123", "post_id": "456"}

        actual_path = path_pattern
        for key, value in path_values.items():
            actual_path = actual_path.replace(f"{{{key}}}", value)

        assert actual_path == "/users/123/posts/456"
