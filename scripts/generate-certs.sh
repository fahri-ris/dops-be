#!/bin/bash
# generate-certs.sh - Generate mTLS certificates for local development
# Usage: ./scripts/generate-certs.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CERTS_DIR="$PROJECT_ROOT/certs"

echo "=== mTLS Certificate Generator for Local Development ==="
echo ""

# Create certs directory
mkdir -p "$CERTS_DIR"

# Generate CA
echo "[1/3] Generating CA (Certificate Authority)..."
openssl genrsa -out "$CERTS_DIR/ca.key" 2048 2>/dev/null
openssl req -x509 -new -nodes -key "$CERTS_DIR/ca.key" -sha256 -days 365 \
    -out "$CERTS_DIR/ca.crt" -subj "//CN=DevCA" 2>/dev/null
echo "      Done: $CERTS_DIR/ca.crt"

# Generate Server certificate (for payment-service)
echo "[2/3] Generating server certificate (payment-service)..."
openssl genrsa -out "$CERTS_DIR/server.key" 2048 2>/dev/null
openssl req -new -key "$CERTS_DIR/server.key" -out "$CERTS_DIR/server.csr" \
    -subj "//CN=localhost" 2>/dev/null

# Create server extensions file for SAN
cat > "$CERTS_DIR/server.ext" << EOF
subjectAltName=DNS:localhost,IP:127.0.0.1
EOF

openssl x509 -req -in "$CERTS_DIR/server.csr" -CA "$CERTS_DIR/ca.crt" \
    -CAkey "$CERTS_DIR/ca.key" -CAcreateserial -days 365 \
    -out "$CERTS_DIR/server.crt" -extfile "$CERTS_DIR/server.ext" 2>/dev/null
rm -f "$CERTS_DIR/server.csr" "$CERTS_DIR/server.ext"
echo "      Done: $CERTS_DIR/server.crt, $CERTS_DIR/server.key"

# Generate Client certificate (for order-service)
echo "[3/3] Generating client certificate (order-service)..."
openssl genrsa -out "$CERTS_DIR/client.key" 2048 2>/dev/null
openssl req -new -key "$CERTS_DIR/client.key" -out "$CERTS_DIR/client.csr" \
    -subj "//CN=order-service" 2>/dev/null
openssl x509 -req -in "$CERTS_DIR/client.csr" -CA "$CERTS_DIR/ca.crt" \
    -CAkey "$CERTS_DIR/ca.key" -CAcreateserial -days 365 \
    -out "$CERTS_DIR/client.crt" 2>/dev/null
rm -f "$CERTS_DIR/client.csr"
echo "      Done: $CERTS_DIR/client.crt, $CERTS_DIR/client.key"

# Cleanup serial file
rm -f "$CERTS_DIR/ca.srl" 2>/dev/null

echo ""
echo "=== Certificates Generated Successfully ==="
echo ""
echo "Files created in: $CERTS_DIR"
echo ""
echo "  ca.crt       - CA certificate"
echo "  server.crt   - Server certificate (payment-service)"
echo "  server.key   - Server private key"
echo "  client.crt   - Client certificate (order-service)"
echo "  client.key   - Client private key"
echo ""
echo "Environment variables to use:"
echo ""
echo "=== payment-service/.env ==="
echo "MTLS_CERT=../certs/server.crt"
echo "MTLS_KEY=../certs/server.key"
echo "MTLS_CA_CERT=../certs/ca.crt"
echo ""
echo "=== order-service/.env ==="
echo "MTLS_CERT=../certs/client.crt"
echo "MTLS_KEY=../certs/client.key"
echo "MTLS_CA_CERT=../certs/ca.crt"
