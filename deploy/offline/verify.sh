#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${INSTALL_DIR:-/opt/devloom}"
ENV_FILE="${ENV_FILE:-$ROOT/.env}"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT/source/backend/docker-compose.yml}"
OVERRIDE_FILE="${COMPOSE_OVERRIDE_FILE:-$ROOT/compose.override.yml}"
TIMEOUT="${VERIFY_TIMEOUT_SECONDS:-180}"

die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
env_value() {
  local key=$1 value
  value="$(sed -n -E "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*//p" "$ENV_FILE" | head -n 1 || true)"
  value="${value%$'\r'}"
  case "$value" in
    \"*\") value="${value:1:${#value}-2}" ;;
    \'*\') value="${value:1:${#value}-2}" ;;
  esac
  printf '%s' "$value"
}

usage() {
  cat <<'EOF'
Usage: verify.sh [--root DIR] [--env-file FILE] [--compose-file FILE] [--override-file FILE]
EOF
}

while (($#)); do
  case "$1" in
    --root) (($# >= 2)) || die "--root requires a directory"; ROOT=$2; shift 2 ;;
    --env-file) (($# >= 2)) || die "--env-file requires a file"; ENV_FILE=$2; shift 2 ;;
    --compose-file) (($# >= 2)) || die "--compose-file requires a file"; COMPOSE_FILE=$2; shift 2 ;;
    --override-file) (($# >= 2)) || die "--override-file requires a file"; OVERRIDE_FILE=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -f "$ENV_FILE" ]] || die "missing environment file: $ENV_FILE"
[[ -f "$COMPOSE_FILE" ]] || die "missing Compose file: $COMPOSE_FILE"
command -v docker >/dev/null 2>&1 || die "Docker is not installed"
docker compose version >/dev/null 2>&1 || die "Docker Compose v2 is not available"
command -v curl >/dev/null 2>&1 || die "curl is not installed"

COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
[[ -f "$OVERRIDE_FILE" ]] && COMPOSE+=( -f "$OVERRIDE_FILE" )
"${COMPOSE[@]}" ps

STARTED_AT="$(date +%s)"
for service in db redis clickhouse rustfs ingress taskflow frontend backend preview; do
  while :; do
    state="$(docker inspect -f '{{.State.Status}}' "devloom-$service" 2>/dev/null || true)"
    [[ "$state" == running ]] && break
    now="$(date +%s)"
    (( now - STARTED_AT < TIMEOUT )) || die "service devloom-$service did not become running (state=${state:-missing})"
    sleep 3
  done
done

for service in db clickhouse rustfs; do
  while :; do
    health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "devloom-$service" 2>/dev/null || true)"
    [[ "$health" == healthy ]] && break
    [[ "$health" == unhealthy ]] && die "service devloom-$service reported unhealthy"
    now="$(date +%s)"
    (( now - STARTED_AT < TIMEOUT )) || die "service devloom-$service did not become healthy (health=${health:-unknown})"
    sleep 3
  done
done

REMOTE_IP="$(env_value REMOTE_IP)"
NGINX_PORT="$(env_value NGINX_PORT)"
BASE_URL="$(env_value PUBLIC_BASE_URL)"
[[ -n "$BASE_URL" ]] || BASE_URL="http://${REMOTE_IP}:${NGINX_PORT:-80}"
BASE_URL="${BASE_URL%/}"

http_status() {
  curl --silent --show-error --insecure --max-time 15 --output /dev/null --write-out '%{http_code}' "$1" || printf '000'
}

status="$(http_status "$BASE_URL/")"
[[ "$status" != 000 ]] || die "ingress is not reachable at $BASE_URL/"
printf 'Ingress response: HTTP %s\n' "$status"

status="$(http_status "$BASE_URL/api/v1/users/info")"
[[ "$status" != 000 ]] || die "backend API is not reachable through ingress"
printf 'Backend API response: HTTP %s (an unauthenticated 401/403 or business response is expected)\n' "$status"

printf '\nRecent runtime logs:\n'
"${COMPOSE[@]}" logs --tail=80 backend taskflow preview || true
printf '\nDeployment verification passed.\n'
