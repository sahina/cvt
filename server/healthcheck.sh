#!/bin/sh
# Health check script for CVT Server
# Uses grpc-health-probe to check if the server is healthy
# This script can be used in Docker HEALTHCHECK
#
# Install grpc-health-probe:
#   Mac (Intel):    wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc-health-probe-darwin-amd64 && chmod +x /usr/local/bin/grpc-health-probe
#   Mac (M1/M2):    wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc-health-probe-darwin-arm64 && chmod +x /usr/local/bin/grpc-health-probe
#   Linux:          wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc-health-probe-linux-amd64 && chmod +x /usr/local/bin/grpc-health-probe
#   Windows:        Download grpc-health-probe-windows-amd64.exe from https://github.com/grpc-ecosystem/grpc-health-probe/releases

set -e

# Default values
HOST="${GRPC_HOST:-localhost}"
PORT="${GRPC_PORT:-9550}"
SERVICE="${GRPC_SERVICE:-cvt.ContractValidator}"

# Check if grpc-health-probe is installed
if ! command -v grpc-health-probe > /dev/null 2>&1; then
    echo "Error: grpc-health-probe is not installed"
    echo ""
    echo "Install instructions:"
    echo "  Mac (Intel):  wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc-health-probe-darwin-amd64 && chmod +x /usr/local/bin/grpc-health-probe"
    echo "  Mac (M1/M2):  wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc-health-probe-darwin-arm64 && chmod +x /usr/local/bin/grpc-health-probe"
    echo "  Linux:        wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc-health-probe-linux-amd64 && chmod +x /usr/local/bin/grpc-health-probe"
    echo "  Windows:      Download grpc-health-probe-windows-amd64.exe from https://github.com/grpc-ecosystem/grpc-health-probe/releases"
    exit 1
fi

# Perform health check
grpc-health-probe -addr="${HOST}:${PORT}" -service="${SERVICE}"

exit $?
