# Codex Game Atelier

Codex Game Atelier is an open-source framework for Codex-native game-development collaboration. The planned v1.0 scope is production-grade Godot support through a Codex Plugin, Starter Template, deterministic CLI, and file-based state/evidence system.

The v1 deterministic CLI production implementation language is Go. The first production vertical slice now implements read-only `detect`, `doctor`, and `status`; it is not yet a release-ready CLI. Rust remains a Phase 1 comparison artifact only and is not part of the production or distribution path.

## Current status

Phase 1 contract and vertical-skeleton work. Phase 0 was approved on 2026-08-24. The Go CLI currently implements read-only detection/doctor/status plus atomic project-state initialization, and the Godot reference fixture exists. There is still no v1.0-ready implementation, published package, commit, or remote repository; the Support Matrix target is approved but lacks full production evidence.

Do not treat this repository as ready for game development until the v1.0 acceptance gates are implemented and verified.

## v1.0 boundary

- Godot only; Unity is deferred beyond v1.0.
- No concrete model IDs in distributed prompts, Skills, Agents, templates, or runtime code.
- No default telemetry, hidden planner, hidden external writes, automatic engine installation, or automatic Git-hook installation.
- Plugin and Starter Template are the primary user entry points; the CLI serves automation, CI, and advanced users.

## Project documents

Start with [docs/project-brief.md](docs/project-brief.md), [docs/architecture.md](docs/architecture.md), [docs/support-matrix.md](docs/support-matrix.md), and [docs/v1-acceptance.md](docs/v1-acceptance.md). ADR 0004 freezes Go as the v1 CLI production language, ADR 0005 defines the proposed state/evidence contract, and ADR 0006 records the first production command slice.

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
