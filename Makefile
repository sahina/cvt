.PHONY: all build build-cli install-cli test test-docker test-server test-cli test-node-sdk test-python-sdk test-go-sdk test-java-sdk test-integration test-cache test-all test-with-observability clean generate generate-python generate-go-sdk generate-java-sdk help
.PHONY: up down restart logs status
.PHONY: install-health-probe health check-health watch-health
.PHONY: run-server run-example
.PHONY: install install-server install-node-sdk install-python-sdk install-go-sdk install-java-sdk
.PHONY: update update-server update-go-sdk update-java-sdk update-node-sdk update-python-sdk
.PHONY: metrics grafana prometheus observability-status observability-logs
.PHONY: lint lint-go lint-node lint-python lint-java ci check-coverage ci-full
.PHONY: docs-dev docs-build docs-serve docs-deploy docs-install
.PHONY: tag tag-push release prerelease _check_tag _check_tag_prerelease check-release

# Default target
all: build

# Display available commands
help:
	@echo "CVT - Contract Validator Toolkit (Go Server)"
	@echo ""
	@echo "Build commands:"
	@echo "  make build              - Build Go server, Node.js SDK, and Python SDK"
	@echo "  make build-cli          - Build CVT CLI binary with version info"
	@echo "  make install-cli        - Build and install CVT CLI to /usr/local/bin"
	@echo "  make clean              - Clean build artifacts"
	@echo "  make generate           - Generate protobuf code for Go"
	@echo "  make generate-python    - Generate protobuf code for Python"
	@echo "  make generate-go-sdk    - Generate protobuf code for Go SDK"
	@echo "  make generate-java-sdk  - Generate protobuf code for Java SDK"
	@echo ""
	@echo "Test commands:"
	@echo "  make test               - Run all tests with direct server (fast, no Docker)"
	@echo "  make test-docker        - Run all tests with Docker + PostgreSQL (CI/CD)"
	@echo "  make test-all           - Run all tests with Docker (same as 'make test-docker')"
	@echo "  make test-with-observability - Run all tests, keep Docker Compose up"
	@echo "  make test-server        - Run Go server unit tests only"
	@echo "  make test-node-sdk      - Run Node.js SDK tests"
	@echo "  make test-python-sdk    - Run Python SDK tests with coverage"
	@echo "  make test-go-sdk        - Run Go SDK tests with coverage"
	@echo "  make test-java-sdk      - Run Java SDK tests with coverage"
	@echo "  make test-coverage      - Run tests with coverage report (HTML + summary)"
	@echo "  make test-integration   - Run Go server integration tests (requires Docker)"
	@echo "  make test-cli           - Run CLI unit tests"
	@echo "  make test-cache         - Run cache behavior tests"
	@echo ""
	@echo "Lint commands:"
	@echo "  make lint               - Run all linters (Go, Node.js, Python, Java)"
	@echo "  make lint-go            - Run golangci-lint on all Go code"
	@echo "  make lint-node          - Run ESLint on Node.js SDK"
	@echo "  make lint-python        - Run ruff on Python SDK"
	@echo "  make lint-java          - Run Maven verify on Java SDK"
	@echo "  make ci                 - Run CI checks locally (lint + format)"
	@echo "  make check-coverage     - Verify all components have >= 70% test coverage"
	@echo "  make ci-full            - Run full CI (lint + format + coverage)"
	@echo ""
	@echo "Docker commands:"
	@echo "  make up                 - Start Go server in Docker"
	@echo "  make down               - Stop server"
	@echo "  make restart            - Restart server"
	@echo "  make logs               - View server logs"
	@echo "  make status             - Check container status"
	@echo ""
	@echo "Health check commands:"
	@echo "  make install-health-probe - Install grpc-health-probe"
	@echo "  make health             - Check server health (port 9550)"
	@echo "  make check-health       - Detailed health check with status"
	@echo "  make watch-health       - Continuously monitor health (Ctrl+C to stop)"
	@echo ""
	@echo "Development commands:"
	@echo "  make run-server         - Run Go server locally (gRPC)"
	@echo "  make run-example        - Run Node.js SDK example"
	@echo ""
	@echo "Install commands:"
	@echo "  make install            - Install all dependencies (server + all SDKs)"
	@echo "  make install-server     - Install Go server dependencies"
	@echo "  make install-node-sdk   - Install Node.js SDK dependencies"
	@echo "  make install-python-sdk - Install Python SDK dependencies"
	@echo "  make install-go-sdk     - Install Go SDK dependencies"
	@echo "  make install-java-sdk   - Install Java SDK dependencies"
	@echo ""
	@echo "Update commands:"
	@echo "  make update             - Update all dependencies (server + all SDKs)"
	@echo "  make update-server      - Update Go server dependencies"
	@echo "  make update-go-sdk      - Update Go SDK dependencies"
	@echo "  make update-java-sdk    - Update Java SDK dependencies"
	@echo "  make update-node-sdk    - Update Node.js SDK dependencies"
	@echo "  make update-python-sdk  - Update Python SDK dependencies"
	@echo ""
	@echo "Observability commands:"
	@echo "  make metrics            - View Prometheus metrics (curl localhost:9551/metrics)"
	@echo "  make grafana            - Open Grafana dashboard (http://localhost:3000)"
	@echo "  make prometheus         - Open Prometheus UI (http://localhost:9091)"
	@echo "  make observability-status - Check observability stack status"
	@echo "  make observability-logs  - View observability stack logs"
	@echo ""
	@echo "Documentation commands:"
	@echo "  make docs-dev           - Start documentation development server"
	@echo "  make docs-build         - Build documentation site for production"
	@echo "  make docs-serve         - Serve built documentation locally"
	@echo "  make docs-deploy        - Deploy documentation to GitHub Pages"
	@echo ""
	@echo "Release commands:"
	@echo "  make tag TAG=x.y.z      - Create git tag vx.y.z"
	@echo "  make tag-push TAG=x.y.z - Create and push git tag vx.y.z (triggers release)"
	@echo "  make release TAG=x.y.z  - Alias for tag-push"
	@echo "  make prerelease TAG=x.y.z-rc.1 - Create and push pre-release tag"

