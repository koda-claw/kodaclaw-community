#!/bin/bash
set -euo pipefail

# KodaClaw Community - Deploy Script
# Downloads latest server binary from GitHub Release and deploys to docker container

REPO="koda-claw/kodaclaw-community"
TAG="deploy"
COMPOSE_DIR="/opt/kodaclaw-community"
BIN_DIR="${COMPOSE_DIR}/bin"

echo "=== KodaClaw Community Deploy ==="

# 1. Download binary from GitHub Release
echo "Downloading kc-server from release '${TAG}'..."
curl -sfL -o "${BIN_DIR}/kc-server" \
  "https://github.com/${REPO}/releases/download/${TAG}/kc-server"

# 2. Verify download
if [ ! -s "${BIN_DIR}/kc-server" ]; then
    echo "ERROR: Download failed or empty file"
    exit 1
fi

chmod +x "${BIN_DIR}/kc-server"
echo "Binary downloaded ($(du -h "${BIN_DIR}/kc-server" | cut -f1))"

# 3. Restart container (binary is mounted via volume, no docker cp needed)
cd "$COMPOSE_DIR"
docker restart kodaclaw-community

# 4. Wait and verify
echo "Waiting for container to start..."
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
