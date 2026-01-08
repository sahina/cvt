"""Tests for the requests adapter."""

import re
from unittest.mock import MagicMock

import pytest
import responses

from cvt_sdk import ContractValidator
from cvt_sdk.adapters import ContractValidatingSession, create_validating_session
from cvt_sdk.adapters.types import matches_path_filter, should_validate_path


@pytest.fixture
def mock_validator():
    """Create a mocked validator."""
    validator = MagicMock(spec=ContractValidator)
    validator.validate.return_value = {"valid": True, "errors": []}
    return validator


@pytest.fixture
def session(mock_validator):
    """Create a validating session with auto_validate disabled."""
    return ContractValidatingSession(mock_validator, auto_validate=False)


class TestContractValidatingSession:
    """Tests for ContractValidatingSession."""

    def test_create_session(self, mock_validator):
        """Should create a session instance."""
        session = ContractValidatingSession(mock_validator)
        assert session is not None

    def test_create_validating_session_factory(self, mock_validator):
        """Should create a session via factory function."""
        session = create_validating_session(mock_validator)
        assert isinstance(session, ContractValidatingSession)


class TestRequestResponseCapture:
    """Tests for request/response capture."""

    @responses.activate
    def test_capture_successful_request(self, mock_validator):
        """Should capture successful requests."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(
            responses.GET,
            "http://api.test/pet/1",
            json={"id": 1, "name": "Fluffy"},
            status=200,
        )

        session.get("http://api.test/pet/1")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].request["method"] == "GET"
        assert interactions[0].request["path"] == "/pet/1"
        assert interactions[0].response["status_code"] == 200

    @responses.activate
    def test_capture_request_body(self, mock_validator):
        """Should capture request body."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(responses.POST, "http://api.test/pet", json={"id": 1}, status=201)

        request_body = {"name": "Fluffy", "photoUrls": ["http://example.com/photo.jpg"]}
        session.post("http://api.test/pet", json=request_body)

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].request["body"] == request_body

    @responses.activate
    def test_capture_response_body(self, mock_validator):
        """Should capture response body."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        response_body = {"id": 1, "name": "Fluffy"}
        responses.add(
            responses.GET, "http://api.test/pet/1", json=response_body, status=200
        )

        session.get("http://api.test/pet/1")

        interactions = session.get_interactions()
        assert interactions[0].response["body"] == response_body

    @responses.activate
    def test_capture_includes_timestamp(self, mock_validator):
        """Should include timestamp in captured interactions."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(responses.GET, "http://api.test/test", json={}, status=200)

        from datetime import datetime

        before_time = datetime.now()
        session.get("http://api.test/test")
        after_time = datetime.now()

        interactions = session.get_interactions()
        assert interactions[0].timestamp >= before_time
        assert interactions[0].timestamp <= after_time


