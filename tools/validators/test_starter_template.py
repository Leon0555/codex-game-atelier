#!/usr/bin/env python3
"""Regression tests for the tracked Starter Template contract."""

from __future__ import annotations

import importlib.util
import hashlib
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().with_name("validate_starter_template.py")
SPEC = importlib.util.spec_from_file_location("validate_starter_template", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
validator = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(validator)
EVIDENCE = validator.ROOT / "docs" / "validation" / "evidence" / "phase1-starter-template-2026-08-26"


class StarterTemplateTests(unittest.TestCase):
    def copy_template(self, root: Path) -> Path:
        target = root / "template"
        shutil.copytree(validator.DEFAULT_TEMPLATE, target)
        return target

    def test_tracked_template_passes(self) -> None:
        validator.validate_template(validator.DEFAULT_TEMPLATE)

    def test_generated_state_and_unknown_content_are_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            template = self.copy_template(Path(temporary))
            state = template / ".gameatelier"
            state.mkdir()
            (state / "project.json").write_text("{}\n", encoding="utf-8")
            with self.assertRaises(validator.TemplateError):
                validator.validate_template(template)

    def test_dotnet_and_missing_fixed_runner_are_rejected(self) -> None:
        for mutation in ("dotnet", "runner"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                template = self.copy_template(Path(temporary))
                if mutation == "dotnet":
                    project = template / "project.godot"
                    project.write_text(project.read_text(encoding="utf-8") + "\n[dotnet]\nproject/assembly_name=\"Unsafe\"\n", encoding="utf-8")
                else:
                    (template / "tests" / "atelier_test_runner.gd").write_text("extends SceneTree\n", encoding="utf-8")
                with self.assertRaises(validator.TemplateError):
                    validator.validate_template(template)

    def test_symbolic_and_hard_links_are_rejected(self) -> None:
        for mutation in ("symlink", "hardlink", "root-symlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                template = self.copy_template(root)
                target = template / "scripts" / "game_state.gd"
                replacement = template / "scripts" / "replacement.gd"
                if mutation == "symlink":
                    replacement.symlink_to(target.name)
                elif mutation == "hardlink":
                    os.link(target, replacement)
                else:
                    replacement = root / "template-link"
                    replacement.symlink_to(template, target_is_directory=True)
                    template = replacement
                with self.assertRaises(validator.TemplateError):
                    validator.validate_template(template)

    def test_identifiers_and_required_structural_markers_are_enforced(self) -> None:
        for mutation in ("model-gpt", "model-claude", "old-name", "test-comment", "ignore", "readme"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                template = self.copy_template(Path(temporary))
                if mutation == "model-gpt":
                    readme = template / "README.md"
                    readme.write_text(readme.read_text(encoding="utf-8") + "\nUse gpt-4.1.\n", encoding="utf-8")
                elif mutation == "model-claude":
                    readme = template / "README.md"
                    readme.write_text(readme.read_text(encoding="utf-8") + "\nUse claude-3.7-sonnet.\n", encoding="utf-8")
                elif mutation == "old-name":
                    readme = template / "README.md"
                    readme.write_text(readme.read_text(encoding="utf-8") + "\nCODEX GAME FOUNDRY\n", encoding="utf-8")
                elif mutation == "test-comment":
                    runner = template / "tests" / "atelier_test_runner.gd"
                    runner.write_text(runner.read_text(encoding="utf-8").replace("extends SceneTree", "# extends SceneTree", 1), encoding="utf-8")
                elif mutation == "ignore":
                    ignore = template / ".gitignore"
                    ignore.write_text(ignore.read_text(encoding="utf-8").replace(".godot/\n", "", 1), encoding="utf-8")
                else:
                    readme = template / "README.md"
                    readme.write_text(readme.read_text(encoding="utf-8").replace("Copy this directory and rename the copy for your game.", "Start somehow."), encoding="utf-8")
                with self.assertRaises(validator.TemplateError):
                    validator.validate_template(template)

    def test_persisted_evidence_matches_the_current_template(self) -> None:
        def load(name: str) -> dict[str, object]:
            with (EVIDENCE / name).open(encoding="utf-8") as source:
                value = json.load(source)
            self.assertIsInstance(value, dict)
            return value

        recorded_hashes: dict[str, str] = {}
        for line in (EVIDENCE / "source-files.sha256").read_text(encoding="utf-8").splitlines():
            digest, relative = line.split("  ", 1)
            self.assertRegex(digest, r"^[0-9a-f]{64}$")
            self.assertNotIn(relative, recorded_hashes)
            recorded_hashes[relative] = digest
        self.assertEqual(set(recorded_hashes), validator.EXPECTED_FILES)
        actual_hashes = {
            relative: hashlib.sha256((validator.DEFAULT_TEMPLATE / relative).read_bytes()).hexdigest()
            for relative in validator.EXPECTED_FILES
        }
        self.assertEqual(recorded_hashes, actual_hashes)

        execution = load("execution.json")
        first = load("initialize-first.json")
        repeat = load("initialize-repeat.json")
        project_state = load("project-state.json")
        validate_result = load("validate-result.json")
        validation_report = load("validation-report.json")
        test_result = load("test-result.json")
        test_report = load("test-report.json")

        state_bytes = json.dumps(project_state, ensure_ascii=False, indent=2).encode("utf-8") + b"\n"
        state_digest = hashlib.sha256(state_bytes).hexdigest()
        self.assertEqual(state_digest, execution["state_after_initialize"]["sha256"])
        self.assertEqual(execution["state_after_initialize"], execution["state_after_repeat_initialize"])
        self.assertFalse(execution["state_before_initialize"]["gameatelier_exists"])
        self.assertTrue(first["data"]["created"])
        self.assertFalse(repeat["data"]["created"])
        self.assertEqual(first["data"]["project_id"], repeat["data"]["project_id"])
        self.assertEqual(first["data"]["project_id"], project_state["project_id"])
        self.assertEqual(first["data"]["updated_at"], repeat["data"]["updated_at"])

        checks = validation_report["checks"]
        cases = test_report["tests"]
        self.assertEqual(validate_result["outcome"], validation_report["outcome"])
        self.assertEqual(validate_result["data"]["check_count"], len(checks))
        self.assertTrue(all(check["outcome"] == "PASS" for check in checks))
        self.assertEqual(test_result["outcome"], test_report["outcome"])
        self.assertEqual(test_result["data"]["test_count"], len(cases))
        self.assertEqual(test_result["data"]["passed_count"], len(cases))
        self.assertEqual(test_result["data"]["failed_count"], 0)
        self.assertTrue(all(case["outcome"] == "PASS" for case in cases))


if __name__ == "__main__":
    unittest.main()
