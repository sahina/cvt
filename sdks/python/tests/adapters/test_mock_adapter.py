"""Tests for the mock adapter."""

import re
from datetime import datetime
from unittest.mock import MagicMock

import pytest

from cvt_sdk import ContractValidator
from cvt_sdk.adapters import (
    MockResponse,
    MockSession,
    create_mock_session,
)


@pytest.fixture
def mock_validator():
    """Create a mocked validator."""
    validator = MagicMock(spec=ContractValidator)
    validator.generate_response.return_value = {
        "status_code": 200,
        "headers": {"content-type": "application/json"},
        "body": {"id": "123", "name": "Test User"},
    }
    return validator


class TestMockSession:
    """Tests for MockSession."""

    def test_create_session(self, mock_validator):
        """Should create a session instance."""
        session = MockSession(mock_validator)
        assert session is not None

    def test_create_mock_session_factory(self, mock_validator):
        """Should create a session via factory function."""
        session = create_mock_session(mock_validator)
        assert isinstance(session, MockSession)

    def test_return_empty_interactions_initially(self, mock_validator):
        """Should return empty interactions initially."""
        session = MockSession(mock_validator)
        assert session.get_interactions() == []


class TestMockFetch:
    """Tests for fetch functionality."""

    def test_return_schema_generated_response(self, mock_validator):
        """Should return schema-generated response for GET requests."""
        session = MockSession(mock_validator)

        response = session.get("http://mock.api/users/123")

        assert response.status_code == 200
        assert response.headers.get("content-type") == "application/json"
        assert response.json() == {"id": "123", "name": "Test User"}

    def test_call_generate_response_with_correct_params(self, mock_validator):
        """Should call generateResponse with correct method and path."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users/123")

        mock_validator.generate_response.assert_called_with("GET", "/users/123", {})

    def test_handle_post_requests(self, mock_validator):
        """Should handle POST requests."""
        session = MockSession(mock_validator)

        session.post(
            "http://mock.api/users",
            headers={"Content-Type": "application/json"},
            json_body={"name": "New User"},
        )

        mock_validator.generate_response.assert_called_with("POST", "/users", {})

    def test_capture_request_in_interactions(self, mock_validator):
        """Should capture request in interactions."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users/123")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].request["method"] == "GET"
        assert interactions[0].request["path"] == "/users/123"

    def test_capture_response_in_interactions(self, mock_validator):
        """Should capture response in interactions."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users/123")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].response["status_code"] == 200
        assert interactions[0].response["body"] == {"id": "123", "name": "Test User"}

    def test_capture_request_body_for_post(self, mock_validator):
        """Should capture request body for POST."""
        session = MockSession(mock_validator)
        request_body = {"name": "New User", "email": "test@example.com"}

        session.post(
            "http://mock.api/users",
            headers={"Content-Type": "application/json"},
            json_body=request_body,
        )

        interactions = session.get_interactions()
        assert interactions[0].request["body"] == request_body

    def test_include_query_string_in_path(self, mock_validator):
        """Should include query string in path."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users?status=active&limit=10")

        mock_validator.generate_response.assert_called_with(
            "GET", "/users?status=active&limit=10", {}
        )

    def test_handle_params_dict(self, mock_validator):
        """Should handle params dict."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users", params={"status": "active", "limit": "10"})

        call_args = mock_validator.generate_response.call_args
        # Check that params are added to path
        assert "status=active" in call_args[0][1]
        assert "limit=10" in call_args[0][1]


class TestCaching:
    """Tests for caching functionality."""

    def test_not_cache_by_default(self, mock_validator):
        """Should not cache by default."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users/123")
        session.get("http://mock.api/users/123")

        assert mock_validator.generate_response.call_count == 2

    def test_cache_responses_when_enabled(self, mock_validator):
        """Should cache responses when enabled."""
        session = MockSession(mock_validator, cache=True)

        session.get("http://mock.api/users/123")
        session.get("http://mock.api/users/123")

        assert mock_validator.generate_response.call_count == 1

    def test_cache_by_method_and_path(self, mock_validator):
        """Should cache by method+path."""
        session = MockSession(mock_validator, cache=True)

        session.get("http://mock.api/users/123")
        session.get("http://mock.api/users/456")

        assert mock_validator.generate_response.call_count == 2

    def test_clear_cache(self, mock_validator):
        """Should clear cache."""
        session = MockSession(mock_validator, cache=True)

        session.get("http://mock.api/users/123")
        session.clear_cache()
        session.get("http://mock.api/users/123")

        assert mock_validator.generate_response.call_count == 2


