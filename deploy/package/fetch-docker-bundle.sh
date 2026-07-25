#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: fetch-docker-bundle.sh OUTPUT ARCH DOCKER_VERSION COMPOSE_VERSION DOCKER_SHA256 COMPOSE_SHA256

ARCH is amd64 or arm64. The resulting archive contains Docker Engine static
binaries and the Docker Compose CLI plugin for an offline host.
EOF
}

[[ $# -eq 6 ]] || { usage >&2; exit 2; }
OUTPUT=$1
ARCH=$2
DOCKER_VERSION=$3
COMPOSE_VERSION=$4
DOCKER_SHA256=$5
COMPOSE_SHA256=$6

case "$ARCH" in
  amd64) DOCKER_ARCH=x86_64; COMPOSE_ARCH=x86_64 ;;
  arm64) DOCKER_ARCH=aarch64; COMPOSE_ARCH=aarch64 ;;
  *) printf 'unsupported architecture: %s\n' "$ARCH" >&2; exit 1 ;;
esac

for command in curl tar sha256sum; do
  command -v "$command" >/dev/null 2>&1 || { printf 'missing command: %s\n' "$command" >&2; exit 1; }
done

WORK_DIR="$(mktemp -d)"
trap 'rm -rf -- "$WORK_DIR"' EXIT
DOCKER_URL="https://download.docker.com/linux/static/stable/${DOCKER_ARCH}/docker-${DOCKER_VERSION}.tgz"
COMPOSE_BASE="https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-${COMPOSE_ARCH}"

printf 'Downloading Docker Engine %s (%s)...\n' "$DOCKER_VERSION" "$ARCH"
curl --fail --location --retry 3 --output "$WORK_DIR/docker.tgz" "$DOCKER_URL"
(
  cd "$WORK_DIR"
  printf '%s  %s\n' "$DOCKER_SHA256" docker.tgz | sha256sum --check --status -
)
tar -xzf "$WORK_DIR/docker.tgz" -C "$WORK_DIR"

printf 'Downloading Docker Compose %s (%s)...\n' "$COMPOSE_VERSION" "$ARCH"
curl --fail --location --retry 3 --output "$WORK_DIR/docker-compose" "$COMPOSE_BASE"
(
  cd "$WORK_DIR"
  printf '%s  %s\n' "$COMPOSE_SHA256" docker-compose | sha256sum --check --status -
)
chmod +x "$WORK_DIR/docker-compose" "$WORK_DIR/docker/"*

mkdir -p "$(dirname -- "$OUTPUT")"
tar -C "$WORK_DIR" -cf - docker docker-compose | gzip -n > "$OUTPUT"
printf 'Created %s\n' "$OUTPUT"
