---
name: develop-godot-game
description: Inspect, initialize, validate, test, or read verified run diagnostics for a supported Godot/GDScript project through the bundled Codex Game Atelier CLI. Use for Atelier project readiness and recorded Godot workflows; build, export, release, arbitrary scripts, and raw log access are not implemented.
---

# Develop Godot Game

Use Codex for judgment and the bundled Atelier CLI for deterministic project operations. Structured CLI results are authoritative; do not infer success from prose or file presence.

## Bundled CLI

Anchor paths at this installed Skill directory; do not search for a source checkout, Go toolchain, npm package, or same-named executable on `PATH`.

- macOS Apple Silicon: `../../bin/darwin-universal2/codex-game-atelier`
- Linux x64 artifact inventory only: `../../bin/linux-amd64/codex-game-atelier`
- Windows x64 artifact inventory only: `..\\..\\bin\\windows-amd64\\codex-game-atelier.exe`

In Phase 1, execute this Skill only on macOS Apple Silicon. Reject every other runtime host as unsupported, including Linux x64 and Windows x64: their files are cross-build artifact evidence and must not be executed or presented as native support until separate validation is recorded. The public macOS CLI and its sibling `codex-game-atelier-runner` must both exist as regular files in the selected directory before `validate --headless` or `test`.

## Operations

- Resolve the user's Godot project path before invoking the CLI. `detect`, `doctor`, `status`, `logs --run-id`, and `clean --list` are read-only. `clean --list` is only a preview and never authorizes deletion.
- Invoke `initialize` only when the user explicitly asks to initialize Atelier state. A successful first run creates `.gameatelier/project.json` and a persistent advisory-lock file; it does not modify `project.godot`, install Godot, or run the engine. Never use it as repair, migration, force, or overwrite.
- `validate` records an immutable run even for the default static baseline. Use `--headless` only after the user explicitly authorizes Godot's standard `user://` writes. Do not add arbitrary Godot arguments.
- `test` runs only `res://tests/atelier_test_runner.gd` and executes trusted project GDScript. Confirm the project is owned or reviewed and obtain explicit authorization for standard `user://` writes before passing `--allow-engine-user-data`. Do not substitute another script, filter, shell command, or eval path.
- `logs --run-id <strict-id>` returns a zero-free-text structural projection of one verified committed validate/test run. It does not return source IDs, error text, report summaries, payload paths, or raw stdout/stderr.
- Treat `BLOCKED`, `FAIL`, nonzero exit codes, missing evidence, and unsafe state exactly as reported. Preserve incomplete/corrupt runs and failure evidence.

Build, export, release, recovery, deletion, raw log capture, arbitrary code execution, Git-hook installation, dependency installation, telemetry, login, and external publication are outside this Skill. Do not fabricate them or replace them with ad hoc shell workflows. If a bundled executable is unavailable, stop the affected operation and report the missing artifact.