class TestInteractionsManagement:
    """Tests for interactions management."""

    def test_return_copy_of_interactions(self, mock_validator):
        """Should return copy of interactions."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users/123")

        interactions1 = session.get_interactions()
        interactions2 = session.get_interactions()

        assert interactions1 is not interactions2
        assert interactions1 == interactions2

    def test_clear_interactions(self, mock_validator):
        """Should clear interactions."""
        session = MockSession(mock_validator)

        session.get("http://mock.api/users/123")
        assert len(session.get_interactions()) == 1

        session.clear_interactions()
        assert len(session.get_interactions()) == 0

    def test_record_timestamp(self, mock_validator):
        """Should record timestamp."""
        session = MockSession(mock_validator)

        before = datetime.now()
        session.get("http://mock.api/users/123")
        after = datetime.now()

        interactions = session.get_interactions()
        assert interactions[0].timestamp >= before
        assert interactions[0].timestamp <= after


class TestPathFiltering:
    """Tests for path filtering."""

    def test_exclude_paths_matching_exclude_paths(self, mock_validator):
        """Should exclude paths matching excludePaths."""
        session = MockSession(
            mock_validator,
            exclude_paths=["/health"],
        )

        with pytest.raises(ValueError, match="excluded from mocking"):
            session.get("http://mock.api/health")

    def test_exclude_paths_matching_exclude_paths_regex(self, mock_validator):
        """Should exclude paths matching excludePaths regex."""
        session = MockSession(
            mock_validator,
            exclude_paths=[re.compile(r"^/internal")],
        )

        with pytest.raises(ValueError, match="excluded from mocking"):
            session.get("http://mock.api/internal/status")

    def test_only_include_paths_matching_include_paths(self, mock_validator):
        """Should only include paths matching includePaths."""
        session = MockSession(
            mock_validator,
            include_paths=["/api/"],
        )

        # Included path works
        session.get("http://mock.api/api/users/123")
        assert len(session.get_interactions()) == 1

        # Non-included path fails
        with pytest.raises(ValueError, match="excluded from mocking"):
            session.get("http://mock.api/other/path")


class TestGenerateOptions:
    """Tests for generate options."""

    def test_pass_generate_options_to_generate_response(self, mock_validator):
        """Should pass generateOptions to generateResponse."""
        generate_opts = {
            "status_code": 201,
            "use_examples": True,
        }
        session = MockSession(
            mock_validator,
            generate_options=generate_opts,
        )

        session.post("http://mock.api/users")

        mock_validator.generate_response.assert_called_with(
            "POST", "/users", generate_opts
        )


class TestCustomStatusCodes:
    """Tests for custom status codes."""

    def test_handle_different_status_codes(self, mock_validator):
        """Should handle different status codes."""
        mock_validator.generate_response.return_value = {
            "status_code": 201,
            "headers": {"content-type": "application/json"},
            "body": {"id": "new-123"},
        }

        session = MockSession(mock_validator)

        response = session.post("http://mock.api/users")

        assert response.status_code == 201
        assert response.reason == "Created"

    def test_handle_404_status(self, mock_validator):
        """Should handle 404 status."""
        mock_validator.generate_response.return_value = {
            "status_code": 404,
            "headers": {"content-type": "application/json"},
            "body": {"error": "Not Found"},
        }

        session = MockSession(mock_validator)

        response = session.get("http://mock.api/users/unknown")

        assert response.status_code == 404
        assert response.reason == "Not Found"


class TestMockResponse:
    """Tests for MockResponse class."""

    def test_json_body(self):
        """Should handle JSON body correctly."""
        response = MockResponse(
            status_code=200,
            headers={"content-type": "application/json"},
            _body={"key": "value"},
        )

        assert response.json() == {"key": "value"}
        assert response.text == '{"key": "value"}'
        assert response.content == b'{"key": "value"}'

    def test_string_body(self):
        """Should handle string body correctly."""
        response = MockResponse(
            status_code=200,
            headers={"content-type": "text/plain"},
            _body="hello world",
        )

        assert response.text == "hello world"
        assert response.content == b"hello world"

    def test_ok_property(self):
        """Should set ok property based on status code."""
        ok_response = MockResponse(status_code=200, headers={}, _body=None)
        assert ok_response.ok is True

        error_response = MockResponse(status_code=404, headers={}, _body=None)
        assert error_response.ok is False

    def test_raise_for_status(self):
        """Should raise for error status codes."""
        response = MockResponse(status_code=500, headers={}, _body=None)

        with pytest.raises(Exception, match="HTTP Error 500"):
            response.raise_for_status()


class TestHTTPMethods:
    """Tests for HTTP methods."""

    def test_get_method(self, mock_validator):
        """Should support GET method."""
        session = MockSession(mock_validator)
        response = session.get("http://mock.api/users")
        assert response.status_code == 200

    def test_post_method(self, mock_validator):
        """Should support POST method."""
        session = MockSession(mock_validator)
        response = session.post("http://mock.api/users")
        assert response.status_code == 200

    def test_put_method(self, mock_validator):
        """Should support PUT method."""
        session = MockSession(mock_validator)
        response = session.put("http://mock.api/users/123")
        assert response.status_code == 200

    def test_patch_method(self, mock_validator):
        """Should support PATCH method."""
        session = MockSession(mock_validator)
        response = session.patch("http://mock.api/users/123")
        assert response.status_code == 200

    def test_delete_method(self, mock_validator):
        """Should support DELETE method."""
        session = MockSession(mock_validator)
        response = session.delete("http://mock.api/users/123")
        assert response.status_code == 200

    def test_head_method(self, mock_validator):
        """Should support HEAD method."""
        session = MockSession(mock_validator)
        response = session.head("http://mock.api/users")
        assert response.status_code == 200

    def test_options_method(self, mock_validator):
        """Should support OPTIONS method."""
        session = MockSession(mock_validator)
        response = session.options("http://mock.api/users")
        assert response.status_code == 200


class TestSessionHeaders:
    """Tests for session-level headers."""

    def test_merge_session_headers(self, mock_validator):
        """Should merge session-level headers with request headers."""
        session = MockSession(mock_validator)
        session.headers["Authorization"] = "Bearer token123"

        session.get("http://mock.api/users/123", headers={"Accept": "application/json"})

        interactions = session.get_interactions()
        assert "authorization" in interactions[0].request["headers"]
        assert "accept" in interactions[0].request["headers"]

    def test_request_headers_override_session_headers(self, mock_validator):
        """Should allow request headers to override session headers."""
        session = MockSession(mock_validator)
        session.headers["Content-Type"] = "text/plain"

        session.post(
            "http://mock.api/users",
            headers={"Content-Type": "application/json"},
        )

        interactions = session.get_interactions()
        assert interactions[0].request["headers"]["content-type"] == "application/json"
