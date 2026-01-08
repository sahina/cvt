"""Core producer validation logic."""

import logging
from typing import Any, Optional

from .config import (
    Interaction,
    ProducerConfig,
    ValidationMode,
    ValidationResult,
    should_validate_path,
)

logger = logging.getLogger(__name__)


class Producer:
    """Producer handles server-side validation of HTTP requests and responses."""

    def __init__(self, config: ProducerConfig):
        """
        Initialize the producer with configuration.

        Args:
            config: Producer configuration including validator and mode.

        Raises:
            ValueError: If schema_id or validator is not provided.
        """
        if not config.schema_id:
            raise ValueError("schema_id is required")
        if not config.validator:
            raise ValueError("validator is required")

        self.config = config
        self._log_prefix = config.log_prefix

    def should_validate_path(self, path: str) -> bool:
        """Check if a path should be validated."""
        return should_validate_path(
            path,
            self.config.include_paths,
            self.config.exclude_paths,
        )

    async def validate_request(
        self,
        method: str,
        path: str,
        headers: Optional[dict[str, str]] = None,
        body: Any = None,
    ) -> ValidationResult:
        """
        Validates an incoming HTTP request.

        Args:
            method: HTTP method (GET, POST, etc.)
            path: Request path including query string
            headers: Request headers
            body: Request body (parsed JSON or raw)

        Returns:
            ValidationResult with valid flag and any errors.
        """
        if not self.config.validate_request:
            return ValidationResult(valid=True, errors=[], type="request")

        interaction: Interaction = {
            "method": method,
            "path": path,
            "headers": headers or {},
            "body": body,
            # Minimal valid response for request-only validation
            "status_code": 200,
            "response_body": {},
        }

        result = await self._validate(interaction)
        result["type"] = "request"
        return result

    async def validate_response(
        self,
        method: str,
        path: str,
        req_headers: Optional[dict[str, str]] = None,
        req_body: Any = None,
        status_code: int = 200,
        resp_headers: Optional[dict[str, str]] = None,
        resp_body: Any = None,
    ) -> ValidationResult:
        """
        Validates an outgoing HTTP response.

        Args:
            method: HTTP method of the original request
            path: Request path including query string
            req_headers: Original request headers
            req_body: Original request body
            status_code: Response status code
            resp_headers: Response headers
            resp_body: Response body

        Returns:
            ValidationResult with valid flag and any errors.
        """
        if not self.config.validate_response:
            return ValidationResult(valid=True, errors=[], type="response")

        interaction: Interaction = {
            "method": method,
            "path": path,
            "headers": req_headers or {},
            "body": req_body,
            "status_code": status_code,
            "response_headers": resp_headers or {},
            "response_body": resp_body,
        }

        result = await self._validate(interaction)
        result["type"] = "response"
        return result

    async def _validate(self, interaction: Interaction) -> ValidationResult:
        """
        Performs the actual validation using the configured validator.

        Args:
            interaction: The HTTP interaction to validate.

        Returns:
            ValidationResult with valid flag and any errors.
        """
        validator = self.config.validator

        # Check if it's a ContractValidator (gRPC client)
        if hasattr(validator, "validate") and hasattr(validator, "register_schema"):
            # ContractValidator - use its validate method
            try:
                # Build request/response for ContractValidator.validate()
                request = {
                    "method": interaction.get("method", "GET"),
                    "path": interaction.get("path", "/"),
                    "headers": interaction.get("headers", {}),
                    "body": interaction.get("body"),
                }
                response = {
                    "status_code": interaction.get("status_code", 200),
                    "headers": interaction.get("response_headers", {}),
                    "body": interaction.get("response_body"),
                }
                result = validator.validate(request, response)
                return ValidationResult(
                    valid=result["valid"],
                    errors=result.get("errors", []),
                    type="",
                )
            except Exception as e:
                logger.error(f"[{self._log_prefix}] Validation error: {e}")
                return ValidationResult(valid=False, errors=[str(e)], type="")
        elif hasattr(validator, "validate"):
            # Custom Validator protocol
            try:
                result = validator.validate(self.config.schema_id, interaction)
                return ValidationResult(
                    valid=result["valid"],
                    errors=result.get("errors", []),
                    type="",
                )
            except Exception as e:
                logger.error(f"[{self._log_prefix}] Validation error: {e}")
                return ValidationResult(valid=False, errors=[str(e)], type="")
        else:
            raise ValueError("Invalid validator type")

    async def handle_request_failure(
        self,
        result: ValidationResult,
        request: Any,
    ) -> tuple[bool, Optional[Any]]:
        """
        Handles request validation failure based on mode.

        Args:
            result: The validation result.
            request: The original request object.

        Returns:
            Tuple of (should_continue, custom_response).
            If should_continue is False, the request should be rejected.
            If custom_response is not None, it should be sent instead of default.
        """
        # Call custom handler if configured
        if self.config.on_request_failure:
            try:
                custom_response = self.config.on_request_failure(result, request)
                if custom_response is not None:
                    return False, custom_response
            except Exception as e:
                logger.error(f"[{self._log_prefix}] Error in on_request_failure: {e}")

        mode = self.config.mode

        if mode == ValidationMode.STRICT:
            record_rejection()
            return False, None

        if mode == ValidationMode.WARN:
            self._log_validation_failure("request", request, result)
            return True, None

        if mode == ValidationMode.SHADOW:
            record_validation_metrics("request", result)
            return True, None

        return True, None

    async def handle_response_failure(
        self,
        result: ValidationResult,
        request: Any,
        response: Any,
    ) -> None:
        """
        Handles response validation failure based on mode.

        Args:
            result: The validation result.
            request: The original request object.
            response: The response object.
        """
        # Call custom handler if configured
        if self.config.on_response_failure:
            try:
                self.config.on_response_failure(result, request, response)
                return
            except Exception as e:
                logger.error(f"[{self._log_prefix}] Error in on_response_failure: {e}")

        mode = self.config.mode

        if mode == ValidationMode.STRICT or mode == ValidationMode.WARN:
            # Log the failure (can't modify response - already sent)
            self._log_validation_failure("response", request, result)
        elif mode == ValidationMode.SHADOW:
            # Metrics only
            record_validation_metrics("response", result)

    def _log_validation_failure(
        self,
        validation_type: str,
        request: Any,
        result: ValidationResult,
    ) -> None:
        """Logs a validation failure."""
        method = getattr(request, "method", "UNKNOWN")
        path = getattr(request, "url", getattr(request, "path", "UNKNOWN"))

        logger.warning(
            f"[{self._log_prefix}] {validation_type} validation failed for "
            f"{method} {path}: {result.get('errors', [])}"
        )

    @property
    def mode(self) -> ValidationMode:
        """Gets the validation mode."""
        return self.config.mode


