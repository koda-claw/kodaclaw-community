#!/bin/bash
set -euo pipefail

# KodaClaw Community - Deploy Script
# Downloads latest server binary from GitHub Release and deploys to docker container

REPO="koda-claw/kodaclaw-community"
TAG="deploy"
CONTAINER="kodaclaw-community"
COMPOSE_DIR="/opt/kodaclaw-community"

echo "=== KodaClaw Community Deploy ==="

# 1. Download binary from GitHub Release
echo "Downloading kc-server from release '${TAG}'..."
curl -sfL -o /tmp/kc-server \
  "https://github.com/${REPO}/releases/download/${TAG}/kc-server"

# 2. Verify download
if [ ! -s /tmp/kc-server ]; then
    echo "ERROR: Download failed or empty file"
    exit 1
fi

chmod +x /tmp/kc-server
echo "Binary downloaded ($(du -h /tmp/kc-server | cut -f1))"

# 3. Replace binary in container and restart
cd "$COMPOSE_DIR"
docker cp /tmp/kc-server "${CONTAINER}:/app/kc-server"
docker exec "$CONTAINER" chmod +x /app/kc-server
docker restart "$CONTAINER"

# 4. Wait and verify
echo "Waiting for container to start..."
sleep 3

HEALTH=$(curl -sf http://localhost:8080/api/v1/health 2>/dev/null || echo "UNHEALTHY")
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo "Deploy successful! Health check passed."
else
    echo "WARNING: Health check failed. Container may need manual inspection."
    docker logs --tail 20 "$CONTAINER" 2>&1
    exit 1
fi

rm -f /tmp/kc-server
echo "=== Deploy Complete ==="
