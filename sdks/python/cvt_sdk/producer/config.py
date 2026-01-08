"""Producer validation configuration types."""

from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Callable, Optional, Pattern, Protocol, TypedDict, Union

# Path filter type - can be a string (substring match) or Pattern (regex match)
PathFilter = Union[str, Pattern[str]]


class ValidationMode(Enum):
    """Validation mode determines how failures are handled."""

    STRICT = "strict"
    """Reject invalid requests with 400/422 errors."""

    WARN = "warn"
    """Log violations but allow requests to proceed."""

    SHADOW = "shadow"
    """Validate asynchronously, record metrics only."""


class Interaction(TypedDict, total=False):
    """HTTP request/response pair for validation."""

    method: str
    path: str
    headers: dict[str, str]
    body: Any
    status_code: int
    response_headers: dict[str, str]
    response_body: Any


class ValidationResult(TypedDict):
    """Validation result from producer validation."""

    valid: bool
    errors: list[str]
    type: str  # "request" or "response"


class Validator(Protocol):
    """Protocol for validators that can validate interactions."""

    def validate(self, schema_id: str, interaction: Interaction) -> ValidationResult:
        """Validates an interaction against a schema."""
        ...


@dataclass
class ProducerConfig:
    """Configuration for producer-side validation middleware."""

    schema_id: str
    """The schema ID to validate against (required)."""

    validator: Any
    """The validator to use. Can be ContractValidator or custom Validator."""

    mode: ValidationMode = ValidationMode.STRICT
    """Validation mode determines how failures are handled."""

    validate_request: bool = True
    """Whether to validate incoming requests."""

    validate_response: bool = True
    """Whether to validate outgoing responses."""

    include_paths: list[PathFilter] = field(default_factory=list)
    """Only validate requests matching these patterns."""

    exclude_paths: list[PathFilter] = field(default_factory=list)
    """Exclude requests matching these patterns from validation."""

    on_request_failure: Optional[Callable[[ValidationResult, Any], Any]] = None
    """Called when request validation fails. Return response to override default."""

    on_response_failure: Optional[Callable[[ValidationResult, Any, Any], None]] = None
    """Called when response validation fails. Used for logging/alerting."""

    log_prefix: str = "cvt"
    """Prefix for log messages."""


def matches_path_filter(path: str, pattern: PathFilter) -> bool:
    """Check if a path matches a filter pattern."""
    if isinstance(pattern, str):
        return pattern in path
    return bool(pattern.search(path))


def should_validate_path(
    path: str,
    include_paths: list[PathFilter],
    exclude_paths: list[PathFilter],
) -> bool:
    """Determine if a path should be validated based on include/exclude filters."""
    # Check excludes first
    for pattern in exclude_paths:
        if matches_path_filter(path, pattern):
            return False

    # If includes specified, must match at least one
    if include_paths:
        for pattern in include_paths:
            if matches_path_filter(path, pattern):
                return True
        return False

    return True


# Default configuration values
DEFAULT_MODE = ValidationMode.STRICT
DEFAULT_VALIDATE_REQUEST = True
DEFAULT_VALIDATE_RESPONSE = True
