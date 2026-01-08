"""Tests for producer ASGI/WSGI middleware adapters."""

import json
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from cvt_sdk.producer import ProducerConfig, ValidationMode
from cvt_sdk.producer.adapters.fastapi import (
    ASGIMiddleware,
    create_fastapi_middleware,
)
from cvt_sdk.producer.adapters.flask import (
    WSGIMiddleware,
    create_flask_middleware,
)


@pytest.fixture
def mock_validator():
    """Create a mock validator."""
    validator = MagicMock()
    validator.validate = MagicMock(return_value={"valid": True, "errors": []})
    return validator


@pytest.fixture
def producer_config(mock_validator):
    """Create a producer config with mock validator."""
    return ProducerConfig(
        schema_id="test-api",
        validator=mock_validator,
        mode=ValidationMode.WARN,
    )


@pytest.fixture
def strict_config(mock_validator):
    """Create a strict mode producer config."""
    return ProducerConfig(
        schema_id="test-api",
        validator=mock_validator,
        mode=ValidationMode.STRICT,
    )


@pytest.fixture
def shadow_config(mock_validator):
    """Create a shadow mode producer config."""
    return ProducerConfig(
        schema_id="test-api",
        validator=mock_validator,
        mode=ValidationMode.SHADOW,
    )


class TestASGIMiddleware:
    """Tests for ASGIMiddleware."""

    def test_init(self, producer_config):
        """Test middleware initialization."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        assert middleware.app == app
        assert middleware.config == producer_config
        assert middleware.producer is not None

    def test_parse_body_empty(self, producer_config):
        """Test parsing empty body."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        result = middleware._parse_body(b"")
        assert result is None

    def test_parse_body_json(self, producer_config):
        """Test parsing JSON body."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        result = middleware._parse_body(b'{"key": "value"}')
        assert result == {"key": "value"}

    def test_parse_body_invalid_json(self, producer_config):
        """Test parsing invalid JSON body."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        result = middleware._parse_body(b"not json")
        assert result == "not json"

    def test_parse_body_binary(self, producer_config):
        """Test parsing binary body with decode errors."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        # Invalid UTF-8 bytes
        result = middleware._parse_body(b"\xff\xfe")
        assert isinstance(result, str)

    @pytest.mark.asyncio
    async def test_call_non_http(self, producer_config):
        """Test middleware passes through non-HTTP requests."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        scope = {"type": "websocket"}
        receive = AsyncMock()
        send = AsyncMock()

        await middleware(scope, receive, send)

        app.assert_awaited_once_with(scope, receive, send)

    @pytest.mark.asyncio
    async def test_call_excluded_path(self, mock_validator):
        """Test middleware passes through excluded paths."""
        config = ProducerConfig(
            schema_id="test-api",
            validator=mock_validator,
            mode=ValidationMode.WARN,
            exclude_paths=["/health"],
        )
        app = AsyncMock()
        middleware = ASGIMiddleware(app, config)

        scope = {"type": "http", "path": "/health", "method": "GET", "headers": []}
        receive = AsyncMock()
        send = AsyncMock()

        await middleware(scope, receive, send)

        app.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_call_shadow_mode(self, shadow_config):
        """Test middleware in shadow mode."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, shadow_config)

        scope = {
            "type": "http",
            "path": "/api/test",
            "method": "GET",
            "headers": [],
            "query_string": b"",
        }

        receive = AsyncMock(
            return_value={"type": "http.request", "body": b"", "more_body": False}
        )
        send = AsyncMock()

        await middleware(scope, receive, send)
        app.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_send_error_response(self, producer_config):
        """Test sending error response."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        send = AsyncMock()
        result = {"valid": False, "errors": ["test error"]}

        await middleware._send_error_response(send, result)

        assert send.await_count == 2
        # First call should be response start
        first_call = send.await_args_list[0]
        assert first_call[0][0]["type"] == "http.response.start"
        assert first_call[0][0]["status"] == 400

    @pytest.mark.asyncio
    async def test_send_error_response_custom(self, producer_config):
        """Test sending custom error response."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        send = AsyncMock()
        result = {"valid": False, "errors": ["test error"]}
        custom_response = {"custom": "error"}

        await middleware._send_error_response(send, result, custom_response)

        assert send.await_count == 2
        # Check body contains custom response
        second_call = send.await_args_list[1]
        body = json.loads(second_call[0][0]["body"])
        assert body == {"custom": "error"}

    @pytest.mark.asyncio
    async def test_validate_request_async_success(self, producer_config):
        """Test async request validation success."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        with patch.object(
            middleware.producer, "validate_request", new_callable=AsyncMock
        ) as mock_validate:
            mock_validate.return_value = {"valid": True, "errors": []}

            await middleware._validate_request_async("GET", "/test", {}, None)

            mock_validate.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_validate_request_async_failure(self, producer_config):
        """Test async request validation failure."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        with (
            patch.object(
                middleware.producer, "validate_request", new_callable=AsyncMock
            ) as mock_validate,
            patch.object(
                middleware.producer, "handle_request_failure", new_callable=AsyncMock
            ) as mock_handle,
        ):
            mock_validate.return_value = {"valid": False, "errors": ["error"]}

            await middleware._validate_request_async("GET", "/test", {}, None)

            mock_handle.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_validate_request_async_exception(self, producer_config):
        """Test async request validation exception handling."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        with patch.object(
            middleware.producer, "validate_request", new_callable=AsyncMock
        ) as mock_validate:
            mock_validate.side_effect = Exception("test error")

            # Should not raise
            await middleware._validate_request_async("GET", "/test", {}, None)

    @pytest.mark.asyncio
    async def test_validate_response_async_success(self, producer_config):
        """Test async response validation success."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        with patch.object(
            middleware.producer, "validate_response", new_callable=AsyncMock
        ) as mock_validate:
            mock_validate.return_value = {"valid": True, "errors": []}

            await middleware._validate_response_async(
                "GET", "/test", {}, None, 200, {}, None
            )

            mock_validate.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_validate_response_async_failure(self, producer_config):
        """Test async response validation failure."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        with (
            patch.object(
                middleware.producer, "validate_response", new_callable=AsyncMock
            ) as mock_validate,
            patch.object(
                middleware.producer, "handle_response_failure", new_callable=AsyncMock
            ) as mock_handle,
        ):
            mock_validate.return_value = {"valid": False, "errors": ["error"]}

            await middleware._validate_response_async(
                "GET", "/test", {}, None, 200, {}, None
            )

            mock_handle.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_validate_response_async_exception(self, producer_config):
        """Test async response validation exception handling."""
        app = AsyncMock()
        middleware = ASGIMiddleware(app, producer_config)

        with patch.object(
            middleware.producer, "validate_response", new_callable=AsyncMock
        ) as mock_validate:
            mock_validate.side_effect = Exception("test error")

            # Should not raise
            await middleware._validate_response_async(
                "GET", "/test", {}, None, 200, {}, None
            )


