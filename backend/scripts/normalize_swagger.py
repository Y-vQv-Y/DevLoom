#!/usr/bin/env python3
"""Normalize Swagger extensions that vary between generator environments."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


INTEGER_FORMATS = {
    "types.ConditionStatus": "int32",
}


def _insert_before(
    mapping: dict[str, Any], key: str, value: Any, before: str
) -> dict[str, Any]:
    normalized: dict[str, Any] = {}
    inserted = False
    for current_key, current_value in mapping.items():
        if current_key == key:
            continue
        if current_key == before:
            normalized[key] = value
            inserted = True
        normalized[current_key] = current_value
    if not inserted:
        normalized[key] = value
    return normalized


def normalize(document: dict[str, Any]) -> dict[str, Any]:
    definitions = document.get("definitions")
    if not isinstance(definitions, dict):
        raise ValueError("Swagger document does not contain an object definitions field")

    for name, definition in list(definitions.items()):
        if not isinstance(definition, dict):
            continue
        comments = definition.get("x-enum-comments")
        varnames = definition.get("x-enum-varnames")
        if comments is not None:
            if not isinstance(comments, dict) or not isinstance(varnames, list):
                raise ValueError(f"{name} has invalid enum extension metadata")
            missing = [varname for varname in varnames if varname not in comments]
            if missing:
                raise ValueError(f"{name} has no comments for: {', '.join(missing)}")
            descriptions = [comments[varname] for varname in varnames]
            definitions[name] = _insert_before(
                definition,
                "x-enum-descriptions",
                descriptions,
                "x-enum-varnames",
            )

    for name, integer_format in INTEGER_FORMATS.items():
        definition = definitions.get(name)
        if not isinstance(definition, dict):
            raise ValueError(f"Swagger definition is missing: {name}")
        if definition.get("type") != "integer":
            raise ValueError(f"{name} is not an integer definition")
        definitions[name] = _insert_before(definition, "format", integer_format, "enum")

    return document


def encode(document: dict[str, Any]) -> str:
    rendered = json.dumps(document, ensure_ascii=False, indent=4)
    # Go's encoding/json escapes HTML-sensitive characters by default.
    rendered = (
        rendered.replace("&", "\\u0026")
        .replace("<", "\\u003c")
        .replace(">", "\\u003e")
        .replace("\u2028", "\\u2028")
        .replace("\u2029", "\\u2029")
    )
    return rendered


def normalize_file(path: Path) -> bool:
    original = path.read_text(encoding="utf-8")
    document = json.loads(original)
    normalized = encode(normalize(document))
    if normalized == original:
        return False
    path.write_text(normalized, encoding="utf-8", newline="\n")
    return True


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("swagger", type=Path)
    args = parser.parse_args()
    normalize_file(args.swagger)


if __name__ == "__main__":
    main()
