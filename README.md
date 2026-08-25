# Codex Game Atelier

Codex Game Atelier is an open-source framework for Codex-native game-development collaboration. The planned v1.0 scope is production-grade Godot support through a Codex Plugin, Starter Template, deterministic CLI, and file-based state/evidence system.

The v1 deterministic CLI production implementation language is Go. The Phase 1 production slices now implement read-only `detect`, `doctor`, `status`, and bounded `clean --list`, atomic `initialize`, recorded static and explicitly authorized Godot Headless validation, plus a fixed GDScript `test` protocol with atomic reports; this is not yet a release-ready CLI. Headless packaging uses a public CLI plus a sibling private runner in one prebuilt bundle. Rust remains a Phase 1 comparison artifact only and is not part of the production or distribution path.

## Current status

Phase 1 contract and vertical-skeleton work. Phase 0 was approved on 2026-08-24. The Go CLI now proves a macOS Apple Silicon/APFS run/evidence transaction, static Godot/GDScript baseline validation, a fixed one-frame Godot Headless reference-game path, and a five-case fixed GDScript test slice with explicit standard `user://` authorization. There is still no v1.0-ready implementation, published package, or remote repository; Linux/Windows native evidence, broader test framework features, broader scene/resource validation, and packaging remain outstanding.

Do not treat this repository as ready for game development until the v1.0 acceptance gates are implemented and verified.

## v1.0 boundary

- Godot only; Unity is deferred beyond v1.0.
- No concrete model IDs in distributed prompts, Skills, Agents, templates, or runtime code.
- No default telemetry, hidden planner, hidden external writes, automatic engine installation, or automatic Git-hook installation.
- Plugin and Starter Template are the primary user entry points; the CLI serves automation, CI, and advanced users.

## Project documents

Start with [docs/project-brief.md](docs/project-brief.md), [docs/architecture.md](docs/architecture.md), [docs/support-matrix.md](docs/support-matrix.md), and [docs/v1-acceptance.md](docs/v1-acceptance.md). ADR 0004 freezes Go as the v1 CLI production language; ADRs 0005-0011 define state/evidence, initial commands, initialization, multi-file commit, Godot Headless user-data authorization, the bounded run scanner, and the fixed GDScript test protocol.

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
