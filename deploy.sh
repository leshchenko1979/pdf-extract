#!/bin/bash
#
# Emergency / legacy: upload full source tarball and run docker compose --build on the server.
# Default path: push to main — GitHub Actions builds a GHCR image and deploys to /root/services/pdf-extract.
# To push only compose + runtime .env (no build on server): scripts/sync-vds-service.sh
#
# Requires .env with PUBLIC_BASE_URL, REMOTE_USER, and REMOTE_HOST_IP.
set -e

# Load environment variables
source .env

if [ -z "${PUBLIC_BASE_URL:-}" ]; then
    echo "PUBLIC_BASE_URL must be set in .env (same value as in docker-compose)"
    exit 1
fi
# Normalize: no trailing slash (matches app / .env.example)
PUBLIC_BASE_URL="${PUBLIC_BASE_URL%/}"
HEALTH_URL="${PUBLIC_BASE_URL}/health"

START_TIME=$(date +%s)

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[1;34m'
YELLOW='\033[1;33m'
MAGENTA='\033[1;35m'
NC='\033[0m'

SSH_OPTS="-o ControlMaster=auto -o ControlPath=/tmp/pdf_extract_ssh_%h_%p -o ControlPersist=60"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOCAL_TEMP_DIR="/tmp/pdf_extract_deploy_${TIMESTAMP}"
REMOTE_PROJECT_DIR="/root/services/pdf-extract"

echo -e "${BLUE}[INFO]${NC} Starting deployment preparation..."

mkdir -p "$LOCAL_TEMP_DIR"

echo -e "${YELLOW}[TRANSFER]${NC} Creating project archive..."
COPYFILE_DISABLE=1 tar \
    --exclude='uploads' \
    --exclude='outputs' \
    --exclude='.git' \
    -czf "$LOCAL_TEMP_DIR/pdf-extract.tar.gz" \
    .

echo -e "${YELLOW}[TRANSFER]${NC} Setting up remote environment and transferring files..."
ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST_IP" "mkdir -p $REMOTE_PROJECT_DIR"
scp $SSH_OPTS "$LOCAL_TEMP_DIR/pdf-extract.tar.gz" "$REMOTE_USER@$REMOTE_HOST_IP:$REMOTE_PROJECT_DIR/"
ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST_IP" "cd $REMOTE_PROJECT_DIR && tar xzf pdf-extract.tar.gz && rm pdf-extract.tar.gz"

echo -e "${GREEN}[DEPLOY]${NC} Building and deploying on remote server..."
ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST_IP" << ENDSSH
    cd $REMOTE_PROJECT_DIR

    echo "Stopping existing container..."
    docker compose down || true
    docker rm -f pdf-extract 2>/dev/null || true

    echo "Starting new container..."
    docker compose up -d --build

    echo "Container status:"
    docker ps | grep pdf-extract || true

    echo "Removing source files after successful deployment (keep docker-compose.yml and .env)..."
    rm -f Dockerfile go.mod go.sum openapi.yaml README.md deploy.sh .env.example .dockerignore .gitignore
    rm -rf cmd internal .vscode
ENDSSH

echo -e "${MAGENTA}[HEALTH]${NC} Verifying deployment..."
MAX_ATTEMPTS=12
SLEEP_BETWEEN=5
sleep 5
for i in $(seq 1 $MAX_ATTEMPTS); do
    if curl -fsS --max-time 10 "$HEALTH_URL" >/dev/null 2>&1; then
        echo -e "${GREEN}[HEALTH]${NC} Health check passed (attempt $i/$MAX_ATTEMPTS)"
        break
    fi
    if [ "$i" -eq "$MAX_ATTEMPTS" ]; then
        echo -e "${RED}[HEALTH]${NC} Health check failed after $MAX_ATTEMPTS attempts"
        exit 1
    fi
    echo -e "${YELLOW}[HEALTH]${NC} Attempt $i/$MAX_ATTEMPTS failed, retrying in ${SLEEP_BETWEEN}s..."
    sleep $SLEEP_BETWEEN
done

echo -e "${BLUE}[INFO]${NC} Cleaning up local files..."
rm -rf "$LOCAL_TEMP_DIR"

ELAPSED=$(($(date +%s) - START_TIME))
echo -e "${GREEN}[SUCCESS]${NC} Deployment completed in ${ELAPSED}s at $(date '+%Y-%m-%d %H:%M:%S')"
echo -e "${GREEN}API is available at: ${PUBLIC_BASE_URL}${NC}"
