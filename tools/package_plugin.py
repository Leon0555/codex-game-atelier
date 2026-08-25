#!/usr/bin/env python3
"""Build and verify a zero-source-build Codex Game Atelier plugin directory."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import platform
try:
    import resource
except ImportError:  # pragma: no cover - native smoke is Darwin-only.
    resource = None
import shutil
import signal
import stat
import struct
import subprocess
import sys
import tarfile
import tempfile
import time
from typing import Optional
import unicodedata


ROOT = Path(__file__).resolve().parents[1]
PLUGIN_SOURCE = ROOT / "plugin" / "codex-game-atelier"
MAX_BINARY_BYTES = 64 * 1024 * 1024
MAX_BUNDLE_BYTES = 256 * 1024 * 1024
MAX_ARCHIVE_BYTES = 128 * 1024 * 1024
MAX_ARCHIVE_MEMBERS = 64
BUNDLE_MANIFEST = "BUNDLE-MANIFEST.json"
ARCHIVE_ROOT = "codex-game-atelier"
ALLOWED_SOURCE_FILES = {
    ".codex-plugin/plugin.json",
    "skills/develop-godot-game/SKILL.md",
    "skills/develop-godot-game/agents/openai.yaml",
}

TARGETS = (
    {
        "host": "darwin-universal2",
        "cli_name": "codex-game-atelier",
        "runner_name": "codex-game-atelier-runner",
        "support_statement": "generated Universal 2; runtime validation is limited to Apple Silicon",
        "native_validation": "NOT_RECORDED",
        "binary_format": "mach-o-universal2",
        "architectures": ["x86_64", "arm64"],
        "intel_smoke": False,
    },
    {
        "host": "linux-amd64",
        "cli_name": "codex-game-atelier",
        "runner_name": "codex-game-atelier-runner",
        "support_statement": "cross-build artifact only",
        "native_validation": "NOT_RUN",
        "binary_format": "elf-amd64",
        "architectures": ["amd64"],
    },
    {
        "host": "windows-amd64",
        "cli_name": "codex-game-atelier.exe",
        "runner_name": "codex-game-atelier-runner.exe",
        "support_statement": "cross-build artifact only",
        "native_validation": "NOT_RUN",
        "binary_format": "pe-amd64",
        "architectures": ["amd64"],
    },
)


class BundleError(RuntimeError):
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
        raise BundleError(f"required file is unavailable: {path.name}") from error
    if stat.S_ISLNK(details.st_mode) or not stat.S_ISREG(details.st_mode):
        raise BundleError(f"required path is not a regular file: {path.name}")
    if details.st_nlink != 1:
        raise BundleError(f"required file has multiple hard links: {path.name}")
    if details.st_size < 1 or (maximum is not None and details.st_size > maximum):
        raise BundleError(f"required file is empty or exceeds its bound: {path.name}")
    return details


def validate_relative_path(value: str) -> Path:
    candidate = Path(value)
    if not value or candidate.is_absolute() or "\\" in value or any(part in ("", ".", "..") or ":" in part or any(ord(character) < 32 for character in part) for part in candidate.parts):
        raise BundleError("bundle manifest contains an unsafe relative path")
    return candidate


def copy_source_tree(source: Path, destination: Path) -> None:
    if not source.is_dir() or source.is_symlink():
        raise BundleError("plugin source root is missing or unsafe")
    observed_files = set()
    for path in sorted(source.rglob("*"), key=lambda item: item.as_posix()):
        relative = path.relative_to(source)
        if any(part == "AGENTS.md" for part in relative.parts):
            raise BundleError("internal AGENTS.md must not enter the plugin bundle")
        details = path.lstat()
        target = destination / relative
        if stat.S_ISLNK(details.st_mode):
            raise BundleError("plugin source contains a symbolic link")
        if stat.S_ISDIR(details.st_mode):
            target.mkdir(parents=True, exist_ok=False)
            target.chmod(0o755)
            continue
        if not stat.S_ISREG(details.st_mode) or details.st_size < 1:
            raise BundleError("plugin source contains an empty or special file")
        relative_text = relative.as_posix()
        if relative_text not in ALLOWED_SOURCE_FILES:
            raise BundleError(f"plugin source contains an unapproved file: {relative_text}")
        if details.st_nlink != 1:
            raise BundleError("plugin source contains a multiply linked file")
        observed_files.add(relative_text)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target, follow_symlinks=False)
        target.chmod(0o644)
    if observed_files != ALLOWED_SOURCE_FILES:
        raise BundleError("plugin source allowlist is incomplete")


def inspect_binary(path: Path, expected_format: str) -> None:
    details = regular_file(path, MAX_BINARY_BYTES)
    with path.open("rb") as source:
        header = source.read(min(details.st_size, 4096))
    if expected_format == "mach-o-universal2":
        if len(header) < 8 or header[:4] != b"\xca\xfe\xba\xbe":
            raise BundleError(f"binary is not a 32-bit-table Mach-O universal file: {path.name}")
        count = struct.unpack(">I", header[4:8])[0]
        table_size = 8 + count * 20
        if count != 2 or table_size > len(header):
            raise BundleError(f"Mach-O universal table is invalid: {path.name}")
        architectures = set()
        ranges = []
        entries = []
        for index in range(count):
            offset = 8 + index * 20
            cpu, _, slice_offset, slice_size, _ = struct.unpack(">IIIII", header[offset : offset + 20])
            if slice_size < 1 or slice_offset + slice_size > details.st_size:
                raise BundleError(f"Mach-O universal slice is out of bounds: {path.name}")
            architectures.add(cpu)
            ranges.append((slice_offset, slice_offset + slice_size))
            entries.append((cpu, slice_offset))
        if architectures != {0x01000007, 0x0100000C} or (ranges[0][1] > ranges[1][0] and ranges[1][1] > ranges[0][0]):
            raise BundleError(f"Mach-O universal architectures are invalid: {path.name}")
        with path.open("rb") as source:
            for cpu, slice_offset in entries:
                source.seek(slice_offset)
                thin_header = source.read(32)
                if len(thin_header) != 32 or thin_header[:4] != b"\xcf\xfa\xed\xfe" or struct.unpack("<I", thin_header[4:8])[0] != cpu:
                    raise BundleError(f"Mach-O universal slice header is invalid: {path.name}")
                file_type, command_count, command_bytes = struct.unpack("<III", thin_header[12:24])
                slice_size = next(end - start for start, end in ranges if start == slice_offset)
                if file_type != 2 or command_count < 1 or command_count > 4096 or command_bytes < 8 or 32 + command_bytes > slice_size:
                    raise BundleError(f"Mach-O universal slice is truncated or not executable: {path.name}")
        return
    if expected_format == "elf-amd64":
        if len(header) < 64 or header[:7] != b"\x7fELF\x02\x01\x01" or struct.unpack("<H", header[16:18])[0] not in (2, 3) or struct.unpack("<H", header[18:20])[0] != 62 or struct.unpack("<I", header[20:24])[0] != 1:
            raise BundleError(f"binary is not ELF amd64: {path.name}")
        program_offset = struct.unpack("<Q", header[32:40])[0]
        header_size, program_entry_size, program_count = struct.unpack("<HHH", header[52:58])
        if header_size != 64 or program_entry_size != 56 or program_count < 1 or program_count > 4096 or program_offset < header_size or program_offset + program_entry_size * program_count > details.st_size:
            raise BundleError(f"ELF amd64 program table is truncated or invalid: {path.name}")
        return
    if expected_format == "pe-amd64":
        if len(header) < 64 or header[:2] != b"MZ":
            raise BundleError(f"binary is not PE amd64: {path.name}")
        pe_offset = struct.unpack("<I", header[60:64])[0]
        needed = pe_offset + 24
        if needed > len(header) or header[pe_offset : pe_offset + 4] != b"PE\x00\x00" or struct.unpack("<H", header[pe_offset + 4 : pe_offset + 6])[0] != 0x8664:
            raise BundleError(f"binary is not PE32+ amd64: {path.name}")
        section_count = struct.unpack("<H", header[pe_offset + 6 : pe_offset + 8])[0]
        optional_size = struct.unpack("<H", header[pe_offset + 20 : pe_offset + 22])[0]
        characteristics = struct.unpack("<H", header[pe_offset + 22 : pe_offset + 24])[0]
        optional_offset = pe_offset + 24
        section_table_end = optional_offset + optional_size + section_count * 40
        if section_count < 1 or section_count > 96 or optional_size < 112 or section_table_end > details.st_size or section_table_end > len(header) or characteristics & 0x0002 == 0 or struct.unpack("<H", header[optional_offset : optional_offset + 2])[0] != 0x20B:
            raise BundleError(f"PE32+ amd64 headers are truncated or not executable: {path.name}")
        return
    raise BundleError("unknown binary format contract")


def copy_binary(source: Path, destination: Path, expected_format: str) -> None:
    inspect_binary(source, expected_format)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, destination, follow_symlinks=False)
    destination.chmod(0o755)


def bundle_files(bundle: Path) -> list[dict[str, object]]:
    files: list[dict[str, object]] = []
    for path in sorted(bundle.rglob("*"), key=lambda item: item.as_posix()):
        relative = path.relative_to(bundle).as_posix()
        details = path.lstat()
        if stat.S_ISLNK(details.st_mode):
            raise BundleError("bundle contains a symbolic link")
        if stat.S_ISDIR(details.st_mode):
            continue
        if not stat.S_ISREG(details.st_mode) or details.st_size < 1:
            raise BundleError("bundle contains an empty or special file")
        if details.st_nlink != 1:
            raise BundleError("bundle contains a multiply linked file")
        if path.name == BUNDLE_MANIFEST:
            continue
        files.append(
            {
                "path": relative,
                "byte_size": details.st_size,
                "sha256": sha256_file(path),
                "mode": stat.S_IMODE(details.st_mode),
            }
        )
    return files


def read_plugin_manifest(bundle: Path) -> dict[str, object]:
    path = bundle / ".codex-plugin" / "plugin.json"
    regular_file(path, 1024 * 1024)
    try:
        manifest = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise BundleError("plugin manifest is not valid UTF-8 JSON") from error
    if not isinstance(manifest, dict) or manifest.get("name") != "codex-game-atelier":
        raise BundleError("plugin manifest identity is invalid")
    version = manifest.get("version")
    if not isinstance(version, str) or not version:
        raise BundleError("plugin manifest version is missing")
    return manifest


def write_bundle_manifest(bundle: Path) -> None:
    plugin = read_plugin_manifest(bundle)
    content = {
        "schema_version": "1.0.0",
        "plugin": {"name": plugin["name"], "version": plugin["version"]},
        "source_build_required": False,
        "telemetry_enabled": False,
        "hosts": list(TARGETS),
        "files": bundle_files(bundle),
    }
    content["file_count"] = len(content["files"])
    content["expanded_byte_size"] = sum(item["byte_size"] for item in content["files"])
    encoded = (json.dumps(content, ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode("utf-8")
    path = bundle / BUNDLE_MANIFEST
    path.write_bytes(encoded)
    path.chmod(0o644)


def load_bundle_manifest(bundle: Path) -> dict[str, object]:
    path = bundle / BUNDLE_MANIFEST
    details = regular_file(path, 4 * 1024 * 1024)
    if stat.S_IMODE(details.st_mode) != 0o644:
        raise BundleError("bundle manifest mode is invalid")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise BundleError("bundle manifest is not valid UTF-8 JSON") from error
    if not isinstance(value, dict):
        raise BundleError("bundle manifest root is invalid")
    return value


def verify_bundle(bundle: Path) -> None:
    if not bundle.is_dir() or bundle.is_symlink():
        raise BundleError("bundle root is missing or unsafe")
    if stat.S_IMODE(bundle.lstat().st_mode) != 0o755:
        raise BundleError("bundle root mode is invalid")
    expected_directories = {
        ".codex-plugin",
        "bin",
        "skills",
        "skills/develop-godot-game",
        "skills/develop-godot-game/agents",
    } | {f"bin/{target['host']}" for target in TARGETS}
    observed_directories = set()
    for path in bundle.rglob("*"):
        details = path.lstat()
        if stat.S_ISDIR(details.st_mode):
            observed_directories.add(path.relative_to(bundle).as_posix())
            if stat.S_IMODE(details.st_mode) != 0o755:
                raise BundleError("bundle directory mode is invalid")
    if observed_directories != expected_directories:
        raise BundleError("bundle directory paths do not match the fixed allowlist")
    plugin = read_plugin_manifest(bundle)
    manifest = load_bundle_manifest(bundle)
    if set(manifest) != {"schema_version", "plugin", "source_build_required", "telemetry_enabled", "hosts", "files", "file_count", "expanded_byte_size"}:
        raise BundleError("bundle manifest fields are invalid")
    if manifest["schema_version"] != "1.0.0" or manifest["plugin"] != {"name": plugin["name"], "version": plugin["version"]}:
        raise BundleError("bundle manifest identity is invalid")
    if manifest["source_build_required"] is not False or manifest["telemetry_enabled"] is not False:
        raise BundleError("bundle policy flags are invalid")
    if manifest["hosts"] != list(TARGETS):
        raise BundleError("bundle host declarations are invalid")
    declared = manifest["files"]
    if not isinstance(declared, list) or declared != bundle_files(bundle):
        raise BundleError("bundle file inventory does not match its contents")
    if manifest["file_count"] != len(declared) or manifest["expanded_byte_size"] != sum(entry["byte_size"] for entry in declared) or manifest["expanded_byte_size"] > MAX_BUNDLE_BYTES:
        raise BundleError("bundle aggregate bounds are invalid")
    declared_paths = {entry["path"] for entry in declared if isinstance(entry, dict) and isinstance(entry.get("path"), str)}
    binary_paths = {
        f"bin/{target['host']}/{name}"
        for target in TARGETS
        for name in (target["cli_name"], target["runner_name"])
    }
    expected_paths = ALLOWED_SOURCE_FILES | {"LICENSE", "NOTICE"} | binary_paths
    if declared_paths != expected_paths:
        raise BundleError("bundle content paths do not match the fixed allowlist")
    for entry in declared:
        expected_mode = 0o755 if entry["path"] in binary_paths else 0o644
        if entry.get("mode") != expected_mode:
            raise BundleError("bundle file mode does not match its role")
    if len({unicodedata.normalize("NFC", relative).casefold() for relative in declared_paths}) != len(declared_paths):
        raise BundleError("bundle contains case-folding path collisions")
    for relative in declared_paths:
        validate_relative_path(relative)
    if any(Path(relative).name == "AGENTS.md" for relative in declared_paths):
        raise BundleError("internal AGENTS.md entered the plugin bundle")
    for target in TARGETS:
        host = target["host"]
        for name in (target["cli_name"], target["runner_name"]):
            relative = f"bin/{host}/{name}"
            if relative not in declared_paths:
                raise BundleError("bundle is missing a required platform executable")
            inspect_binary(bundle / relative, target["binary_format"])


def archive_checksum_path(archive: Path) -> Path:
    return archive.with_name(archive.name + ".sha256")


def create_archive(bundle: Path, archive: Path) -> None:
    bundle = bundle.resolve(strict=False)
    archive = archive.resolve(strict=False)
    verify_bundle(bundle)
    checksum = archive_checksum_path(archive)
    if archive.exists() or archive.is_symlink() or checksum.exists() or checksum.is_symlink():
        raise BundleError("archive or checksum output already exists")
    archive.parent.mkdir(parents=True, exist_ok=True)
    try:
        with archive.open("xb") as raw:
            with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as compressed:
                with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as package:
                    paths = [bundle] + sorted(bundle.rglob("*"), key=lambda item: item.relative_to(bundle).as_posix())
                    for path in paths:
                        relative = "" if path == bundle else path.relative_to(bundle).as_posix()
                        name = ARCHIVE_ROOT if not relative else f"{ARCHIVE_ROOT}/{relative}"
                        details = path.lstat()
                        if stat.S_ISLNK(details.st_mode) or not (stat.S_ISDIR(details.st_mode) or stat.S_ISREG(details.st_mode)):
                            raise BundleError("bundle contains an unarchivable path")
                        info = tarfile.TarInfo(name)
                        info.uid = 0
                        info.gid = 0
                        info.uname = ""
                        info.gname = ""
                        info.mtime = 0
                        info.mode = 0o755 if stat.S_ISDIR(details.st_mode) else stat.S_IMODE(details.st_mode)
                        if stat.S_ISDIR(details.st_mode):
                            info.type = tarfile.DIRTYPE
                            package.addfile(info)
                        else:
                            info.size = details.st_size
                            with path.open("rb") as source:
                                package.addfile(info, source)
        regular_file(archive, MAX_ARCHIVE_BYTES)
        digest = sha256_file(archive)
        checksum.write_text(f"{digest}  {archive.name}\n", encoding="ascii")
        checksum.chmod(0o644)
        verify_archive(archive)
    except Exception:
        if archive.exists() and archive.is_file() and not archive.is_symlink():
            archive.unlink()
        if checksum.exists() and checksum.is_file() and not checksum.is_symlink():
            checksum.unlink()
        raise


def verify_archive(archive: Path) -> None:
    archive = archive.resolve(strict=False)
    regular_file(archive, MAX_ARCHIVE_BYTES)
    checksum = archive_checksum_path(archive)
    regular_file(checksum, 1024)
    expected_line = f"{sha256_file(archive)}  {archive.name}\n"
    try:
        if checksum.read_text(encoding="ascii") != expected_line:
            raise BundleError("archive checksum does not match")
    except (OSError, UnicodeError) as error:
        raise BundleError("archive checksum is unreadable") from error
    with tempfile.TemporaryDirectory(prefix="atelier-plugin-verify-") as temporary:
        destination = Path(temporary)
        observed = set()
        total = 0
        try:
            with tarfile.open(archive, mode="r:gz") as package:
                members = package.getmembers()
                if len(members) < 2 or len(members) > MAX_ARCHIVE_MEMBERS:
                    raise BundleError("archive member count is outside its bound")
                for member in members:
                    if member.name == ARCHIVE_ROOT:
                        relative = None
                    elif member.name.startswith(ARCHIVE_ROOT + "/"):
                        relative = member.name[len(ARCHIVE_ROOT) + 1 :]
                        validate_relative_path(relative)
                    else:
                        raise BundleError("archive member is outside the plugin root")
                    folded = unicodedata.normalize("NFC", member.name).casefold()
                    if folded in observed:
                        raise BundleError("archive contains a duplicate or case-folding collision")
                    observed.add(folded)
                    if member.issym() or member.islnk() or not (member.isdir() or member.isfile()):
                        raise BundleError("archive contains a link or special member")
                    if member.uid != 0 or member.gid != 0 or member.mtime != 0:
                        raise BundleError("archive metadata is not reproducible")
                    total += member.size
                    if total > MAX_BUNDLE_BYTES or member.size > MAX_BINARY_BYTES:
                        raise BundleError("archive expanded size exceeds its bound")
                    target = destination / member.name
                    if member.isdir():
                        if member.mode != 0o755:
                            raise BundleError("archive directory mode is invalid")
                        target.mkdir(parents=True, exist_ok=False)
                        target.chmod(0o755)
                    else:
                        if relative is None or member.mode not in (0o644, 0o755):
                            raise BundleError("archive file mode is invalid")
                        source = package.extractfile(member)
                        if source is None:
                            raise BundleError("archive file content is unavailable")
                        target.parent.mkdir(parents=True, exist_ok=True)
                        with target.open("xb") as output:
                            shutil.copyfileobj(source, output, 64 * 1024)
                        target.chmod(member.mode)
        except (OSError, tarfile.TarError) as error:
            raise BundleError("archive is unreadable") from error
        extracted = destination / ARCHIVE_ROOT
        verify_bundle(extracted)


def run_native_smoke_process(command: list[str], expected_code: int, expected_stdout: bytes, expected_stderr: bytes) -> None:
    def limit_output_file_size() -> None:
        if resource is not None:
            resource.setrlimit(resource.RLIMIT_FSIZE, (64 * 1024, 64 * 1024))

    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        try:
            process = subprocess.Popen(command, stdout=stdout, stderr=stderr, start_new_session=True, preexec_fn=limit_output_file_size)
        except OSError as error:
            raise BundleError("trusted native entry smoke could not start") from error
        deadline = time.monotonic() + 10
        failed = False
        while process.poll() is None:
            if time.monotonic() >= deadline or os.fstat(stdout.fileno()).st_size > 64 * 1024 or os.fstat(stderr.fileno()).st_size > 64 * 1024:
                failed = True
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except OSError:
                    process.kill()
                process.wait()
                break
            time.sleep(0.02)
        lingering_group = False
        try:
            os.killpg(process.pid, 0)
            lingering_group = True
        except ProcessLookupError:
            pass
        except PermissionError:
            lingering_group = True
        if lingering_group:
            failed = True
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except OSError:
                pass
        if failed or os.fstat(stdout.fileno()).st_size > 64 * 1024 or os.fstat(stderr.fileno()).st_size > 64 * 1024 or process.returncode != expected_code:
            raise BundleError("trusted native entry smoke failed")
        stdout.seek(0)
        stderr.seek(0)
        if stdout.read() != expected_stdout or stderr.read() != expected_stderr:
            raise BundleError("trusted native entry smoke contract is invalid")


def verify_native_entry(bundle: Path, plugin_version: str) -> None:
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        raise BundleError("bundle assembly requires an Apple Silicon host for the native entry smoke")
    cli = bundle / "bin" / "darwin-universal2" / "codex-game-atelier"
    run_native_smoke_process([str(cli), "--version"], 0, f"codex-game-atelier {plugin_version}\n".encode("utf-8"), b"")
    runner = bundle / "bin" / "darwin-universal2" / "codex-game-atelier-runner"
    run_native_smoke_process([str(runner)], 125, b"", b"codex-game-atelier-runner is an internal fd-only component\n")


def build_bundle(args: argparse.Namespace) -> None:
    output = args.output.resolve(strict=False)
    if output.exists() or output.is_symlink():
        raise BundleError("output path already exists; choose a new directory")
    output.parent.mkdir(parents=True, exist_ok=True)
    output.mkdir(mode=0o755)
    try:
        copy_source_tree(PLUGIN_SOURCE, output)
        copy_regular = ((ROOT / "LICENSE", output / "LICENSE"), (ROOT / "NOTICE", output / "NOTICE"))
        for source, destination in copy_regular:
            regular_file(source, 1024 * 1024)
            shutil.copyfile(source, destination, follow_symlinks=False)
            destination.chmod(0o644)
        sources = {
            "darwin-universal2": (args.darwin_cli, args.darwin_runner),
            "linux-amd64": (args.linux_cli, args.linux_runner),
            "windows-amd64": (args.windows_cli, args.windows_runner),
        }
        for target in TARGETS:
            host = target["host"]
            cli_source, runner_source = sources[host]
            copy_binary(cli_source, output / "bin" / host / target["cli_name"], target["binary_format"])
            copy_binary(runner_source, output / "bin" / host / target["runner_name"], target["binary_format"])
        write_bundle_manifest(output)
        verify_bundle(output)
    except Exception:
        shutil.rmtree(output)
        raise


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    build = commands.add_parser("build", help="assemble and structurally verify a bundle without executing it")
    build.add_argument("--output", type=Path, required=True)
    build.add_argument("--darwin-cli", type=Path, required=True)
    build.add_argument("--darwin-runner", type=Path, required=True)
    build.add_argument("--linux-cli", type=Path, required=True)
    build.add_argument("--linux-runner", type=Path, required=True)
    build.add_argument("--windows-cli", type=Path, required=True)
    build.add_argument("--windows-runner", type=Path, required=True)
    verify = commands.add_parser("verify", help="structurally verify a bundle without executing it")
    verify.add_argument("bundle", type=Path)
    smoke = commands.add_parser("smoke-trusted-bundle", help="execute native entry smoke for a trusted local build only")
    smoke.add_argument("bundle", type=Path)
    archive = commands.add_parser("archive", help="create a reproducible tar.gz and external SHA-256")
    archive.add_argument("--bundle", type=Path, required=True)
    archive.add_argument("--output", type=Path, required=True)
    verify_archive_command = commands.add_parser("verify-archive", help="verify and safely unpack an archive in a temporary directory")
    verify_archive_command.add_argument("archive", type=Path)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        if args.command == "build":
            build_bundle(args)
            print(f"Plugin bundle built and verified: {args.output}")
        elif args.command == "verify":
            bundle = args.bundle.resolve(strict=False)
            verify_bundle(bundle)
            print(f"Plugin bundle structural verification passed: {args.bundle}")
        elif args.command == "smoke-trusted-bundle":
            bundle = args.bundle.resolve(strict=False)
            verify_bundle(bundle)
            verify_native_entry(bundle, read_plugin_manifest(bundle)["version"])
            print(f"Trusted plugin bundle native smoke passed: {args.bundle}")
        elif args.command == "archive":
            create_archive(args.bundle, args.output)
            print(f"Plugin archive built and verified: {args.output}")
        else:
            verify_archive(args.archive)
            print(f"Plugin archive verification passed: {args.archive}")
    except BundleError as error:
        print(f"plugin bundle error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
