"""Tests for the CVT Python SDK producer validation."""

import re
from typing import Any
from unittest.mock import MagicMock

import pytest

from cvt_sdk.producer.config import (
    ProducerConfig,
    ValidationMode,
    ValidationResult,
    matches_path_filter,
    should_validate_path,
)
from cvt_sdk.producer.producer import (
    Producer,
    get_metrics,
    record_rejection,
    record_validation_metrics,
    reset_metrics,
)


class MockValidator:
    """Mock validator for testing."""

    def __init__(self, valid: bool = True, errors: list[str] = None):
        self.valid = valid
        self.errors = errors or []
        self.last_interaction = None
        self.call_count = 0

    def validate(self, schema_id: str, interaction: dict) -> ValidationResult:
        self.last_interaction = interaction
        self.call_count += 1
        return ValidationResult(valid=self.valid, errors=self.errors, type="")


class MockRequest:
    """Mock HTTP request object."""

    def __init__(self, method: str, path: str, headers: dict = None, body: Any = None):
        self.method = method
        self.path = path
        self.url = path
        self.headers = headers or {}
        self.body = body


class MockResponse:
    """Mock HTTP response object."""

    def __init__(self, status_code: int = 200, headers: dict = None, body: Any = None):
        self.status_code = status_code
        self.headers = headers or {}
        self.body = body