class TestAutomaticValidation:
    """Tests for automatic validation mode."""

    @responses.activate
    def test_validate_when_auto_validate_true(self, mock_validator):
        """Should validate requests when auto_validate is true."""
        session = ContractValidatingSession(
            mock_validator, auto_validate=True, on_validation_failure=lambda *args: None
        )
        responses.add(
            responses.GET, "http://api.test/pet/1", json={"id": 1}, status=200
        )

        session.get("http://api.test/pet/1")

        mock_validator.validate.assert_called_once()
        call_args = mock_validator.validate.call_args
        assert call_args[0][0]["method"] == "GET"
        assert call_args[0][0]["path"] == "/pet/1"
        assert call_args[0][1]["status_code"] == 200

    @responses.activate
    def test_store_validation_result(self, mock_validator):
        """Should store validation result in captured interaction."""
        validation_result = {"valid": True, "errors": []}
        mock_validator.validate.return_value = validation_result

        session = ContractValidatingSession(mock_validator, auto_validate=True)
        responses.add(responses.GET, "http://api.test/test", json={}, status=200)

        session.get("http://api.test/test")

        interactions = session.get_interactions()
        assert interactions[0].validation_result == validation_result

    @responses.activate
    def test_call_on_validation_failure(self, mock_validator):
        """Should call on_validation_failure when validation fails."""
        failed_result = {"valid": False, "errors": ["Missing required field"]}
        mock_validator.validate.return_value = failed_result

        on_failure = MagicMock()
        session = ContractValidatingSession(
            mock_validator, auto_validate=True, on_validation_failure=on_failure
        )
        responses.add(responses.GET, "http://api.test/test", json={}, status=200)

        session.get("http://api.test/test")

        on_failure.assert_called_once()
        assert on_failure.call_args[0][0] == failed_result

    @responses.activate
    def test_raise_by_default_when_validation_fails(self, mock_validator):
        """Should raise AssertionError by default when validation fails."""
        failed_result = {"valid": False, "errors": ["Missing required field"]}
        mock_validator.validate.return_value = failed_result

        session = ContractValidatingSession(mock_validator, auto_validate=True)
        responses.add(responses.GET, "http://api.test/test", json={}, status=200)

        with pytest.raises(AssertionError, match="Contract validation failed"):
            session.get("http://api.test/test")

    @responses.activate
    def test_no_validate_when_auto_validate_false(self, mock_validator):
        """Should not validate when auto_validate is false."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(responses.GET, "http://api.test/test", json={}, status=200)

        session.get("http://api.test/test")

        mock_validator.validate.assert_not_called()
        assert len(session.get_interactions()) == 1


class TestPathFiltering:
    """Tests for path filtering."""

    @responses.activate
    def test_exclude_paths_string(self, mock_validator):
        """Should exclude paths matching excludePaths string."""
        session = ContractValidatingSession(
            mock_validator, auto_validate=False, exclude_paths=["/health"]
        )
        responses.add(responses.GET, "http://api.test/health", json={}, status=200)
        responses.add(responses.GET, "http://api.test/pet/1", json={}, status=200)

        session.get("http://api.test/health")
        session.get("http://api.test/pet/1")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].request["path"] == "/pet/1"

    @responses.activate
    def test_exclude_paths_regex(self, mock_validator):
        """Should exclude paths matching excludePaths regex."""
        session = ContractValidatingSession(
            mock_validator,
            auto_validate=False,
            exclude_paths=[re.compile(r"^/health"), re.compile(r"^/metrics")],
        )
        responses.add(
            responses.GET, "http://api.test/health/check", json={}, status=200
        )
        responses.add(responses.GET, "http://api.test/pet/1", json={}, status=200)

        session.get("http://api.test/health/check")
        session.get("http://api.test/pet/1")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].request["path"] == "/pet/1"

    @responses.activate
    def test_include_paths_only(self, mock_validator):
        """Should only validate paths matching includePaths."""
        session = ContractValidatingSession(
            mock_validator, auto_validate=False, include_paths=[re.compile(r"^/pet")]
        )
        responses.add(responses.GET, "http://api.test/user/1", json={}, status=200)
        responses.add(responses.GET, "http://api.test/pet/1", json={}, status=200)

        session.get("http://api.test/user/1")
        session.get("http://api.test/pet/1")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].request["path"] == "/pet/1"

    @responses.activate
    def test_exclude_before_include(self, mock_validator):
        """Should apply excludePaths before includePaths."""
        session = ContractValidatingSession(
            mock_validator,
            auto_validate=False,
            include_paths=[re.compile(r"^/pet")],
            exclude_paths=["/pet/health"],
        )
        responses.add(responses.GET, "http://api.test/pet/health", json={}, status=200)
        responses.add(responses.GET, "http://api.test/pet/1", json={}, status=200)

        session.get("http://api.test/pet/health")
        session.get("http://api.test/pet/1")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].request["path"] == "/pet/1"


class TestManualValidation:
    """Tests for manual validation mode."""

    @responses.activate
    def test_manual_validation(self, mock_validator):
        """Should allow manual validation of captured interactions."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(
            responses.GET, "http://api.test/pet/1", json={"id": 1}, status=200
        )

        session.get("http://api.test/pet/1")

        mock_validator.validate.assert_not_called()

        interactions = session.get_interactions()
        session.validate_interaction(interactions[0])

        mock_validator.validate.assert_called_once()


class TestInteractionManagement:
    """Tests for interaction management."""

    @responses.activate
    def test_get_interactions_returns_copy(self, mock_validator):
        """Should return a copy of interactions."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(responses.GET, "http://api.test/test", json={}, status=200)

        session.get("http://api.test/test")

        interactions1 = session.get_interactions()
        interactions2 = session.get_interactions()

        assert interactions1 is not interactions2
        assert interactions1 == interactions2

    @responses.activate
    def test_clear_interactions(self, mock_validator):
        """Should clear interactions."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(responses.GET, "http://api.test/test", json={}, status=200)

        session.get("http://api.test/test")
        assert len(session.get_interactions()) == 1

        session.clear_interactions()
        assert len(session.get_interactions()) == 0


class TestErrorResponses:
    """Tests for error response handling."""

    @responses.activate
    def test_capture_error_responses(self, mock_validator):
        """Should capture error responses (4xx/5xx)."""
        session = ContractValidatingSession(mock_validator, auto_validate=False)
        responses.add(
            responses.GET,
            "http://api.test/pet/999",
            json={"error": "Not found"},
            status=404,
        )

        session.get("http://api.test/pet/999")

        interactions = session.get_interactions()
        assert len(interactions) == 1
        assert interactions[0].response["status_code"] == 404


class TestPathFilteringUtilities:
    """Tests for path filtering utility functions."""

    def test_matches_path_filter_string(self):
        """Should match string patterns as substrings."""
        assert matches_path_filter("/api/pet/1", "/pet") is True
        assert matches_path_filter("/api/user/1", "/pet") is False

    def test_matches_path_filter_regex(self):
        """Should match regex patterns."""
        assert matches_path_filter("/pet/1", re.compile(r"^/pet")) is True
        assert matches_path_filter("/user/1", re.compile(r"^/pet")) is False

    def test_should_validate_path_empty_filters(self):
        """Should return True for empty filters."""
        assert should_validate_path("/any/path", [], []) is True

    def test_should_validate_path_excluded(self):
        """Should return False for excluded paths."""
        assert should_validate_path("/health", [], ["/health"]) is False

    def test_should_validate_path_included(self):
        """Should return True only for included paths when includePaths is set."""
        assert should_validate_path("/pet/1", ["/pet"], []) is True
        assert should_validate_path("/user/1", ["/pet"], []) is False

    def test_should_validate_path_exclude_before_include(self):
        """Should prioritize excludePaths over includePaths."""
        assert should_validate_path("/pet/health", ["/pet"], ["/health"]) is False
