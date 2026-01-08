"""Tests for auto-registration functionality."""

from datetime import datetime

import pytest

from cvt_sdk.auto_register import (
    AutoRegisterConfig,
    extract_schema_id_from_url,
    extract_schema_id_from_interactions,
    extract_fields_from_body,
    normalize_path_for_endpoint,
    merge_interactions_to_endpoints,
    build_consumer_from_interactions,
)
from cvt_sdk.adapters.types import CapturedInteraction


class TestExtractSchemaIdFromUrl:
    """Tests for extract_schema_id_from_url."""

    def test_mock_prefix(self):
        assert (
            extract_schema_id_from_url("http://mock.user-api/users/123") == "user-api"
        )

    def test_mock_prefix_with_subdomain(self):
        assert (
            extract_schema_id_from_url("http://mock.my-service/api/v1/items")
            == "my-service"
        )

    def test_no_mock_prefix(self):
        assert (
            extract_schema_id_from_url("http://api.example.com/users")
            == "api.example.com"
        )

    def test_https_url(self):
        assert (
            extract_schema_id_from_url("https://mock.secure-api/data") == "secure-api"
        )

    def test_url_with_port(self):
        assert (
            extract_schema_id_from_url("http://mock.test-api:8080/endpoint")
            == "test-api"
        )

    def test_invalid_url(self):
        assert extract_schema_id_from_url("not a url") is None


class TestExtractFieldsFromBody:
    """Tests for extract_fields_from_body."""

    def test_none_body(self):
        assert extract_fields_from_body(None) == []

    def test_flat_object(self):
        body = {"id": "123", "name": "John", "email": "john@example.com"}
        fields = extract_fields_from_body(body)
        assert "id" in fields
        assert "name" in fields
        assert "email" in fields

    def test_nested_object(self):
        body = {"id": "123", "address": {"city": "NYC", "zip": "10001"}}
        fields = extract_fields_from_body(body)
        assert "id" in fields
        assert "address" in fields
        assert "address.city" in fields
        assert "address.zip" in fields

    def test_deeply_nested(self):
        body = {"user": {"profile": {"name": "John"}}}
        fields = extract_fields_from_body(body)
        assert "user" in fields
        assert "user.profile" in fields
        assert "user.profile.name" in fields

    def test_array_with_objects(self):
        body = [{"id": "1", "name": "Item 1"}]
        fields = extract_fields_from_body(body)
        assert "id" in fields
        assert "name" in fields

    def test_empty_array(self):
        assert extract_fields_from_body([]) == []

    def test_empty_object(self):
        assert extract_fields_from_body({}) == []

    def test_with_prefix(self):
        body = {"city": "NYC"}
        fields = extract_fields_from_body(body, "address")
        assert "address.city" in fields


class TestNormalizePathForEndpoint:
    """Tests for normalize_path_for_endpoint."""

    def test_simple_path(self):
        assert normalize_path_for_endpoint("/users/123") == "/users/123"

    def test_path_with_query_string(self):
        assert normalize_path_for_endpoint("/users?page=1&limit=10") == "/users"

    def test_full_url(self):
        assert (
            normalize_path_for_endpoint("http://mock.user-api/users/123")
            == "/users/123"
        )

    def test_full_url_with_query(self):
        assert (
            normalize_path_for_endpoint("http://mock.user-api/users?active=true")
            == "/users"
        )

    def test_https_url(self):
        assert (
            normalize_path_for_endpoint("https://mock.api/data/items") == "/data/items"
        )


class TestExtractSchemaIdFromInteractions:
    """Tests for extract_schema_id_from_interactions."""

    def test_single_schema(self):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={"status_code": 200},
                timestamp=datetime.now(),
            )
        ]
        schema_id, error = extract_schema_id_from_interactions(interactions)
        assert error is None
        assert schema_id == "user-api"

    def test_multiple_interactions_same_schema(self):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={"status_code": 200},
                timestamp=datetime.now(),
            ),
            CapturedInteraction(
                request={"method": "POST", "path": "http://mock.user-api/users"},
                response={"status_code": 201},
                timestamp=datetime.now(),
            ),
        ]
        schema_id, error = extract_schema_id_from_interactions(interactions)
        assert error is None
        assert schema_id == "user-api"

    def test_multiple_different_schemas(self):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={"status_code": 200},
                timestamp=datetime.now(),
            ),
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.order-api/orders/456"},
                response={"status_code": 200},
                timestamp=datetime.now(),
            ),
        ]
        schema_id, error = extract_schema_id_from_interactions(interactions)
        assert "multiple schemas detected" in error
        assert schema_id is None

    def test_no_urls_in_paths(self):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "/users/123"},
                response={"status_code": 200},
                timestamp=datetime.now(),
            )
        ]
        schema_id, error = extract_schema_id_from_interactions(interactions)
        assert "could not extract schema_id" in error
        assert schema_id is None


