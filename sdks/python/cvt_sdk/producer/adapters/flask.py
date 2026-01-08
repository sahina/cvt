"""Flask/WSGI middleware for producer validation."""

import asyncio
import json
import logging
from io import BytesIO
from typing import Any, Callable, Iterator, Optional

from ..config import ProducerConfig, ValidationMode
from ..producer import Producer

logger = logging.getLogger(__name__)

# Type alias for WSGI apps
WSGIApp = Callable[[dict[str, Any], Callable], Iterator[bytes]]


class WSGIMiddleware:
    """
    WSGI middleware for producer validation.
    Works with Flask, Django, and other WSGI frameworks.

    Example:
        >>> from flask import Flask
        >>> from cvt_sdk import ContractValidator
        >>> from cvt_sdk.producer import ProducerConfig, ValidationMode
        >>> from cvt_sdk.producer.adapters import WSGIMiddleware
        >>>
        >>> app = Flask(__name__)
        >>> validator = ContractValidator()
        >>> validator.register_schema("my-api", "./openapi.json")
        >>>
        >>> config = ProducerConfig(
        ...     schema_id="my-api",
        ...     validator=validator,
        ...     mode=ValidationMode.STRICT,
        ... )
        >>> app.wsgi_app = WSGIMiddleware(app.wsgi_app, config)
    """

    def __init__(self, app: WSGIApp, config: ProducerConfig):
        """
        Initialize the WSGI middleware.

        Args:
            app: The WSGI application to wrap.
            config: Producer configuration.
        """
        self.app = app
        self.producer = Producer(config)
        self.config = config
        self._log_prefix = config.log_prefix
        # Create event loop for async operations
        self._loop: Optional[asyncio.AbstractEventLoop] = None

    def _get_loop(self) -> asyncio.AbstractEventLoop:
        """Get or create an event loop."""
        try:
            return asyncio.get_running_loop()
        except RuntimeError:
            if self._loop is None or self._loop.is_closed():
                self._loop = asyncio.new_event_loop()
            return self._loop

    def _run_async(self, coro):
        """Run an async coroutine in the event loop."""
        loop = self._get_loop()
        if loop.is_running():
            # If there's already a running loop, create a task
            future = asyncio.ensure_future(coro, loop=loop)
            return future
        else:
            return loop.run_until_complete(coro)

    def __call__(
        self,
        environ: dict[str, Any],
        start_response: Callable,
    ) -> Iterator[bytes]:
        """Handle WSGI request."""
        # Get path for filtering
        path = environ.get("PATH_INFO", "/")
        query_string = environ.get("QUERY_STRING", "")
        if query_string:
            path = f"{path}?{query_string}"

        # Check path filters
        if not self.producer.should_validate_path(path):
            return self.app(environ, start_response)

        # Get request info
        method = environ.get("REQUEST_METHOD", "GET")
        headers = self._get_headers(environ)

        # Read request body
        content_length = environ.get("CONTENT_LENGTH", "")
        if content_length:
            try:
                content_length = int(content_length)
            except ValueError:
                content_length = 0
        else:
            content_length = 0

        body = b""
        if content_length > 0:
            body = environ["wsgi.input"].read(content_length)
            # Reset input stream for the wrapped app
            environ["wsgi.input"] = BytesIO(body)

        req_body = self._parse_body(body)

        # Shadow mode: don't block on validation
        if self.config.mode == ValidationMode.SHADOW:
            if self.config.validate_request:
                # Fire and forget validation - run in background
                try:
                    self._run_async(
                        self.producer.validate_request(method, path, headers, req_body)
                    )
                except Exception as e:
                    logger.debug(
                        f"[{self._log_prefix}] Shadow mode request validation: {e}"
                    )

            # Capture response
            response_started = False
            response_status = 200
            response_headers: dict[str, str] = {}

            def capture_start_response(status: str, response_headers_list: list):
                nonlocal response_started, response_status, response_headers
                response_started = True
                response_status = int(status.split(" ")[0])
                response_headers = dict(response_headers_list)
                return start_response(status, response_headers_list)

            response_body_parts: list[bytes] = []
            for chunk in self.app(environ, capture_start_response):
                response_body_parts.append(chunk)
                yield chunk

            # Validate response asynchronously (fire and forget)
            if self.config.validate_response and response_started:
                resp_body = self._parse_body(b"".join(response_body_parts))
                try:
                    self._run_async(
                        self.producer.validate_response(
                            method,
                            path,
                            headers,
                            req_body,
                            response_status,
                            response_headers,
                            resp_body,
                        )
                    )
                except Exception as e:
                    logger.error(f"[{self._log_prefix}] Response validation error: {e}")
            return

        # Strict/Warn mode: validate request first
        if self.config.validate_request:
            try:
                result = self._run_async(
                    self.producer.validate_request(method, path, headers, req_body)
                )

                if not result["valid"]:
                    should_continue, custom_response = self._run_async(
                        self.producer.handle_request_failure(result, environ)
                    )
                    if not should_continue:
                        # Send error response
                        yield from self._send_error_response(
                            start_response, result, custom_response
                        )
                        return
            except Exception as e:
                logger.error(f"[{self._log_prefix}] Request validation error: {e}")
                # Continue on validation errors

        # Capture response
        response_started = False
        response_status = 200
        response_headers: dict[str, str] = {}

        def capture_start_response(status: str, response_headers_list: list):
            nonlocal response_started, response_status, response_headers
            response_started = True
            response_status = int(status.split(" ")[0])
            response_headers = dict(response_headers_list)
            return start_response(status, response_headers_list)

        response_body_parts: list[bytes] = []
        for chunk in self.app(environ, capture_start_response):
            response_body_parts.append(chunk)
            yield chunk

        # Validate response
        if self.config.validate_response and response_started:
            resp_body = self._parse_body(b"".join(response_body_parts))
            try:
                result = self._run_async(
                    self.producer.validate_response(
                        method,
                        path,
                        headers,
                        req_body,
                        response_status,
                        response_headers,
                        resp_body,
                    )
                )

                if not result["valid"]:
                    self._run_async(
                        self.producer.handle_response_failure(result, environ, None)
                    )
            except Exception as e:
                logger.error(f"[{self._log_prefix}] Response validation error: {e}")

    def _get_headers(self, environ: dict[str, Any]) -> dict[str, str]:
        """Extract headers from WSGI environ."""
        headers: dict[str, str] = {}
        for key, value in environ.items():
            if key.startswith("HTTP_"):
                # Convert HTTP_CONTENT_TYPE to content-type
                header_name = key[5:].lower().replace("_", "-")
                headers[header_name] = value
            elif key in ("CONTENT_TYPE", "CONTENT_LENGTH"):
                header_name = key.lower().replace("_", "-")
                headers[header_name] = value
        return headers

    def _parse_body(self, body: bytes) -> Any:
        """Parse body bytes to Python object."""
        if not body:
            return None
        try:
            return json.loads(body)
        except (json.JSONDecodeError, UnicodeDecodeError):
            return body.decode("utf-8", errors="replace")

    def _send_error_response(
        self,
        start_response: Callable,
        result: dict[str, Any],
        custom_response: Optional[Any] = None,
    ) -> Iterator[bytes]:
        """Send an error response for failed validation."""
        if custom_response is not None:
            body = json.dumps(custom_response).encode()
        else:
            body = json.dumps(
                {
                    "error": "Request validation failed",
                    "details": result.get("errors", []),
                }
            ).encode()

        status = "400 Bad Request"
        headers = [
            ("Content-Type", "application/json"),
            ("Content-Length", str(len(body))),
        ]

        start_response(status, headers)
        yield body


def create_flask_middleware(
    config: ProducerConfig,
) -> Callable[[WSGIApp], WSGIMiddleware]:
    """
    Creates a Flask middleware wrapper for producer validation.

    Example:
        >>> from flask import Flask
        >>> from cvt_sdk import ContractValidator
        >>> from cvt_sdk.producer import ProducerConfig
        >>> from cvt_sdk.producer.adapters import create_flask_middleware
        >>>
        >>> app = Flask(__name__)
        >>> validator = ContractValidator()
        >>> validator.register_schema("my-api", "./openapi.json")
        >>>
        >>> wrapper = create_flask_middleware(ProducerConfig(
        ...     schema_id="my-api",
        ...     validator=validator,
        ... ))
        >>> app.wsgi_app = wrapper(app.wsgi_app)

    Args:
        config: Producer configuration.

    Returns:
        A callable that wraps a WSGI app with validation middleware.
    """

    def wrapper(app: WSGIApp) -> WSGIMiddleware:
        return WSGIMiddleware(app, config)

    return wrapper
