# Codex Game Atelier Godot Starter

This is a clean Godot 4.7.2-stable standard/GDScript starting point. It contains no generated Atelier identity, run evidence, Godot cache, export output, engine, SDK, dependency, telemetry, Git hook, or account configuration.

## Start in three steps

This three-step count starts after the Codex Game Atelier Plugin is installed and Godot 4.7.2-stable is available. Plugin installation/discovery is not yet a completed Phase 1 acceptance result; this directory is a candidate template, not a released product.

1. Copy this directory and rename the copy for your game.
2. Open the copy in Codex with the Codex Game Atelier Plugin, then ask Codex to initialize the project.
3. Ask Codex to run the supported validation and fixed GDScript tests, then open `project.godot` in Godot 4.7.2-stable.

Godot is an explicit prerequisite and is never installed automatically. The current Phase 1 workflow is natively verified only on macOS Apple Silicon. Linux x64 and Windows x64 artifacts are not yet approved for execution.

The fixed test entry is `res://tests/atelier_test_runner.gd`. Treat it as trusted project code. Headless validation and tests require explicit permission for Godot's standard `user://` writes.

Do not copy the Atelier source repository's `AGENTS.md` into a game project. This template intentionally contains no mandatory internal development workflow.

## Generated local state

Initialization creates `.gameatelier/`; Godot creates `.godot/`. This template ignores Godot caches and build output, but deliberately leaves `.gameatelier/` visible until the version-control policy for project state and evidence is frozen. Review generated files before committing them.