class TestMergeInteractionsToEndpoints:
    """Tests for merge_interactions_to_endpoints."""

    def test_merge_interactions(self):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={"status_code": 200, "body": {"id": "123", "name": "John"}},
                timestamp=datetime.now(),
            ),
            CapturedInteraction(
                request={"method": "POST", "path": "http://mock.user-api/users"},
                response={"status_code": 201, "body": {"id": "789"}},
                timestamp=datetime.now(),
            ),
        ]

        endpoints = merge_interactions_to_endpoints(interactions)
        assert len(endpoints) == 2

        get_endpoint = next(
            (ep for ep in endpoints if ep.method == "GET" and ep.path == "/users/123"),
            None,
        )
        assert get_endpoint is not None
        assert "id" in get_endpoint.used_fields
        assert "name" in get_endpoint.used_fields

        post_endpoint = next(
            (ep for ep in endpoints if ep.method == "POST" and ep.path == "/users"),
            None,
        )
        assert post_endpoint is not None
        assert "id" in post_endpoint.used_fields

    def test_merge_fields_from_duplicate_endpoints(self):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={"status_code": 200, "body": {"id": "123", "name": "John"}},
                timestamp=datetime.now(),
            ),
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={
                    "status_code": 200,
                    "body": {"id": "123", "email": "john@example.com"},
                },
                timestamp=datetime.now(),
            ),
        ]

        endpoints = merge_interactions_to_endpoints(interactions)
        assert len(endpoints) == 1

        endpoint = endpoints[0]
        assert "id" in endpoint.used_fields
        assert "name" in endpoint.used_fields
        assert "email" in endpoint.used_fields


class TestBuildConsumerFromInteractions:
    """Tests for build_consumer_from_interactions."""

    @pytest.fixture
    def valid_config(self):
        return AutoRegisterConfig(
            consumer_id="test-service",
            consumer_version="1.0.0",
            environment="dev",
            schema_version="1.0.0",
        )

    @pytest.fixture
    def valid_interactions(self):
        return [
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={
                    "status_code": 200,
                    "body": {"id": "123", "name": "John", "email": "john@example.com"},
                },
                timestamp=datetime.now(),
            )
        ]

    def test_build_options_success(self, valid_config, valid_interactions):
        options, error = build_consumer_from_interactions(
            valid_interactions, valid_config
        )
        assert error is None
        assert options is not None
        assert options.consumer_id == "test-service"
        assert options.consumer_version == "1.0.0"
        assert options.schema_id == "user-api"
        assert options.schema_version == "1.0.0"
        assert options.environment == "dev"
        assert len(options.used_endpoints) == 1

    def test_missing_consumer_id(self, valid_interactions):
        config = AutoRegisterConfig(
            consumer_id="",
            consumer_version="1.0.0",
            environment="dev",
            schema_version="1.0.0",
        )
        options, error = build_consumer_from_interactions(valid_interactions, config)
        assert error == "consumer_id is required"
        assert options is None

    def test_missing_consumer_version(self, valid_interactions):
        config = AutoRegisterConfig(
            consumer_id="test-service",
            consumer_version="",
            environment="dev",
            schema_version="1.0.0",
        )
        options, error = build_consumer_from_interactions(valid_interactions, config)
        assert error == "consumer_version is required"
        assert options is None

    def test_missing_environment(self, valid_interactions):
        config = AutoRegisterConfig(
            consumer_id="test-service",
            consumer_version="1.0.0",
            environment="",
            schema_version="1.0.0",
        )
        options, error = build_consumer_from_interactions(valid_interactions, config)
        assert error == "environment is required"
        assert options is None

    def test_missing_schema_version(self, valid_interactions):
        config = AutoRegisterConfig(
            consumer_id="test-service",
            consumer_version="1.0.0",
            environment="dev",
            schema_version="",
        )
        options, error = build_consumer_from_interactions(valid_interactions, config)
        assert error == "schema_version is required"
        assert options is None

    def test_empty_interactions(self, valid_config):
        options, error = build_consumer_from_interactions([], valid_config)
        assert error == "no interactions to register"
        assert options is None

    def test_explicit_schema_id_override(self, valid_config):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "/users/123"},
                response={"status_code": 200, "body": {"id": "123"}},
                timestamp=datetime.now(),
            )
        ]
        config = AutoRegisterConfig(
            consumer_id="test-service",
            consumer_version="1.0.0",
            environment="dev",
            schema_version="1.0.0",
            schema_id="my-custom-schema",
        )
        options, error = build_consumer_from_interactions(interactions, config)
        assert error is None
        assert options.schema_id == "my-custom-schema"

    def test_nested_response_fields(self, valid_config):
        interactions = [
            CapturedInteraction(
                request={"method": "GET", "path": "http://mock.user-api/users/123"},
                response={
                    "status_code": 200,
                    "body": {
                        "id": "123",
                        "address": {"city": "NYC", "zip": "10001"},
                    },
                },
                timestamp=datetime.now(),
            )
        ]
        options, error = build_consumer_from_interactions(interactions, valid_config)
        assert error is None

        fields = options.used_endpoints[0].used_fields
        assert "id" in fields
        assert "address" in fields
        assert "address.city" in fields
        assert "address.zip" in fields
