#!/usr/bin/env python3
"""Validate logical profile resolution and distributed model neutrality."""

from __future__ import annotations

import json
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
CATALOG = ROOT / "plugin" / "codex-game-atelier" / "skills" / "develop-godot-game" / "references" / "capability-profiles.json"
MATRIX = ROOT / "tests" / "fixtures" / "profiles" / "profile-resolution-matrix.json"
FORBIDDEN_CONCRETE_MODEL_ID = re.compile(
    r"\b(?:gpt|claude|gemini|deepseek|llama|mistral|qwen)[-_ ]?\d",
    re.IGNORECASE,
)
CAPABILITY_RANK = {"standard": 0, "high": 1, "critical": 2}


def load_json(path: Path) -> dict[str, object]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise AssertionError(f"expected object: {path}")
    return value


def resolve_case(catalog: dict[str, object], case: dict[str, object]) -> tuple[str | None, str]:
    resolution = catalog["binding_resolution"]
    profiles = catalog["profiles"]
    assert isinstance(resolution, dict) and isinstance(profiles, list)
    available = case["available_sources"]
    assert isinstance(available, list)
    source = next((candidate for candidate in resolution["order"] if candidate in available), None)
    profile = next((candidate for candidate in profiles if isinstance(candidate, dict) and candidate.get("id") == case["requested_profile"]), None)
    if source is None or profile is None:
        return source, "BLOCKED"
    required = profile["capability_tier"]
    actual = case["resolved_capability"]
    assert isinstance(required, str) and isinstance(actual, str)
    if CAPABILITY_RANK[actual] < CAPABILITY_RANK[required]:
        if profile["unmet_capability_policy"] == "block":
            return source, "BLOCKED"
        return source, "RESOLVED_WITH_DISCLOSURE"
    if profile["independence"] == "independent" and case["independent_context"] is not True:
        return source, "BLOCKED"
    return source, "RESOLVED"


class CapabilityProfileTests(unittest.TestCase):
    def test_resolution_matrix(self) -> None:
        catalog = load_json(CATALOG)
        matrix = load_json(MATRIX)
        cases = matrix.get("cases")
        self.assertIsInstance(cases, list)
        self.assertEqual(len(cases), 9)
        for case in cases:
            with self.subTest(case=case.get("id")):
                self.assertEqual(
                    resolve_case(catalog, case),
                    (case["expected_source"], case["expected_outcome"]),
                )

    def test_profile_ids_are_unique_and_complete(self) -> None:
        profiles = load_json(CATALOG)["profiles"]
        self.assertIsInstance(profiles, list)
        identifiers = [profile["id"] for profile in profiles]
        self.assertEqual(identifiers, ["lead", "implementation", "fast-read", "independent-audit"])
        self.assertEqual(len(identifiers), len(set(identifiers)))

    def test_distributed_sources_have_no_concrete_model_ids(self) -> None:
        roots = (
            ROOT / "plugin" / "codex-game-atelier",
            ROOT / "starter-template",
            ROOT / "examples" / "reference-game",
            ROOT / "packages" / "cli",
        )
        checked = 0
        for root in roots:
            for path in root.rglob("*"):
                if not path.is_file() or path.name.endswith("_test.go"):
                    continue
                if path.suffix.lower() not in {".go", ".gd", ".json", ".md", ".tscn", ".tres", ".cfg", ".yaml", ".yml"}:
                    continue
                content = path.read_text(encoding="utf-8")
                self.assertIsNone(FORBIDDEN_CONCRETE_MODEL_ID.search(content), str(path.relative_to(ROOT)))
                checked += 1
        self.assertGreaterEqual(checked, 20)


if __name__ == "__main__":
    unittest.main()
