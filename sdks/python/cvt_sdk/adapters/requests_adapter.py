"""CVT adapter for the requests library."""

import json
from datetime import datetime
from typing import Any, Callable, Optional
from urllib.parse import urlparse

from requests import PreparedRequest, Response, Session

from .. import (
    ContractValidator,
    ValidationRequest,
    ValidationResponse,
    ValidationResult,
)
from .types import CapturedInteraction, PathFilter, should_validate_path


class ContractValidatingSession(Session):
    """
    A requests.Session that automatically validates HTTP interactions against CVT.

    This is a drop-in replacement for requests.Session that intercepts all HTTP
    traffic and validates it against a registered OpenAPI schema.

    Example:
        >>> from cvt_sdk import ContractValidator
        >>> from cvt_sdk.adapters import ContractValidatingSession
        >>>
        >>> validator = ContractValidator()
        >>> validator.register_schema('api', './openapi.json')
        >>>
        >>> session = ContractValidatingSession(validator)
        >>> response = session.post('http://localhost:3000/pet', json={'name': 'Fluffy'})
        >>>
        >>> # Check captured interactions
        >>> interactions = session.get_interactions()
        >>> assert interactions[0].validation_result['valid']

    Args:
        validator: CVT validator instance with registered schema.
        auto_validate: Whether to automatically validate each request (default: True).
        on_validation_failure: Callback when validation fails. If None, raises AssertionError.
        include_paths: Only validate requests matching these patterns.
        exclude_paths: Exclude requests matching these patterns (evaluated first).
    """

    def __init__(
        self,
        validator: ContractValidator,
        *,
        auto_validate: bool = True,
        on_validation_failure: Optional[
            Callable[[ValidationResult, Any, Any], None]
        ] = None,
        include_paths: Optional[list[PathFilter]] = None,
        exclude_paths: Optional[list[PathFilter]] = None,
    ):
        super().__init__()
        self._validator = validator
        self._auto_validate = auto_validate
        self._on_validation_failure = (
            on_validation_failure or self._default_failure_handler
        )
        self._include_paths: list[PathFilter] = include_paths or []
        self._exclude_paths: list[PathFilter] = exclude_paths or []
        self._captured_interactions: list[CapturedInteraction] = []

    def request(self, method: str, url: str, **kwargs) -> Response:
        """Override request to capture and validate interactions."""
        # Store the original data before request modifies it
        original_data = kwargs.get("data")
        original_json = kwargs.get("json")

        response = super().request(method, url, **kwargs)

        # Get the path from the prepared request
        prepared_request = response.request
        parsed_url = urlparse(prepared_request.path_url)
        path = parsed_url.path
        if parsed_url.query:
            path += "?" + parsed_url.query

        if should_validate_path(path, self._include_paths, self._exclude_paths):
            self._handle_interaction(
                prepared_request, response, original_data, original_json
            )

        return response

    def _handle_interaction(
        self,
        request: PreparedRequest,
        response: Response,
        original_data: Any,
        original_json: Any,
    ) -> None:
        """Process and optionally validate a captured interaction."""
        validation_request = self._extract_request(
            request, original_data, original_json
        )
        validation_response = self._extract_response(response)

        interaction = CapturedInteraction(
            request=validation_request,
            response=validation_response,
            timestamp=datetime.now(),
        )

        if self._auto_validate:
            try:
                result = self._validator.validate(
                    validation_request, validation_response
                )
                interaction.validation_result = result

                if not result["valid"]:
                    self._on_validation_failure(result, request, response)
            except Exception as e:
                # Re-throw validation failures from callback
                if "Contract validation failed" in str(e):
                    raise
                # Swallow other validation errors but log them
                import sys

                print(f"CVT validation error: {e}", file=sys.stderr)

        self._captured_interactions.append(interaction)

    def _extract_request(
        self,
        request: PreparedRequest,
        original_data: Any,
        original_json: Any,
    ) -> ValidationRequest:
        """Extract ValidationRequest from PreparedRequest."""
        parsed = urlparse(request.path_url)
        path = parsed.path
        if parsed.query:
            path += "?" + parsed.query

        # Parse body
        body = None
        if original_json is not None:
            body = original_json
        elif original_data is not None:
            if isinstance(original_data, (str, bytes)):
                try:
                    body = json.loads(original_data)
                except (json.JSONDecodeError, TypeError):
                    body = (
                        original_data
                        if isinstance(original_data, str)
                        else original_data.decode("utf-8", errors="replace")
                    )
            else:
                body = original_data
        elif request.body:
            try:
                body_str = (
                    request.body
                    if isinstance(request.body, str)
                    else request.body.decode("utf-8", errors="replace")
                )
                body = json.loads(body_str)
            except (json.JSONDecodeError, TypeError, AttributeError):
                pass

        headers = {k.lower(): v for k, v in (request.headers or {}).items()}

        return ValidationRequest(
            method=request.method or "GET",
            path=path,
            headers=headers,
            body=body,
        )

    def _extract_response(self, response: Response) -> ValidationResponse:
        """Extract ValidationResponse from Response."""
        body = None
        try:
            body = response.json()
        except Exception:
            if response.text:
                body = response.text

        headers = {k.lower(): v for k, v in response.headers.items()}

        return ValidationResponse(
            status_code=response.status_code,
            headers=headers,
            body=body,
        )

    def _default_failure_handler(
        self, result: ValidationResult, request: Any, response: Any
    ) -> None:
        """Default handler for validation failures - raises AssertionError."""
        errors = ", ".join(result.get("errors", []))
        raise AssertionError(f"Contract validation failed: {errors}")

    def get_interactions(self) -> list[CapturedInteraction]:
        """Get all captured interactions."""
        return list(self._captured_interactions)

    def clear_interactions(self) -> None:
        """Clear captured interactions."""
        self._captured_interactions.clear()

    def validate_interaction(
        self, interaction: CapturedInteraction
    ) -> ValidationResult:
        """Manually validate a captured interaction."""
        return self._validator.validate(interaction.request, interaction.response)


def create_validating_session(
    validator: ContractValidator,
    **kwargs,
) -> ContractValidatingSession:
    """
    Factory function to create a contract-validating session.

    Args:
        validator: CVT validator instance with registered schema.
        **kwargs: Additional configuration options (auto_validate, include_paths, etc.)

    Returns:
        ContractValidatingSession instance.

    Example:
        >>> session = create_validating_session(
        ...     validator,
        ...     auto_validate=True,
        ...     exclude_paths=['/health', '/metrics'],
        ... )
    """
    return ContractValidatingSession(validator, **kwargs)
