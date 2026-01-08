"""CVT mock adapter that generates responses from OpenAPI schema."""

import json
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Optional
from urllib.parse import urlparse

from .. import (
    ContractValidator,
    GenerateOptions,
    GeneratedResponse,
    ValidationRequest,
    ValidationResponse,
)
from .types import CapturedInteraction, PathFilter, should_validate_path


# HTTP status code to text mapping
STATUS_TEXT: dict[int, str] = {
    200: "OK",
    201: "Created",
    204: "No Content",
    400: "Bad Request",
    401: "Unauthorized",
    403: "Forbidden",
    404: "Not Found",
    500: "Internal Server Error",
}


@dataclass
class MockResponse:
    """
    Mock HTTP response that mimics requests.Response interface.

    This class provides a similar interface to requests.Response for use
    in tests where mock responses are generated from OpenAPI schema.
    """

    status_code: int
    headers: dict[str, str]
    _body: Any
    _content: bytes = field(default=b"", repr=False)
    _text: str = field(default="", repr=False)
    url: str = ""
    reason: str = ""
    ok: bool = True
    encoding: str = "utf-8"

    def __post_init__(self):
        """Initialize computed fields."""
        if self._body is not None:
            if isinstance(self._body, (dict, list)):
                self._text = json.dumps(self._body)
                self._content = self._text.encode(self.encoding)
            elif isinstance(self._body, str):
                self._text = self._body
                self._content = self._body.encode(self.encoding)
            elif isinstance(self._body, bytes):
                self._content = self._body
                self._text = self._body.decode(self.encoding, errors="replace")

        self.reason = STATUS_TEXT.get(self.status_code, "")
        self.ok = 200 <= self.status_code < 400

    @property
    def text(self) -> str:
        """Return response body as text."""
        return self._text

    @property
    def content(self) -> bytes:
        """Return response body as bytes."""
        return self._content

    def json(self) -> Any:
        """Parse response body as JSON."""
        if self._body is not None and isinstance(self._body, (dict, list)):
            return self._body
        return json.loads(self._text)

    def raise_for_status(self) -> None:
        """Raise an exception for error status codes."""
        if not self.ok:
            raise Exception(f"HTTP Error {self.status_code}: {self.reason}")


@dataclass
class MockSessionConfig:
    """Configuration for MockSession."""

    validator: ContractValidator
    cache: bool = False
    generate_options: Optional[GenerateOptions] = None
    include_paths: list[PathFilter] = field(default_factory=list)
    exclude_paths: list[PathFilter] = field(default_factory=list)


