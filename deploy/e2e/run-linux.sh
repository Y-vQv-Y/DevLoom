#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
REQUESTED_REMOTE_IP="${1:?usage: run-linux.sh VM_IP}"
INSTALL_DIR=/opt/devloom-e2e
ENV_FILE="$REPO_ROOT/deploy/e2e/.env"
if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck source=/dev/null
  . "$ENV_FILE"
  set +a
fi
REMOTE_IP="$REQUESTED_REMOTE_IP"
COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$REPO_ROOT/backend/docker-compose.yml" -f "$REPO_ROOT/deploy/e2e/docker-compose.override.yml")

random_secret() { openssl rand -hex "${1:-32}"; }
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(random_secret 24)}"
REDIS_PASSWORD="${REDIS_PASSWORD:-$(random_secret 24)}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-$(random_secret 24)}"
RUSTFS_ACCESS_KEY="${RUSTFS_ACCESS_KEY:-$(random_secret 12)}"
RUSTFS_SECRET_KEY="${RUSTFS_SECRET_KEY:-$(random_secret 24)}"
TEAM_EMAIL="${TEAM_EMAIL:-admin@devloom.local}"
TEAM_PASSWORD="${TEAM_PASSWORD:-$(random_secret 16)}"
RELAY_SECRET="${RELAY_SECRET:-$(random_secret 32)}"

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required; install the docker-buildx package" >&2
  exit 1
fi
export DOCKER_BUILDKIT=1

pull_image() {
  local image=$1 path=$1
  [[ "$path" == */* ]] || path="library/$path"
  local mirror="docker.m.daocloud.io/$path"
  if ! docker image inspect "$image" >/dev/null 2>&1; then
    docker pull "$mirror"
    docker tag "$mirror" "$image"
  fi
}

for image in \
  golang:1.25.8-alpine3.23 docker:29-cli ubuntu:24.04 \
  node:22.22.1-alpine3.23 nginx:1.29.5-alpine3.23 nginx:1.30.0 \
  postgres:17.4-alpine3.21 redis:8.0-alpine3.21 \
  clickhouse/clickhouse-server:26.3.9 rustfs/rustfs:1.0.0-beta.2; do
  pull_image "$image"
done

mkdir -p "$INSTALL_DIR"/{data/postgres,data/redis,data/clickhouse,data/rustfs,data/workspaces,logs/clickhouse,logs/rustfs,static,tls}
chown -R 10001:10001 "$INSTALL_DIR/data/rustfs" "$INSTALL_DIR/logs/rustfs"
if [[ ! -f "$INSTALL_DIR/tls/server.crt" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 7 \
    -keyout "$INSTALL_DIR/tls/server.key" -out "$INSTALL_DIR/tls/server.crt" \
    -subj "/CN=$REMOTE_IP" -addext "subjectAltName=IP:$REMOTE_IP" >/dev/null 2>&1
fi

cat > "$ENV_FILE" <<EOF
INSTALL_DIR=$INSTALL_DIR
COMPOSE_PROJECT_NAME=devloom-e2e
CONTAINER_PREFIX=devloom-e2e
REMOTE_IP=$REMOTE_IP
NGINX_PORT=8080
SUBNET_PREFIX=10.101.50
POSTGRES_IMAGE=postgres:17.4-alpine3.21
POSTGRES_DB=devloom
POSTGRES_USER=devloom
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
REDIS_IMAGE=redis:8.0-alpine3.21
REDIS_PASSWORD=$REDIS_PASSWORD
CLICKHOUSE_IMAGE=clickhouse/clickhouse-server:26.3.9
CLICKHOUSE_DB=devloom
CLICKHOUSE_USER=devloom
CLICKHOUSE_PASSWORD=$CLICKHOUSE_PASSWORD
RUSTFS_IMAGE=rustfs/rustfs:1.0.0-beta.2
RUSTFS_ACCESS_KEY=$RUSTFS_ACCESS_KEY
RUSTFS_SECRET_KEY=$RUSTFS_SECRET_KEY
FRONTEND_IMAGE=devloom.local/devloom-frontend:e2e
BACKEND_IMAGE=devloom.local/devloom-backend:e2e
INGRESS_IMAGE=devloom.local/devloom-ingress:e2e
TASKFLOW_IMAGE=devloom.local/devloom-taskflow:e2e
PREVIEW_IMAGE=devloom.local/devloom-preview:e2e
DEVBOX_IMAGE=devloom.local/devloom-devbox:e2e
TEAM_EMAIL=$TEAM_EMAIL
TEAM_NAME=DevLoom E2E
TEAM_PASSWORD=$TEAM_PASSWORD
INIT_TEAM_IMAGE=devloom.local/devloom-devbox:e2e
RELAY_SECRET=$RELAY_SECRET
WORKSPACE_ISOLATED=true
WORKSPACE_BRANCH_PREFIX=devloom
WORKSPACE_PUSH_MODE=pull_request
WORKSPACE_OPENHANDS_WORKTREE=true
PUBLIC_BASE_URL=http://$REMOTE_IP:8080
EOF

docker build --build-arg NPM_REGISTRY=https://registry.npmmirror.com -t devloom.local/devloom-frontend:e2e -f "$REPO_ROOT/frontend/docker/Dockerfile.source" "$REPO_ROOT/frontend"
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t devloom.local/devloom-backend:e2e -f "$REPO_ROOT/backend/build/Dockerfile" "$REPO_ROOT/backend"
docker build -t devloom.local/devloom-ingress:e2e -f "$REPO_ROOT/backend/build/Dockerfile.ingress" "$REPO_ROOT/backend"
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t devloom.local/devloom-taskflow:e2e -f "$REPO_ROOT/backend/build/Dockerfile.taskflow" "$REPO_ROOT/backend"
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t devloom.local/devloom-preview:e2e -f "$REPO_ROOT/backend/build/Dockerfile.preview" "$REPO_ROOT/backend"
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t devloom.local/devloom-orchestrator:e2e -f "$REPO_ROOT/backend/build/Dockerfile.orchestrator" "$REPO_ROOT/backend"
docker build -t devloom.local/devloom-devbox:e2e "$REPO_ROOT/devbox"
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t devloom.local/devloom-e2ellm:e2e -f "$REPO_ROOT/backend/build/Dockerfile.e2ellm" "$REPO_ROOT/backend"

"${COMPOSE[@]}" up -d
deadline=$((SECONDS + 600))
until curl --fail --silent --show-error "http://$REMOTE_IP:8080/" >/dev/null 2>&1; do
  if ((SECONDS >= deadline)); then
    "${COMPOSE[@]}" ps
    "${COMPOSE[@]}" logs --tail=200 backend taskflow ingress
    exit 1
  fi
  sleep 5
done

"${COMPOSE[@]}" ps
printf 'DEVLOOM_E2E_URL=http://%s:8080\n' "$REMOTE_IP"
printf 'DEVLOOM_E2E_EMAIL=%s\n' "$TEAM_EMAIL"
printf 'DEVLOOM_E2E_PASSWORD=%s\n' "$TEAM_PASSWORD"
