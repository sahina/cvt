"""Auto-registration of consumers from captured test interactions."""

from dataclasses import dataclass
from typing import Any, Optional
from urllib.parse import urlparse

from .adapters.types import CapturedInteraction
from . import EndpointUsage, RegisterConsumerOptions


@dataclass
class AutoRegisterConfig:
    """Configuration for auto-registration of consumers from captured interactions."""

    consumer_id: str
    """Unique consumer identifier (required, e.g., "order-service")."""

    consumer_version: str
    """Consumer's version (required, e.g., "2.1.0")."""

    environment: str
    """Deployment environment (required, e.g., "dev", "staging", "prod")."""

    schema_version: str
    """Schema version being tested against (required, e.g., "1.0.0")."""

    schema_id: Optional[str] = None
    """Override auto-extraction from URL hostname (optional).
    If empty, schemaId is extracted from the mock URL hostname.
    For example, "http://mock.user-api/users/123" extracts "user-api".
    """


def extract_schema_id_from_url(url_str: str) -> Optional[str]:
    """
    Extracts the schemaId from a mock URL.
    For example: "http://mock.user-api/users/123" returns "user-api"
    """
    try:
        parsed = urlparse(url_str)
        host = parsed.hostname
        if not host:
            return None

        # Strip "mock." prefix if present
        if host.startswith("mock."):
            return host[5:]
        return host
    except Exception:
        return None


def extract_schema_id_from_interactions(
    interactions: list[CapturedInteraction],
) -> tuple[Optional[str], Optional[str]]:
    """
    Extracts the schemaId from captured interactions.
    Returns (schemaId, error) tuple.
    """
    schema_ids: set[str] = set()

    for interaction in interactions:
        path = interaction.request.get("path", "")

        # If path starts with http:// or https://, extract hostname
        if path.startswith("http://") or path.startswith("https://"):
            schema_id = extract_schema_id_from_url(path)
            if schema_id:
                schema_ids.add(schema_id)

    if not schema_ids:
        return (
            None,
            "could not extract schema_id from interactions; provide schema_id in config",
        )

    if len(schema_ids) > 1:
        ids = ", ".join(sorted(schema_ids))
        return (
            None,
            f"multiple schemas detected ({ids}); provide explicit schema_id in config",
        )

    return next(iter(schema_ids)), None


def normalize_path_for_endpoint(path_or_url: str) -> str:
    """Normalizes a path by extracting just the path portion from a URL."""
    # If it's a full URL, parse it to extract just the path
    if path_or_url.startswith("http://") or path_or_url.startswith("https://"):
        try:
            parsed = urlparse(path_or_url)
            path_or_url = parsed.path
        except Exception:
            pass  # Keep original if parse fails

    # Remove query string if present
    query_index = path_or_url.find("?")
    if query_index != -1:
        path_or_url = path_or_url[:query_index]

    return path_or_url


def extract_fields_from_body(body: Any, prefix: str = "") -> list[str]:
    """
    Recursively extracts all field paths from a JSON body.
    Uses dot notation for nested fields (e.g., "user.address.city").
    """
    if body is None:
        return []

    fields: list[str] = []

    if isinstance(body, dict):
        for key, value in body.items():
            field_path = f"{prefix}.{key}" if prefix else key
            fields.append(field_path)
            # Recursively extract nested fields
            fields.extend(extract_fields_from_body(value, field_path))
    elif isinstance(body, list) and len(body) > 0:
        # For arrays, extract fields from the first element as representative
        fields.extend(extract_fields_from_body(body[0], prefix))

    return fields


def merge_arrays(a: list[str], b: list[str]) -> list[str]:
    """Merges two arrays, removing duplicates."""
    return list(set(a) | set(b))


def merge_interactions_to_endpoints(
    interactions: list[CapturedInteraction],
) -> list[EndpointUsage]:
    """
    Converts captured interactions to endpoint usage,
    deduplicating by method+path and merging usedFields.
    """
    endpoint_map: dict[str, EndpointUsage] = {}

    for interaction in interactions:
        method = interaction.request.get("method", "")
        path = normalize_path_for_endpoint(interaction.request.get("path", ""))
        key = f"{method}:{path}"

        body = interaction.response.get("body")
        fields = extract_fields_from_body(body)

        existing = endpoint_map.get(key)
        if existing:
            # Merge fields (union)
            existing.used_fields = merge_arrays(existing.used_fields or [], fields)
        else:
            endpoint_map[key] = EndpointUsage(
                method=method,
                path=path,
                used_fields=fields,
            )

    # Convert to sorted list for deterministic output
    result = []
    for key in sorted(endpoint_map.keys()):
        ep = endpoint_map[key]
        if ep.used_fields:
            ep.used_fields = sorted(ep.used_fields)
        result.append(ep)

    return result


def build_consumer_from_interactions(
    interactions: list[CapturedInteraction],
    config: AutoRegisterConfig,
) -> tuple[Optional[RegisterConsumerOptions], Optional[str]]:
    """
    Builds consumer registration options from captured interactions.
    Returns (options, error) tuple.
    """
    # Validate required fields
    if not config.consumer_id:
        return None, "consumer_id is required"
    if not config.consumer_version:
        return None, "consumer_version is required"
    if not config.environment:
        return None, "environment is required"
    if not config.schema_version:
        return None, "schema_version is required"

    # Validate interactions
    if not interactions:
        return None, "no interactions to register"

    # Extract schema_id from interactions or use provided override
    schema_id = config.schema_id
    if not schema_id:
        schema_id, error = extract_schema_id_from_interactions(interactions)
        if error:
            return None, error

    # Merge interactions into endpoint usage
    used_endpoints = merge_interactions_to_endpoints(interactions)

    return RegisterConsumerOptions(
        consumer_id=config.consumer_id,
        consumer_version=config.consumer_version,
        schema_id=schema_id,  # type: ignore
        schema_version=config.schema_version,
        environment=config.environment,
        used_endpoints=used_endpoints,
    ), None


__all__ = [
    "AutoRegisterConfig",
    "extract_schema_id_from_url",
    "extract_schema_id_from_interactions",
    "extract_fields_from_body",
    "normalize_path_for_endpoint",
    "merge_interactions_to_endpoints",
    "build_consumer_from_interactions",
]
