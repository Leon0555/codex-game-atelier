#!/usr/bin/env python3
"""Regression tests for the deterministic paired Starter Template package."""

from __future__ import annotations

import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "package_starter_template.py"
SPEC = importlib.util.spec_from_file_location("package_starter_template", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
packager = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(packager)
EVIDENCE = packager.ROOT / "docs" / "validation" / "evidence" / "phase1-starter-template-package-2026-08-26"


class StarterTemplatePackageTests(unittest.TestCase):
    def build(self, root: Path) -> Path:
        package = root / "package"
        packager.build_package(package)
        return package

    def test_package_is_paired_without_embedding_plugin_content(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            package = self.build(Path(temporary))
            packager.verify_package(package)
            manifest = json.loads((package / packager.PACKAGE_MANIFEST).read_text(encoding="utf-8"))
            self.assertEqual(manifest["pairing"]["name"], "codex-game-atelier")
            self.assertFalse(manifest["pairing"]["embedded"])
            self.assertEqual(manifest["template"]["version"], manifest["pairing"]["verified_plugin_version"])
            self.assertFalse((package / "bin").exists())
            self.assertFalse((package / "skills").exists())
            self.assertFalse(any(path.name == "AGENTS.md" for path in package.rglob("*")))

    def test_archive_is_reproducible_and_checksum_tamper_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            package = self.build(root)
            first = root / "first.tar.gz"
            second = root / "second.tar.gz"
            packager.create_archive(package, first)
            packager.create_archive(package, second)
            self.assertEqual(first.read_bytes(), second.read_bytes())
            packager.verify_archive(first)
            first.write_bytes(first.read_bytes() + b"tampered")
            with self.assertRaises(packager.TemplatePackageError):
                packager.verify_archive(first)

    def test_unknown_content_mode_and_manifest_tamper_are_rejected(self) -> None:
        for mutation in ("unknown", "mode", "pairing", "model-with-rebuilt-manifest"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                package = self.build(Path(temporary))
                if mutation == "unknown":
                    (package / "unexpected.txt").write_text("unexpected\n", encoding="utf-8")
                elif mutation == "mode":
                    (package / "README.md").chmod(0o755)
                elif mutation == "pairing":
                    manifest = package / packager.PACKAGE_MANIFEST
                    value = json.loads(manifest.read_text(encoding="utf-8"))
                    value["pairing"]["embedded"] = True
                    manifest.write_text(json.dumps(value) + "\n", encoding="utf-8")
                else:
                    readme = package / "README.md"
                    readme.write_text(readme.read_text(encoding="utf-8") + "\nUse gpt-4.1.\n", encoding="utf-8")
                    packager.write_package_manifest(package, packager.read_plugin_version())
                with self.assertRaises(packager.TemplatePackageError):
                    packager.verify_package(package)

    def test_source_links_and_existing_output_are_rejected(self) -> None:
        for mutation in ("symlink", "hardlink", "root-symlink", "existing"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                source = root / "source"
                shutil.copytree(packager.TEMPLATE_SOURCE, source)
                output = root / "output"
                if mutation == "symlink":
                    (source / "linked.gd").symlink_to("scripts/main.gd")
                elif mutation == "hardlink":
                    target = source / "scripts" / "game_state.gd"
                    target.unlink()
                    os.link(source / "scripts" / "main.gd", target)
                elif mutation == "root-symlink":
                    linked = root / "source-link"
                    linked.symlink_to(source, target_is_directory=True)
                    source = linked
                else:
                    output.mkdir()
                with self.assertRaises((packager.TemplatePackageError, packager.TemplateError)):
                    packager.build_package(output, source=source)

    def test_archive_traversal_and_link_members_are_rejected(self) -> None:
        for mutation in ("traversal", "symlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                archive = root / "unsafe.tar.gz"
                with tarfile.open(archive, "w:gz") as tar:
                    if mutation == "traversal":
                        member = tarfile.TarInfo("../escape")
                        member.size = 1
                        tar.addfile(member, io.BytesIO(b"x"))
                    else:
                        member = tarfile.TarInfo(f"{packager.ARCHIVE_ROOT}/linked")
                        member.type = tarfile.SYMTYPE
                        member.linkname = "README.md"
                        tar.addfile(member)
                digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                packager.archive_checksum_path(archive).write_text(f"{digest}  {archive.name}\n", encoding="ascii")
                with self.assertRaises(packager.TemplatePackageError):
                    packager.verify_archive(archive)

    def test_metadata_bombs_and_concatenated_gzip_are_rejected(self) -> None:
        for mutation in ("pax", "gnu-long-name", "concatenated"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                archive = root / "unsafe.tar.gz"
                if mutation == "concatenated":
                    package = self.build(root)
                    packager.create_archive(package, archive)
                    archive.write_bytes(archive.read_bytes() + gzip.compress(b"hidden"))
                else:
                    archive_format = tarfile.PAX_FORMAT if mutation == "pax" else tarfile.GNU_FORMAT
                    with tarfile.open(archive, "w:gz", format=archive_format) as tar:
                        member = tarfile.TarInfo(f"{packager.ARCHIVE_ROOT}/payload")
                        if mutation == "pax":
                            member.pax_headers = {"comment": "A" * (packager.MAX_TAR_BYTES + 1024)}
                        else:
                            member.name = "A" * (packager.MAX_TAR_BYTES + 1024)
                        member.size = 1
                        tar.addfile(member, io.BytesIO(b"x"))
                digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                packager.archive_checksum_path(archive).write_text(f"{digest}  {archive.name}\n", encoding="ascii")
                with self.assertRaises(packager.TemplatePackageError):
                    packager.verify_archive(archive)

    def test_verified_plugin_version_is_self_contained_and_existing_archive_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            package = self.build(root)
            plugin = root / "plugin.json"
            plugin.write_text('{"name":"codex-game-atelier","version":"0.2.1"}\n', encoding="utf-8")
            newer = root / "newer"
            packager.build_package(newer, plugin_manifest=plugin)
            packager.verify_package(package)
            packager.verify_package(newer)
            newer_manifest = json.loads((newer / packager.PACKAGE_MANIFEST).read_text(encoding="utf-8"))
            self.assertEqual(newer_manifest["pairing"]["verified_plugin_version"], "0.2.1")
            archive = root / "existing.tar.gz"
            archive.write_bytes(b"existing")
            with self.assertRaises(packager.TemplatePackageError):
                packager.create_archive(package, archive)

    def test_persisted_manifest_matches_the_current_package(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            package = self.build(Path(temporary))
            generated = json.loads((package / packager.PACKAGE_MANIFEST).read_text(encoding="utf-8"))
            persisted = json.loads((EVIDENCE / "template-manifest.json").read_text(encoding="utf-8"))
            artifact = json.loads((EVIDENCE / "artifact.json").read_text(encoding="utf-8"))
            self.assertEqual(generated, persisted)
            self.assertEqual(artifact["decision"], {
                "adr": "0014-starter-template-boundary",
                "option": "A",
                "status": "Accepted",
            })
            self.assertEqual(artifact["pairing"], {
                "plugin": "codex-game-atelier",
                "verified_plugin_version": generated["pairing"]["verified_plugin_version"],
                "embedded": False,
            })
            self.assertEqual(artifact["manifest"]["sha256"], hashlib.sha256(
                (EVIDENCE / "template-manifest.json").read_bytes()
            ).hexdigest())


if __name__ == "__main__":
    unittest.main()
