#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
CONFIG_FILE="$SCRIPT_DIR/package.env"
VERSION=""

usage() {
  cat <<'EOF'
Usage: build.sh [--config FILE] [--version VERSION]

Builds a complete linux/amd64 DevLoom offline package from the current source
tree and pinned images already present in the local Docker daemon.
EOF
}

die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
log() { printf '[package] %s\n' "$*"; }

while (($#)); do
  case "$1" in
    --config) (($# >= 2)) || die "--config requires a file"; CONFIG_FILE=$2; shift 2 ;;
    --version) (($# >= 2)) || die "--version requires a value"; VERSION=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -f "$CONFIG_FILE" ]] || die "missing $CONFIG_FILE; copy package.env.example and pin all runtime images"
set -a
# shellcheck source=/dev/null
. "$CONFIG_FILE"
set +a

for command in docker python3 tar gzip sha256sum cp; do
  command -v "$command" >/dev/null 2>&1 || die "missing build dependency: $command"
done
if [[ -z "${SOURCE_COMMIT:-}" || -z "$VERSION" ]]; then
  command -v git >/dev/null 2>&1 || die "git is required unless SOURCE_COMMIT and --version are provided"
fi
docker info >/dev/null 2>&1 || die "Docker Engine is not running"
docker buildx version >/dev/null 2>&1 || die "Docker Buildx is unavailable"

BRAND_NAME="${BRAND_NAME:-DevLoom}"
BRAND_SLUG="${BRAND_SLUG:-devloom}"
DEFAULT_INSTALL_DIR="${DEFAULT_INSTALL_DIR:-/opt/$BRAND_SLUG}"
TARGET_ARCH="${TARGET_ARCH:-amd64}"
IMAGE_PREFIX="${IMAGE_PREFIX:-devloom.local}"
OUTPUT_DIR="${OUTPUT_DIR:-deploy/out}"
PROJECT_TEMPLATE_DIR="${PROJECT_TEMPLATE_DIR:-deploy/package/project-template}"
[[ "$TARGET_ARCH" == amd64 ]] || die "the current builder supports TARGET_ARCH=amd64"
[[ "$BRAND_SLUG" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die "BRAND_SLUG must contain lowercase letters, numbers, underscore, or dash"

COMMIT="${SOURCE_COMMIT:-$(git -C "$REPO_ROOT" rev-parse HEAD)}"
[[ "$COMMIT" =~ ^[0-9a-fA-F]{40}$ ]] || die "SOURCE_COMMIT must be a full 40-character Git commit"
[[ -n "$VERSION" ]] || VERSION="$(git -C "$REPO_ROOT" describe --tags --always --dirty | tr '/ ' '--')"
[[ "$VERSION" =~ ^[A-Za-z0-9._-]+$ ]] || die "invalid package version: $VERSION"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

FRONTEND_IMAGE="${IMAGE_PREFIX}/${BRAND_SLUG}-frontend:${VERSION}"
BACKEND_IMAGE="${IMAGE_PREFIX}/${BRAND_SLUG}-backend:${VERSION}"
INGRESS_IMAGE="${IMAGE_PREFIX}/${BRAND_SLUG}-ingress:${VERSION}"
TASKFLOW_IMAGE="${IMAGE_PREFIX}/${BRAND_SLUG}-taskflow:${VERSION}"
PREVIEW_IMAGE="${IMAGE_PREFIX}/${BRAND_SLUG}-preview:${VERSION}"
DEVBOX_IMAGE="${IMAGE_PREFIX}/${BRAND_SLUG}-devbox:${VERSION}"
ORCHESTRATOR_IMAGE="${IMAGE_PREFIX}/${BRAND_SLUG}-orchestrator:${VERSION}"
RUNTIME_VARS=(POSTGRES_IMAGE REDIS_IMAGE CLICKHOUSE_IMAGE RUSTFS_IMAGE)
for name in "${RUNTIME_VARS[@]}"; do
  value="${!name:-}"
  [[ -n "$value" ]] || die "$name is empty in $CONFIG_FILE"
  lower="${value,,}"
  [[ "$lower" != *change_me* && "$lower" != *pinned_version* && "$lower" != *:latest ]] || die "$name must use a pinned, non-placeholder image"
  docker image inspect "$value" >/dev/null 2>&1 || die "$name is not present in the local Docker daemon: $value"
  image_arch="$(docker image inspect --format '{{.Architecture}}' "$value")"
  [[ "$image_arch" == amd64 ]] || die "$name has architecture $image_arch, expected amd64"
done

if [[ "$OUTPUT_DIR" != /* ]]; then OUTPUT_DIR="$REPO_ROOT/$OUTPUT_DIR"; fi
if [[ "$PROJECT_TEMPLATE_DIR" != /* ]]; then PROJECT_TEMPLATE_DIR="$REPO_ROOT/$PROJECT_TEMPLATE_DIR"; fi
PACKAGE_NAME="${BRAND_SLUG}-offline-linux-${TARGET_ARCH}"
PACKAGE_ROOT="$OUTPUT_DIR/$PACKAGE_NAME"
[[ "$PACKAGE_ROOT" == "$OUTPUT_DIR/"* && "$PACKAGE_NAME" == *-offline-linux-* ]] || die "unsafe output path"
rm -rf -- "$PACKAGE_ROOT"
mkdir -p "$PACKAGE_ROOT"/{images,static/installer/x86_64,extensions/packages,tools}

log "building $BRAND_NAME frontend ($FRONTEND_IMAGE)"
docker buildx build --load --platform linux/amd64 --tag "$FRONTEND_IMAGE" \
  --file "$REPO_ROOT/frontend/docker/Dockerfile.source" "$REPO_ROOT/frontend"

log "building backend ($BACKEND_IMAGE)"
docker buildx build --load --platform linux/amd64 --tag "$BACKEND_IMAGE" \
  --file "$REPO_ROOT/backend/build/Dockerfile" "$REPO_ROOT/backend"

log "building ingress ($INGRESS_IMAGE)"
docker buildx build --load --platform linux/amd64 --tag "$INGRESS_IMAGE" \
  --file "$REPO_ROOT/backend/build/Dockerfile.ingress" "$REPO_ROOT/backend"

log "building source Taskflow ($TASKFLOW_IMAGE)"
docker buildx build --load --platform linux/amd64 --tag "$TASKFLOW_IMAGE" \
  --file "$REPO_ROOT/backend/build/Dockerfile.taskflow" "$REPO_ROOT/backend"

log "building source preview relay ($PREVIEW_IMAGE)"
docker buildx build --load --platform linux/amd64 --tag "$PREVIEW_IMAGE" \
  --file "$REPO_ROOT/backend/build/Dockerfile.preview" "$REPO_ROOT/backend"

log "building source development image ($DEVBOX_IMAGE)"
docker buildx build --load --platform linux/amd64 --tag "$DEVBOX_IMAGE" "$REPO_ROOT/devbox"

log "building source host orchestrator ($ORCHESTRATOR_IMAGE)"
docker buildx build --load --platform linux/amd64 --tag "$ORCHESTRATOR_IMAGE" \
  --file "$REPO_ROOT/backend/build/Dockerfile.orchestrator" "$REPO_ROOT/backend"

save_image() {
  local name=$1 image=$2 output=$3
  log "exporting $name ($image)"
  docker save "$image" | gzip -n > "$output"
}

save_image frontend "$FRONTEND_IMAGE" "$PACKAGE_ROOT/images/frontend.tar.gz"
save_image backend "$BACKEND_IMAGE" "$PACKAGE_ROOT/images/backend.tar.gz"
save_image ingress "$INGRESS_IMAGE" "$PACKAGE_ROOT/images/ingress.tar.gz"
save_image taskflow "$TASKFLOW_IMAGE" "$PACKAGE_ROOT/images/taskflow.tar.gz"
save_image preview "$PREVIEW_IMAGE" "$PACKAGE_ROOT/images/preview.tar.gz"
save_image postgres "$POSTGRES_IMAGE" "$PACKAGE_ROOT/images/postgres.tar.gz"
save_image redis "$REDIS_IMAGE" "$PACKAGE_ROOT/images/redis.tar.gz"
save_image clickhouse "$CLICKHOUSE_IMAGE" "$PACKAGE_ROOT/images/clickhouse.tar.gz"
save_image rustfs "$RUSTFS_IMAGE" "$PACKAGE_ROOT/images/rustfs.tar.gz"

RUNNER_ROOT="$(mktemp -d)"
trap 'rm -rf -- "$RUNNER_ROOT"' EXIT
mkdir -p "$RUNNER_ROOT/images"
cp "$SCRIPT_DIR/runtime/runner-compose.yml" "$RUNNER_ROOT/docker-compose.yml"
cp "$SCRIPT_DIR/runtime/runner.env" "$RUNNER_ROOT/.env"
python3 "$SCRIPT_DIR/manifest_tool.py" render-env "$RUNNER_ROOT/.env" "$RUNNER_ROOT/.env.tmp" \
  --set "ORCHESTRATOR_IMAGE=$ORCHESTRATOR_IMAGE" \
  --set "PREVIEW_IMAGE=$PREVIEW_IMAGE" \
  --set "DEVBOX_IMAGE=$DEVBOX_IMAGE" \
  --set "RUNNER_COMPOSE_PROJECT_NAME=${BRAND_SLUG}_runner" \
  --set "RUNNER_CONTAINER_PREFIX=$BRAND_SLUG"
mv "$RUNNER_ROOT/.env.tmp" "$RUNNER_ROOT/.env"
save_image orchestrator "$ORCHESTRATOR_IMAGE" "$RUNNER_ROOT/images/orchestrator.tar.gz"
save_image preview "$PREVIEW_IMAGE" "$RUNNER_ROOT/images/preview.tar.gz"
save_image devbox "$DEVBOX_IMAGE" "$RUNNER_ROOT/images/devbox.tar.gz"
tar -C "$RUNNER_ROOT" -cf - . | gzip -n > "$PACKAGE_ROOT/static/installer/x86_64/host.tgz"

if [[ -n "${DOCKER_BUNDLE_FILE:-}" ]]; then
  if [[ "$DOCKER_BUNDLE_FILE" != /* ]]; then DOCKER_BUNDLE_FILE="$REPO_ROOT/$DOCKER_BUNDLE_FILE"; fi
  [[ -f "$DOCKER_BUNDLE_FILE" ]] || die "DOCKER_BUNDLE_FILE not found: $DOCKER_BUNDLE_FILE"
  cp "$DOCKER_BUNDLE_FILE" "$PACKAGE_ROOT/docker.tgz"
else
  [[ "${DOCKER_TGZ_SHA256:-}" =~ ^[a-fA-F0-9]{64}$ ]] || die "DOCKER_TGZ_SHA256 must be a 64-character checksum"
  [[ "${COMPOSE_BINARY_SHA256:-}" =~ ^[a-fA-F0-9]{64}$ ]] || die "COMPOSE_BINARY_SHA256 must be a 64-character checksum"
  "$SCRIPT_DIR/fetch-docker-bundle.sh" "$PACKAGE_ROOT/docker.tgz" amd64 \
    "${DOCKER_VERSION:?}" "${COMPOSE_VERSION:?}" "$DOCKER_TGZ_SHA256" "$COMPOSE_BINARY_SHA256"
fi
cp "$PACKAGE_ROOT/docker.tgz" "$PACKAGE_ROOT/static/installer/x86_64/docker.tgz"
cp "$SCRIPT_DIR/runtime/host-installer.sh" "$PACKAGE_ROOT/static/installer/x86_64/installer"
chmod +x "$PACKAGE_ROOT/static/installer/x86_64/installer"
sha256sum "$PACKAGE_ROOT/static/installer/x86_64/installer" | awk '{print $1}' > "$PACKAGE_ROOT/static/installer/x86_64/installer.sha256"
sha256sum "$PACKAGE_ROOT/static/installer/x86_64/host.tgz" | awk '{print $1}' > "$PACKAGE_ROOT/static/installer/x86_64/host.tgz.sha256"
sha256sum "$PACKAGE_ROOT/static/installer/x86_64/docker.tgz" | awk '{print $1}' > "$PACKAGE_ROOT/static/installer/x86_64/docker.tgz.sha256"

[[ -d "$PROJECT_TEMPLATE_DIR" ]] || die "PROJECT_TEMPLATE_DIR not found: $PROJECT_TEMPLATE_DIR"
python3 "$SCRIPT_DIR/manifest_tool.py" zip-dir "$PROJECT_TEMPLATE_DIR" "$PACKAGE_ROOT/static/project-tpl.zip"
if [[ -n "${EXTENSION_PACKAGES_DIR:-}" ]]; then
  if [[ "$EXTENSION_PACKAGES_DIR" != /* ]]; then EXTENSION_PACKAGES_DIR="$REPO_ROOT/$EXTENSION_PACKAGES_DIR"; fi
  [[ -d "$EXTENSION_PACKAGES_DIR" ]] || die "EXTENSION_PACKAGES_DIR not found: $EXTENSION_PACKAGES_DIR"
  cp -a "$EXTENSION_PACKAGES_DIR/." "$PACKAGE_ROOT/extensions/packages/"
fi

cp "$REPO_ROOT/backend/docker-compose.yml" "$PACKAGE_ROOT/docker-compose.yml"
cp "$SCRIPT_DIR/runtime/install.sh" "$PACKAGE_ROOT/install.sh"
cp "$REPO_ROOT/deploy/offline/preflight.sh" "$PACKAGE_ROOT/preflight.sh"
cp "$REPO_ROOT/deploy/offline/verify.sh" "$PACKAGE_ROOT/verify.sh"
cp "$REPO_ROOT/LICENSE" "$PACKAGE_ROOT/LICENSE"
chmod +x "$PACKAGE_ROOT/install.sh" "$PACKAGE_ROOT/preflight.sh" "$PACKAGE_ROOT/verify.sh"

python3 "$SCRIPT_DIR/manifest_tool.py" render-env "$REPO_ROOT/deploy/offline/.env.example" "$PACKAGE_ROOT/.env.example" \
  --set "INSTALL_DIR=$DEFAULT_INSTALL_DIR" \
  --set "COMPOSE_PROJECT_NAME=$BRAND_SLUG" \
  --set "CONTAINER_PREFIX=$BRAND_SLUG" \
  --set "DEVLOOM_VERSION=$VERSION" \
  --set "REMOTE_IP=CHANGE_ME_HOST" \
  --set "POSTGRES_IMAGE=$POSTGRES_IMAGE" \
  --set "REDIS_IMAGE=$REDIS_IMAGE" \
  --set "CLICKHOUSE_IMAGE=$CLICKHOUSE_IMAGE" \
  --set "RUSTFS_IMAGE=$RUSTFS_IMAGE" \
  --set "FRONTEND_IMAGE=$FRONTEND_IMAGE" \
  --set "BACKEND_IMAGE=$BACKEND_IMAGE" \
  --set "INGRESS_IMAGE=$INGRESS_IMAGE" \
  --set "TASKFLOW_IMAGE=$TASKFLOW_IMAGE" \
  --set "PREVIEW_IMAGE=$PREVIEW_IMAGE" \
  --set "DEVBOX_IMAGE=$DEVBOX_IMAGE" \
  --set "TEAM_EMAIL=CHANGE_ME_ADMIN_EMAIL" \
  --set "TEAM_NAME=$BRAND_NAME" \
  --set "INIT_TEAM_IMAGE=$DEVBOX_IMAGE" \
  --set "PUBLIC_BASE_URL=http://CHANGE_ME_HOST"

python3 "$SCRIPT_DIR/manifest_tool.py" build "$PACKAGE_ROOT" \
  --brand "$BRAND_NAME" --version "$VERSION" --commit "$COMMIT" --arch "$TARGET_ARCH" --built-at "$BUILD_TIME"
python3 "$SCRIPT_DIR/manifest_tool.py" verify "$PACKAGE_ROOT"

ARCHIVE="$OUTPUT_DIR/$PACKAGE_NAME.tgz"
rm -f -- "$ARCHIVE" "$ARCHIVE.sha256"
tar -C "$OUTPUT_DIR" -cf - "$PACKAGE_NAME" | gzip -n > "$ARCHIVE"
(
  cd "$OUTPUT_DIR"
  sha256sum "$PACKAGE_NAME.tgz" > "$PACKAGE_NAME.tgz.sha256"
)
log "created $ARCHIVE"
log "checksum: $(awk '{print $1}' "$ARCHIVE.sha256")"
