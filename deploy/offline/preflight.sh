#!/usr/bin/env bash
set -Eeuo pipefail

# Validate every local prerequisite needed for a complete intranet deployment.

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
ROOT="${INSTALL_DIR:-/opt/devloom}"
ENV_FILE="${ENV_FILE:-}"
COMPOSE_FILE="${COMPOSE_FILE:-}"
OVERRIDE_FILE="${COMPOSE_OVERRIDE_FILE:-}"
SKIP_DOCKER=0

usage() {
  cat <<'EOF'
Usage: preflight.sh [options]

Options:
  --root DIR           installation root (default: /opt/devloom)
  --env-file FILE      Compose environment file
  --compose-file FILE  base Compose file
  --override-file FILE optional Compose override file
  --skip-docker        skip Docker checks (for offline preparation only)
  -h, --help           show this help
EOF
}

die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
warn() { printf 'WARN: %s\n' "$*" >&2; }

env_value() {
  local key=$1 value
  [[ -f "$ENV_FILE" ]] || return 0
  value="$(sed -n -E "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*//p" "$ENV_FILE" | head -n 1 || true)"
  value="${value%$'\r'}"
  case "$value" in
    \"*\") value="${value:1:${#value}-2}" ;;
    \'*\') value="${value:1:${#value}-2}" ;;
  esac
  printf '%s' "$value"
}

has_placeholder() {
  local value
  value="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"
  [[ "$value" == *replace-with* || "$value" == *change_me* || "$value" == *example.com* || "$value" == *example.org* || "$value" == *.example || "$value" == *compatible-version* || "$value" == *release_tag* || "$value" == *your-domain* || "$value" == *'<'* || "$value" == *'>'* ]]
}

while (($#)); do
  case "$1" in
    --root) (($# >= 2)) || die "--root requires a directory"; ROOT=$2; shift 2 ;;
    --env-file) (($# >= 2)) || die "--env-file requires a file"; ENV_FILE=$2; shift 2 ;;
    --compose-file) (($# >= 2)) || die "--compose-file requires a file"; COMPOSE_FILE=$2; shift 2 ;;
    --override-file) (($# >= 2)) || die "--override-file requires a file"; OVERRIDE_FILE=$2; shift 2 ;;
    --skip-docker) SKIP_DOCKER=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -n "$ENV_FILE" ]] || ENV_FILE="$ROOT/.env"
if [[ -z "$COMPOSE_FILE" ]]; then
  if [[ -f "$ROOT/docker-compose.yml" ]]; then
    COMPOSE_FILE="$ROOT/docker-compose.yml"
  else
    COMPOSE_FILE="$ROOT/source/backend/docker-compose.yml"
  fi
fi
[[ -n "$OVERRIDE_FILE" ]] || OVERRIDE_FILE="$ROOT/compose.override.yml"

if [[ ! -f "$COMPOSE_FILE" && -f "$REPO_ROOT/backend/docker-compose.yml" ]]; then
  COMPOSE_FILE="$REPO_ROOT/backend/docker-compose.yml"
fi

[[ -f "$ENV_FILE" ]] || die "missing environment file: $ENV_FILE (copy deploy/offline/.env.example and fill every value)"
[[ -f "$COMPOSE_FILE" ]] || die "missing Compose file: $COMPOSE_FILE"

[[ "$(env_value INSTALL_DIR)" == "$ROOT" ]] || die "INSTALL_DIR in $ENV_FILE must equal $ROOT"

REQUIRED_VARS=(
  INSTALL_DIR COMPOSE_PROJECT_NAME CONTAINER_PREFIX REMOTE_IP NGINX_PORT SUBNET_PREFIX
  POSTGRES_IMAGE POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD
  REDIS_IMAGE REDIS_PASSWORD
  CLICKHOUSE_IMAGE CLICKHOUSE_DB CLICKHOUSE_USER CLICKHOUSE_PASSWORD
  RUSTFS_IMAGE RUSTFS_ACCESS_KEY RUSTFS_SECRET_KEY
  FRONTEND_IMAGE BACKEND_IMAGE INGRESS_IMAGE TASKFLOW_IMAGE PREVIEW_IMAGE
  TEAM_EMAIL TEAM_NAME TEAM_PASSWORD RELAY_SECRET
)
for name in "${REQUIRED_VARS[@]}"; do
  value="$(env_value "$name")"
  [[ -n "$value" ]] || die "$name is empty in $ENV_FILE"
  has_placeholder "$value" && die "$name still contains a placeholder value"
done

for name in POSTGRES_IMAGE REDIS_IMAGE CLICKHOUSE_IMAGE RUSTFS_IMAGE FRONTEND_IMAGE BACKEND_IMAGE INGRESS_IMAGE TASKFLOW_IMAGE PREVIEW_IMAGE; do
  value="$(env_value "$name")"
  [[ "$value" != *:latest ]] || die "$name must be pinned to a version or digest, not :latest"
done

RELAY_VALUE="$(env_value RELAY_SECRET)"
(( ${#RELAY_VALUE} >= 32 )) || die "RELAY_SECRET must contain at least 32 characters"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH_DIR=x86_64 ;;
  aarch64|arm64) ARCH_DIR=aarch64 ;;
  *) die "unsupported CPU architecture: $ARCH" ;;
esac

[[ -s "$ROOT/static/project-tpl.zip" ]] || die "missing $ROOT/static/project-tpl.zip"
for file in installer host.tgz docker.tgz; do
  path="$ROOT/static/installer/$ARCH_DIR/$file"
  [[ -s "$path" ]] || die "missing $path"
  [[ -s "$path.sha256" ]] || die "missing $path.sha256"
done
[[ -x "$ROOT/static/installer/$ARCH_DIR/installer" ]] || die "installer is not executable: $ROOT/static/installer/$ARCH_DIR/installer"

TLS_CERT="$ROOT/tls/server.crt"
TLS_KEY="$ROOT/tls/server.key"
[[ -s "$TLS_CERT" ]] || die "missing TLS certificate: $TLS_CERT"
[[ -s "$TLS_KEY" ]] || die "missing TLS private key: $TLS_KEY"
if command -v openssl >/dev/null 2>&1; then
  openssl x509 -in "$TLS_CERT" -noout >/dev/null 2>&1 || die "invalid TLS certificate: $TLS_CERT"
fi

if ((SKIP_DOCKER)); then
  warn "Docker checks skipped by request"
else
  command -v docker >/dev/null 2>&1 || die "Docker is not installed"
  docker compose version >/dev/null 2>&1 || die "Docker Compose v2 is not available"
  COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
  [[ -f "$OVERRIDE_FILE" ]] && COMPOSE+=( -f "$OVERRIDE_FILE" )
  "${COMPOSE[@]}" config >/dev/null || die "docker compose config failed; fix the environment or override file"
fi

printf 'Preflight passed for %s (%s). Required runtime and host assets are present.\n' "$ROOT" "$ARCH_DIR"