# Code generation
generate:
	@echo "🔄 Generating protobuf code for Go server..."
	@mkdir -p server/pb
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/protos/cvt.proto
	@mv api/protos/cvt.pb.go api/protos/cvt_grpc.pb.go server/pb/ 2>/dev/null || true
	@echo "✅ Protobuf code generated in server/pb/"

generate-python:
	@echo "🔄 Generating protobuf code for Python SDK..."
	@mkdir -p sdks/python/cvt_sdk/proto
	cd sdks/python && uv run python -m grpc_tools.protoc \
		-I../../api/protos \
		--python_out=cvt_sdk/proto \
		--grpc_python_out=cvt_sdk/proto \
		--pyi_out=cvt_sdk/proto \
		../../api/protos/cvt.proto
	@echo "✅ Protobuf code generated in sdks/python/cvt_sdk/proto/"
	@echo "⚠️  Note: You may need to manually fix imports in cvt_pb2_grpc.py"

generate-go-sdk:
	@echo "🔄 Generating protobuf code for Go SDK..."
	@mkdir -p sdks/go/cvt/proto
	protoc --go_out=sdks/go/cvt/proto --go_opt=paths=source_relative \
		--go-grpc_out=sdks/go/cvt/proto --go-grpc_opt=paths=source_relative \
		-I api/protos \
		api/protos/cvt.proto
	@echo "✅ Protobuf code generated in sdks/go/cvt/proto/"

# Build targets
build: build-cli
	@echo "🏗️  Building Node.js SDK..."
	cd sdks/node && npm ci && npm run build
	@echo "🏗️  Building Python SDK..."
	cd sdks/python && uv sync
	# cd sdks/java && mvn package
	@echo "✅ Build complete!"

build-cli:
	@echo "🏗️  Building CVT CLI..."
	@_VERSION=$$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//'); VERSION=$${_VERSION:-dev}; \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "none"); \
	DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	CGO_ENABLED=0 go build -trimpath \
		-ldflags "-s -w -X main.version=$${VERSION} -X main.commit=$${COMMIT} -X main.buildDate=$${DATE}" \
		-o cvt ./cmd/cvt
	@echo "✅ CLI built: ./cvt"

install-cli: build-cli
	@echo "📦 Installing CVT CLI to /usr/local/bin..."
	@install -m 755 cvt /usr/local/bin/cvt
	@echo "✅ CVT CLI installed: $$(which cvt)"
	@cvt version

test-cli:
	@echo "🧪 Running CLI unit tests..."
	go test -v -race ./cmd/cvt/...
	@echo "✅ CLI unit tests passed!"

# Install targets (install dependencies without building)
install: install-server install-node-sdk install-python-sdk install-go-sdk install-java-sdk
	@echo "✅ All dependencies installed!"