class MockSession:
    """
    Mock HTTP session that generates responses from OpenAPI schema.

    This is useful for testing consumers against OpenAPI schemas without
    requiring the producer API to be running.

    Example:
        >>> from cvt_sdk import ContractValidator
        >>> from cvt_sdk.adapters import MockSession
        >>>
        >>> validator = ContractValidator()
        >>> validator.register_schema('api', './openapi.json')
        >>>
        >>> session = MockSession(validator, cache=True)
        >>> response = session.get('http://mock.api/users/123')
        >>> data = response.json()
        >>>
        >>> # Check captured interactions
        >>> interactions = session.get_interactions()

    Args:
        validator: CVT validator instance with registered schema.
        cache: Whether to cache generated responses by method+path.
        generate_options: Options for response generation.
        include_paths: Only mock requests matching these patterns.
        exclude_paths: Exclude requests matching these patterns.
    """

    def __init__(
        self,
        validator: ContractValidator,
        *,
        cache: bool = False,
        generate_options: Optional[GenerateOptions] = None,
        include_paths: Optional[list[PathFilter]] = None,
        exclude_paths: Optional[list[PathFilter]] = None,
    ):
        self._validator = validator
        self._cache_enabled = cache
        self._generate_options = generate_options or {}
        self._include_paths: list[PathFilter] = include_paths or []
        self._exclude_paths: list[PathFilter] = exclude_paths or []
        self._captured_interactions: list[CapturedInteraction] = []
        self._response_cache: dict[str, GeneratedResponse] = {}

        # Request-level state (like requests.Session)
        self.headers: dict[str, str] = {}

    def request(
        self,
        method: str,
        url: str,
        *,
        params: Optional[dict] = None,
        data: Optional[Any] = None,
        json_body: Optional[Any] = None,
        headers: Optional[dict[str, str]] = None,
        **kwargs,
    ) -> MockResponse:
        """
        Generate a mock response from the schema.

        Args:
            method: HTTP method (GET, POST, etc.)
            url: Request URL
            params: Query parameters
            data: Request body (form data or raw)
            json_body: Request body as JSON
            headers: Request headers
            **kwargs: Additional arguments (ignored for compatibility)

        Returns:
            MockResponse with schema-generated data

        Raises:
            ValueError: If the path is excluded from mocking
        """
        method = method.upper()

        # Parse URL to extract path
        parsed = urlparse(url)
        path = parsed.path
        if parsed.query:
            path += "?" + parsed.query
        elif params:
            query_str = "&".join(f"{k}={v}" for k, v in params.items())
            path += "?" + query_str

        # Check if we should mock this path
        if not should_validate_path(path, self._include_paths, self._exclude_paths):
            raise ValueError(f'cvt: path "{path}" is excluded from mocking')

        # Generate or retrieve cached response
        generated = self._get_or_generate_response(method, path)

        # Build request body
        request_body = None
        if json_body is not None:
            request_body = json_body
        elif data is not None:
            if isinstance(data, str):
                try:
                    request_body = json.loads(data)
                except json.JSONDecodeError:
                    request_body = data
            else:
                request_body = data

        # Merge headers
        merged_headers = {**self.headers, **(headers or {})}
        merged_headers = {k.lower(): v for k, v in merged_headers.items()}

        # Build validation request
        validation_request = ValidationRequest(
            method=method,
            path=path,
            headers=merged_headers,
            body=request_body,
        )

        # Build validation response
        validation_response = ValidationResponse(
            status_code=generated.get("status_code", 200),
            headers=generated.get("headers", {}),
            body=generated.get("body"),
        )

        # Record interaction
        interaction = CapturedInteraction(
            request=validation_request,
            response=validation_response,
            timestamp=datetime.now(),
        )
        self._captured_interactions.append(interaction)

        # Build and return MockResponse
        return self._build_response(generated, url)

    def _get_or_generate_response(self, method: str, path: str) -> GeneratedResponse:
        """Get cached response or generate a new one."""
        cache_key = f"{method}:{path}"

        # Check cache
        if self._cache_enabled:
            cached = self._response_cache.get(cache_key)
            if cached is not None:
                return cached

        # Generate new response
        generated = self._validator.generate_response(
            method, path, self._generate_options
        )

        # Cache if enabled
        if self._cache_enabled:
            self._response_cache[cache_key] = generated

        return generated

    def _build_response(self, generated: GeneratedResponse, url: str) -> MockResponse:
        """Build a MockResponse from GeneratedResponse."""
        status_code = generated.get("status_code", 200)
        headers = generated.get("headers", {})
        body = generated.get("body")

        # Ensure content-type is set
        headers_lower = {k.lower(): v for k, v in headers.items()}
        if "content-type" not in headers_lower and body is not None:
            headers["content-type"] = "application/json"

        return MockResponse(
            status_code=status_code,
            headers=headers,
            _body=body,
            url=url,
        )

    def get(self, url: str, **kwargs) -> MockResponse:
        """Send a GET request."""
        return self.request("GET", url, **kwargs)

    def post(self, url: str, **kwargs) -> MockResponse:
        """Send a POST request."""
        return self.request("POST", url, **kwargs)

    def put(self, url: str, **kwargs) -> MockResponse:
        """Send a PUT request."""
        return self.request("PUT", url, **kwargs)

    def patch(self, url: str, **kwargs) -> MockResponse:
        """Send a PATCH request."""
        return self.request("PATCH", url, **kwargs)

    def delete(self, url: str, **kwargs) -> MockResponse:
        """Send a DELETE request."""
        return self.request("DELETE", url, **kwargs)

    def head(self, url: str, **kwargs) -> MockResponse:
        """Send a HEAD request."""
        return self.request("HEAD", url, **kwargs)

    def options(self, url: str, **kwargs) -> MockResponse:
        """Send an OPTIONS request."""
        return self.request("OPTIONS", url, **kwargs)

    def get_interactions(self) -> list[CapturedInteraction]:
        """Get all captured interactions."""
        return list(self._captured_interactions)

    def clear_interactions(self) -> None:
        """Clear captured interactions."""
        self._captured_interactions.clear()

    def clear_cache(self) -> None:
        """Clear the response cache."""
        self._response_cache.clear()


def create_mock_session(
    validator: ContractValidator,
    *,
    cache: bool = False,
    generate_options: Optional[GenerateOptions] = None,
    include_paths: Optional[list[PathFilter]] = None,
    exclude_paths: Optional[list[PathFilter]] = None,
) -> MockSession:
    """
    Factory function to create a mock session.

    This is the recommended way to create a mock session for testing.

    Args:
        validator: CVT validator instance with registered schema.
        cache: Whether to cache generated responses by method+path.
        generate_options: Options for response generation.
        include_paths: Only mock requests matching these patterns.
        exclude_paths: Exclude requests matching these patterns.

    Returns:
        MockSession instance.

    Example:
        >>> session = create_mock_session(
        ...     validator,
        ...     cache=True,
        ...     exclude_paths=['/health'],
        ... )
        >>> response = session.get('http://mock.api/users/123')
    """
    return MockSession(
        validator,
        cache=cache,
        generate_options=generate_options,
        include_paths=include_paths,
        exclude_paths=exclude_paths,
    )
