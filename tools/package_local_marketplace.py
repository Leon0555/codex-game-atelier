#!/usr/bin/env python3
"""Build and verify a local Codex marketplace around a trusted Plugin bundle."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import shutil
import stat
import sys


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from tools import package_plugin  # noqa: E402


MARKETPLACE_NAME = "codex-game-atelier-local"
PLUGIN_NAME = "codex-game-atelier"
MARKETPLACE_FILE = Path(".agents/plugins/marketplace.json")
PLUGIN_PATH = Path("plugins/codex-game-atelier")


class MarketplaceError(RuntimeError):
    pass


def marketplace_document() -> dict[str, object]:
    return {
        "name": MARKETPLACE_NAME,
        "interface": {"displayName": "Codex Game Atelier Local"},
        "plugins": [
            {
                "name": PLUGIN_NAME,
                "source": {"source": "local", "path": "./plugins/codex-game-atelier"},
                "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"},
                "category": "Productivity",
            }
        ],
    }


def copy_verified_bundle(source: Path, destination: Path) -> None:
    destination.mkdir(mode=0o755)
    for path in sorted(source.rglob("*"), key=lambda item: item.relative_to(source).as_posix()):
        relative = path.relative_to(source)
        details = path.lstat()
        target = destination / relative
        if stat.S_ISLNK(details.st_mode):
            raise MarketplaceError("Plugin bundle contains a symbolic link")
        if stat.S_ISDIR(details.st_mode):
            target.mkdir(parents=True, exist_ok=False)
            target.chmod(0o755)
            continue
        if not stat.S_ISREG(details.st_mode) or details.st_nlink != 1 or details.st_size < 1:
            raise MarketplaceError("Plugin bundle contains an unsafe file")
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target, follow_symlinks=False)
        target.chmod(stat.S_IMODE(details.st_mode))


def verify_marketplace(marketplace: Path) -> None:
    try:
        root_details = marketplace.lstat()
    except OSError as error:
        raise MarketplaceError("marketplace root is missing") from error
    if stat.S_ISLNK(root_details.st_mode) or not stat.S_ISDIR(root_details.st_mode):
        raise MarketplaceError("marketplace root is unsafe")
    marketplace = marketplace.resolve(strict=True)
    expected_root_entries = {".agents", "plugins"}
    if {path.name for path in marketplace.iterdir()} != expected_root_entries:
        raise MarketplaceError("marketplace root entries do not match the fixed contract")
    expected_directories = {
        ".agents",
        ".agents/plugins",
        "plugins",
        "plugins/codex-game-atelier",
    }
    for relative in expected_directories:
        path = marketplace / relative
        details = path.lstat()
        if stat.S_ISLNK(details.st_mode) or not stat.S_ISDIR(details.st_mode) or stat.S_IMODE(details.st_mode) != 0o755:
            raise MarketplaceError("marketplace directory shape or mode is invalid")
    index = marketplace / MARKETPLACE_FILE
    details = index.lstat()
    if stat.S_ISLNK(details.st_mode) or not stat.S_ISREG(details.st_mode) or details.st_nlink != 1 or stat.S_IMODE(details.st_mode) != 0o644:
        raise MarketplaceError("marketplace index is unsafe")
    try:
        document = json.loads(index.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise MarketplaceError("marketplace index is not valid UTF-8 JSON") from error
    if document != marketplace_document():
        raise MarketplaceError("marketplace index differs from the fixed contract")
    try:
        package_plugin.verify_bundle(marketplace / PLUGIN_PATH)
    except package_plugin.BundleError as error:
        raise MarketplaceError("marketplace Plugin bundle verification failed") from error


def build_marketplace(output: Path, plugin_bundle: Path) -> None:
    output = output.resolve(strict=False)
    if output.exists() or output.is_symlink():
        raise MarketplaceError("output path already exists; choose a new directory")
    plugin_bundle = plugin_bundle.resolve(strict=False)
    try:
        package_plugin.verify_bundle(plugin_bundle)
    except package_plugin.BundleError as error:
        raise MarketplaceError("input Plugin bundle verification failed") from error
    output.parent.mkdir(parents=True, exist_ok=True)
    output.mkdir(mode=0o755)
    try:
        index = output / MARKETPLACE_FILE
        index.parent.mkdir(parents=True, mode=0o755)
        (output / "plugins").mkdir(mode=0o755)
        encoded = (json.dumps(marketplace_document(), ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode("utf-8")
        index.write_bytes(encoded)
        index.chmod(0o644)
        copy_verified_bundle(plugin_bundle, output / PLUGIN_PATH)
        verify_marketplace(output)
    except Exception:
        shutil.rmtree(output)
        raise


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    build = commands.add_parser("build", help="build a local marketplace around a verified bundle")
    build.add_argument("--output", type=Path, required=True)
    build.add_argument("--plugin-bundle", type=Path, required=True)
    verify = commands.add_parser("verify", help="verify a generated local marketplace")
    verify.add_argument("marketplace", type=Path)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        if args.command == "build":
            build_marketplace(args.output, args.plugin_bundle)
            print(f"Local marketplace built and verified: {args.output}")
        else:
            verify_marketplace(args.marketplace)
            print(f"Local marketplace verification passed: {args.marketplace}")
    except MarketplaceError as error:
        print(f"local marketplace error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