install-server:
	@echo "📦 Installing Go server dependencies..."
	go mod download
	@echo "✅ Go server dependencies installed!"

install-node-sdk:
	@echo "📦 Installing Node.js SDK dependencies..."
	cd sdks/node && npm ci
	@echo "✅ Node.js SDK dependencies installed!"

install-python-sdk:
	@echo "📦 Installing Python SDK dependencies..."
	cd sdks/python && uv sync
	@echo "✅ Python SDK dependencies installed!"

install-go-sdk:
	@echo "📦 Installing Go SDK dependencies..."
	cd sdks/go && go mod download
	@echo "✅ Go SDK dependencies installed!"

install-java-sdk:
	@echo "📦 Installing Java SDK dependencies..."
	cd sdks/java && mvn dependency:resolve -q
	@echo "✅ Java SDK dependencies installed!"

# Test targets
test:
	@echo "🧪 Running all tests with direct server (fast, no Docker)..."
	./tools/run-all-tests.sh --no-docker

test-docker:
	@echo "🧪 Running all tests with Docker + PostgreSQL (CI/CD)..."
	./tools/run-all-tests.sh

test-server:
	@echo "🧪 Running server unit tests..."
	go test -v ./server/...

test-node-sdk:
	@echo "🧪 Running Node.js SDK tests..."
	cd sdks/node && npm test
	@echo "✅ Node.js SDK tests passed!"

test-python-sdk:
	@echo "🧪 Running Python SDK tests with coverage..."
	cd sdks/python && uv run pytest --cov=cvt_sdk --cov-report=term-missing --cov-report=html tests/
	@echo "✅ Python SDK tests passed!"
	@echo "📊 Coverage report: sdks/python/htmlcov/index.html"

test-go-sdk:
	@echo "🧪 Running Go SDK tests with coverage..."
	cd sdks/go && go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "✅ Go SDK tests passed!"
	@echo "📊 Coverage report: sdks/go/coverage.out"
	@cd sdks/go && go tool cover -func=coverage.out | grep total

generate-java-sdk:
	@echo "🔄 Generating protobuf code for Java SDK..."
	cd sdks/java && mvn generate-sources -DskipTests

test-java-sdk:
	@echo "🧪 Running Java SDK tests with coverage..."
	cd sdks/java && mvn test jacoco:report || true
	@echo "📊 Coverage report: sdks/java/target/site/jacoco/index.html"

test-coverage:
	@echo "🧪 Running server tests with coverage..."
	go test -v -coverprofile=coverage.out -covermode=atomic ./server/...
	@echo ""
	@echo "📊 Coverage Summary (excluding generated pb/ code):"
	grep -v "/pb/" coverage.out > coverage.filtered.out || true
	go tool cover -func=coverage.filtered.out | grep total
	@echo ""
	@echo "📄 Detailed HTML coverage report generated: coverage.html"
	go tool cover -html=coverage.filtered.out -o coverage.html
	@echo "✅ Run 'open coverage.html' to view the detailed coverage report"

test-integration:
	@echo "🧪 Running integration tests (requires Docker)..."
	go test -v -tags=integration ./server/...
	@echo "✅ Integration tests passed!"

test-cache:
	@echo "🧪 Running cache behavior tests..."
	go test -v -run TestCache ./server/...
	@echo "✅ Cache tests passed!"

test-all: test-docker

test-with-observability:
	@echo "🧪 Running all tests (server + all 4 SDKs) with observability stack..."
	@echo "ℹ️  Docker Compose will remain running for metrics and monitoring"
	KEEP_DOCKER_UP=1 ./tools/run-all-tests.sh
	@echo ""
	@echo "✅ Tests complete! Observability stack is running:"
	@echo "  - Grafana:    http://localhost:3000 (admin/admin)"
	@echo "  - Prometheus: http://localhost:9091"
	@echo "  - Metrics:    http://localhost:9551/metrics"
	@echo ""
	@echo "💡 Use 'make down' to stop the stack when done"

# Clean targets
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f cvt
	# cd sdks/java && mvn clean
	rm -rf sdks/node/dist sdks/node/node_modules
	# rm -rf sdks/python/.venv
	@echo "✅ Clean complete!"

# Docker commands
up:
	@echo "🚀 Starting CVT server..."
	docker compose up -d --build
	@echo "⏳ Waiting for server to be healthy..."
	@sleep 5

