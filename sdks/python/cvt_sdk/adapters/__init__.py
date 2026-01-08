"""
CVT SDK Adapters

This module provides convenience adapters for popular HTTP clients
that automatically capture and validate HTTP interactions.

Example:
    >>> from cvt_sdk import ContractValidator
    >>> from cvt_sdk.adapters import ContractValidatingSession
    >>>
    >>> validator = ContractValidator()
    >>> validator.register_schema('api', './openapi.json')
    >>>
    >>> session = ContractValidatingSession(validator)
    >>> response = session.post('http://api/pet', json={'name': 'Fluffy'})
"""

from .types import (
    AdapterConfig,
    CapturedInteraction,
    PathFilter,
    matches_path_filter,
    should_validate_path,
)
from .requests_adapter import (
    ContractValidatingSession,
    create_validating_session,
)
from .mock_adapter import (
    MockResponse,
    MockSession,
    MockSessionConfig,
    create_mock_session,
)

__all__ = [
    "AdapterConfig",
    "CapturedInteraction",
    "PathFilter",
    "matches_path_filter",
    "should_validate_path",
    "ContractValidatingSession",
    "create_validating_session",
    "MockResponse",
    "MockSession",
    "MockSessionConfig",
    "create_mock_session",
]
