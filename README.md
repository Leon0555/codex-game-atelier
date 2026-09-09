# Codex Game Atelier

Codex Game Atelier is an open-source framework for Codex-native game-development collaboration. v1.0 provides production-grade Godot support through one Codex Plugin containing the Starter Template, deterministic Go CLI, and file-based state/evidence system.

The v1 deterministic CLI production implementation language is Go. The Phase 1 production slices implement verified `starter create`, read-only `detect`, `doctor`, `status`, bounded `clean --list`, zero-free-text committed-run `logs`, three-mode `release check`, atomic `initialize`, recorded static and explicitly authorized Godot Headless validation, a fixed GDScript `test` protocol, and the shared `build`/`export` macOS technical-export pipeline with isolated project snapshots, verified Universal 2 ZIP evidence, and automatic Apple Silicon target smoke. v1 production support is limited to macOS Apple Silicon. The Plugin still contains Linux amd64 and Windows amd64 cross-build artifacts for future validation, but they are unsupported in v1 and are not release gates. Rust remains a Phase 1 comparison artifact only and is not part of the production or distribution path.

## Current status

Phase 1's three milestones are complete. M1 closed the macOS Apple Silicon Godot workflow and playable Atelier Spark vertical slice. M2 delivered logical capability Profiles, recoverable native collaboration, monotonic workflow gates, an optional explicit hook, and required hosted CI. M3 rejected rc.2 through rc.5, used the audited `0.3.0-rc.6` as the lifecycle baseline, then rebuilt and verified `1.0.0`. The stable release passed reproducible packaging, isolated remote Plugin installation, a real user-level lifecycle with exact state restoration, new-task Skill discovery, special-path Godot E2E, required CI, strict 12/12, and an independent final read-only audit at 0 Blocker/High/Medium/Low. The public `v1.0.0` tag is the immutable Plugin-only installation ref. There is no standalone v1.0 package.

## Install

Prerequisite: Godot `4.7.2-stable` standard/GDScript on macOS Apple Silicon.

1. `codex plugin marketplace add Leon0555/codex-game-atelier --ref v1.0.0 --sparse .agents --sparse plugins/codex-game-atelier --json`
2. `codex plugin add codex-game-atelier@codex-game-atelier --json`
3. Start a new Codex task and ask Codex Game Atelier to create a Starter project or inspect an existing Godot project.

The Plugin includes its CLI, private runner, and Starter Template. No source clone, npm build, Go toolchain, separate binary download, signing bypass, or Apple notarization step is required.

## v1.0 boundary

- Godot only; Unity is deferred beyond v1.0.
- No concrete model IDs in distributed prompts, Skills, Agents, templates, or runtime code.
- No default telemetry, hidden planner, hidden external writes, automatic engine installation, or automatic Git-hook installation.
- The Codex Plugin is the only v1 user installation entry; it contains the Starter Template and prebuilt CLI/runner. Standalone CLI archives, npm packages, DMGs, and PKGs are not v1 distribution channels.

## Project documents

Start with [docs/project-brief.md](docs/project-brief.md), [docs/architecture.md](docs/architecture.md), [docs/support-matrix.md](docs/support-matrix.md), and [docs/v1-acceptance.md](docs/v1-acceptance.md). ADR 0004 freezes Go as the v1 CLI production language; ADRs 0005-0028 define state/evidence, initial commands, Godot workflows, reproducible packaging, release gates, Plugin-only v1 distribution, embedded Starter creation, the macOS-only v1 support scope, bound external release evidence, single-machine isolated final validation, and the `1.0.0` stable-version promotion.

## License

Project code is MIT. See [LICENSE](LICENSE), [NOTICE](NOTICE), and [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES) for prebuilt Go binary notices.

## Security

See [SECURITY.md](SECURITY.md). Private Vulnerability Reporting is enabled and verified; do not post sensitive vulnerability details in a public issue.