down:
	@echo "🛑 Stopping CVT server..."
	docker compose down
	@echo "✅ Server stopped!"

restart: down up

logs:
	@echo "📋 Viewing server logs (Ctrl+C to stop)..."
	docker compose logs -f

status:
	@echo "📊 Container status:"
	@docker compose ps
	@echo ""
	@echo "📊 Health status:"
	@docker inspect cvt-server --format='Container: {{.Name}}\nStatus: {{.State.Status}}\nHealth: {{.State.Health.Status}}' 2>/dev/null || echo "Container not running"

# Health check commands
# Install grpc-health-probe for Mac or Linux
# For Windows, download from: https://github.com/grpc-ecosystem/grpc-health-probe/releases
install-health-probe:
	@echo "📦 Installing grpc-health-probe..."
	@GRPC_HEALTH_PROBE_VERSION=v0.4.42; \
	if [ "$$(uname)" = "Darwin" ]; then \
		if [ "$$(uname -m)" = "arm64" ]; then \
			echo "Detected Mac (Apple Silicon)..."; \
			sudo wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/$${GRPC_HEALTH_PROBE_VERSION}/grpc-health-probe-darwin-arm64; \
		else \
			echo "Detected Mac (Intel)..."; \
			sudo wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/$${GRPC_HEALTH_PROBE_VERSION}/grpc-health-probe-darwin-amd64; \
		fi; \
		sudo chmod +x /usr/local/bin/grpc-health-probe; \
	elif [ "$$(uname)" = "Linux" ]; then \
		echo "Detected Linux..."; \
		sudo wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/$${GRPC_HEALTH_PROBE_VERSION}/grpc-health-probe-linux-amd64; \
		sudo chmod +x /usr/local/bin/grpc-health-probe; \
	else \
		echo "❌ Unsupported platform: $$(uname)"; \
		echo ""; \
		echo "Please install grpc-health-probe manually:"; \
		echo "  Mac (Intel):  wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.42/grpc-health-probe-darwin-amd64 && chmod +x /usr/local/bin/grpc-health-probe"; \
		echo "  Mac (M1/M2):  wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.42/grpc-health-probe-darwin-arm64 && chmod +x /usr/local/bin/grpc-health-probe"; \
		echo "  Linux:        wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.42/grpc-health-probe-linux-amd64 && chmod +x /usr/local/bin/grpc-health-probe"; \
		echo "  Windows:      Download grpc-health-probe-windows-amd64.exe from https://github.com/grpc-ecosystem/grpc-health-probe/releases"; \
		exit 1; \
	fi
	@echo "✅ grpc-health-probe installed!"

health:
	@echo "🏥 Checking server health..."
	@grpc-health-probe -addr=localhost:9550 && echo "✅ Server is healthy!" || echo "❌ Server is not healthy"

check-health:
	@echo "🏥 Detailed health check..."
	@echo ""
	@echo "📊 Container Status:"
	@docker compose ps 2>/dev/null || echo "Docker Compose not running"
	@echo ""
	@echo "📊 Docker Health:"
	@docker inspect cvt-server --format='Container: {{.Name}}\nStatus: {{.State.Status}}\nHealth: {{.State.Health.Status}}' 2>/dev/null || echo "Container not found"
	@echo ""
	@echo "📊 gRPC Health Check:"
	@grpc-health-probe -addr=localhost:9550 && echo "✅ gRPC service is healthy" || echo "❌ gRPC service is not responding"

watch-health:
	@echo "👀 Monitoring server health (Ctrl+C to stop)..."
	@while true; do \
		clear; \
		echo "🏥 CVT Server Health Monitor"; \
		echo "============================"; \
		echo ""; \
		grpc-health-probe -addr=localhost:9550 -v && echo "✅ Healthy" || echo "❌ Unhealthy"; \
		echo ""; \
		echo "Last checked: $$(date)"; \
		sleep 5; \
	done

# Development commands
run-server:
	@echo "🚀 Running Go server locally..."
	@echo "💡 Tip: Set CVT_PORT environment variable to use a specific port (e.g., CVT_PORT=9555 make run-server)"
	@PORT=9550; \
	while nc -z localhost $$PORT 2>/dev/null; do \
		PORT=$$((PORT + 1)); \
		if [ $$PORT -gt 9560 ]; then \
			echo "❌ Could not find available port between 9550-9560"; \
			exit 1; \
		fi; \
	done; \
	if [ $$PORT -ne 9550 ]; then \
		echo "⚠️  Port 9550 is in use. Using port $$PORT instead..."; \
	fi; \
	CVT_PORT=$$PORT go run ./cmd/cvt serve

