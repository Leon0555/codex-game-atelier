# Codex Game Atelier

Codex Game Atelier is an open-source framework for Codex-native game-development collaboration. The planned v1.0 scope is production-grade Godot support distributed through one Codex Plugin containing the Starter Template, deterministic Go CLI, and file-based state/evidence system.

The v1 deterministic CLI production implementation language is Go. The Phase 1 production slices now implement read-only `detect`, `doctor`, `status`, bounded `clean --list`, zero-free-text committed-run `logs`, and three-mode `release check`, atomic `initialize`, recorded static and explicitly authorized Godot Headless validation, a fixed GDScript `test` protocol, and the shared `build`/`export` macOS technical-export pipeline with isolated project snapshots, verified Universal 2 ZIP evidence, and automatic Apple Silicon target smoke; this is not yet a release-ready CLI. A local Plugin candidate packages a public CLI plus sibling private runner for Universal 2, Linux amd64, and Windows amd64 without requiring a source build, with format checks, deterministic archive assembly, and external checksum verification. Rust remains a Phase 1 comparison artifact only and is not part of the production or distribution path.

## Current status

Phase 1 follows a three-milestone plan. M1 is complete locally on macOS Apple Silicon: the Go CLI now closes a special-path `detect → doctor → initialize → validate → test → Debug/Release export → target smoke` flow, and the deterministic Starter Template contains the playable six-test Atelier Spark vertical slice. M2 has locally implemented logical capability Profiles, one real file-recovered native collaboration trace with separate read-only audit and repair, the monotonic gate-policy contract, build/export policy consumption, read-only three-mode `release check`, an explicitly managed optional pre-commit hook, and a minimum macOS Apple Silicon CI workflow. M3 has a historical closed local candidate and has completed one minimal real local Codex lifecycle: dedicated marketplace registration, Plugin installation, new-task Skill discovery, bundled CLI invocation, uninstall, and cleanup all passed. The next candidate migrates to one Plugin archive containing Starter and CLI/runner. Remote Plugin installation without Gatekeeper intervention, the unresolved Windows/Linux Tier 1 promise, required hosted CI, real upgrade/rollback, and final independent audit remain open. No remote repository has been created or pushed, and there is still no v1.0-ready or published package.

Do not treat this repository as ready for game development until the v1.0 acceptance gates are implemented and verified.

## v1.0 boundary

- Godot only; Unity is deferred beyond v1.0.
- No concrete model IDs in distributed prompts, Skills, Agents, templates, or runtime code.
- No default telemetry, hidden planner, hidden external writes, automatic engine installation, or automatic Git-hook installation.
- The Codex Plugin is the only v1 user installation entry; it contains the Starter Template and prebuilt CLI/runner. Standalone CLI archives, npm packages, DMGs, and PKGs are not v1 distribution channels.

## Project documents

Start with [docs/project-brief.md](docs/project-brief.md), [docs/architecture.md](docs/architecture.md), [docs/support-matrix.md](docs/support-matrix.md), and [docs/v1-acceptance.md](docs/v1-acceptance.md). ADR 0004 freezes Go as the v1 CLI production language; ADRs 0005-0023 define state/evidence, initial commands, Godot workflows, reproducible packaging, release gates, and the Plugin-only v1 distribution decision.

## License

Project code is MIT. See [LICENSE](LICENSE), [NOTICE](NOTICE), and [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES) for prebuilt Go binary notices.