class TestCreateFastAPIMiddleware:
    """Tests for create_fastapi_middleware factory."""

    def test_creates_configured_middleware(self, producer_config):
        """Test that factory creates configured middleware class."""
        MiddlewareClass = create_fastapi_middleware(producer_config)

        app = AsyncMock()
        middleware = MiddlewareClass(app)

        assert isinstance(middleware, ASGIMiddleware)
        assert middleware.config == producer_config


class TestWSGIMiddleware:
    """Tests for WSGIMiddleware."""

    def test_init(self, producer_config):
        """Test middleware initialization."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        assert middleware.app == app
        assert middleware.config == producer_config
        assert middleware.producer is not None

    def test_parse_body_empty(self, producer_config):
        """Test parsing empty body."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        result = middleware._parse_body(b"")
        assert result is None

    def test_parse_body_json(self, producer_config):
        """Test parsing JSON body."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        result = middleware._parse_body(b'{"key": "value"}')
        assert result == {"key": "value"}

    def test_parse_body_invalid_json(self, producer_config):
        """Test parsing invalid JSON body."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        result = middleware._parse_body(b"not json")
        assert result == "not json"

    def test_get_headers(self, producer_config):
        """Test extracting headers from WSGI environ."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        environ = {
            "HTTP_CONTENT_TYPE": "application/json",
            "HTTP_AUTHORIZATION": "Bearer token",
            "CONTENT_TYPE": "application/json",
            "CONTENT_LENGTH": "100",
            "SERVER_NAME": "localhost",  # Should be ignored
        }

        headers = middleware._get_headers(environ)

        assert headers["content-type"] == "application/json"
        assert headers["authorization"] == "Bearer token"
        assert headers["content-length"] == "100"
        assert "server-name" not in headers

    def test_send_error_response(self, producer_config):
        """Test sending error response."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        start_response = MagicMock()
        result = {"valid": False, "errors": ["test error"]}

        response_parts = list(middleware._send_error_response(start_response, result))

        start_response.assert_called_once()
        call_args = start_response.call_args
        assert call_args[0][0] == "400 Bad Request"
        assert len(response_parts) == 1
        body_data = json.loads(response_parts[0])
        assert body_data["error"] == "Request validation failed"

    def test_send_error_response_custom(self, producer_config):
        """Test sending custom error response."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        start_response = MagicMock()
        result = {"valid": False, "errors": ["test error"]}
        custom_response = {"custom": "error"}

        response_parts = list(
            middleware._send_error_response(start_response, result, custom_response)
        )

        assert len(response_parts) == 1
        body_data = json.loads(response_parts[0])
        assert body_data == {"custom": "error"}

    def test_call_excluded_path(self, mock_validator):
        """Test middleware passes through excluded paths."""
        config = ProducerConfig(
            schema_id="test-api",
            validator=mock_validator,
            mode=ValidationMode.WARN,
            exclude_paths=["/health"],
        )
        app = MagicMock(return_value=[b"response"])
        middleware = WSGIMiddleware(app, config)

        environ = {
            "REQUEST_METHOD": "GET",
            "PATH_INFO": "/health",
            "QUERY_STRING": "",
        }
        start_response = MagicMock()

        list(middleware(environ, start_response))

        app.assert_called_once()

    def test_call_shadow_mode(self, shadow_config):
        """Test middleware in shadow mode passes through."""
        app = MagicMock(return_value=[b"response"])
        middleware = WSGIMiddleware(app, shadow_config)

        from io import BytesIO

        environ = {
            "REQUEST_METHOD": "GET",
            "PATH_INFO": "/api/test",
            "QUERY_STRING": "",
            "wsgi.input": BytesIO(b""),
            "CONTENT_LENGTH": "0",
        }
        start_response = MagicMock()

        list(middleware(environ, start_response))

        app.assert_called_once()

    def test_get_loop(self, producer_config):
        """Test getting event loop."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        loop = middleware._get_loop()
        assert loop is not None

    def test_get_loop_reuses_existing(self, producer_config):
        """Test that _get_loop reuses existing loop."""
        app = MagicMock()
        middleware = WSGIMiddleware(app, producer_config)

        loop1 = middleware._get_loop()
        loop2 = middleware._get_loop()
        assert loop1 is loop2


class TestCreateFlaskMiddleware:
    """Tests for create_flask_middleware factory."""

    def test_creates_configured_middleware(self, producer_config):
        """Test that factory creates configured middleware wrapper."""
        wrapper = create_flask_middleware(producer_config)

        # The factory returns a callable
        assert callable(wrapper)

        # Call the wrapper with an app to get the middleware
        app = MagicMock()
        middleware = wrapper(app)

        assert isinstance(middleware, WSGIMiddleware)
        assert middleware.config == producer_config
