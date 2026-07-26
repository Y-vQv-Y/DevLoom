#!/usr/bin/env bash
set -Eeuo pipefail

# Bootstrap an independently built DevLoom offline bundle.

INSTALL_ROOT="${INSTALL_ROOT:-/opt/devloom}"
PACKAGE="${OFFLINE_PACKAGE:-}"
CHECKSUM="${OFFLINE_PACKAGE_SHA256:-}"
PASSTHROUGH=()

usage() {
  cat <<'EOF'
Usage: install.sh [options] [-- package-installer-args]

Options:
  --package FILE       use a local offline package
  --install-root DIR   installation root (default: /opt/devloom)
  --sha256 HEX         verify the package checksum
  -h, --help           show this help
EOF
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH_DIR=amd64 ;;
  *) die "unsupported CPU architecture: $ARCH (this package targets linux/amd64)" ;;
esac

while (($#)); do
  case "$1" in
    --package)
      (($# >= 2)) || die "--package requires a file"
      PACKAGE=$2
      shift 2
      ;;
    --install-root)
      (($# >= 2)) || die "--install-root requires a directory"
      INSTALL_ROOT=$2
      shift 2
      ;;
    --sha256)
      (($# >= 2)) || die "--sha256 requires a checksum"
      CHECKSUM=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      PASSTHROUGH=("$@")
      break
      ;;
    *)
      die "unknown option: $1 (use -- to pass arguments to the bundle installer)"
      ;;
  esac
done

[[ "$(uname -s)" == Linux ]] || die "the offline installer must run on Linux"
[[ "$(id -u)" == 0 ]] || die "run this installer as root"
need_cmd tar
need_cmd mktemp

INSTALL_ROOT="$(mkdir -p -- "$INSTALL_ROOT" && cd -- "$INSTALL_ROOT" && pwd)"
WORK_DIR="$(mktemp -d)"
cleanup() { rm -rf -- "$WORK_DIR"; }
trap cleanup EXIT

[[ -n "$PACKAGE" ]] || die "provide the independently built DevLoom TGZ with --package"
if [[ "$PACKAGE" == file://* ]]; then
  PACKAGE="${PACKAGE#file://}"
fi

[[ -f "$PACKAGE" ]] || die "offline package not found: $PACKAGE"
if [[ -n "$CHECKSUM" ]]; then
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s  %s\n' "$CHECKSUM" "$PACKAGE" | sha256sum --check --status - || die "offline package checksum mismatch"
  elif command -v shasum >/dev/null 2>&1; then
    ACTUAL="$(shasum -a 256 "$PACKAGE" | awk '{print $1}')"
    [[ "$ACTUAL" == "$CHECKSUM" ]] || die "offline package checksum mismatch"
  else
    die "sha256 verification requested but sha256sum/shasum is unavailable"
  fi
fi

tar -tzf "$PACKAGE" >/dev/null || die "invalid gzip tar package: $PACKAGE"
tar -xzf "$PACKAGE" -C "$WORK_DIR"
INSTALLER="$(find "$WORK_DIR" -mindepth 2 -maxdepth 2 -type f -path '*/devloom-offline-linux-amd64/install.sh' -print -quit)"
[[ -n "$INSTALLER" ]] || die "package does not contain devloom-offline-linux-amd64/install.sh"

chmod +x "$INSTALLER"
export INSTALL_DIR="$INSTALL_ROOT"
export DEVLOOM_INSTALL_ROOT="$INSTALL_ROOT"
export TARGET_ARCH="$ARCH_DIR"
export OFFLINE_INSTALL=1
export NONINTERACTIVE="${NONINTERACTIVE:-1}"

printf 'Running bundle installer from %s (arch=%s, root=%s)...\n' "$INSTALLER" "$ARCH_DIR" "$INSTALL_ROOT"
(
  cd -- "$(dirname -- "$INSTALLER")"
  bash "$INSTALLER" "${PASSTHROUGH[@]}"
)

printf '\nOffline bundle installation finished. Run:\n  %s/tools/verify.sh --root %s\n' "$INSTALL_ROOT" "$INSTALL_ROOT"