class TestProducerInit:
    """Tests for Producer initialization."""

    def test_create_producer_with_valid_config(self):
        """Should create producer with valid configuration."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
            )
        )
        assert producer is not None
        assert producer.mode == ValidationMode.STRICT

    def test_raise_error_when_schema_id_missing(self):
        """Should raise ValueError when schema_id is empty."""
        with pytest.raises(ValueError, match="schema_id is required"):
            Producer(
                ProducerConfig(
                    schema_id="",
                    validator=MockValidator(),
                )
            )

    def test_raise_error_when_validator_missing(self):
        """Should raise ValueError when validator is None."""
        with pytest.raises(ValueError, match="validator is required"):
            Producer(
                ProducerConfig(
                    schema_id="test-schema",
                    validator=None,
                )
            )

    def test_apply_default_config_values(self):
        """Should use default config values."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
            )
        )
        assert producer.config.mode == ValidationMode.STRICT
        assert producer.config.validate_request is True
        assert producer.config.validate_response is True

    def test_use_custom_mode(self):
        """Should use custom mode when provided."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                mode=ValidationMode.WARN,
            )
        )
        assert producer.mode == ValidationMode.WARN


class TestValidateRequest:
    """Tests for request validation."""

    @pytest.mark.asyncio
    async def test_validate_valid_request(self):
        """Should return valid result for valid request."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(valid=True),
            )
        )

        result = await producer.validate_request(
            method="POST",
            path="/users",
            headers={"content-type": "application/json"},
            body={"name": "test"},
        )

        assert result["valid"] is True
        assert result["type"] == "request"

    @pytest.mark.asyncio
    async def test_validate_invalid_request(self):
        """Should return invalid result with errors."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(
                    valid=False, errors=["missing required field: email"]
                ),
            )
        )

        result = await producer.validate_request(
            method="POST",
            path="/users",
            headers={},
            body={"name": "test"},
        )

        assert result["valid"] is False
        assert "missing required field: email" in result["errors"]
        assert result["type"] == "request"

    @pytest.mark.asyncio
    async def test_skip_validation_when_disabled(self):
        """Should skip validation when validateRequest is False."""
        validator = MockValidator()
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=validator,
                validate_request=False,
            )
        )

        result = await producer.validate_request("POST", "/users", {}, {})

        assert result["valid"] is True
        assert validator.call_count == 0

    @pytest.mark.asyncio
    async def test_pass_correct_interaction(self):
        """Should pass correct interaction to validator."""
        validator = MockValidator()
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=validator,
            )
        )

        await producer.validate_request(
            method="POST",
            path="/users?page=1",
            headers={"content-type": "application/json"},
            body={"name": "test"},
        )

        assert validator.last_interaction is not None
        assert validator.last_interaction["method"] == "POST"
        assert validator.last_interaction["path"] == "/users?page=1"
        assert validator.last_interaction["body"] == {"name": "test"}


class TestValidateResponse:
    """Tests for response validation."""

    @pytest.mark.asyncio
    async def test_validate_valid_response(self):
        """Should return valid result for valid response."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(valid=True),
            )
        )

        result = await producer.validate_response(
            method="GET",
            path="/users/123",
            req_headers={},
            req_body=None,
            status_code=200,
            resp_headers={"content-type": "application/json"},
            resp_body={"id": 123, "name": "test"},
        )

        assert result["valid"] is True
        assert result["type"] == "response"

    @pytest.mark.asyncio
    async def test_validate_invalid_response(self):
        """Should return invalid result with errors."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(
                    valid=False, errors=["response body type mismatch"]
                ),
            )
        )

        result = await producer.validate_response(
            method="GET",
            path="/users/123",
            req_headers={},
            req_body=None,
            status_code=200,
            resp_headers={},
            resp_body={"id": "not-a-number"},
        )

        assert result["valid"] is False
        assert "response body type mismatch" in result["errors"]

    @pytest.mark.asyncio
    async def test_skip_validation_when_disabled(self):
        """Should skip validation when validateResponse is False."""
        validator = MockValidator()
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=validator,
                validate_response=False,
            )
        )

        result = await producer.validate_response(
            "GET", "/users/123", {}, None, 200, {}, {}
        )

        assert result["valid"] is True
        assert validator.call_count == 0


class TestHandleRequestFailure:
    """Tests for request failure handling."""

    @pytest.mark.asyncio
    async def test_strict_mode_rejects(self):
        """Should return False (reject) in strict mode."""
        reset_metrics()
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                mode=ValidationMode.STRICT,
            )
        )

        request = MockRequest("POST", "/users")
        result = ValidationResult(valid=False, errors=["test error"], type="request")

        should_continue, custom_response = await producer.handle_request_failure(
            result, request
        )

        assert should_continue is False
        assert custom_response is None
        assert get_metrics()["requests_rejected"] == 1

    @pytest.mark.asyncio
    async def test_warn_mode_continues(self):
        """Should return True (continue) in warn mode."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                mode=ValidationMode.WARN,
            )
        )

        request = MockRequest("POST", "/users")
        result = ValidationResult(valid=False, errors=["test error"], type="request")

        should_continue, custom_response = await producer.handle_request_failure(
            result, request
        )

        assert should_continue is True

    @pytest.mark.asyncio
    async def test_shadow_mode_continues(self):
        """Should return True (continue) in shadow mode."""
        reset_metrics()
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                mode=ValidationMode.SHADOW,
            )
        )

        request = MockRequest("POST", "/users")
        result = ValidationResult(valid=False, errors=["test error"], type="request")

        should_continue, custom_response = await producer.handle_request_failure(
            result, request
        )

        assert should_continue is True
        assert get_metrics()["request_validations_failed"] == 1

    @pytest.mark.asyncio
    async def test_custom_handler_called(self):
        """Should call custom onRequestFailure handler."""
        custom_handler = MagicMock(return_value={"custom": "response"})

        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                mode=ValidationMode.STRICT,
                on_request_failure=custom_handler,
            )
        )

        request = MockRequest("POST", "/users")
        result = ValidationResult(valid=False, errors=["test error"], type="request")

        should_continue, custom_response = await producer.handle_request_failure(
            result, request
        )

        custom_handler.assert_called_once_with(result, request)
        assert should_continue is False
        assert custom_response == {"custom": "response"}


