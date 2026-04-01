#!/bin/bash
set -euo pipefail

# KodaClaw Community - Deploy Script
# Downloads latest server binary from GitHub Actions and deploys to docker container

REPO="koda-claw/kodaclaw-community"
CONTAINER="kodaclaw-community"
COMPOSE_DIR="/opt/kodaclaw-community"

echo "=== KodaClaw Community Deploy ==="

# 1. Get latest run ID for the deploy workflow
RUN_ID=$(curl -sf "https://api.github.com/repos/${REPO}/actions/workflows/deploy.yml/runs?branch=master&status=success&per_page=1" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['workflow_runs'][0]['id'])" 2>/dev/null)

if [ -z "$RUN_ID" ]; then
    echo "ERROR: No successful deploy workflow run found"
    exit 1
fi

echo "Latest successful deploy run: ${RUN_ID}"

# 2. Download artifact
ARTIFACT_URL="https://api.github.com/repos/${REPO}/actions/runs/${RUN_ID}/artifacts"
ARTIFACT_ID=$(curl -sf "$ARTIFACT_URL" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['artifacts'][0]['id'])" 2>/dev/null)

if [ -z "$ARTIFACT_ID" ]; then
    echo "ERROR: No artifact found for run ${RUN_ID}"
    exit 1
fi

echo "Downloading artifact ${ARTIFACT_ID}..."
curl -sfL -o /tmp/artifact.zip \
  "https://api.github.com/repos/${REPO}/actions/artifacts/${ARTIFACT_ID}/zip"

# 3. Extract binary
cd /tmp
unzip -o artifact.zip
mv server-linux-amd64/kc-server /tmp/kc-server
chmod +x /tmp/kc-server
rm -rf server-linux-amd64 artifact.zip

echo "Binary downloaded to /tmp/kc-server ($(du -h /tmp/kc-server | cut -f1))"

# 4. Stop container, replace binary, restart
cd "$COMPOSE_DIR"
docker cp /tmp/kc-server "${CONTAINER}:/app/kc-server"
docker exec "$CONTAINER" chmod +x /app/kc-server
docker restart "$CONTAINER"

# 5. Wait and verify
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
