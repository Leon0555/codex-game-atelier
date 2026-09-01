#!/usr/bin/env python3
"""Assemble and verify a closed local Codex Game Atelier release candidate."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import tarfile
from typing import Optional
import unicodedata


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from tools import package_plugin  # noqa: E402


MANIFEST_NAME = "DISTRIBUTION-MANIFEST.json"
THIRD_PARTY_NOTICES = "THIRD_PARTY_NOTICES"
MAX_FILE_BYTES = 160 * 1024 * 1024
MAX_CANDIDATE_BYTES = 192 * 1024 * 1024
SEMVER = re.compile(
    r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)


class DistributionError(RuntimeError):
    pass


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(64 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def regular_file(path: Path, maximum: Optional[int] = None) -> os.stat_result:
    try:
        details = path.lstat()
    except OSError as error:
        raise DistributionError(f"required file is unavailable: {path.name}") from error
    if stat.S_ISLNK(details.st_mode) or not stat.S_ISREG(details.st_mode) or details.st_nlink != 1:
        raise DistributionError(f"required path is not a single regular file: {path.name}")
    if details.st_size < 1 or (maximum is not None and details.st_size > maximum):
        raise DistributionError(f"required file is empty or exceeds its bound: {path.name}")
    return details


def safe_directory(path: Path, label: str) -> Path:
    try:
        details = path.lstat()
    except OSError as error:
        raise DistributionError(f"{label} is missing or unsafe") from error
    if stat.S_ISLNK(details.st_mode) or not stat.S_ISDIR(details.st_mode):
        raise DistributionError(f"{label} is missing or unsafe")
    return path.resolve(strict=True)


def validate_filename(value: str) -> None:
    if (
        not value
        or value in (".", "..")
        or Path(value).name != value
        or "/" in value
        or "\\" in value
        or ":" in value
        or any(ord(character) < 32 for character in value)
    ):
        raise DistributionError("candidate manifest contains an unsafe file name")


def archive_member_json(archive: Path, member_name: str) -> dict[str, object]:
    try:
        with tarfile.open(archive, mode="r:gz") as package:
            member = package.getmember(member_name)
            if not member.isfile() or member.size < 1 or member.size > 1024 * 1024:
                raise DistributionError("component manifest member is invalid")
            stream = package.extractfile(member)
            if stream is None:
                raise DistributionError("component manifest member is unavailable")
            payload = stream.read(1024 * 1024 + 1)
    except (KeyError, OSError, tarfile.TarError) as error:
        raise DistributionError("component archive manifest is unreadable") from error
    if len(payload) > 1024 * 1024:
        raise DistributionError("component archive manifest exceeds its bound")
    try:
        value = json.loads(payload.decode("utf-8"))
    except (UnicodeError, json.JSONDecodeError) as error:
        raise DistributionError("component archive manifest is not valid UTF-8 JSON") from error
    if not isinstance(value, dict):
        raise DistributionError("component archive manifest root is invalid")
    return value


def component_names(version: str) -> dict[str, str]:
    if SEMVER.fullmatch(version) is None:
        raise DistributionError("distribution version is not valid semantic versioning")
    return {
        "plugin": f"codex-game-atelier-{version}.tar.gz",
    }


def load_component_versions(candidate: Path, names: dict[str, str]) -> tuple[str, dict[str, object]]:
    plugin_archive = candidate / names["plugin"]
    try:
        package_plugin.verify_archive(plugin_archive)
    except package_plugin.BundleError as error:
        raise DistributionError("component archive verification failed") from error
    plugin = archive_member_json(plugin_archive, "codex-game-atelier/BUNDLE-MANIFEST.json")
    plugin_identity = plugin.get("plugin")
    starter_identity = plugin.get("starter_template")
    plugin_provenance = plugin.get("build_provenance")
    plugin_version = plugin_identity.get("version") if isinstance(plugin_identity, dict) else None
    starter_version = starter_identity.get("version") if isinstance(starter_identity, dict) else None
    if (
        not isinstance(plugin_version, str)
        or starter_version != plugin_version
        or starter_identity != {
            "name": "codex-game-atelier-starter",
            "version": plugin_version,
            "path": "starter-template",
            "distribution": "embedded-in-plugin",
        }
        or not isinstance(plugin_provenance, dict)
    ):
        raise DistributionError("component version metadata is invalid")
    try:
        package_plugin.validate_build_provenance(plugin_provenance)
    except package_plugin.BundleError as error:
        raise DistributionError("component build provenance is invalid") from error
    return plugin_version, plugin_provenance


def inventory(candidate: Path) -> list[dict[str, object]]:
    files: list[dict[str, object]] = []
    for path in sorted(candidate.iterdir(), key=lambda item: item.name):
        if path.name == MANIFEST_NAME:
            continue
        details = regular_file(path, MAX_FILE_BYTES)
        files.append(
            {
                "path": path.name,
                "byte_size": details.st_size,
                "sha256": sha256_file(path),
                "mode": stat.S_IMODE(details.st_mode),
            }
        )
    return files


def write_manifest(candidate: Path, version: str, build_provenance: dict[str, object]) -> None:
    try:
        package_plugin.validate_build_provenance(build_provenance)
    except package_plugin.BundleError as error:
        raise DistributionError("distribution build provenance is invalid") from error
    names = component_names(version)
    files = inventory(candidate)
    content = {
        "schema_version": "1.2.0",
        "release": {
            "name": "codex-game-atelier",
            "version": version,
            "status": "local-candidate",
            "external_publication_performed": False,
        },
        "components": {
            "plugin": {
                "name": "codex-game-atelier",
                "version": version,
                "archive": names["plugin"],
                "checksum": names["plugin"] + ".sha256",
                "cli_version": version,
                "private_runner_version": version,
            },
            "starter_template": {
                "name": "codex-game-atelier-starter",
                "version": version,
                "path": "starter-template",
                "verified_plugin_version": version,
                "distribution": "embedded-in-plugin",
            },
        },
        "engine": {
            "kind": "godot",
            "version": "4.7.2-stable",
            "edition": "standard",
            "language": "gdscript",
        },
        "build_provenance": build_provenance,
        "policies": {
            "license": "MIT",
            "source_build_required": False,
            "telemetry_enabled": False,
            "hidden_external_writes": False,
            "git_hooks_automatically_installed": False,
            "game_export_signing_notarization_required": False,
            "distribution_channel": "codex-plugin-only",
            "standalone_user_binary_published": False,
            "apple_notarization_required": False,
            "remote_plugin_gatekeeper_validation": "NOT_RUN",
        },
        "files": files,
        "file_count": len(files),
        "expanded_byte_size": sum(item["byte_size"] for item in files),
    }
    encoded = (json.dumps(content, ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode("utf-8")
    target = candidate / MANIFEST_NAME
    target.write_bytes(encoded)
    target.chmod(0o644)


def load_manifest(candidate: Path) -> dict[str, object]:
    path = candidate / MANIFEST_NAME
    details = regular_file(path, 4 * 1024 * 1024)
    if stat.S_IMODE(details.st_mode) != 0o644:
        raise DistributionError("distribution manifest mode is invalid")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise DistributionError("distribution manifest is not valid UTF-8 JSON") from error
    if not isinstance(value, dict):
        raise DistributionError("distribution manifest root is invalid")
    return value


def verify_candidate(candidate: Path) -> None:
    candidate = safe_directory(candidate, "distribution candidate root")
    if stat.S_IMODE(candidate.lstat().st_mode) != 0o755:
        raise DistributionError("distribution candidate root mode is invalid")
    manifest = load_manifest(candidate)
    expected_keys = {
        "schema_version", "release", "components", "engine", "build_provenance", "policies",
        "files", "file_count", "expanded_byte_size",
    }
    if set(manifest) != expected_keys or manifest["schema_version"] != "1.2.0":
        raise DistributionError("distribution manifest fields are invalid")
    release = manifest["release"]
    if not isinstance(release, dict):
        raise DistributionError("release identity is invalid")
    version = release.get("version")
    if release != {
        "name": "codex-game-atelier",
        "version": version,
        "status": "local-candidate",
        "external_publication_performed": False,
    } or not isinstance(version, str):
        raise DistributionError("release identity is invalid")
    names = component_names(version)
    expected_files = {
        "LICENSE", "NOTICE", THIRD_PARTY_NOTICES, names["plugin"], names["plugin"] + ".sha256",
        MANIFEST_NAME,
    }
    observed = {path.name for path in candidate.iterdir()}
    if observed != expected_files:
        raise DistributionError("distribution candidate paths do not match the fixed allowlist")
    if len({unicodedata.normalize("NFC", name).casefold() for name in observed}) != len(observed):
        raise DistributionError("distribution candidate contains case-folding collisions")
    for name in observed:
        validate_filename(name)
        details = regular_file(candidate / name, MAX_FILE_BYTES)
        if stat.S_IMODE(details.st_mode) != 0o644:
            raise DistributionError("distribution candidate file mode is invalid")
    plugin_version, plugin_provenance = load_component_versions(candidate, names)
    if plugin_version != version:
        raise DistributionError("Plugin, CLI, runner, and Starter versions are not closed")
    try:
        package_plugin.validate_build_provenance(manifest["build_provenance"])
    except package_plugin.BundleError as error:
        raise DistributionError("distribution build provenance is invalid") from error
    if manifest["build_provenance"] != plugin_provenance:
        raise DistributionError("distribution and Plugin build provenance differ")
    components = manifest["components"]
    expected_components = {
        "plugin": {
            "name": "codex-game-atelier", "version": version,
            "archive": names["plugin"], "checksum": names["plugin"] + ".sha256",
            "cli_version": version, "private_runner_version": version,
        },
        "starter_template": {
            "name": "codex-game-atelier-starter", "version": version,
            "path": "starter-template", "verified_plugin_version": version,
            "distribution": "embedded-in-plugin",
        },
    }
    if components != expected_components:
        raise DistributionError("distribution component closure is invalid")
    if manifest["engine"] != {
        "kind": "godot", "version": "4.7.2-stable",
        "edition": "standard", "language": "gdscript",
    }:
        raise DistributionError("distribution engine contract is invalid")
    if manifest["policies"] != {
        "license": "MIT", "source_build_required": False,
        "telemetry_enabled": False, "hidden_external_writes": False,
        "git_hooks_automatically_installed": False,
        "game_export_signing_notarization_required": False,
        "distribution_channel": "codex-plugin-only",
        "standalone_user_binary_published": False,
        "apple_notarization_required": False,
        "remote_plugin_gatekeeper_validation": "NOT_RUN",
    }:
        raise DistributionError("distribution policy contract is invalid")
    actual_inventory = inventory(candidate)
    if manifest["files"] != actual_inventory or manifest["file_count"] != len(actual_inventory):
        raise DistributionError("distribution file inventory is invalid")
    total = sum(item["byte_size"] for item in actual_inventory)
    if manifest["expanded_byte_size"] != total or total > MAX_CANDIDATE_BYTES:
        raise DistributionError("distribution aggregate size is invalid")
    for name in ("LICENSE", "NOTICE", THIRD_PARTY_NOTICES):
        if (candidate / name).read_bytes() != (ROOT / name).read_bytes():
            raise DistributionError("distribution license or notice differs from the repository source")


def build_candidate(output: Path, plugin_bundle: Path) -> None:
    output = output.resolve(strict=False)
    if output.exists() or output.is_symlink():
        raise DistributionError("output path already exists; choose a new directory")
    try:
        plugin_bundle = plugin_bundle.resolve(strict=False)
        package_plugin.verify_bundle(plugin_bundle)
        plugin_manifest = package_plugin.load_bundle_manifest(plugin_bundle)
    except package_plugin.BundleError as error:
        raise DistributionError("input component verification failed") from error
    plugin_identity = plugin_manifest.get("plugin")
    starter_identity = plugin_manifest.get("starter_template")
    plugin_provenance = plugin_manifest.get("build_provenance")
    version = plugin_identity.get("version") if isinstance(plugin_identity, dict) else None
    try:
        package_plugin.validate_build_provenance(plugin_provenance)
    except package_plugin.BundleError as error:
        raise DistributionError("input Plugin provenance is not verified-clean") from error
    if not isinstance(version, str) or starter_identity != {
        "name": "codex-game-atelier-starter",
        "version": version,
        "path": "starter-template",
        "distribution": "embedded-in-plugin",
    }:
        raise DistributionError("input Plugin embedded Starter closure is invalid")
    names = component_names(version)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.mkdir(mode=0o755)
    try:
        package_plugin.create_archive(plugin_bundle, output / names["plugin"])
        for name in ("LICENSE", "NOTICE", THIRD_PARTY_NOTICES):
            shutil.copyfile(ROOT / name, output / name, follow_symlinks=False)
            (output / name).chmod(0o644)
        write_manifest(output, version, plugin_provenance)
        verify_candidate(output)
    except Exception:
        shutil.rmtree(output)
        raise


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    build = commands.add_parser("build", help="assemble a verified local release candidate")
    build.add_argument("--output", type=Path, required=True)
    build.add_argument("--plugin-bundle", type=Path, required=True)
    verify = commands.add_parser("verify", help="verify a local release candidate without executing it")
    verify.add_argument("candidate", type=Path)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        if args.command == "build":
            build_candidate(args.output, args.plugin_bundle)
            print(f"Distribution candidate built and verified: {args.output}")
        else:
            verify_candidate(args.candidate)
            print(f"Distribution candidate verification passed: {args.candidate}")
    except DistributionError as error:
        print(f"distribution candidate error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
