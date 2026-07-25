#!/usr/bin/env bash
set -Eeuo pipefail

# Auditable, non-interactive-capable center installer for a DevLoom bundle.

PACKAGE_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/opt/devloom}"
ACCESS_HOST="${DEVLOOM_ACCESS_HOST:-}"
ADMIN_EMAIL="${DEVLOOM_ADMIN_EMAIL:-}"
ADMIN_PASSWORD="${DEVLOOM_ADMIN_PASSWORD:-}"
NO_START=0

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --install-dir DIR      installation directory (default: /opt/devloom)
  --host HOST_OR_IP      address used by browsers and runner hosts
  --admin-email EMAIL    initial administrator email
  --admin-password PASS  initial administrator password (generated if omitted)
  --no-start             prepare files and load images without starting services
  -h, --help             show this help

The same command upgrades an existing installation while preserving .env and data.
EOF
}

die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
log() { printf '[devloom] %s\n' "$*"; }

while (($#)); do
  case "$1" in
    --install-dir) (($# >= 2)) || die "--install-dir requires a directory"; INSTALL_DIR=$2; shift 2 ;;
    --host) (($# >= 2)) || die "--host requires a value"; ACCESS_HOST=$2; shift 2 ;;
    --admin-email) (($# >= 2)) || die "--admin-email requires a value"; ADMIN_EMAIL=$2; shift 2 ;;
    --admin-password) (($# >= 2)) || die "--admin-password requires a value"; ADMIN_PASSWORD=$2; shift 2 ;;
    --no-start) NO_START=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ "$(uname -s)" == Linux ]] || die "Linux is required"
[[ "$(id -u)" == 0 ]] || die "run as root"
case "$(uname -m)" in
  x86_64|amd64) ;;
  *) die "this package targets linux/amd64" ;;
esac
for command in awk gzip sha256sum tar; do
  command -v "$command" >/dev/null 2>&1 || die "missing command: $command"
done
[[ -f "$PACKAGE_DIR/manifest.json" && -f "$PACKAGE_DIR/SHA256SUMS" ]] || die "package manifest is missing"

log "verifying package checksums"
(
  cd "$PACKAGE_DIR"
  sha256sum --check --strict SHA256SUMS
)

WORK_DIR="$(mktemp -d)"
trap 'rm -rf -- "$WORK_DIR"' EXIT

install_docker_bundle() {
  local archive=$1 unpack="$WORK_DIR/docker"
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    return
  fi
  mkdir -p "$unpack"
  tar -xzf "$archive" -C "$unpack"
  [[ -x "$unpack/docker/dockerd" ]] || die "invalid Docker static bundle"
  install -m 0755 "$unpack/docker/"* /usr/local/bin/
  mkdir -p /usr/local/lib/docker/cli-plugins
  install -m 0755 "$unpack/docker-compose" /usr/local/lib/docker/cli-plugins/docker-compose
  install -m 0755 "$unpack/docker-compose" /usr/local/bin/docker-compose
  command -v systemctl >/dev/null 2>&1 || die "systemd is required to install the bundled Docker Engine"

  cat > /etc/systemd/system/containerd.service <<'EOF'
[Unit]
Description=containerd container runtime
After=network.target local-fs.target
[Service]
ExecStart=/usr/local/bin/containerd
Delegate=yes
KillMode=process
Restart=always
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
[Install]
WantedBy=multi-user.target
EOF
  cat > /etc/systemd/system/docker.service <<'EOF'
[Unit]
Description=Docker Application Container Engine
After=network-online.target containerd.service
Wants=network-online.target
Requires=containerd.service
[Service]
Type=notify
ExecStart=/usr/local/bin/dockerd --host=unix:///var/run/docker.sock --containerd=/run/containerd/containerd.sock
ExecReload=/bin/kill -s HUP $MAINPID
Restart=always
StartLimitBurst=3
StartLimitIntervalSec=60
Delegate=yes
KillMode=process
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now containerd docker
  docker info >/dev/null 2>&1 || die "Docker Engine failed to start"
  docker compose version >/dev/null 2>&1 || die "Docker Compose plugin is unavailable"
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "${1:-32}"
  else
    od -An -N "${1:-32}" -tx1 /dev/urandom | tr -d ' \n'
  fi
}

set_env() {
  local file=$1 key=$2 value=$3 tmp
  tmp="$(mktemp)"
  awk -v key="$key" -v value="$value" '
    BEGIN { found=0 }
    $0 ~ "^" key "=" { print key "=" value; found=1; next }
    { print }
    END { if (!found) print key "=" value }
  ' "$file" > "$tmp"
  mv "$tmp" "$file"
}

install_docker_bundle "$PACKAGE_DIR/docker.tgz"

log "loading container images"
for archive in "$PACKAGE_DIR"/images/*.tar.gz; do
  [[ -f "$archive" ]] || continue
  log "loading $(basename "$archive")"
  gzip -dc "$archive" | docker load
done

mkdir -p "$INSTALL_DIR"/{data,logs,static,tls,extensions/packages,backup,metadata,tools}
if [[ -f "$INSTALL_DIR/docker-compose.yml" ]]; then
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  mkdir -p "$INSTALL_DIR/backup/$stamp"
  cp -a "$INSTALL_DIR/docker-compose.yml" "$INSTALL_DIR/backup/$stamp/"
  [[ -f "$INSTALL_DIR/.env" ]] && cp -a "$INSTALL_DIR/.env" "$INSTALL_DIR/backup/$stamp/"
fi

cp -a "$PACKAGE_DIR/docker-compose.yml" "$INSTALL_DIR/docker-compose.yml"
cp -a "$PACKAGE_DIR/manifest.json" "$INSTALL_DIR/metadata/package-manifest.json"
cp -a "$PACKAGE_DIR/static/." "$INSTALL_DIR/static/"
if [[ -d "$PACKAGE_DIR/extensions/packages" ]]; then
  cp -a "$PACKAGE_DIR/extensions/packages/." "$INSTALL_DIR/extensions/packages/"
fi
cp -a "$PACKAGE_DIR/preflight.sh" "$PACKAGE_DIR/verify.sh" "$INSTALL_DIR/tools/"
chmod +x "$INSTALL_DIR/tools/"*.sh "$INSTALL_DIR/static/installer/"*/installer

ENV_FILE="$INSTALL_DIR/.env"
GENERATED_PASSWORD=0
if [[ ! -f "$ENV_FILE" ]]; then
  cp "$PACKAGE_DIR/.env.example" "$ENV_FILE"
  [[ -n "$ACCESS_HOST" ]] || read -r -p "DevLoom access hostname or IP: " ACCESS_HOST
  [[ -n "$ADMIN_EMAIL" ]] || read -r -p "Initial administrator email: " ADMIN_EMAIL
  [[ -n "$ACCESS_HOST" ]] || die "access host is required"
  [[ "$ADMIN_EMAIL" == *@* ]] || die "a valid administrator email is required"
  if [[ -z "$ADMIN_PASSWORD" ]]; then
    ADMIN_PASSWORD="$(random_secret 16)"
    GENERATED_PASSWORD=1
  fi
  set_env "$ENV_FILE" INSTALL_DIR "$INSTALL_DIR"
  set_env "$ENV_FILE" REMOTE_IP "$ACCESS_HOST"
  set_env "$ENV_FILE" PUBLIC_BASE_URL "http://$ACCESS_HOST"
  set_env "$ENV_FILE" TEAM_EMAIL "$ADMIN_EMAIL"
  set_env "$ENV_FILE" TEAM_PASSWORD "$ADMIN_PASSWORD"
  set_env "$ENV_FILE" POSTGRES_PASSWORD "$(random_secret 32)"
  set_env "$ENV_FILE" REDIS_PASSWORD "$(random_secret 32)"
  set_env "$ENV_FILE" CLICKHOUSE_PASSWORD "$(random_secret 32)"
  set_env "$ENV_FILE" RUSTFS_ACCESS_KEY "$(random_secret 16)"
  set_env "$ENV_FILE" RUSTFS_SECRET_KEY "$(random_secret 32)"
  set_env "$ENV_FILE" RELAY_SECRET "$(random_secret 32)"
else
  log "preserving existing $ENV_FILE"
fi
chmod 600 "$ENV_FILE"

if [[ ! -s "$INSTALL_DIR/tls/server.crt" || ! -s "$INSTALL_DIR/tls/server.key" ]]; then
  command -v openssl >/dev/null 2>&1 || die "openssl is required to generate the ingress certificate"
  [[ -n "$ACCESS_HOST" ]] || ACCESS_HOST="$(awk -F= '$1=="REMOTE_IP" {print substr($0,index($0,"=")+1); exit}' "$ENV_FILE")"
  if [[ "$ACCESS_HOST" =~ ^[0-9a-fA-F:.]+$ ]]; then
    SAN="IP:$ACCESS_HOST"
  else
    SAN="DNS:$ACCESS_HOST"
  fi
  openssl req -x509 -nodes -newkey rsa:3072 -days 825 \
    -keyout "$INSTALL_DIR/tls/server.key" \
    -out "$INSTALL_DIR/tls/server.crt" \
    -subj "/CN=$ACCESS_HOST" \
    -addext "subjectAltName=$SAN"
  chmod 600 "$INSTALL_DIR/tls/server.key"
fi

"$INSTALL_DIR/tools/preflight.sh" --root "$INSTALL_DIR"
if ((NO_START)); then
  log "package prepared at $INSTALL_DIR; services were not started"
  exit 0
fi

docker compose --env-file "$ENV_FILE" -f "$INSTALL_DIR/docker-compose.yml" up -d
"$INSTALL_DIR/tools/verify.sh" --root "$INSTALL_DIR"
log "installation completed: http://$ACCESS_HOST"
if ((GENERATED_PASSWORD)); then
  printf 'Initial administrator: %s\nInitial password: %s\n' "$ADMIN_EMAIL" "$ADMIN_PASSWORD"
  printf 'Change this password immediately after the first login.\n'
fi
