"""Shared types for CVT adapters."""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Callable, Optional, Pattern, Union

from .. import (
    ContractValidator,
    ValidationRequest,
    ValidationResponse,
    ValidationResult,
)

# Path filter type - can be a string (substring match) or Pattern (regex match)
PathFilter = Union[str, Pattern[str]]


@dataclass
class CapturedInteraction:
    """A captured HTTP interaction with optional validation result."""

    request: ValidationRequest
    response: ValidationResponse
    timestamp: datetime = field(default_factory=datetime.now)
    validation_result: Optional[ValidationResult] = None


@dataclass
class AdapterConfig:
    """Base configuration for CVT adapters."""

    validator: ContractValidator
    auto_validate: bool = True
    on_validation_failure: Optional[Callable[[ValidationResult, Any, Any], None]] = None
    include_paths: list[PathFilter] = field(default_factory=list)
    exclude_paths: list[PathFilter] = field(default_factory=list)


def matches_path_filter(path: str, pattern: PathFilter) -> bool:
    """Check if a path matches a filter pattern."""
    if isinstance(pattern, str):
        return pattern in path
    return bool(pattern.search(path))


def should_validate_path(
    path: str, include_paths: list[PathFilter], exclude_paths: list[PathFilter]
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
