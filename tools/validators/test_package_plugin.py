#!/usr/bin/env python3
"""Regression tests for the deterministic plugin bundle builder."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import io
import os
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "package_plugin.py"
SPEC = importlib.util.spec_from_file_location("package_plugin", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
package_plugin = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(package_plugin)


class PluginBundleTests(unittest.TestCase):
    def sources(self, root: Path) -> dict[str, Path]:
        root.mkdir(parents=True, exist_ok=True)
        result: dict[str, Path] = {}
        for name in (
            "darwin-cli",
            "darwin-runner",
            "linux-cli",
            "linux-runner",
            "windows-cli",
            "windows-runner",
        ):
            path = root / name
            if name.startswith("darwin"):
                first_offset = 4096
                second_offset = 8192
                header = b"\xca\xfe\xba\xbe" + (2).to_bytes(4, "big")
                slice_size = 64
                header += (0x01000007).to_bytes(4, "big") + (3).to_bytes(4, "big") + first_offset.to_bytes(4, "big") + slice_size.to_bytes(4, "big") + (12).to_bytes(4, "big")
                header += (0x0100000C).to_bytes(4, "big") + (0).to_bytes(4, "big") + second_offset.to_bytes(4, "big") + slice_size.to_bytes(4, "big") + (12).to_bytes(4, "big")
                def thin(cpu: int, fill: bytes) -> bytes:
                    value = b"\xcf\xfa\xed\xfe" + cpu.to_bytes(4, "little") + (0).to_bytes(4, "little")
                    value += (2).to_bytes(4, "little") + (1).to_bytes(4, "little") + (8).to_bytes(4, "little")
                    value += (0).to_bytes(8, "little") + (1).to_bytes(4, "little") + (8).to_bytes(4, "little")
                    return value.ljust(slice_size, fill)
                content = header.ljust(first_offset, b"\x00") + thin(0x01000007, b"x")
                content = content.ljust(second_offset, b"\x00") + thin(0x0100000C, b"a")
            elif name.startswith("linux"):
                content = bytearray(120)
                content[:7] = b"\x7fELF\x02\x01\x01"
                content[16:18] = (2).to_bytes(2, "little")
                content[18:20] = (62).to_bytes(2, "little")
                content[20:24] = (1).to_bytes(4, "little")
                content[32:40] = (64).to_bytes(8, "little")
                content[52:54] = (64).to_bytes(2, "little")
                content[54:56] = (56).to_bytes(2, "little")
                content[56:58] = (1).to_bytes(2, "little")
                content = bytes(content)
            else:
                content = bytearray(512)
                content[:2] = b"MZ"
                content[60:64] = (128).to_bytes(4, "little")
                content[128:132] = b"PE\x00\x00"
                content[132:134] = (0x8664).to_bytes(2, "little")
                content[134:136] = (1).to_bytes(2, "little")
                content[148:150] = (240).to_bytes(2, "little")
                content[150:152] = (2).to_bytes(2, "little")
                content[152:154] = (0x20B).to_bytes(2, "little")
                content = bytes(content)
            path.write_bytes(content)
            path.chmod(0o755)
            result[name] = path
        return result

    def arguments(self, output: Path, sources: dict[str, Path]) -> argparse.Namespace:
        return argparse.Namespace(
            output=output,
            darwin_cli=sources["darwin-cli"],
            darwin_runner=sources["darwin-runner"],
            linux_cli=sources["linux-cli"],
            linux_runner=sources["linux-runner"],
            windows_cli=sources["windows-cli"],
            windows_runner=sources["windows-runner"],
        )

    def test_build_is_reproducible_and_contains_no_internal_agents(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root)
            first = root / "first"
            second = root / "second"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(first, sources))
                package_plugin.build_bundle(self.arguments(second, sources))
            package_plugin.verify_bundle(first)
            package_plugin.verify_bundle(second)
            self.assertEqual(package_plugin.bundle_files(first), package_plugin.bundle_files(second))
            self.assertFalse(any(path.name == "AGENTS.md" for path in first.rglob("*")))

    def test_tamper_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root)
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, sources))
            target = bundle / "bin" / "linux-amd64" / "codex-game-atelier"
            target.write_bytes(b"tampered\n")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.verify_bundle(bundle)

    def test_symlink_binary_and_existing_output_are_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root)
            link = root / "linked-cli"
            link.symlink_to(sources["darwin-cli"])
            sources["darwin-cli"] = link
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.build_bundle(self.arguments(root / "bundle", sources))
            existing = root / "existing"
            existing.mkdir()
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.build_bundle(self.arguments(existing, self.sources(root / "more")))

    def test_text_disguised_as_binary_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root)
            sources["linux-cli"].write_text("not an ELF\n", encoding="utf-8")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.build_bundle(self.arguments(root / "bundle", sources))

    def test_native_version_must_match_plugin(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            cli = root / "cli"
            cli.write_text("#!/bin/sh\nprintf 'codex-game-atelier 0.1.0-dev\\n'\n", encoding="utf-8")
            cli.chmod(0o755)
            bundle = root / "bundle"
            target = bundle / "bin" / "darwin-universal2"
            target.mkdir(parents=True)
            shutil_target = target / "codex-game-atelier"
            shutil_target.write_bytes(cli.read_bytes())
            shutil_target.chmod(0o755)
            with mock.patch.object(package_plugin.platform, "system", return_value="Darwin"), mock.patch.object(package_plugin.platform, "machine", return_value="arm64"):
                with self.assertRaises(package_plugin.BundleError):
                    package_plugin.verify_native_entry(bundle, "0.2.0")

    def test_archive_is_reproducible_and_tamper_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, sources))
                first = root / "first.tar.gz"
                second = root / "second.tar.gz"
                package_plugin.create_archive(bundle, first)
                package_plugin.create_archive(bundle, second)
                self.assertEqual(first.read_bytes(), second.read_bytes())
                package_plugin.verify_archive(first)
                first.write_bytes(first.read_bytes() + b"tampered")
                with self.assertRaises(package_plugin.BundleError):
                    package_plugin.verify_archive(first)

    def test_hard_linked_binary_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            linked = root / "linked-linux-cli"
            os.link(sources["linux-cli"], linked)
            sources["linux-cli"] = linked
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.build_bundle(self.arguments(root / "bundle", sources))

    def test_archive_path_traversal_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            archive = root / "unsafe.tar.gz"
            with tarfile.open(archive, "w:gz") as package:
                member = tarfile.TarInfo("../escape")
                member.size = 1
                package.addfile(member, io.BytesIO(b"x"))
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            package_plugin.archive_checksum_path(archive).write_text(f"{digest}  {archive.name}\n", encoding="ascii")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.verify_archive(archive)

    def test_header_shaped_truncated_binaries_are_rejected(self) -> None:
        samples = {
            "darwin-cli": b"\xca\xfe\xba\xbe" + (2).to_bytes(4, "big") + b"\x00" * 40,
            "linux-cli": b"\x7fELF\x02\x01\x01" + b"\x00" * 57,
            "windows-cli": b"MZ" + b"\x00" * 254,
        }
        for name, content in samples.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                sources = self.sources(root / "sources")
                sources[name].write_bytes(content)
                with self.assertRaises(package_plugin.BundleError):
                    package_plugin.build_bundle(self.arguments(root / "bundle", sources))

    def test_verify_is_static_and_trusted_smoke_runs_native_closure(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, sources))
            with mock.patch.object(package_plugin, "verify_native_entry") as native, mock.patch.object(package_plugin.sys, "argv", ["package_plugin.py", "verify", str(bundle)]):
                self.assertEqual(package_plugin.main(), 0)
                native.assert_not_called()
            with mock.patch.object(package_plugin, "verify_native_entry") as native, mock.patch.object(package_plugin.sys, "argv", ["package_plugin.py", "smoke-trusted-bundle", str(bundle)]):
                self.assertEqual(package_plugin.main(), 0)
                native.assert_called_once_with(bundle.resolve(), "0.2.0")

    def test_unknown_content_and_role_mode_are_rejected(self) -> None:
        for mutation in ("unknown", "mode"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                sources = self.sources(root / "sources")
                bundle = root / "bundle"
                with mock.patch.object(package_plugin, "verify_native_entry"):
                    package_plugin.build_bundle(self.arguments(bundle, sources))
                if mutation == "unknown":
                    (bundle / "unexpected.txt").write_text("unexpected\n", encoding="utf-8")
                else:
                    (bundle / "LICENSE").chmod(0o755)
                with self.assertRaises(package_plugin.BundleError):
                    package_plugin.verify_bundle(bundle)

    def test_trusted_smoke_kills_background_children_and_caps_output(self) -> None:
        with self.assertRaises(package_plugin.BundleError):
            package_plugin.run_native_smoke_process(["/bin/sh", "-c", "sleep 30 & exit 0"], 0, b"", b"")
        with self.assertRaises(package_plugin.BundleError):
            package_plugin.run_native_smoke_process(["/usr/bin/yes"], 0, b"", b"")


if __name__ == "__main__":
    unittest.main()
