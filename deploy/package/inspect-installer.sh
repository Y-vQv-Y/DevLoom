#!/usr/bin/env bash
set -Eeuo pipefail

FILE=${1:-}
[[ -n "$FILE" && -f "$FILE" ]] || { printf 'Usage: inspect-installer.sh FILE\n' >&2; exit 2; }

printf '== File ==\n'
file "$FILE"
stat "$FILE"
sha256sum "$FILE"

if command -v go >/dev/null 2>&1; then
  printf '\n== Go build metadata ==\n'
  go version -m "$FILE" || true
fi

if command -v readelf >/dev/null 2>&1; then
  printf '\n== ELF header ==\n'
  readelf -h "$FILE" || true
fi

printf '\n== Installer-specific symbols and strings ==\n'
if command -v strings >/dev/null 2>&1; then
  strings -a "$FILE" | grep -E \
    'MonkeyCodePro|pkg/installer/(app|deploy|steps)|InstallCenter|UpgradeCenter|RollbackCenter|InstallHost|InstallDocker|GenerateSelfSignedTLS|vcs\.(revision|time)|go1\.[0-9]+' \
    | sort -u || true
else
  printf 'strings is unavailable; install binutils for symbol-string extraction.\n'
fi
