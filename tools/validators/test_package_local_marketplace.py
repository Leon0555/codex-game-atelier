#!/usr/bin/env python3
"""Regression tests for the local Codex marketplace packager."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "package_local_marketplace.py"
SPEC = importlib.util.spec_from_file_location("package_local_marketplace", SCRIPT)
assert SPEC and SPEC.loader
package_local_marketplace = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(package_local_marketplace)


class LocalMarketplacePackageTests(unittest.TestCase):
    def bundle(self, root: Path) -> Path:
        bundle = root / "bundle"
        manifest = bundle / ".codex-plugin" / "plugin.json"
        manifest.parent.mkdir(parents=True)
        manifest.write_text(
            json.dumps({"name": "codex-game-atelier", "version": "0.2.0"}),
            encoding="utf-8",
        )
        manifest.chmod(0o644)
        return bundle

    def test_build_uses_canonical_marketplace_entry_and_copies_bundle(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-marketplace-") as temporary:
            root = Path(temporary)
            bundle = self.bundle(root)
            output = root / "marketplace"
            with mock.patch.object(package_local_marketplace.package_plugin, "verify_bundle"):
                package_local_marketplace.build_marketplace(output, bundle)
            document = json.loads((output / ".agents/plugins/marketplace.json").read_text(encoding="utf-8"))
            self.assertEqual(document, package_local_marketplace.marketplace_document(remote=False))
            copied = output / "plugins/codex-game-atelier/.codex-plugin/plugin.json"
            self.assertEqual(copied.read_bytes(), (bundle / ".codex-plugin/plugin.json").read_bytes())

    def test_remote_mode_uses_release_identity_and_is_not_local_compatible(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-marketplace-") as temporary:
            root = Path(temporary)
            bundle = self.bundle(root)
            output = root / "marketplace"
            with mock.patch.object(package_local_marketplace.package_plugin, "verify_bundle"):
                package_local_marketplace.build_marketplace(output, bundle, remote=True)
                document = json.loads((output / ".agents/plugins/marketplace.json").read_text(encoding="utf-8"))
                self.assertEqual(document, package_local_marketplace.marketplace_document(remote=True))
                self.assertEqual(document["name"], "codex-game-atelier")
                self.assertEqual(document["interface"]["displayName"], "Codex Game Atelier")
                with self.assertRaises(package_local_marketplace.MarketplaceError):
                    package_local_marketplace.verify_marketplace(output, remote=False)

                local_output = root / "local-marketplace"
                package_local_marketplace.build_marketplace(local_output, bundle, remote=False)
                with self.assertRaises(package_local_marketplace.MarketplaceError):
                    package_local_marketplace.verify_marketplace(local_output, remote=True)

    def test_cli_routes_remote_mode_to_build_and_verify(self) -> None:
        with mock.patch.object(
            package_local_marketplace.sys,
            "argv",
            ["package_local_marketplace.py", "build", "--output", "out", "--plugin-bundle", "bundle", "--remote"],
        ), mock.patch.object(package_local_marketplace, "build_marketplace") as build:
            self.assertEqual(package_local_marketplace.main(), 0)
            build.assert_called_once_with(Path("out"), Path("bundle"), remote=True)

        with mock.patch.object(
            package_local_marketplace.sys,
            "argv",
            ["package_local_marketplace.py", "verify", "marketplace", "--remote"],
        ), mock.patch.object(package_local_marketplace, "verify_marketplace") as verify:
            self.assertEqual(package_local_marketplace.main(), 0)
            verify.assert_called_once_with(Path("marketplace"), remote=True)

    def test_existing_output_is_never_overwritten(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-marketplace-") as temporary:
            root = Path(temporary)
            output = root / "marketplace"
            output.mkdir()
            marker = output / "user.txt"
            marker.write_text("preserve\n", encoding="utf-8")
            with self.assertRaises(package_local_marketplace.MarketplaceError):
                package_local_marketplace.build_marketplace(output, self.bundle(root))
            self.assertEqual(marker.read_text(encoding="utf-8"), "preserve\n")

    def test_copy_rejects_symlinks_even_after_input_verification(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-marketplace-") as temporary:
            root = Path(temporary)
            bundle = self.bundle(root)
            (bundle / "unsafe").symlink_to(bundle / ".codex-plugin/plugin.json")
            output = root / "marketplace"
            with mock.patch.object(package_local_marketplace.package_plugin, "verify_bundle"):
                with self.assertRaises(package_local_marketplace.MarketplaceError):
                    package_local_marketplace.build_marketplace(output, bundle)
            self.assertFalse(output.exists())

    def test_verify_rejects_unknown_root_content(self) -> None:
        with tempfile.TemporaryDirectory(prefix="atelier-marketplace-") as temporary:
            root = Path(temporary)
            bundle = self.bundle(root)
            output = root / "marketplace"
            with mock.patch.object(package_local_marketplace.package_plugin, "verify_bundle"):
                package_local_marketplace.build_marketplace(output, bundle)
                (output / "unexpected.txt").write_text("unexpected\n", encoding="utf-8")
                with self.assertRaises(package_local_marketplace.MarketplaceError):
                    package_local_marketplace.verify_marketplace(output)


if __name__ == "__main__":
    unittest.main()
