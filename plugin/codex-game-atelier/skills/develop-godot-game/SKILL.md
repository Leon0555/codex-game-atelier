---
name: develop-godot-game
description: Inspect a supported Godot project or explicitly initialize its Codex Game Atelier state through the Phase 1 CLI. Use for detect, doctor, initialize, or status; validate, test, build, export, and evidence persistence are not implemented yet.
---

# Develop Godot Game

Use Codex for judgment and the available Atelier CLI for deterministic `detect`, `doctor`, `initialize`, and `status` operations.

- Resolve the user's project path before invoking the CLI. `detect`, `doctor`, and `status` are read-only.
- Invoke `initialize` only when the user explicitly asks to initialize Atelier state. It is currently enabled only on verified macOS Apple Silicon; Linux x64 and Windows x64 return `INITIALIZE_HOST_NOT_VERIFIED` pending native transaction validation. Explain that a successful first initialization creates `.gameatelier/project.json` and a persistent advisory-lock file, but does not modify `project.godot`, install Godot, or run the engine.
- Never use `initialize` as a repair, migration, force, or overwrite path. If existing state is invalid or unsafe, report the structured failure and preserve it.
- Treat structured results as authoritative for command status. Every current result must contain an empty `evidence` array; atomic state initialization does not mean a persisted evidence chain exists.
- If the user requests validate, test, build, export, logs, clean, or release, report that the operation is not implemented in this source skeleton.
- Do not turn arbitrary shell or script evaluation into a substitute workflow.
- Never install Godot or large dependencies, install Git hooks, log in, enable telemetry, or publish without the user's explicit approval for that action.
- When bounded parallel work materially helps, assign non-overlapping ownership and keep review independent and read-only. Continue serially when native subagents are unavailable.
- Do not fabricate task persistence, handoff recovery, gates, or background services that this slice does not implement.
- If the CLI executable is unavailable, stop the affected operation. A Godot executable is required by `doctor`, but not by `initialize`; do not fabricate fallback success for either case.
