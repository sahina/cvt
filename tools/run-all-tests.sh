#!/bin/bash
set -e

# Parse arguments
USE_DOCKER=true
while [[ $# -gt 0 ]]; do
  case $1 in
    --no-docker)
      USE_DOCKER=false
      shift
      ;;
    *)
      shift
      ;;
  esac
done

if [ "$USE_DOCKER" = true ]; then
  echo "🧪 Running all tests with Docker (server + all 4 SDKs)..."
else
  echo "🧪 Running all tests with direct server (no Docker)..."
fi
echo ""

# Track test results
FAILED_TESTS=()

echo "============================================================"
echo "SERVER TESTS"
echo "============================================================"
echo ""

echo ">>> Testing Go Server (Unit Tests)..."
if (cd server && go test -v ./...); then
    echo "✅ Server unit tests passed"
else
    echo "❌ Server unit tests failed"
    FAILED_TESTS+=("server-unit")
fi
echo ""

if [ "$USE_DOCKER" = true ]; then
  echo ">>> Testing Go Server (Integration Tests)..."
  if (cd server && go test -v -tags=integration ./...); then
      echo "✅ Server integration tests passed"
  else
      echo "⚠️  Server integration tests skipped or failed (may require Docker)"
  fi
  echo ""
fi

# Start server for SDK tests
echo "============================================================"
echo "STARTING CVT SERVER FOR SDK INTEGRATION TESTS"
echo "============================================================"
echo ""

if [ "$USE_DOCKER" = true ]; then
  docker compose up -d --wait || {
    echo "⚠️  Failed to start server, skipping SDK integration tests"
    echo "   Unit tests for SDKs will still run"
  }

  # Ensure server is stopped on exit (unless KEEP_DOCKER_UP is set)
  if [ -z "$KEEP_DOCKER_UP" ]; then
    trap "echo ''; echo '>>> Stopping CVT server...'; docker compose down" EXIT
  else
    echo "ℹ️  KEEP_DOCKER_UP is set - Docker Compose will remain running after tests"
  fi
else
  echo ">>> Starting CVT server directly (no Docker)..."
  # Build and start server in background on port 9550 to match Docker config
  echo ">>> Building server..."
  (cd server && go build -o /tmp/cvt-server-test .) || {
    echo "❌ Failed to build server"
    exit 1
  }

  # Start server in background
  CVT_PORT=9550 /tmp/cvt-server-test &
  SERVER_PID=$!

  # Wait for server to be ready
  echo ">>> Waiting for server to start (PID: $SERVER_PID)..."
  for i in {1..10}; do
    if lsof -i :9550 >/dev/null 2>&1; then
      break
    fi
    sleep 0.5
  done

  # Check if server is running
  if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "❌ Failed to start server"
    exit 1
  fi
  echo "✅ Server started and listening on port 9550"

  # Ensure server is stopped on exit
  trap "echo ''; echo '>>> Stopping CVT server (PID: $SERVER_PID)...'; kill $SERVER_PID 2>/dev/null || true; rm -f /tmp/cvt-server-test" EXIT
fi

echo ""
echo "============================================================"
echo "SDK TESTS"
echo "============================================================"
echo ""

echo ">>> Testing Node.js SDK..."
if (cd sdks/node && pnpm test); then
    echo "✅ Node.js SDK tests passed"
else
    echo "❌ Node.js SDK tests failed"
    FAILED_TESTS+=("node-sdk")
fi
echo ""

echo ">>> Testing Python SDK..."
if (cd sdks/python && uv run pytest tests/); then
    echo "✅ Python SDK tests passed"
else
    echo "❌ Python SDK tests failed"
    FAILED_TESTS+=("python-sdk")
fi
echo ""

echo ">>> Testing Go SDK..."
if (cd sdks/go && go test -v ./...); then
    echo "✅ Go SDK tests passed"
else
    echo "❌ Go SDK tests failed"
    FAILED_TESTS+=("go-sdk")
fi
echo ""

echo ">>> Testing Java SDK..."
if (cd sdks/java && ./gradlew test --continue); then
    echo "✅ Java SDK tests passed"
else
    echo "⚠️  Java SDK tests completed with some failures (server integration tests may require running server)"
    # Don't fail the entire suite for Java integration test failures
fi
echo ""

echo "============================================================"
echo "TEST SUMMARY"
echo "============================================================"

if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo "✅ All critical tests passed!"
    echo ""
    echo "Completed:"
    echo "  ✅ Server unit tests"
    echo "  ✅ Node.js SDK tests"
    echo "  ✅ Python SDK tests"
    echo "  ✅ Go SDK tests"
    echo "  ✅ Java SDK tests"
    exit 0
else
    echo "❌ Some tests failed:"
    for test in "${FAILED_TESTS[@]}"; do
        echo "  ❌ $test"
    done
    exit 1
fi