# Simple metrics tracking
_metrics = {
    "request_validations": 0,
    "request_validations_passed": 0,
    "request_validations_failed": 0,
    "response_validations": 0,
    "response_validations_passed": 0,
    "response_validations_failed": 0,
    "requests_rejected": 0,
}


def record_validation_metrics(
    validation_type: str,
    result: ValidationResult,
) -> None:
    """Records validation metrics."""
    if validation_type == "request":
        _metrics["request_validations"] += 1
        if result["valid"]:
            _metrics["request_validations_passed"] += 1
        else:
            _metrics["request_validations_failed"] += 1
    else:
        _metrics["response_validations"] += 1
        if result["valid"]:
            _metrics["response_validations_passed"] += 1
        else:
            _metrics["response_validations_failed"] += 1


def record_rejection() -> None:
    """Records a request rejection."""
    _metrics["requests_rejected"] += 1


def get_metrics() -> dict[str, int]:
    """Gets the current metrics snapshot."""
    return _metrics.copy()


def reset_metrics() -> None:
    """Resets all metrics."""
    global _metrics
    _metrics = {
        "request_validations": 0,
        "request_validations_passed": 0,
        "request_validations_failed": 0,
        "response_validations": 0,
        "response_validations_passed": 0,
        "response_validations_failed": 0,
        "requests_rejected": 0,
    }
