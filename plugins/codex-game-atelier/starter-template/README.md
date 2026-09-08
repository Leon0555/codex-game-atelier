# Codex Game Atelier Godot Starter

This is a clean Godot 4.7.2-stable standard/GDScript starting point with a small complete game called **Atelier Spark**. Click the button or press Space/Enter to collect five sparks, reach the win state, then reset and play again. The slice demonstrates input, signals, localized resources, UI, deterministic state, tests, and an unsigned macOS technical export without external assets or dependencies.

It contains no generated Atelier identity, run evidence, Godot cache, export output, engine, SDK, dependency, telemetry, Git hook, or account configuration. Change the sample name and bundle identifier before using the project for real distribution.

## Start in three steps

This three-step count starts after the Codex Game Atelier Plugin is installed and Godot 4.7.2-stable is available. The template intentionally does not embed the Plugin, Skill, or platform CLI binaries. Plugin installation/discovery is not yet a completed Phase 1 acceptance result; this directory is a candidate template, not a released product.

1. Copy this directory and rename the copy for your game.
2. Open the copy in Codex with the Codex Game Atelier Plugin, then ask Codex to initialize the project.
3. Ask Codex to run validation and the six fixed GDScript tests, then open `project.godot` in Godot 4.7.2-stable and play Atelier Spark.

Godot is an explicit prerequisite and is never installed automatically. The current Phase 1 workflow is natively verified only on macOS Apple Silicon. Linux x64 and Windows x64 artifacts are not yet approved for execution.

The fixed test entry is `res://tests/atelier_test_runner.gd`. Treat it as trusted project code. Headless validation and tests require explicit permission for Godot's standard `user://` writes. The included `macOS Technical` preset is unsigned and not notarized; it is for the supported technical export workflow, not public distribution.

Do not copy the Atelier source repository's `AGENTS.md` into a game project. This template intentionally contains no mandatory internal development workflow.

## Generated local state

Initialization creates `.gameatelier/`; Godot creates `.godot/`. This template ignores Godot caches and build output, but deliberately leaves `.gameatelier/` visible until the version-control policy for project state and evidence is frozen. Review generated files before committing them.
