"""FastAPI/ASGI middleware for producer validation."""

import asyncio
import json
import logging
from typing import Any, Awaitable, Callable, Optional

from ..config import ProducerConfig, ValidationMode
from ..producer import Producer

logger = logging.getLogger(__name__)

# Type alias for ASGI apps
ASGIApp = Callable[
    [dict[str, Any], Callable[[], Awaitable[dict]], Callable[[dict], Awaitable[None]]],
    Awaitable[None],
]


class ASGIMiddleware:
    """
    ASGI middleware for producer validation.
    Works with FastAPI, Starlette, and other ASGI frameworks.

    Example:
        >>> from fastapi import FastAPI
        >>> from cvt_sdk import ContractValidator
        >>> from cvt_sdk.producer import ProducerConfig, ValidationMode
        >>> from cvt_sdk.producer.adapters import ASGIMiddleware
        >>>
        >>> app = FastAPI()
        >>> validator = ContractValidator()
        >>> validator.register_schema("my-api", "./openapi.json")
        >>>
        >>> config = ProducerConfig(
        ...     schema_id="my-api",
        ...     validator=validator,
        ...     mode=ValidationMode.STRICT,
        ... )
        >>> app.add_middleware(ASGIMiddleware, config=config)
    """

    def __init__(self, app: ASGIApp, config: ProducerConfig):
        """
        Initialize the ASGI middleware.

        Args:
            app: The ASGI application to wrap.
            config: Producer configuration.
        """
        self.app = app
        self.producer = Producer(config)
        self.config = config
        self._log_prefix = config.log_prefix

    async def __call__(
        self,
        scope: dict[str, Any],
        receive: Callable[[], Awaitable[dict]],
        send: Callable[[dict], Awaitable[None]],
    ) -> None:
        """Handle ASGI request."""
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        # Get path for filtering
        path = scope.get("path", "/")
        query_string = scope.get("query_string", b"").decode()
        if query_string:
            path = f"{path}?{query_string}"

        # Check path filters
        if not self.producer.should_validate_path(path):
            await self.app(scope, receive, send)
            return

        # Get request info
        method = scope.get("method", "GET")
        headers = dict((k.decode(), v.decode()) for k, v in scope.get("headers", []))

        # Capture request body
        body_parts: list[bytes] = []

        async def receive_wrapper() -> dict:
            message = await receive()
            if message["type"] == "http.request":
                body_parts.append(message.get("body", b""))
            return message

        # Parse request body
        req_body: Any = None

        # Shadow mode: don't block on request validation
        if self.config.mode == ValidationMode.SHADOW:
            if self.config.validate_request:
                # Validate asynchronously
                asyncio.create_task(
                    self._validate_request_async(method, path, headers, req_body)
                )

            # Capture response and validate
            response_started = False
            response_status = 200
            response_headers: dict[str, str] = {}
            response_body_parts: list[bytes] = []

            async def send_wrapper(message: dict) -> None:
                nonlocal response_started, response_status, response_headers

                if message["type"] == "http.response.start":
                    response_started = True
                    response_status = message.get("status", 200)
                    response_headers = dict(
                        (k.decode(), v.decode()) for k, v in message.get("headers", [])
                    )
                elif message["type"] == "http.response.body":
                    body = message.get("body", b"")
                    if body:
                        response_body_parts.append(body)

                await send(message)

            await self.app(scope, receive_wrapper, send_wrapper)

            # Validate response asynchronously
            if self.config.validate_response and response_started:
                resp_body = self._parse_body(b"".join(response_body_parts))
                asyncio.create_task(
                    self._validate_response_async(
                        method,
                        path,
                        headers,
                        req_body,
                        response_status,
                        response_headers,
                        resp_body,
                    )
                )
            return

        # Strict/Warn mode: validate request first
        if self.config.validate_request:
            # We need to consume the body first for validation
            # Read all body chunks
            body = b""
            while True:
                message = await receive()
                if message["type"] == "http.request":
                    body += message.get("body", b"")
                    if not message.get("more_body", False):
                        break
                elif message["type"] == "http.disconnect":
                    return

            req_body = self._parse_body(body)

            # Validate request
            result = await self.producer.validate_request(
                method, path, headers, req_body
            )

            if not result["valid"]:
                (
                    should_continue,
                    custom_response,
                ) = await self.producer.handle_request_failure(result, scope)
                if not should_continue:
                    # Send error response
                    await self._send_error_response(send, result, custom_response)
                    return

            # Create a new receive that returns the already-read body
            body_sent = False

            async def receive_with_body() -> dict:
                nonlocal body_sent
                if not body_sent:
                    body_sent = True
                    return {"type": "http.request", "body": body, "more_body": False}
                return await receive()

            receive = receive_with_body

        # Capture response for validation
        response_started = False
        response_status = 200
        response_headers: dict[str, str] = {}
        response_body_parts: list[bytes] = []

        async def send_wrapper(message: dict) -> None:
            nonlocal response_started, response_status, response_headers

            if message["type"] == "http.response.start":
                response_started = True
                response_status = message.get("status", 200)
                response_headers = dict(
                    (k.decode(), v.decode()) for k, v in message.get("headers", [])
                )
            elif message["type"] == "http.response.body":
                body = message.get("body", b"")
                if body:
                    response_body_parts.append(body)

            await send(message)

        await self.app(scope, receive, send_wrapper)

        # Validate response
        if self.config.validate_response and response_started:
            resp_body = self._parse_body(b"".join(response_body_parts))
            result = await self.producer.validate_response(
                method,
                path,
                headers,
                req_body,
                response_status,
                response_headers,
                resp_body,
            )

            if not result["valid"]:
                await self.producer.handle_response_failure(result, scope, None)

    async def _validate_request_async(
        self,
        method: str,
        path: str,
        headers: dict[str, str],
        body: Any,
    ) -> None:
        """Validate request asynchronously (for shadow mode)."""
        try:
            result = await self.producer.validate_request(method, path, headers, body)
            if not result["valid"]:
                await self.producer.handle_request_failure(result, None)
        except Exception as e:
            logger.error(f"[{self._log_prefix}] Async request validation error: {e}")

    async def _validate_response_async(
        self,
        method: str,
        path: str,
        req_headers: dict[str, str],
        req_body: Any,
        status_code: int,
        resp_headers: dict[str, str],
        resp_body: Any,
    ) -> None:
        """Validate response asynchronously (for shadow mode)."""
        try:
            result = await self.producer.validate_response(
                method,
                path,
                req_headers,
                req_body,
                status_code,
                resp_headers,
                resp_body,
            )
            if not result["valid"]:
                await self.producer.handle_response_failure(result, None, None)
        except Exception as e:
            logger.error(f"[{self._log_prefix}] Async response validation error: {e}")

    async def _send_error_response(
        self,
        send: Callable[[dict], Awaitable[None]],
        result: dict[str, Any],
        custom_response: Optional[Any] = None,
    ) -> None:
        """Send an error response for failed validation."""
        if custom_response is not None:
            # Use custom response
            body = json.dumps(custom_response).encode()
            status = 400
        else:
            # Use default error response
            body = json.dumps(
                {
                    "error": "Request validation failed",
                    "details": result.get("errors", []),
                }
            ).encode()
            status = 400

        await send(
            {
                "type": "http.response.start",
                "status": status,
                "headers": [(b"content-type", b"application/json")],
            }
        )
        await send(
            {
                "type": "http.response.body",
                "body": body,
            }
        )

    def _parse_body(self, body: bytes) -> Any:
        """Parse body bytes to Python object."""
        if not body:
            return None
        try:
            return json.loads(body)
        except (json.JSONDecodeError, UnicodeDecodeError):
            return body.decode("utf-8", errors="replace")


def create_fastapi_middleware(config: ProducerConfig) -> type:
    """
    Creates a FastAPI middleware class for producer validation.

    This is a convenience function for FastAPI's add_middleware pattern.

    Example:
        >>> from fastapi import FastAPI
        >>> from cvt_sdk import ContractValidator
        >>> from cvt_sdk.producer import ProducerConfig
        >>> from cvt_sdk.producer.adapters import create_fastapi_middleware
        >>>
        >>> app = FastAPI()
        >>> validator = ContractValidator()
        >>> validator.register_schema("my-api", "./openapi.json")
        >>>
        >>> MiddlewareClass = create_fastapi_middleware(ProducerConfig(
        ...     schema_id="my-api",
        ...     validator=validator,
        ... ))
        >>> app.add_middleware(MiddlewareClass)

    Args:
        config: Producer configuration.

    Returns:
        A middleware class that can be used with FastAPI.
    """

    class ConfiguredMiddleware(ASGIMiddleware):
        def __init__(self, app: ASGIApp):
            super().__init__(app, config)

    return ConfiguredMiddleware
