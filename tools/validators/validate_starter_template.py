#!/usr/bin/env python3
"""Validate the tracked Godot Starter Template source without running project code."""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import stat
import sys


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_TEMPLATE = ROOT / "starter-template"
MAX_FILE_BYTES = 1024 * 1024
MAX_TEMPLATE_BYTES = 4 * 1024 * 1024

EXPECTED_FILES = {
    ".gitignore",
    "README.md",
    "main.tscn",
    "project.godot",
    "scripts/game_state.gd",
    "scripts/main.gd",
    "tests/atelier_test_runner.gd",
    "中文 资源/status_payload.gd",
}
EXPECTED_DIRECTORIES = {"scripts", "tests", "中文 资源"}
CONCRETE_MODEL = re.compile(
    r"\b(?:gpt-[a-z0-9][a-z0-9._-]*|o[1-9](?:-[a-z0-9][a-z0-9._-]*)?|claude-[a-z0-9][a-z0-9._-]*|gemini-[a-z0-9][a-z0-9._-]*|deepseek-[a-z0-9][a-z0-9._-]*|qwen[0-9][a-z0-9._-]*|llama-[a-z0-9][a-z0-9._-]*|mistral-[a-z0-9][a-z0-9._-]*|grok-[a-z0-9][a-z0-9._-]*)\b",
    re.IGNORECASE,
)
REQUIRED_IGNORE_RULES = {".DS_Store", ".godot/", ".import/", "build/", "export.cfg", "export_credentials.cfg"}
REQUIRED_README_PHRASES = {
    "This three-step count starts after the Codex Game Atelier Plugin is installed",
    "Copy this directory and rename the copy for your game.",
    "Open the copy in Codex with the Codex Game Atelier Plugin",
    "Ask Codex to run the supported validation and fixed GDScript tests",
    "Godot is an explicit prerequisite and is never installed automatically.",
    "this directory is a candidate template, not a released product.",
}


class TemplateError(RuntimeError):
    pass


def read_text(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as error:
        raise TemplateError(f"template file is not readable UTF-8: {path.name}") from error


def validate_template(root: Path) -> None:
    try:
        root_details = root.lstat()
    except OSError as error:
        raise TemplateError("starter template root is missing or unsafe") from error
    if stat.S_ISLNK(root_details.st_mode) or not stat.S_ISDIR(root_details.st_mode):
        raise TemplateError("starter template root is missing or unsafe")
    root = root.resolve(strict=True)

    files = set()
    directories = set()
    total = 0
    for path in sorted(root.rglob("*"), key=lambda item: item.as_posix()):
        details = path.lstat()
        relative = path.relative_to(root).as_posix()
        if stat.S_ISLNK(details.st_mode):
            raise TemplateError("starter template contains a symbolic link")
        if stat.S_ISDIR(details.st_mode):
            directories.add(relative)
            continue
        if not stat.S_ISREG(details.st_mode) or details.st_nlink != 1 or details.st_size < 1 or details.st_size > MAX_FILE_BYTES:
            raise TemplateError("starter template contains an empty, linked, special, or oversized file")
        if details.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH):
            raise TemplateError("starter template content must not be executable")
        files.add(relative)
        total += details.st_size
    if files != EXPECTED_FILES or directories != EXPECTED_DIRECTORIES or total > MAX_TEMPLATE_BYTES:
        raise TemplateError("starter template paths do not match the fixed allowlist")

    content = {relative: read_text(root / relative) for relative in EXPECTED_FILES}
    combined = "\n".join(content.values())
    folded = combined.casefold()
    if CONCRETE_MODEL.search(combined) or "codex game foundry" in folded or "codex-game-foundry" in folded:
        raise TemplateError("starter template contains a forbidden product or model identifier")

    project = content["project.godot"]
    if "config_version=5" not in project or 'run/main_scene="res://main.tscn"' not in project or "[dotnet]" in project.lower() or "assembly_name" in project.lower():
        raise TemplateError("starter template is not the fixed standard Godot/GDScript project")

    scene_lines = {line.strip() for line in content["main.tscn"].splitlines() if line.strip() and not line.lstrip().startswith(";")}
    required_scene_lines = {
        "[gd_scene load_steps=2 format=3]",
        '[ext_resource path="res://scripts/main.gd" type="Script" id="1_main"]',
        '[node name="Main" type="Node"]',
        '[node name="Status" type="Label" parent="Interface/Panel/Margin/Content"]',
        '[node name="PlayButton" type="Button" parent="Interface/Panel/Margin/Content"]',
    }
    if not required_scene_lines.issubset(scene_lines):
        raise TemplateError("starter scene or basic UI contract is incomplete")

    tests = content["tests/atelier_test_runner.gd"]
    test_lines = {line.strip() for line in tests.splitlines() if line.strip() and not line.lstrip().startswith("#")}
    required_test_lines = {
        "extends SceneTree",
        'const REPORT_PREFIX := "CODEX_GAME_ATELIER_TEST_REPORT "',
        'var scene := load("res://main.tscn") as PackedScene',
        "func _initialize() -> void:",
        "func _record(id: String, passed: bool, summary: String) -> void:",
        '"schema_version": "1.0.0",',
        "quit(0 if failed == 0 else 1)",
    }
    if not required_test_lines.issubset(test_lines):
        raise TemplateError("starter template fixed test protocol structure is incomplete")

    readme = content["README.md"]
    if sum(1 for line in readme.splitlines() if re.match(r"^[123]\. ", line)) != 3 or not all(phrase in readme for phrase in REQUIRED_README_PHRASES):
        raise TemplateError("starter template does not contain the fixed three-step start path")

    ignore_rules = [line.strip() for line in content[".gitignore"].splitlines() if line.strip() and not line.lstrip().startswith("#")]
    if not REQUIRED_IGNORE_RULES.issubset(set(ignore_rules)) or any(rule.rstrip("/") == ".gameatelier" for rule in ignore_rules):
        raise TemplateError("starter template cache or Atelier-state ignore policy is invalid")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("template", nargs="?", type=Path, default=DEFAULT_TEMPLATE)
    args = parser.parse_args()
    try:
        validate_template(args.template)
    except TemplateError as error:
        print(f"starter template error: {error}", file=sys.stderr)
        return 1
    print(f"Starter Template validation passed: {args.template}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
