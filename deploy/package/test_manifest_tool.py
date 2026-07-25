from __future__ import annotations

import argparse
from contextlib import redirect_stderr
import io
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

import manifest_tool


class ManifestToolTest(unittest.TestCase):
    def test_render_zip_build_and_verify(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            template = root / "template.env"
            manifest_tool.write_text_lf(template, "NAME=old\nKEEP=value\n")
            rendered = root / "rendered.env"
            manifest_tool.render_env(
                argparse.Namespace(
                    template=str(template),
                    output=str(rendered),
                    set=["NAME=DevLoom Internal", "ADDED=yes"],
                )
            )
            self.assertEqual(
                rendered.read_text(encoding="utf-8"),
                "NAME=DevLoom Internal\nKEEP=value\nADDED=yes\n",
            )

            source = root / "project"
            source.mkdir()
            manifest_tool.write_text_lf(source / "AGENTS.md", "workspace\n")
            archive = root / "project.zip"
            manifest_tool.zip_dir(argparse.Namespace(source=str(source), output=str(archive)))
            with zipfile.ZipFile(archive) as package:
                self.assertEqual(package.namelist(), ["AGENTS.md"])
                self.assertEqual(package.read("AGENTS.md"), b"workspace\n")

            build_root = root / "bundle"
            build_root.mkdir()
            (build_root / "payload").write_bytes(b"payload")
            args = argparse.Namespace(
                root=str(build_root),
                brand="DevLoom",
                version="test",
                commit="0123456",
                arch="amd64",
                built_at="2026-07-25T00:00:00Z",
            )
            manifest_tool.build_manifest(args)
            manifest_tool.verify_manifest(argparse.Namespace(root=str(build_root)))

            manifest = json.loads((build_root / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(manifest["brand"], "DevLoom")
            self.assertIn("manifest.json", (build_root / "SHA256SUMS").read_text(encoding="utf-8"))

            (build_root / "payload").write_bytes(b"tampered")
            with redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
                manifest_tool.verify_manifest(argparse.Namespace(root=str(build_root)))


if __name__ == "__main__":
    unittest.main()
