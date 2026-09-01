#!/usr/bin/env python3
"""Regression tests for the deterministic plugin bundle builder."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import io
import json
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
    def setUp(self) -> None:
        self.provenance = {
            "source_revision": "a" * 40,
            "source_clean": True,
            "go_version": "go1.27.0",
            "trimpath": True,
            "cgo_enabled": False,
            "binary_file_count": 6,
            "binary_build_record_count": 8,
        }
        patcher = mock.patch.object(
            package_plugin,
            "collect_build_provenance",
            return_value=self.provenance,
        )
        patcher.start()
        self.addCleanup(patcher.stop)

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
            go_tool=Path("/fixture/go"),
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
            self.assertEqual(
                package_plugin.load_bundle_manifest(first)["build_provenance"],
                self.provenance,
            )
            self.assertEqual(
                (first / "THIRD_PARTY_NOTICES").read_bytes(),
                (package_plugin.ROOT / "THIRD_PARTY_NOTICES").read_bytes(),
            )

    def test_dirty_source_and_mismatched_go_record_are_rejected(self) -> None:
        with mock.patch.object(package_plugin, "checked_text_command", return_value=" M packages/cli/main.go\n"):
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.clean_source_revision()

        record = {
            "go_version": "go1.27.0",
            "path": package_plugin.GO_PACKAGES["cli"],
            "mod": package_plugin.GO_MODULE,
            "-trimpath": "true",
            "CGO_ENABLED": "0",
            "GOOS": "linux",
            "GOARCH": "amd64",
            "vcs.revision": "a" * 40,
            "vcs.modified": "true",
        }
        with self.assertRaises(package_plugin.BundleError):
            package_plugin.validate_go_build_record(
                record,
                go_version="go1.27.0",
                revision="a" * 40,
                package=package_plugin.GO_PACKAGES["cli"],
                goos="linux",
                goarch="amd64",
            )

    def test_third_party_notice_tamper_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bundle = root / "bundle"
            package_plugin.build_bundle(self.arguments(bundle, self.sources(root / "sources")))
            (bundle / "THIRD_PARTY_NOTICES").write_text("tampered\n", encoding="utf-8")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.verify_bundle(bundle)

    def test_build_can_override_one_closed_semver_without_changing_source(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            arguments = self.arguments(root / "bundle", sources)
            arguments.version = "0.2.0+codex.lifecycle-a"
            package_plugin.build_bundle(arguments)
            packaged = package_plugin.read_plugin_manifest(arguments.output)
            source = json.loads((package_plugin.PLUGIN_SOURCE / ".codex-plugin/plugin.json").read_text(encoding="utf-8"))
            self.assertEqual(packaged["version"], "0.2.0+codex.lifecycle-a")
            self.assertNotEqual(source["version"], packaged["version"])

            invalid = self.arguments(root / "invalid", sources)
            invalid.version = "not semver"
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.build_bundle(invalid)

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

    def test_profile_catalog_and_concrete_model_policy_are_enforced(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, sources))
            catalog = bundle / "skills" / "develop-godot-game" / "references" / "capability-profiles.json"
            value = json.loads(catalog.read_text(encoding="utf-8"))
            value["binding_resolution"]["cli_selects_models"] = True
            catalog.write_text(json.dumps(value), encoding="utf-8")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.validate_profile_catalog(bundle)

        forbidden = "Use " + "gpt" + "-9 for this task."
        self.assertIsNotNone(package_plugin.FORBIDDEN_CONCRETE_MODEL_ID.search(forbidden))
        self.assertIsNone(package_plugin.FORBIDDEN_CONCRETE_MODEL_ID.search("Use the independent-audit logical profile."))

    def test_native_collaboration_reference_is_packaged_and_policy_checked(self) -> None:
        relative = "skills/develop-godot-game/references/native-collaboration.md"
        self.assertIn(relative, package_plugin.ALLOWED_SOURCE_FILES)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, sources))
            distributed = bundle / relative
            source = package_plugin.PLUGIN_SOURCE / relative
            self.assertEqual(distributed.read_bytes(), source.read_bytes())
            skill = (bundle / "skills/develop-godot-game/SKILL.md").read_text(encoding="utf-8")
            self.assertIn("load `references/native-collaboration.md`", skill)
            text = distributed.read_text(encoding="utf-8")
            self.assertIn("no more than three delegated agents concurrently", text)
            self.assertIn("exactly one active write owner", text)
            self.assertIn("genuinely separate read-only context", text)
            self.assertIn("Task, handoff, and evidence files are the recovery source of truth", text)
            self.assertIn("Anchor recovery contracts at `../../schemas/v1/` relative to the installed `develop-godot-game` Skill directory", text)
            self.assertIn("validate the task with `task.schema.json`, the handoff with `handoff.schema.json`, and every evidence record with `evidence.schema.json`", text)
            blocked_pairs = (
                ("`game`", "`game/scenes/player.gd`"),
                ("`Scripts/Player.gd`", "`scripts/player.gd`"),
                ("`art/café.png`", "`art/café.png`"),
                ("`linked/player.gd`", "`src/player.gd`"),
            )
            for left, right in blocked_pairs:
                with self.subTest(left=left, right=right):
                    self.assertIn(f"{left} and {right}", text)
            self.assertIn("If any component is a symbolic link, return `BLOCKED`", text)
            self.assertIn("Normalize every segment to Unicode NFC, case-fold it, and normalize the folded value to NFC again", text)
            self.assertIn("either key is an ancestor of the other", text)

            distributed.write_text(text + "\nUse " + "gpt" + "-9.\n", encoding="utf-8")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.verify_distributed_model_policy(bundle)

    def test_skill_exposes_only_the_verified_embedded_starter_flow(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, self.sources(root / "sources")))
            skill = (bundle / "skills/develop-godot-game/SKILL.md").read_text(encoding="utf-8")
            self.assertIn("starter create --project <new-directory>", skill)
            self.assertIn("run `initialize --project <new-directory>`", skill)
            self.assertIn("Do not shell-copy or manually reconstruct the template", skill)
            self.assertIn("never installs Godot or dependencies", skill)
            self.assertIn("writes user-level Codex state", skill)

    def test_recovery_schema_closure_is_packaged_and_bundle_only(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, sources))

            for relative in package_plugin.RECOVERY_SCHEMA_FILES:
                source = package_plugin.ROOT / relative
                distributed = bundle / relative
                self.assertEqual(distributed.read_bytes(), source.read_bytes())

            with mock.patch.object(package_plugin, "SCHEMA_SOURCE", root / "missing-source-checkout"):
                package_plugin.verify_bundle(bundle)

            common = bundle / "schemas/v1/common.schema.json"
            original_common = common.read_bytes()
            common.unlink()
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.validate_recovery_schema_closure(bundle)
            common.write_bytes(original_common)
            common.chmod(0o644)

            task = bundle / "schemas/v1/task.schema.json"
            task_value = json.loads(task.read_text(encoding="utf-8"))
            task_value["properties"]["owner"]["$ref"] = "missing.schema.json#/$defs/owner"
            task.write_text(json.dumps(task_value), encoding="utf-8")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.validate_recovery_schema_closure(bundle)

    def test_gate_policy_is_packaged_and_semantically_enforced(self) -> None:
        relative = "skills/develop-godot-game/references/gate-policy.json"
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            sources = self.sources(root / "sources")
            bundle = root / "bundle"
            with mock.patch.object(package_plugin, "verify_native_entry"):
                package_plugin.build_bundle(self.arguments(bundle, sources))
            distributed = bundle / relative
            self.assertEqual(distributed.read_bytes(), (package_plugin.PLUGIN_SOURCE / relative).read_bytes())
            policy = json.loads(distributed.read_text(encoding="utf-8"))
            policy["commands"]["build"]["manual"].remove("target-smoke")
            distributed.write_text(json.dumps(policy), encoding="utf-8")
            with self.assertRaises(package_plugin.BundleError):
                package_plugin.validate_gate_policy(bundle)

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
