#!/usr/bin/env python3
"""Validate the distributed monotonic command-gate policy."""

from __future__ import annotations

import json
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
POLICY = ROOT / "plugin" / "codex-game-atelier" / "skills" / "develop-godot-game" / "references" / "gate-policy.json"
FIXTURE = ROOT / "tests" / "fixtures" / "schemas" / "v1" / "gate-policy.default.json"


def load(path: Path) -> dict[str, object]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise AssertionError(f"expected object: {path}")
    return value


class GatePolicyTests(unittest.TestCase):
    def test_distributed_policy_matches_contract_fixture(self) -> None:
        self.assertEqual(load(POLICY), load(FIXTURE))

    def test_modes_are_monotonic_and_build_export_match(self) -> None:
        commands = load(POLICY)["commands"]
        self.assertIsInstance(commands, dict)
        self.assertEqual(commands["build"], commands["export"])
        for name, modes in commands.items():
            with self.subTest(command=name):
                manual = set(modes["manual"])
                standard = set(modes["standard"])
                strict = set(modes["strict"])
                self.assertTrue(manual < standard)
                self.assertTrue(standard < strict)

    def test_manual_cannot_remove_mandatory_export_safety(self) -> None:
        required = {
            "project-state",
            "supported-host",
            "godot-standard-version",
            "gdscript-only",
            "engine-user-data-authorization",
            "fixed-export-preset",
            "artifact-integrity",
            "target-smoke",
        }
        policy = load(POLICY)
        for command in ("build", "export"):
            self.assertTrue(required.issubset(policy["commands"][command]["manual"]))

    def test_only_strict_can_include_complete_release_set(self) -> None:
        release = load(POLICY)["commands"]["release-check"]
        complete = {"plugin-bundle", "starter-package", "license-and-provenance", "required-ci"}
        self.assertTrue(complete.isdisjoint(release["manual"]))
        self.assertTrue(complete.isdisjoint(release["standard"]))
        self.assertTrue(complete.issubset(release["strict"]))


if __name__ == "__main__":
    unittest.main()
