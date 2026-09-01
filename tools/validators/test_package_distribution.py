#!/usr/bin/env python3
"""Regression tests for the closed local distribution candidate builder."""

from __future__ import annotations

import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "package_distribution.py"
SPEC = importlib.util.spec_from_file_location("package_distribution", SCRIPT)
assert SPEC and SPEC.loader
package_distribution = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(package_distribution)


class DistributionPackageTests(unittest.TestCase):
    VERSION = "0.2.0"
    BUILD_PROVENANCE = {
        "source_revision": "a" * 40,
        "source_clean": True,
        "go_version": "go1.27.0",
        "trimpath": True,
        "cgo_enabled": False,
        "binary_file_count": 6,
        "binary_build_record_count": 8,
    }

    def inputs(self, root: Path, starter_version: str | None = None) -> Path:
        plugin = root / "plugin"
        plugin.mkdir()
        (plugin / "BUNDLE-MANIFEST.json").write_text(
            json.dumps(
                {
                    "plugin": {"name": "codex-game-atelier", "version": self.VERSION},
                    "starter_template": {
                        "name": "codex-game-atelier-starter",
                        "version": starter_version or self.VERSION,
                        "path": "starter-template",
                        "distribution": "embedded-in-plugin",
                    },
                    "build_provenance": self.BUILD_PROVENANCE,
                }
            ),
            encoding="utf-8",
        )
        return plugin

    @staticmethod
    def fake_archive(source: Path, output: Path) -> None:
        member_name = "codex-game-atelier/BUNDLE-MANIFEST.json"
        payload = (source / "BUNDLE-MANIFEST.json").read_bytes()
        with tarfile.open(output, mode="w:gz") as archive:
            info = tarfile.TarInfo(member_name)
            info.size = len(payload)
            info.mode = 0o644
            archive.addfile(info, io.BytesIO(payload))
        output.chmod(0o644)
        checksum = output.with_name(output.name + ".sha256")
        checksum.write_text(
            f"{package_distribution.sha256_file(output)}  {output.name}\n",
            encoding="ascii",
        )
        checksum.chmod(0o644)

    def patches(self):
        return (
            mock.patch.object(package_distribution.package_plugin, "verify_bundle"),
            mock.patch.object(package_distribution.package_plugin, "create_archive", side_effect=self.fake_archive),
            mock.patch.object(package_distribution.package_plugin, "verify_archive"),
        )

    def build(self, root: Path) -> Path:
        plugin = self.inputs(root)
        output = root / "candidate"
        contexts = self.patches()
        with contexts[0], contexts[1], contexts[2]:
            package_distribution.build_candidate(output, plugin)
        return output

    def test_build_closes_plugin_cli_runner_and_starter_versions(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-distribution-") as temporary:
            output = self.build(Path(temporary))
            manifest = package_distribution.load_manifest(output)
            self.assertEqual(manifest["release"]["version"], self.VERSION)
            self.assertEqual(manifest["components"]["plugin"]["cli_version"], self.VERSION)
            self.assertEqual(manifest["components"]["plugin"]["private_runner_version"], self.VERSION)
            self.assertEqual(
                manifest["components"]["starter_template"]["verified_plugin_version"],
                self.VERSION,
            )
            self.assertEqual(manifest["components"]["starter_template"]["distribution"], "embedded-in-plugin")
            self.assertEqual(manifest["policies"]["distribution_channel"], "codex-plugin-only")
            self.assertFalse(manifest["policies"]["apple_notarization_required"])
            self.assertFalse(manifest["release"]["external_publication_performed"])
            self.assertEqual(manifest["build_provenance"], self.BUILD_PROVENANCE)
            self.assertFalse(manifest["policies"]["source_build_required"])
            self.assertFalse(manifest["policies"]["telemetry_enabled"])

    def test_verify_rejects_tampering_and_unknown_files(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-distribution-") as temporary:
            output = self.build(Path(temporary))
            (output / "NOTICE").write_text("tampered\n", encoding="utf-8")
            with self.assertRaises(package_distribution.DistributionError):
                package_distribution.verify_candidate(output)

            (output / "NOTICE").write_bytes((package_distribution.ROOT / "NOTICE").read_bytes())
            (output / "unexpected.txt").write_text("unexpected\n", encoding="utf-8")
            with self.assertRaises(package_distribution.DistributionError):
                package_distribution.verify_candidate(output)

    def test_verify_rejects_provenance_that_differs_from_plugin(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-distribution-") as temporary:
            output = self.build(Path(temporary))
            manifest = package_distribution.load_manifest(output)
            manifest["build_provenance"]["source_revision"] = "b" * 40
            (output / package_distribution.MANIFEST_NAME).write_text(
                json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
                encoding="utf-8",
            )
            with self.assertRaises(package_distribution.DistributionError):
                package_distribution.verify_candidate(output)

    def test_build_rejects_component_version_mismatch_without_output(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-distribution-") as temporary:
            root = Path(temporary)
            plugin = self.inputs(root, starter_version="0.2.1")
            output = root / "candidate"
            with mock.patch.object(package_distribution.package_plugin, "verify_bundle"):
                with self.assertRaises(package_distribution.DistributionError):
                    package_distribution.build_candidate(output, plugin)
            self.assertFalse(output.exists())

    def test_build_never_overwrites_existing_output(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-distribution-") as temporary:
            root = Path(temporary)
            plugin = self.inputs(root)
            output = root / "candidate"
            output.mkdir()
            marker = output / "user.txt"
            marker.write_text("preserve\n", encoding="utf-8")
            with self.assertRaises(package_distribution.DistributionError):
                package_distribution.build_candidate(output, plugin)
            self.assertEqual(marker.read_text(encoding="utf-8"), "preserve\n")


if __name__ == "__main__":
    unittest.main()
