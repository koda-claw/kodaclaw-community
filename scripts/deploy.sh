#!/bin/bash
set -euo pipefail

REPO="koda-claw/kodaclaw-community"
TAG="deploy"
COMPOSE_DIR="/opt/kodaclaw-community"
BIN_DIR="${COMPOSE_DIR}/bin"
OS="linux"
ARCH="amd64"
HOST_ARCH="$(uname -m)"
if [ "$HOST_ARCH" = "aarch64" ] || [ "$HOST_ARCH" = "arm64" ]; then
    ARCH="arm64"
elif [ "$HOST_ARCH" = "x86_64" ]; then
    ARCH="amd64"
fi
TMP_PKG="/tmp/kodaclaw-server-${TAG}.tar.gz"
TMP_EXTRACT_DIR="/tmp/kodaclaw-server-${TAG}"
trap 'rm -rf "$TMP_EXTRACT_DIR" "$TMP_PKG"' EXIT

echo "=== KodaClaw Community Deploy ==="

# 1. Git pull to get latest source (frontend files, deploy.sh itself, etc.)
echo "Pulling latest source..."
cd "$COMPOSE_DIR"
git pull --ff-only 2>/dev/null || git pull 2>/dev/null || true
echo "Source updated."

# 2. Download server binary from dedicated server package
echo "Downloading kc-server from release '${TAG}'..."
curl -sfL -o "$TMP_PKG" \
  "https://github.com/${REPO}/releases/download/${TAG}/kodaclaw-server-${OS}-${ARCH}.tar.gz"

if [ ! -s "$TMP_PKG" ]; then
    echo "ERROR: Download failed or empty file: ${TMP_PKG}"
    exit 1
fi

mkdir -p "$TMP_EXTRACT_DIR"
tar -xzf "$TMP_PKG" -C "$TMP_EXTRACT_DIR" kc-server
if [ ! -f "$TMP_EXTRACT_DIR/kc-server" ]; then
    echo "ERROR: Extracted package does not contain kc-server"
    exit 1
fi

chmod +x "$TMP_EXTRACT_DIR/kc-server"
echo "Binary downloaded ($(du -h "$TMP_EXTRACT_DIR/kc-server" | cut -f1))"

# 3. Stop container
echo "Stopping container..."
docker stop kodaclaw-community

# 4. Replace binary (on mounted volume, no need to copy into container)
mv "$TMP_EXTRACT_DIR/kc-server" "${BIN_DIR}/kc-server"

# 5. Start container (static files are already synced via volume mount)
echo "Starting container..."
docker start kodaclaw-community

# 6. Wait and verify
sleep 3

HEALTH=$(curl -sf http://localhost:8080/api/v1/health 2>/dev/null || echo "UNHEALTHY")
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo "Deploy successful! Health check passed."
else
    echo "WARNING: Health check failed. Container may need manual inspection."
    docker logs --tail 20 kodaclaw-community 2>&1
    exit 1
fi

echo "=== Deploy Complete ==="
