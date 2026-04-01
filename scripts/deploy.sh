#!/bin/bash
set -euo pipefail

REPO="koda-claw/kodaclaw-community"
TAG="deploy"
COMPOSE_DIR="/opt/kodaclaw-community"
BIN_DIR="${COMPOSE_DIR}/bin"

echo "=== KodaClaw Community Deploy ==="

# 1. Download binary to temp location (not the mounted path)
echo "Downloading kc-server from release '${TAG}'..."
curl -sfL -o /tmp/kc-server-new \
  "https://github.com/${REPO}/releases/download/${TAG}/kc-server"

if [ ! -s /tmp/kc-server-new ]; then
    echo "ERROR: Download failed or empty file"
    exit 1
fi

chmod +x /tmp/kc-server-new
echo "Binary downloaded ($(du -h /tmp/kc-server-new | cut -f1))"

# 2. Stop container before replacing binary (file busy if running)
echo "Stopping container..."
cd "$COMPOSE_DIR"
docker stop kodaclaw-community

# 3. Replace binary
mv /tmp/kc-server-new "${BIN_DIR}/kc-server"

# 4. Start container
echo "Starting container..."
docker start kodaclaw-community

# 5. Wait and verify
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