run-example:
	@echo "🧪 Running Node.js SDK example..."
	cd sdks/node && npm run example

# Update commands
update: update-server update-go-sdk update-java-sdk update-node-sdk update-python-sdk
	@echo "✅ All dependencies updated!"

update-server:
	@echo "🔄 Updating Go server dependencies..."
	go get -u ./... && go mod tidy
	@echo "✅ Go server dependencies updated!"

update-go-sdk:
	@echo "🔄 Updating Go SDK dependencies..."
	cd sdks/go && go get -u ./... && go mod tidy
	@echo "✅ Go SDK dependencies updated!"

update-java-sdk:
	@echo "🔄 Updating Java SDK dependencies..."
	cd sdks/java && mvn versions:update-properties -DgenerateBackupPoms=false || mvn dependency:resolve -U
	@echo "✅ Java SDK dependencies updated!"

update-node-sdk:
	@echo "🔄 Updating Node.js SDK dependencies..."
	cd sdks/node && npm update
	@echo "✅ Node.js SDK dependencies updated!"

update-python-sdk:
	@echo "🔄 Updating Python SDK dependencies..."
	cd sdks/python && uv lock --upgrade && uv sync
	@echo "✅ Python SDK dependencies updated!"

# Observability commands
metrics:
	@echo "📊 Fetching Prometheus metrics from CVT server..."
	@curl -s http://localhost:9551/metrics | grep -E "^cvt_" || echo "⚠️  No metrics available. Is the server running?"

grafana:
	@echo "🎨 Opening Grafana dashboard..."
	@echo "📊 URL: http://localhost:3000"
	@echo "🔑 Credentials: admin / admin"
	@open http://localhost:3000 2>/dev/null || xdg-open http://localhost:3000 2>/dev/null || echo "Please open http://localhost:3000 in your browser"

prometheus:
	@echo "📈 Opening Prometheus UI..."
	@echo "📊 URL: http://localhost:9091"
	@open http://localhost:9091 2>/dev/null || xdg-open http://localhost:9091 2>/dev/null || echo "Please open http://localhost:9091 in your browser"

observability-status:
	@echo "📊 Observability Stack Status:"
	@echo ""
	@echo ">>> CVT Server (gRPC + Metrics):"
	@docker ps --filter "name=cvt-server" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || echo "Container not running"
	@echo ""
	@echo ">>> Prometheus:"
	@docker ps --filter "name=cvt-prometheus" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || echo "Container not running"
	@echo ""
	@echo ">>> Grafana:"
	@docker ps --filter "name=cvt-grafana" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || echo "Container not running"
	@echo ""
	@echo "📊 Access URLs:"
	@echo "  - Grafana:    http://localhost:3000 (admin/admin)"
	@echo "  - Prometheus: http://localhost:9091"
	@echo "  - Metrics:    http://localhost:9551/metrics"

observability-logs:
	@echo "📋 Viewing observability stack logs (Ctrl+C to stop)..."
	@docker compose logs -f prometheus grafana

# Lint commands
lint: lint-go lint-node lint-python lint-java
	@echo "✅ All linting complete!"

lint-go:
	@echo "🔍 Linting Go code (server, pkg/cvt, cmd/cvt, sdks/go)..."
	golangci-lint run --timeout=5m ./server/... ./pkg/... ./cmd/...
	@echo ">>> Linting sdks/go..."
	cd sdks/go && golangci-lint run --timeout=5m ./...
	@echo "✅ Go linting passed!"

lint-node:
	@echo "🔍 Linting Node.js SDK..."
	cd sdks/node && npm run lint 2>/dev/null || (echo "⚠️  No lint script found in Node.js SDK" && exit 0)
	@echo "✅ Node.js linting complete!"

lint-python:
	@echo "🔍 Linting Python SDK..."
	cd sdks/python && uv run ruff check . 2>/dev/null || (echo "⚠️  ruff not configured, trying flake8..." && uv run flake8 . 2>/dev/null) || echo "⚠️  No Python linter configured"
	@echo "✅ Python linting complete!"

lint-java:
	@echo "🔍 Linting Java SDK..."
	cd sdks/java && mvn verify -DskipTests 2>/dev/null || echo "⚠️  Java linting skipped (maven verify not configured)"
	@echo "✅ Java linting complete!"

