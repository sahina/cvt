#!/bin/bash
# Generate self-signed certificates for CVT local development and testing.
#
# Usage: ./tools/gen-certs.sh [output_dir] [domain]
#
# This creates:
#   - ca.crt, ca.key           - Certificate Authority
#   - server.crt, server.key   - Server certificate (for TLS)
#   - client.crt, client.key   - Client certificate (for mTLS testing)

set -e

CERT_DIR="${1:-./certs}"
DOMAIN="${2:-localhost}"
VALIDITY_DAYS=365

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Create output directory
mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

log_info "Generating certificates in: $CERT_DIR"
log_info "Domain: $DOMAIN"
log_info "Validity: $VALIDITY_DAYS days"

# Check if certificates already exist
if [ -f "ca.crt" ] && [ -f "server.crt" ]; then
    log_warn "Certificates already exist. Remove them first to regenerate."
    log_warn "  rm -rf $CERT_DIR/*.crt $CERT_DIR/*.key $CERT_DIR/*.csr"
    exit 0
fi

# Generate CA private key
log_info "Generating CA private key..."
openssl genrsa -out ca.key 4096

# Generate CA certificate
log_info "Generating CA certificate..."
openssl req -new -x509 -days $VALIDITY_DAYS -key ca.key -out ca.crt \
    -subj "/C=US/ST=Development/L=Local/O=CVT/OU=Testing/CN=CVT CA"

# Generate server private key
log_info "Generating server private key..."
openssl genrsa -out server.key 4096

# Create server certificate signing request
log_info "Creating server CSR..."
openssl req -new -key server.key -out server.csr \
    -subj "/C=US/ST=Development/L=Local/O=CVT/OU=Server/CN=$DOMAIN"

# Create server certificate extensions file (for SAN)
cat > server.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = $DOMAIN
DNS.2 = *.${DOMAIN}
DNS.3 = cvt-server
DNS.4 = *.cvt-server
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# Sign server certificate with CA
log_info "Signing server certificate..."
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out server.crt -days $VALIDITY_DAYS -extfile server.ext

# Generate client private key (for mTLS testing)
log_info "Generating client private key..."
openssl genrsa -out client.key 4096

# Create client certificate signing request
log_info "Creating client CSR..."
openssl req -new -key client.key -out client.csr \
    -subj "/C=US/ST=Development/L=Local/O=CVT/OU=Client/CN=cvt-client"

# Create client certificate extensions file
cat > client.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF

# Sign client certificate with CA
log_info "Signing client certificate..."
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out client.crt -days $VALIDITY_DAYS -extfile client.ext

# Clean up temporary files
rm -f *.csr *.ext *.srl

# Set restrictive permissions
chmod 600 *.key
chmod 644 *.crt

log_info "Certificates generated successfully!"
echo ""
echo "Files created:"
echo "  CA Certificate:     $CERT_DIR/ca.crt"
echo "  CA Key:             $CERT_DIR/ca.key"
echo "  Server Certificate: $CERT_DIR/server.crt"
echo "  Server Key:         $CERT_DIR/server.key"
echo "  Client Certificate: $CERT_DIR/client.crt"
echo "  Client Key:         $CERT_DIR/client.key"
echo ""
echo "Usage:"
echo "  # Start server with TLS"
echo "  CVT_TLS_ENABLED=true \\"
echo "  CVT_TLS_CERT_FILE=$CERT_DIR/server.crt \\"
echo "  CVT_TLS_KEY_FILE=$CERT_DIR/server.key \\"
echo "  make run-server"
echo ""
echo "  # For mTLS (mutual TLS)"
echo "  CVT_TLS_CA_FILE=$CERT_DIR/ca.crt \\"
echo "  CVT_TLS_CLIENT_AUTH=require"