class TestHandleResponseFailure:
    """Tests for response failure handling."""

    @pytest.mark.asyncio
    async def test_logs_in_strict_mode(self):
        """Should log in strict mode."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                mode=ValidationMode.STRICT,
            )
        )

        request = MockRequest("GET", "/users/123")
        response = MockResponse(200)
        result = ValidationResult(
            valid=False, errors=["response error"], type="response"
        )

        # Should not raise
        await producer.handle_response_failure(result, request, response)

    @pytest.mark.asyncio
    async def test_custom_handler_called(self):
        """Should call custom onResponseFailure handler."""
        custom_handler = MagicMock()

        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                mode=ValidationMode.STRICT,
                on_response_failure=custom_handler,
            )
        )

        request = MockRequest("GET", "/users/123")
        response = MockResponse(200)
        result = ValidationResult(
            valid=False, errors=["response error"], type="response"
        )

        await producer.handle_response_failure(result, request, response)

        custom_handler.assert_called_once_with(result, request, response)


class TestShouldValidatePath:
    """Tests for path filtering on Producer."""

    def test_validate_all_paths_by_default(self):
        """Should validate all paths when no filters configured."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
            )
        )

        assert producer.should_validate_path("/api/users") is True
        assert producer.should_validate_path("/health") is True
        assert producer.should_validate_path("/any/path") is True

    def test_exclude_matching_paths(self):
        """Should exclude paths matching excludePaths."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                exclude_paths=["/health", "/metrics"],
            )
        )

        assert producer.should_validate_path("/api/users") is True
        assert producer.should_validate_path("/health") is False
        assert producer.should_validate_path("/metrics") is False

    def test_include_matching_paths_only(self):
        """Should only validate paths matching includePaths."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                include_paths=["/api/"],
            )
        )

        assert producer.should_validate_path("/api/users") is True
        assert producer.should_validate_path("/health") is False

    def test_exclude_takes_precedence(self):
        """Should give precedence to excludePaths over includePaths."""
        producer = Producer(
            ProducerConfig(
                schema_id="test-schema",
                validator=MockValidator(),
                include_paths=["/api/"],
                exclude_paths=["/api/internal"],
            )
        )

        assert producer.should_validate_path("/api/users") is True
        assert producer.should_validate_path("/api/internal/debug") is False


class TestPathFilteringUtilities:
    """Tests for path filtering utility functions."""

    def test_matches_string_pattern(self):
        """Should match string patterns (substring match)."""
        assert matches_path_filter("/api/users", "/api/") is True
        assert matches_path_filter("/api/users", "/users") is True
        assert matches_path_filter("/api/users", "/other") is False

    def test_matches_regex_pattern(self):
        """Should match regex patterns."""
        assert matches_path_filter("/api/users/123", re.compile(r"^/api/")) is True
        assert matches_path_filter("/health", re.compile(r"^/api/")) is False
        assert matches_path_filter("/users/123", re.compile(r"/users/\d+")) is True

    def test_should_validate_all_when_no_filters(self):
        """Should validate all paths when no filters."""
        assert should_validate_path("/any/path", [], []) is True
        assert should_validate_path("/another/path", [], []) is True

    def test_should_apply_exclude_first(self):
        """Should apply excludePaths first."""
        assert should_validate_path("/health", [], ["/health"]) is False
        assert should_validate_path("/api/users", [], ["/health"]) is True

    def test_should_apply_include_after_exclude(self):
        """Should apply includePaths after excludePaths."""
        assert should_validate_path("/api/users", ["/api/"], []) is True
        assert should_validate_path("/health", ["/api/"], []) is False


class TestMetrics:
    """Tests for metrics tracking."""

    def setup_method(self):
        """Reset metrics before each test."""
        reset_metrics()

    def test_record_request_validations(self):
        """Should record request validations."""
        record_validation_metrics(
            "request", ValidationResult(valid=True, errors=[], type="request")
        )
        record_validation_metrics(
            "request",
            ValidationResult(valid=False, errors=["error"], type="request"),
        )

        metrics = get_metrics()
        assert metrics["request_validations"] == 2
        assert metrics["request_validations_passed"] == 1
        assert metrics["request_validations_failed"] == 1

    def test_record_response_validations(self):
        """Should record response validations."""
        record_validation_metrics(
            "response", ValidationResult(valid=True, errors=[], type="response")
        )
        record_validation_metrics(
            "response", ValidationResult(valid=True, errors=[], type="response")
        )
        record_validation_metrics(
            "response",
            ValidationResult(valid=False, errors=["error"], type="response"),
        )

        metrics = get_metrics()
        assert metrics["response_validations"] == 3
        assert metrics["response_validations_passed"] == 2
        assert metrics["response_validations_failed"] == 1

    def test_record_rejections(self):
        """Should record rejections."""
        record_rejection()
        record_rejection()

        metrics = get_metrics()
        assert metrics["requests_rejected"] == 2

    def test_reset_metrics(self):
        """Should reset all metrics to zero."""
        record_validation_metrics(
            "request", ValidationResult(valid=True, errors=[], type="request")
        )
        record_validation_metrics(
            "response",
            ValidationResult(valid=False, errors=[], type="response"),
        )
        record_rejection()

        reset_metrics()

        metrics = get_metrics()
        assert metrics["request_validations"] == 0
        assert metrics["response_validations"] == 0
        assert metrics["requests_rejected"] == 0