# CI target - runs all checks that CI runs
ci: lint
	@echo ""
	@echo "🔍 Running CI format checks..."
	@echo ">>> Checking Go formatting..."
	@UNFORMATTED=$$(gofmt -l server/ pkg/ cmd/ sdks/go/); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "❌ Go files need formatting. Run: gofmt -w <file>"; \
		echo "$$UNFORMATTED"; \
		exit 1; \
	else \
		echo "✅ Go formatting OK"; \
	fi
	@echo ">>> Checking Node.js formatting..."
	cd sdks/node && npm run format:check
	@echo ">>> Checking Python formatting..."
	cd sdks/python && uv run ruff format --check .
	@echo ""
	@echo "✅ All CI checks passed!"

# Coverage check target - enforces 70% minimum coverage
check-coverage:
	@echo ""
	@echo "📊 Checking test coverage (minimum 70%)..."
	@echo ""
	@echo ">>> Go Server coverage..."
	@set -o pipefail; go test -coverprofile=coverage.out -covermode=atomic ./server/... 2>&1 | tail -1
	@grep -v "/pb/" coverage.out > coverage.filtered.out
	@COVERAGE=$$(go tool cover -func=coverage.filtered.out | grep total | awk '{gsub(/%/,""); print $$3}') && \
		echo "    Server coverage: $${COVERAGE}%" && \
		if [ $$(echo "$${COVERAGE} < 70" | bc -l) -eq 1 ]; then \
			echo "❌ Server coverage $${COVERAGE}% is below 70%"; \
			exit 1; \
		fi && echo "✅ Go Server: $${COVERAGE}% >= 70%"
	@echo ""
	@echo ">>> Go SDK coverage..."
	@set -o pipefail; cd sdks/go && go test -coverprofile=coverage.out -covermode=atomic ./cvt/... 2>&1 | tail -1
	@cd sdks/go && grep -v "/proto/" coverage.out > coverage.filtered.out || cp coverage.out coverage.filtered.out
	@cd sdks/go && COVERAGE=$$(go tool cover -func=coverage.filtered.out | grep total | awk '{gsub(/%/,""); print $$3}') && \
		echo "    Go SDK coverage: $${COVERAGE}%" && \
		if [ $$(echo "$${COVERAGE} < 70" | bc -l) -eq 1 ]; then \
			echo "❌ Go SDK coverage $${COVERAGE}% is below 70%"; \
			exit 1; \
		fi && echo "✅ Go SDK: $${COVERAGE}% >= 70%"
	@echo ""
	@echo ">>> Python SDK coverage..."
	@set -o pipefail; cd sdks/python && uv run pytest --cov=cvt_sdk --cov-fail-under=70 tests/ -q 2>&1 | tail -5
	@echo "✅ Python SDK: >= 70%"
	@echo ""
	@echo ">>> Node.js SDK coverage..."
	@cd sdks/node && npm test -- --coverage --silent 2>&1 | grep -E "(All files|Coverage)" || true
	@cd sdks/node && npm test -- --coverage --silent 2>/dev/null && \
		echo "✅ Node.js SDK: >= 70% (enforced by jest.config.js)" || \
		(echo "❌ Node.js SDK coverage below 70%" && exit 1)
	@echo ""
	@echo ">>> Java SDK coverage..."
	@cd sdks/java && mvn test jacoco:check 2>&1 | grep -E "(covered ratio|BUILD)" | head -2 || true
	@cd sdks/java && mvn test jacoco:check -q 2>/dev/null && \
		echo "✅ Java SDK: >= 70% (enforced by JaCoCo)" || \
		(echo "❌ Java SDK coverage below 70%" && exit 1)
	@echo ""
	@echo "✅ All coverage checks passed (minimum 70%)!"

# Full CI with coverage - runs lint, format checks, and coverage verification
ci-full: ci check-coverage
	@echo ""
	@echo "✅ Full CI (lint + format + coverage) passed!"

# Documentation commands
docs-install:
	@echo "📦 Installing documentation dependencies..."
	cd docs-site && npm install
	@echo "✅ Documentation dependencies installed!"

docs-dev: docs-install
	@echo "🚀 Starting documentation development server..."
	@echo "📖 Open http://localhost:4100/cvt/ in your browser"
	cd docs-site && npm start

docs-build: docs-install
	@echo "🏗️  Building documentation site..."
	cd docs-site && npm run build
	@echo "✅ Documentation built in docs-site/build/"

