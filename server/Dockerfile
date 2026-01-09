# Multi-stage Dockerfile for CVT Go Server
# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./
COPY pb/ ./pb/
COPY cvtservice/ ./cvtservice/
COPY storage/ ./storage/

# Build the binary with optimizations
# - CGO_ENABLED=0: Build a static binary
# - -ldflags="-w -s": Strip debug information to reduce size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cvt-server .

# Stage 2: Create minimal runtime image
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Install grpc-health-probe for health checks
# Detect architecture and download the appropriate binary
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then \
        BINARY="grpc_health_probe-linux-arm64"; \
    else \
        BINARY="grpc_health_probe-linux-amd64"; \
    fi && \
    wget -qO /bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.42/${BINARY} && \
    chmod +x /bin/grpc_health_probe

# Create non-root user
RUN addgroup -g 1000 cvt && \
    adduser -D -u 1000 -G cvt cvt

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cvt-server /app/cvt-server

# Copy health check script
COPY healthcheck.sh /app/healthcheck.sh
RUN chmod +x /app/healthcheck.sh

# Change ownership
RUN chown -R cvt:cvt /app

# Switch to non-root user
USER cvt

# Expose gRPC and metrics ports
EXPOSE 9550 9551

# Health check using grpc_health_probe
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/bin/grpc_health_probe", "-addr=:9550", "-service=cvt.ContractValidator"]

# Run the server
ENTRYPOINT ["/app/cvt-server"]
