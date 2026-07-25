#!/usr/bin/env python3
"""Build and verify deterministic metadata for DevLoom offline bundles."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import sys
import zipfile


EXCLUDED = {"manifest.json", "SHA256SUMS"}


def write_text_lf(path: Path, content: str) -> None:
    with path.open("w", encoding="utf-8", newline="\n") as stream:
        stream.write(content)


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def package_files(root: Path) -> list[Path]:
    return sorted(
        path
        for path in root.rglob("*")
        if path.is_file() and path.relative_to(root).as_posix() not in EXCLUDED
    )


def build_manifest(args: argparse.Namespace) -> None:
    root = Path(args.root).resolve()
    entries = []
    sum_lines = []
    for path in package_files(root):
        relative = path.relative_to(root).as_posix()
        sha256 = file_sha256(path)
        entries.append({"path": relative, "size": path.stat().st_size, "sha256": sha256})
        sum_lines.append(f"{sha256}  {relative}")

    manifest = {
        "schema": 1,
        "brand": args.brand,
        "version": args.version,
        "commit": args.commit,
        "arch": args.arch,
        "built_at": args.built_at,
        "files": entries,
    }
    manifest_path = root / "manifest.json"
    write_text_lf(manifest_path, json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    sum_lines.append(f"{file_sha256(manifest_path)}  manifest.json")
    write_text_lf(root / "SHA256SUMS", "\n".join(sum_lines) + "\n")


def verify_manifest(args: argparse.Namespace) -> None:
    root = Path(args.root).resolve()
    manifest = json.loads((root / "manifest.json").read_text(encoding="utf-8"))
    errors = []
    expected_paths = set()
    for entry in manifest.get("files", []):
        relative = entry["path"]
        expected_paths.add(relative)
        path = root / relative
        if not path.is_file():
            errors.append(f"missing: {relative}")
            continue
        if path.stat().st_size != entry["size"]:
            errors.append(f"size mismatch: {relative}")
        actual = file_sha256(path)
        if actual != entry["sha256"]:
            errors.append(f"sha256 mismatch: {relative}")

    actual_paths = {path.relative_to(root).as_posix() for path in package_files(root)}
    for extra in sorted(actual_paths - expected_paths):
        errors.append(f"untracked package file: {extra}")
    if errors:
        print("\n".join(errors), file=sys.stderr)
        raise SystemExit(1)
    print(f"verified {len(expected_paths)} files in {root}")


def render_env(args: argparse.Namespace) -> None:
    values: dict[str, str] = {}
    for item in args.set:
        if "=" not in item:
            raise SystemExit(f"invalid --set value: {item}")
        key, value = item.split("=", 1)
        values[key] = value

    output = []
    found = set()
    for line in Path(args.template).read_text(encoding="utf-8").splitlines():
        if line and not line.startswith("#") and "=" in line:
            key = line.split("=", 1)[0]
            if key in values:
                line = f"{key}={values[key]}"
                found.add(key)
        output.append(line)
    for key in values:
        if key not in found:
            output.append(f"{key}={values[key]}")
    write_text_lf(Path(args.output), "\n".join(output) + "\n")


def zip_dir(args: argparse.Namespace) -> None:
    source = Path(args.source).resolve()
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in sorted(source.rglob("*")):
            if path.is_file():
                relative = path.relative_to(source).as_posix()
                info = zipfile.ZipInfo(relative, (2020, 1, 1, 0, 0, 0))
                info.compress_type = zipfile.ZIP_DEFLATED
                info.external_attr = (path.stat().st_mode & 0xFFFF) << 16
                archive.writestr(info, path.read_bytes())


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="command", required=True)

    build = commands.add_parser("build")
    build.add_argument("root")
    build.add_argument("--brand", required=True)
    build.add_argument("--version", required=True)
    build.add_argument("--commit", required=True)
    build.add_argument("--arch", required=True)
    build.add_argument("--built-at", required=True)
    build.set_defaults(func=build_manifest)

    verify = commands.add_parser("verify")
    verify.add_argument("root")
    verify.set_defaults(func=verify_manifest)

    render = commands.add_parser("render-env")
    render.add_argument("template")
    render.add_argument("output")
    render.add_argument("--set", action="append", default=[])
    render.set_defaults(func=render_env)

    zipper = commands.add_parser("zip-dir")
    zipper.add_argument("source")
    zipper.add_argument("output")
    zipper.set_defaults(func=zip_dir)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