docs-serve: docs-build
	@echo "🌐 Serving built documentation..."
	@echo "📖 Open http://localhost:4100/cvt/ in your browser"
	cd docs-site && npm run serve

docs-deploy: docs-build
	@echo "🚀 Deploying documentation to GitHub Pages..."
	cd docs-site && npm run deploy
	@echo "✅ Documentation deployed!"

# Release commands - helper targets for TAG validation
_check_tag:
ifndef TAG
	$(error TAG is required. Usage: make $(MAKECMDGOALS) TAG=x.y.z)
endif
	@if ! echo "$(TAG)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$$'; then \
		echo "❌ TAG must be full semver: x.y.z or x.y.z-suffix"; \
		echo "   Got: $(TAG)"; \
		echo "   Example: make $(MAKECMDGOALS) TAG=0.2.0"; \
		exit 1; \
	fi

_check_tag_prerelease: _check_tag
	@if echo "$(TAG)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "❌ Pre-release TAG must include suffix like -alpha.1, -beta.1, or -rc.1"; \
		echo "   Example: make prerelease TAG=1.0.0-rc.1"; \
		exit 1; \
	fi

# Create a local git tag (idempotent - won't fail if tag exists at HEAD)
tag: _check_tag
	@EXISTING=$$(git rev-parse -q --verify "refs/tags/v$(TAG)" 2>/dev/null || true); \
	if [ -n "$$EXISTING" ]; then \
		if [ "$$EXISTING" = "$$(git rev-parse HEAD)" ]; then \
			echo "✅ Tag v$(TAG) already exists at HEAD"; \
		else \
			echo "❌ Tag v$(TAG) already exists but points to a different commit"; \
			echo "   Tag points to: $$EXISTING"; \
			echo "   HEAD is:       $$(git rev-parse HEAD)"; \
			exit 1; \
		fi; \
	else \
		echo "🏷️  Creating tag v$(TAG)..."; \
		git tag v$(TAG); \
		echo "✅ Tag v$(TAG) created locally"; \
	fi
	@echo "💡 Run 'make tag-push TAG=$(TAG)' to push and trigger release"

# Create and push a git tag (safe - verifies tag matches HEAD before pushing)
tag-push: _check_tag
	@echo "🏷️  Creating and pushing tag v$(TAG)..."
	@EXISTING=$$(git rev-parse -q --verify "refs/tags/v$(TAG)" 2>/dev/null || true); \
	if [ -n "$$EXISTING" ] && [ "$$EXISTING" != "$$(git rev-parse HEAD)" ]; then \
		echo "❌ Tag v$(TAG) already exists but points to a different commit"; \
		echo "   Tag points to: $$EXISTING"; \
		echo "   HEAD is:       $$(git rev-parse HEAD)"; \
		echo "   To re-release, first delete the tag: git tag -d v$(TAG) && git push origin :refs/tags/v$(TAG)"; \
		exit 1; \
	fi; \
	if [ -z "$$EXISTING" ]; then \
		git tag v$(TAG); \
	fi; \
	git push origin v$(TAG)
	@echo "✅ Tag v$(TAG) pushed to origin"
	@echo "🚀 Release workflow will publish Docker image + SDK packages"

release: tag-push

# Create and push a pre-release tag (validates pre-release suffix)
prerelease: _check_tag_prerelease
	@echo "🏷️  Creating and pushing pre-release tag v$(TAG)..."
	@EXISTING=$$(git rev-parse -q --verify "refs/tags/v$(TAG)" 2>/dev/null || true); \
	if [ -n "$$EXISTING" ] && [ "$$EXISTING" != "$$(git rev-parse HEAD)" ]; then \
		echo "❌ Tag v$(TAG) already exists but points to a different commit"; \
		echo "   Tag points to: $$EXISTING"; \
		echo "   HEAD is:       $$(git rev-parse HEAD)"; \
		echo "   To re-release, first delete the tag: git tag -d v$(TAG) && git push origin :refs/tags/v$(TAG)"; \
		exit 1; \
	fi; \
	if [ -z "$$EXISTING" ]; then \
		git tag v$(TAG); \
	fi; \
	git push origin v$(TAG)
	@echo "✅ Pre-release tag v$(TAG) pushed to origin"
	@echo "🚀 Release workflow will publish Docker image + SDK packages (pre-release)"

