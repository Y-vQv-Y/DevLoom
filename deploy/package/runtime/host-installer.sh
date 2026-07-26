#!/usr/bin/env bash
set -Eeuo pipefail

# Transparent replacement for the opaque host installer. This file is served
# as /static/installer/<arch>/installer and is invoked with the "host" command.

die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
log() { printf '[devloom-host] %s\n' "$*"; }

[[ "${1:-host}" == host ]] || die "supported command: host"
[[ "$(uname -s)" == Linux ]] || die "Linux is required"
[[ "$(id -u)" == 0 ]] || die "run as root"
for command in curl tar gzip sha256sum awk; do
  command -v "$command" >/dev/null 2>&1 || die "missing command: $command"
done

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=x86_64 ;;
  aarch64|arm64|armv8l) ARCH=aarch64 ;;
  *) die "unsupported architecture: $ARCH" ;;
esac
if [[ "$ARCH" == x86_64 ]] && { [[ ! -r /proc/cpuinfo ]] || ! grep -qiE '(^|[[:space:]])avx([[:space:]]|$)' /proc/cpuinfo; }; then
  die "the runner image requires an x86_64 CPU with AVX support"
fi

BASE_URL="${DEVLOOM_BASE_URL:-}"
HOST_BUNDLE_PATH="${DEVLOOM_HOST_BUNDLE_PATH:-}"
DOCKER_BUNDLE_PATH="${DEVLOOM_DOCKER_BUNDLE_PATH:-}"
TOKEN="${DEVLOOM_HOST_TOKEN:-}"
GRPC_URL="${DEVLOOM_TASKFLOW_GRPC_URL:-}"
EXTENSION_MANIFEST_PATH="${DEVLOOM_EXTENSION_IMAGES_MANIFEST_PATH:-}"
RUNNER_DIR="${DEVLOOM_RUNNER_DIR:-/opt/devloom-runner}"

[[ -n "$BASE_URL" ]] || die "DEVLOOM_BASE_URL is empty"
[[ -n "$HOST_BUNDLE_PATH" ]] || die "DEVLOOM_HOST_BUNDLE_PATH is empty"
[[ -n "$DOCKER_BUNDLE_PATH" ]] || die "DEVLOOM_DOCKER_BUNDLE_PATH is empty"
[[ -n "$TOKEN" ]] || die "DEVLOOM_HOST_TOKEN is empty"
[[ -n "$GRPC_URL" ]] || die "DEVLOOM_TASKFLOW_GRPC_URL is empty"

resolve_url() {
  case "$1" in
    http://*|https://*) printf '%s' "$1" ;;
    /*) printf '%s%s' "${BASE_URL%/}" "$1" ;;
    *) printf '%s/%s' "${BASE_URL%/}" "$1" ;;
  esac
}

WORK_DIR="$(mktemp -d)"
trap 'rm -rf -- "$WORK_DIR"' EXIT

download() {
  local url=$1 output=$2
  log "downloading $url"
  curl --fail --location --retry 3 --output "$output" "$url"
  curl --fail --location --silent --show-error --output "$output.sha256" "$url.sha256" || die "missing checksum sidecar: $url.sha256"
  expected="$(awk 'NR==1 {print $1}' "$output.sha256")"
  [[ "$expected" =~ ^[a-fA-F0-9]{64}$ ]] || die "invalid checksum sidecar: $url.sha256"
  printf '%s  %s\n' "$expected" "$output" | sha256sum --check --status - || die "checksum mismatch: $url"
}

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

HOST_URL="$(resolve_url "$HOST_BUNDLE_PATH")"
DOCKER_URL="$(resolve_url "$DOCKER_BUNDLE_PATH")"
download "$HOST_URL" "$WORK_DIR/host.tgz"
download "$DOCKER_URL" "$WORK_DIR/docker.tgz"
install_docker_bundle "$WORK_DIR/docker.tgz"

mkdir -p "$RUNNER_DIR"
tar -xzf "$WORK_DIR/host.tgz" -C "$RUNNER_DIR"
[[ -f "$RUNNER_DIR/docker-compose.yml" && -f "$RUNNER_DIR/.env" ]] || die "invalid runner host bundle"
set_env "$RUNNER_DIR/.env" TOKEN "$TOKEN"
set_env "$RUNNER_DIR/.env" GRPC_URL "$GRPC_URL"
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[[ -n "$HOST_IP" ]] || die "cannot determine host IP; set a routable address before starting the runner"
if [[ -r /etc/machine-id ]]; then
  MACHINE_ID="$(tr -cd 'a-zA-Z0-9_.-' </etc/machine-id | head -c 64)"
else
  MACHINE_ID="$(hostname | tr -cd 'a-zA-Z0-9_.-' | head -c 64)"
fi
[[ -n "$MACHINE_ID" ]] || die "cannot determine machine ID"
set_env "$RUNNER_DIR/.env" CENTER_URL "${BASE_URL%/}/runner"
set_env "$RUNNER_DIR/.env" MACHINE_ID "$MACHINE_ID"
set_env "$RUNNER_DIR/.env" HOST_NAME "$(hostname)"
set_env "$RUNNER_DIR/.env" ADVERTISE_URL "http://$HOST_IP:8890"
set_env "$RUNNER_DIR/.env" PUBLIC_IP "$HOST_IP"
set_env "$RUNNER_DIR/.env" WORKSPACE_HOST_ROOT "$RUNNER_DIR/data/workspaces"
set_env "$RUNNER_DIR/.env" PREVIEW_BASE_URL "http://$HOST_IP:9080"
chmod 600 "$RUNNER_DIR/.env"

for archive in "$RUNNER_DIR"/images/*.tar.gz; do
  [[ -f "$archive" ]] || continue
  log "loading $(basename "$archive")"
  gzip -dc "$archive" | docker load
done

if [[ -n "$EXTENSION_MANIFEST_PATH" ]]; then
  command -v python3 >/dev/null 2>&1 || die "python3 is required to import configured extension images"
  EXTENSION_MANIFEST_URL="$(resolve_url "$EXTENSION_MANIFEST_PATH")"
  curl --fail --location --retry 3 --output "$WORK_DIR/extensions.json" "$EXTENSION_MANIFEST_URL"
  index=0
  while IFS=$'\t' read -r archive_path archive_sha; do
    [[ -n "$archive_path" ]] || continue
    [[ "$archive_sha" =~ ^[a-fA-F0-9]{64}$ ]] || die "extension image is missing a valid SHA-256: $archive_path"
    index=$((index + 1))
    archive_file="$WORK_DIR/extension-$index"
    archive_url="$(resolve_url "$archive_path")"
    curl --fail --location --retry 3 --output "$archive_file" "$archive_url"
    printf '%s  %s\n' "$archive_sha" "$archive_file" | sha256sum --check --status - || die "extension image checksum mismatch: $archive_path"
    if gzip -t "$archive_file" >/dev/null 2>&1; then
      gzip -dc "$archive_file" | docker load
    else
      docker load --input "$archive_file"
    fi
  done < <(python3 - "$WORK_DIR/extensions.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    manifest = json.load(stream)
for package in manifest.get("packages", []):
    for image in package.get("images", []):
        print(f"{image.get('archive_url', '')}\t{image.get('sha256', '')}")
PY
  )
fi

docker compose --env-file "$RUNNER_DIR/.env" -f "$RUNNER_DIR/docker-compose.yml" up -d
docker compose --env-file "$RUNNER_DIR/.env" -f "$RUNNER_DIR/docker-compose.yml" ps
log "runner installed at $RUNNER_DIR"
