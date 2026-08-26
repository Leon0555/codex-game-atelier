#!/usr/bin/env python3
"""Build and verify the paired Codex Game Atelier Starter Template archive."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import tarfile
import tempfile
from typing import BinaryIO, Optional
import unicodedata
import zlib


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from tools.validators.validate_starter_template import (  # noqa: E402
    EXPECTED_DIRECTORIES as SOURCE_DIRECTORIES,
    EXPECTED_FILES as SOURCE_FILES,
    TemplateError,
    validate_template,
)


TEMPLATE_SOURCE = ROOT / "starter-template"
PLUGIN_MANIFEST = ROOT / "plugin" / "codex-game-atelier" / ".codex-plugin" / "plugin.json"
ARCHIVE_ROOT = "codex-game-atelier-starter"
PACKAGE_MANIFEST = "TEMPLATE-MANIFEST.json"
PACKAGE_FILES = SOURCE_FILES | {"LICENSE", "NOTICE", PACKAGE_MANIFEST}
MAX_FILE_BYTES = 1024 * 1024
MAX_PACKAGE_BYTES = 8 * 1024 * 1024
MAX_ARCHIVE_BYTES = 16 * 1024 * 1024
MAX_TAR_BYTES = MAX_PACKAGE_BYTES + 1024 * 1024
MAX_ARCHIVE_MEMBERS = 24
SEMVER = re.compile(
    r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)


class TemplatePackageError(RuntimeError):
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
        raise TemplatePackageError(f"required file is unavailable: {path.name}") from error
    if stat.S_ISLNK(details.st_mode) or not stat.S_ISREG(details.st_mode) or details.st_nlink != 1:
        raise TemplatePackageError(f"required path is not a single regular file: {path.name}")
    if details.st_size < 1 or (maximum is not None and details.st_size > maximum):
        raise TemplatePackageError(f"required file is empty or exceeds its bound: {path.name}")
    return details


def safe_directory(path: Path, label: str) -> Path:
    try:
        details = path.lstat()
    except OSError as error:
        raise TemplatePackageError(f"{label} is missing or unsafe") from error
    if stat.S_ISLNK(details.st_mode) or not stat.S_ISDIR(details.st_mode):
        raise TemplatePackageError(f"{label} is missing or unsafe")
    return path.resolve(strict=True)


def validate_relative_path(value: str) -> Path:
    candidate = Path(value)
    if (
        not value
        or candidate.is_absolute()
        or "\\" in value
        or any(
            part in ("", ".", "..")
            or ":" in part
            or any(ord(character) < 32 for character in part)
            for part in candidate.parts
        )
    ):
        raise TemplatePackageError("package contains an unsafe relative path")
    return candidate


def read_plugin_version(path: Path = PLUGIN_MANIFEST) -> str:
    regular_file(path, MAX_FILE_BYTES)
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise TemplatePackageError("paired Plugin manifest is not valid UTF-8 JSON") from error
    version = value.get("version") if isinstance(value, dict) and value.get("name") == "codex-game-atelier" else None
    if not isinstance(version, str) or not version:
        raise TemplatePackageError("paired Plugin identity or version is invalid")
    return version


def package_files(package: Path) -> list[dict[str, object]]:
    files: list[dict[str, object]] = []
    for path in sorted(package.rglob("*"), key=lambda item: item.as_posix()):
        details = path.lstat()
        if stat.S_ISDIR(details.st_mode):
            continue
        relative = path.relative_to(package).as_posix()
        if relative == PACKAGE_MANIFEST:
            continue
        regular_file(path, MAX_FILE_BYTES)
        files.append(
            {
                "path": relative,
                "byte_size": details.st_size,
                "sha256": sha256_file(path),
                "mode": stat.S_IMODE(details.st_mode),
            }
        )
    return files


def write_package_manifest(package: Path, plugin_version: str) -> None:
    files = package_files(package)
    content = {
        "schema_version": "1.0.0",
        "template": {"name": ARCHIVE_ROOT, "version": plugin_version},
        "pairing": {
            "kind": "codex-plugin",
            "name": "codex-game-atelier",
            "verified_plugin_version": plugin_version,
            "embedded": False,
        },
        "engine": {
            "kind": "godot",
            "version": "4.7.2-stable",
            "edition": "standard",
            "language": "gdscript",
        },
        "telemetry_enabled": False,
        "files": files,
        "file_count": len(files),
        "expanded_byte_size": sum(item["byte_size"] for item in files),
    }
    encoded = (json.dumps(content, ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode("utf-8")
    target = package / PACKAGE_MANIFEST
    target.write_bytes(encoded)
    target.chmod(0o644)


def load_package_manifest(package: Path) -> dict[str, object]:
    path = package / PACKAGE_MANIFEST
    details = regular_file(path, MAX_FILE_BYTES)
    if stat.S_IMODE(details.st_mode) != 0o644:
        raise TemplatePackageError("template manifest mode is invalid")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise TemplatePackageError("template manifest is not valid UTF-8 JSON") from error
    if not isinstance(value, dict):
        raise TemplatePackageError("template manifest root is invalid")
    return value


def validate_packaged_source(package: Path) -> None:
    """Reapply the source contract without trusting the package manifest hashes."""
    with tempfile.TemporaryDirectory(prefix="atelier-starter-source-") as temporary:
        projection = Path(temporary) / "starter-template"
        projection.mkdir(mode=0o755)
        for directory in sorted(SOURCE_DIRECTORIES):
            (projection / directory).mkdir(parents=True, exist_ok=False)
        for relative in sorted(SOURCE_FILES):
            shutil.copyfile(package / relative, projection / relative, follow_symlinks=False)
        try:
            validate_template(projection)
        except TemplateError as error:
            raise TemplatePackageError("packaged Godot source violates the Starter Template contract") from error


def verify_package(package: Path) -> None:
    package = safe_directory(package, "Starter Template package root")
    if stat.S_IMODE(package.lstat().st_mode) != 0o755:
        raise TemplatePackageError("Starter Template package root mode is invalid")
    observed_files: set[str] = set()
    observed_directories: set[str] = set()
    total = 0
    for path in sorted(package.rglob("*"), key=lambda item: item.as_posix()):
        details = path.lstat()
        relative = path.relative_to(package).as_posix()
        validate_relative_path(relative)
        if stat.S_ISLNK(details.st_mode):
            raise TemplatePackageError("Starter Template package contains a symbolic link")
        if stat.S_ISDIR(details.st_mode):
            if stat.S_IMODE(details.st_mode) != 0o755:
                raise TemplatePackageError("Starter Template package directory mode is invalid")
            observed_directories.add(relative)
            continue
        regular_file(path, MAX_FILE_BYTES)
        if stat.S_IMODE(details.st_mode) != 0o644:
            raise TemplatePackageError("Starter Template package file mode is invalid")
        observed_files.add(relative)
        total += details.st_size
    if observed_files != PACKAGE_FILES or observed_directories != SOURCE_DIRECTORIES or total > MAX_PACKAGE_BYTES:
        raise TemplatePackageError("Starter Template package paths or size do not match the fixed contract")
    if len({unicodedata.normalize("NFC", path).casefold() for path in observed_files | observed_directories}) != len(observed_files) + len(observed_directories):
        raise TemplatePackageError("Starter Template package contains a case-folding path collision")
    validate_packaged_source(package)

    manifest = load_package_manifest(package)
    expected_keys = {
        "schema_version",
        "template",
        "pairing",
        "engine",
        "telemetry_enabled",
        "files",
        "file_count",
        "expanded_byte_size",
    }
    if set(manifest) != expected_keys or manifest["schema_version"] != "1.0.0":
        raise TemplatePackageError("Starter Template manifest fields are invalid")
    pairing = manifest["pairing"]
    verified_version = pairing.get("verified_plugin_version") if isinstance(pairing, dict) else None
    if not isinstance(verified_version, str) or SEMVER.fullmatch(verified_version) is None:
        raise TemplatePackageError("Starter Template verified Plugin version is invalid")
    if manifest["template"] != {"name": ARCHIVE_ROOT, "version": verified_version}:
        raise TemplatePackageError("Starter Template manifest identity is invalid")
    if pairing != {
        "kind": "codex-plugin",
        "name": "codex-game-atelier",
        "verified_plugin_version": verified_version,
        "embedded": False,
    }:
        raise TemplatePackageError("Starter Template Plugin pairing is invalid")
    if manifest["engine"] != {
        "kind": "godot",
        "version": "4.7.2-stable",
        "edition": "standard",
        "language": "gdscript",
    } or manifest["telemetry_enabled"] is not False:
        raise TemplatePackageError("Starter Template engine or telemetry policy is invalid")
    declared = manifest["files"]
    actual = package_files(package)
    if declared != actual or manifest["file_count"] != len(actual):
        raise TemplatePackageError("Starter Template manifest file inventory is invalid")
    if manifest["expanded_byte_size"] != sum(item["byte_size"] for item in actual):
        raise TemplatePackageError("Starter Template manifest size is invalid")


def build_package(output: Path, source: Path = TEMPLATE_SOURCE, plugin_manifest: Path = PLUGIN_MANIFEST) -> None:
    if output.exists() or output.is_symlink():
        raise TemplatePackageError("output path already exists; choose a new directory")
    validate_template(source)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.mkdir(mode=0o755)
    try:
        for directory in sorted(SOURCE_DIRECTORIES):
            target = output / directory
            target.mkdir(parents=True, exist_ok=False)
            target.chmod(0o755)
        for relative in sorted(SOURCE_FILES):
            source_file = source / relative
            regular_file(source_file, MAX_FILE_BYTES)
            target = output / relative
            shutil.copyfile(source_file, target, follow_symlinks=False)
            target.chmod(0o644)
        for name in ("LICENSE", "NOTICE"):
            source_file = ROOT / name
            regular_file(source_file, MAX_FILE_BYTES)
            shutil.copyfile(source_file, output / name, follow_symlinks=False)
            (output / name).chmod(0o644)
        write_package_manifest(output, read_plugin_version(plugin_manifest))
        verify_package(output)
    except Exception:
        shutil.rmtree(output)
        raise


def archive_checksum_path(archive: Path) -> Path:
    return archive.with_name(archive.name + ".sha256")


def create_archive(package: Path, archive: Path) -> None:
    package = package.resolve(strict=False)
    archive = archive.resolve(strict=False)
    verify_package(package)
    checksum = archive_checksum_path(archive)
    if archive.exists() or archive.is_symlink() or checksum.exists() or checksum.is_symlink():
        raise TemplatePackageError("archive or checksum output already exists")
    archive.parent.mkdir(parents=True, exist_ok=True)
    try:
        with archive.open("xb") as raw:
            with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as compressed:
                with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as tar:
                    paths = [package] + sorted(package.rglob("*"), key=lambda item: item.relative_to(package).as_posix())
                    for path in paths:
                        relative = "" if path == package else path.relative_to(package).as_posix()
                        name = ARCHIVE_ROOT if not relative else f"{ARCHIVE_ROOT}/{relative}"
                        details = path.lstat()
                        if stat.S_ISLNK(details.st_mode) or not (stat.S_ISDIR(details.st_mode) or stat.S_ISREG(details.st_mode)):
                            raise TemplatePackageError("Starter Template package contains an unarchivable path")
                        info = tarfile.TarInfo(name)
                        info.uid = info.gid = info.mtime = 0
                        info.uname = info.gname = ""
                        info.mode = 0o755 if stat.S_ISDIR(details.st_mode) else stat.S_IMODE(details.st_mode)
                        if stat.S_ISDIR(details.st_mode):
                            info.type = tarfile.DIRTYPE
                            tar.addfile(info)
                        else:
                            info.size = details.st_size
                            with path.open("rb") as source_file:
                                tar.addfile(info, source_file)
        regular_file(archive, MAX_ARCHIVE_BYTES)
        checksum.write_text(f"{sha256_file(archive)}  {archive.name}\n", encoding="ascii")
        checksum.chmod(0o644)
        verify_archive(archive)
    except Exception:
        if archive.exists() and archive.is_file() and not archive.is_symlink():
            archive.unlink()
        if checksum.exists() and checksum.is_file() and not checksum.is_symlink():
            checksum.unlink()
        raise


def decompress_gzip_bounded(archive: Path, destination: BinaryIO) -> None:
    """Decompress exactly one gzip member while bounding the complete tar stream."""
    decompressor = zlib.decompressobj(16 + zlib.MAX_WBITS)
    total = 0
    with archive.open("rb") as source:
        while compressed := source.read(64 * 1024):
            pending = compressed
            while pending:
                remaining = MAX_TAR_BYTES - total
                expanded = decompressor.decompress(pending, remaining + 1)
                if len(expanded) > remaining:
                    raise TemplatePackageError("archive total decompressed stream exceeds its bound")
                destination.write(expanded)
                total += len(expanded)
                pending = decompressor.unconsumed_tail
                if decompressor.unused_data:
                    raise TemplatePackageError("archive contains concatenated gzip members or trailing data")
                if decompressor.eof:
                    if pending or source.read(1):
                        raise TemplatePackageError("archive contains concatenated gzip members or trailing data")
                    break
            if decompressor.eof:
                break
        if not decompressor.eof:
            raise TemplatePackageError("archive gzip stream is truncated")
        remaining = MAX_TAR_BYTES - total
        expanded = decompressor.flush(remaining + 1)
        if len(expanded) > remaining:
            raise TemplatePackageError("archive total decompressed stream exceeds its bound")
        destination.write(expanded)
    destination.seek(0)


def verify_archive(archive: Path) -> None:
    archive = archive.resolve(strict=False)
    regular_file(archive, MAX_ARCHIVE_BYTES)
    checksum = archive_checksum_path(archive)
    regular_file(checksum, 1024)
    expected = f"{sha256_file(archive)}  {archive.name}\n"
    try:
        if checksum.read_text(encoding="ascii") != expected:
            raise TemplatePackageError("archive checksum does not match")
    except (OSError, UnicodeError) as error:
        raise TemplatePackageError("archive checksum is unreadable") from error

    with tempfile.TemporaryFile() as tar_stream, tempfile.TemporaryDirectory(prefix="atelier-starter-verify-") as temporary:
        try:
            decompress_gzip_bounded(archive, tar_stream)
        except zlib.error as error:
            raise TemplatePackageError("archive gzip stream is invalid") from error
        destination = Path(temporary)
        observed: set[str] = set()
        total = 0
        try:
            with tarfile.open(fileobj=tar_stream, mode="r|") as tar:
                member_count = 0
                for member in tar:
                    member_count += 1
                    if member_count > MAX_ARCHIVE_MEMBERS:
                        raise TemplatePackageError("archive member count is outside its bound")
                    if member.name == ARCHIVE_ROOT:
                        relative = None
                    elif member.name.startswith(ARCHIVE_ROOT + "/"):
                        relative = member.name[len(ARCHIVE_ROOT) + 1 :]
                        validate_relative_path(relative)
                    else:
                        raise TemplatePackageError("archive member is outside the Starter Template root")
                    folded = unicodedata.normalize("NFC", member.name).casefold()
                    if folded in observed:
                        raise TemplatePackageError("archive contains a duplicate or case-folding collision")
                    observed.add(folded)
                    if member.issym() or member.islnk() or not (member.isdir() or member.isfile()):
                        raise TemplatePackageError("archive contains a link or special member")
                    if member.uid != 0 or member.gid != 0 or member.mtime != 0:
                        raise TemplatePackageError("archive metadata is not reproducible")
                    total += member.size
                    if total > MAX_PACKAGE_BYTES or member.size > MAX_FILE_BYTES:
                        raise TemplatePackageError("archive expanded size exceeds its bound")
                    target = destination / member.name
                    if member.isdir():
                        if member.mode != 0o755:
                            raise TemplatePackageError("archive directory mode is invalid")
                        target.mkdir(parents=True, exist_ok=False)
                        target.chmod(0o755)
                    else:
                        if relative is None or member.mode != 0o644:
                            raise TemplatePackageError("archive file mode is invalid")
                        source_file = tar.extractfile(member)
                        if source_file is None:
                            raise TemplatePackageError("archive file content is unavailable")
                        target.parent.mkdir(parents=True, exist_ok=True)
                        with target.open("xb") as output:
                            shutil.copyfileobj(source_file, output, 64 * 1024)
                        target.chmod(0o644)
                if member_count < 2:
                    raise TemplatePackageError("archive member count is outside its bound")
        except (OSError, tarfile.TarError) as error:
            raise TemplatePackageError("archive is unreadable") from error
        verify_package(destination / ARCHIVE_ROOT)


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    build = commands.add_parser("build", help="assemble and structurally verify a paired template package")
    build.add_argument("--output", type=Path, required=True)
    verify = commands.add_parser("verify", help="structurally verify a template package")
    verify.add_argument("package", type=Path)
    archive = commands.add_parser("archive", help="create a reproducible tar.gz and external SHA-256")
    archive.add_argument("--package", type=Path, required=True)
    archive.add_argument("--output", type=Path, required=True)
    verify_archive_command = commands.add_parser(
        "verify-archive",
        help="verify self-consistency and safely unpack; does not establish publisher trust",
        description=(
            "Verify archive/checksum self-consistency and safely unpack within fixed bounds. "
            "This does not establish publisher identity or provenance."
        ),
    )
    verify_archive_command.add_argument("archive", type=Path)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        if args.command == "build":
            build_package(args.output)
            print(f"Starter Template package built and verified: {args.output}")
        elif args.command == "verify":
            verify_package(args.package)
            print(f"Starter Template package verification passed: {args.package}")
        elif args.command == "archive":
            create_archive(args.package, args.output)
            print(f"Starter Template archive built and verified: {args.output}")
        else:
            verify_archive(args.archive)
            print(f"Starter Template archive verification passed: {args.archive}")
    except TemplatePackageError as error:
        print(f"starter template package error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