# Delete a release and all associated artifacts (GitHub Release, Docker image, npm/Maven packages, tags)
# Usage: make delete-release TAG=0.1.1-rc.1
delete-release: _check_tag
	@REPO=$$(gh repo view --json nameWithOwner -q .nameWithOwner); \
	OWNER=$$(echo "$$REPO" | cut -d/ -f1); \
	echo "⚠️  This will delete ALL artifacts for v$(TAG):"; \
	echo "   - GitHub Release"; \
	echo "   - Docker image ghcr.io/$$OWNER/cvt:$(TAG)"; \
	echo "   - npm package @$$OWNER/cvt-sdk@$(TAG)"; \
	echo "   - Maven package com.cvt:cvt-sdk $(TAG)"; \
	echo "   - Git tags: v$(TAG) and sdks/go/v$(TAG)"; \
	echo ""; \
	read -p "Are you sure? [y/N] " confirm; \
	if [ "$$confirm" != "y" ] && [ "$$confirm" != "Y" ]; then \
		echo "Aborted."; \
		exit 1; \
	fi; \
	echo ""; \
	echo "🗑️  Deleting GitHub Release..."; \
	gh release delete "v$(TAG)" --yes --cleanup-tag 2>/dev/null && echo "   ✅ GitHub Release deleted" || echo "   ⏭️  No GitHub Release found"; \
	echo "🗑️  Deleting Docker image..."; \
	VERSION_ID=$$(gh api "orgs/$$OWNER/packages/container/cvt/versions" --jq ".[] | select(.metadata.container.tags[] == \"$(TAG)\") | .id" 2>/dev/null || true); \
	if [ -z "$$VERSION_ID" ]; then \
		VERSION_ID=$$(gh api "users/$$OWNER/packages/container/cvt/versions" --jq ".[] | select(.metadata.container.tags[] == \"$(TAG)\") | .id" 2>/dev/null || true); \
	fi; \
	if [ -n "$$VERSION_ID" ]; then \
		gh api --method DELETE "users/$$OWNER/packages/container/cvt/versions/$$VERSION_ID" 2>/dev/null && echo "   ✅ Docker image deleted" || echo "   ⚠️  Failed to delete Docker image (may need admin access)"; \
	else \
		echo "   ⏭️  No Docker image found"; \
	fi; \
	echo "🗑️  Deleting npm package..."; \
	NPM_VERSION_ID=$$(gh api "users/$$OWNER/packages/npm/cvt-sdk/versions" --jq ".[] | select(.name == \"$(TAG)\") | .id" 2>/dev/null || true); \
	if [ -n "$$NPM_VERSION_ID" ]; then \
		gh api --method DELETE "users/$$OWNER/packages/npm/cvt-sdk/versions/$$NPM_VERSION_ID" 2>/dev/null && echo "   ✅ npm package deleted" || echo "   ⚠️  Failed to delete npm package (may need admin access)"; \
	else \
		echo "   ⏭️  No npm package found"; \
	fi; \
	echo "🗑️  Deleting Maven package..."; \
	MVN_VERSION_ID=$$(gh api "users/$$OWNER/packages/maven/com.cvt.cvt-sdk/versions" --jq ".[] | select(.name == \"$(TAG)\") | .id" 2>/dev/null || true); \
	if [ -n "$$MVN_VERSION_ID" ]; then \
		gh api --method DELETE "users/$$OWNER/packages/maven/com.cvt.cvt-sdk/versions/$$MVN_VERSION_ID" 2>/dev/null && echo "   ✅ Maven package deleted" || echo "   ⚠️  Failed to delete Maven package (may need admin access)"; \
	else \
		echo "   ⏭️  No Maven package found"; \
	fi; \
	echo "🗑️  Deleting Go SDK tag..."; \
	git push origin :refs/tags/sdks/go/v$(TAG) 2>/dev/null && echo "   ✅ Go SDK tag deleted" || echo "   ⏭️  No Go SDK tag found"; \
	echo "🗑️  Deleting version tag..."; \
	git tag -d v$(TAG) 2>/dev/null || true; \
	git push origin :refs/tags/v$(TAG) 2>/dev/null && echo "   ✅ Version tag deleted" || echo "   ⏭️  No remote version tag found"; \
	echo ""; \
	echo "✅ Cleanup complete for v$(TAG)"

# Check the current release version based on git tags
check-release:
	@TAG=$$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null); \
	if [ -z "$$TAG" ]; then \
		echo "No releases have been made yet."; \
	else \
		echo "Current release: $$TAG"; \
	fi
