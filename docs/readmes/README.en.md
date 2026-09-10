# Codex Game Atelier

Design and create games with Codex, with help to build, test, and export them—through a process you can inspect and evidence you can verify. Currently supports Godot.

[简体中文](../../README.md) | **English**

[![CI](https://github.com/Leon0555/codex-game-atelier/actions/workflows/ci.yml/badge.svg)](https://github.com/Leon0555/codex-game-atelier/actions/workflows/ci.yml)
[![Version](https://img.shields.io/badge/version-v1.0.0-2f81f7)](https://github.com/Leon0555/codex-game-atelier/tree/v1.0.0)
[![Godot](https://img.shields.io/badge/Godot-4.7.2--stable-478cbf?logo=godot-engine&logoColor=white)](../support-matrix.md)
[![License](https://img.shields.io/badge/license-MIT-green)](../../LICENSE)

Codex Game Atelier is an open-source, Codex-native collaboration framework for Godot development. Codex handles planning, judgment, implementation, and bounded collaboration; the bundled Go CLI handles deterministic project operations, workflow gates, structured results, and durable evidence.

v1.0 is intentionally compact: one Codex Plugin, one top-level Skill, one embedded Starter project, and one verified Godot workflow. Users do not need to clone this repository, compile a CLI, install Node.js, or learn a large command surface.

## Contents

- [Why it exists](#why-it-exists)
- [Quick start](#quick-start)
- [What is included](#what-is-included)
- [How it works](#how-it-works)
- [Verified workflow](#verified-workflow)
- [Support matrix](#support-matrix)
- [Safety and product boundaries](#safety-and-product-boundaries)
- [Workflow modes](#workflow-modes)
- [Uninstall](#uninstall)
- [Repository map](#repository-map)
- [Project status](#project-status)
- [Documentation](#documentation)
- [Inspiration and provenance](#inspiration-and-provenance)
- [License](#license)

## Why it exists

A useful game-development assistant needs more than a collection of prompts. It must be able to prove which engine ran, which tests passed, what was exported, where evidence was recorded, and what remains unsupported.

Codex Game Atelier provides that operational layer without becoming a hosted project manager or a background multi-agent service:

- Codex keeps creative and engineering decisions visible to the user.
- Native subagents are used only for bounded work with explicit ownership and read-only review boundaries.
- The CLI performs defined Godot and file operations and returns structured JSON with stable exit codes.
- Project state and run evidence remain inspectable under `.gameatelier/`.
- Build, export, and release checks enforce their own gates instead of relying on an optional Git hook.

## Quick start

### Prerequisites

- macOS on Apple Silicon.
- Godot `4.7.2-stable`, standard build with GDScript support.
- Codex with Plugin support.

### 1. Add the versioned Marketplace source

```sh
codex plugin marketplace add Leon0555/codex-game-atelier \
  --ref v1.0.0 \
  --sparse .agents \
  --sparse plugins/codex-game-atelier \
  --json
```

### 2. Install the Plugin

```sh
codex plugin add codex-game-atelier@codex-game-atelier --json
```

These two commands update user-level Codex Plugin state under `~/.codex`. They do not install Godot, change system settings, add Git hooks, or modify a game project.

### 3. Start a new Codex task

Ask Codex Game Atelier to create a project:

```text
Use Codex Game Atelier to create a new Godot Starter project at
/absolute/path/My Atelier Game, then initialize and inspect it.
```

Or bring an existing project:

```text
Use Codex Game Atelier to inspect and run doctor on the Godot project at
/absolute/path/My Existing Game.
```

The Plugin's `develop-godot-game` Skill locates its own bundled CLI, runner, schemas, and Starter Template. It does not search for a source checkout or a same-named executable on `PATH`.

## What is included

| Component | Purpose |
| --- | --- |
| `develop-godot-game` Skill | Routes project creation, inspection, validation, tests, build/export, evidence reading, and release checks |
| Deterministic Go CLI | Executes bounded project and Godot operations with structured JSON and stable exit codes |
| Private runner | Performs fixed internal Godot checks; it is not a second user-facing command |
| Atelier Spark Starter | Small playable Godot/GDScript vertical slice covering input, signals, resources, UI, win, and reset flows |
| File-based state and evidence | Records project policy and immutable run closures under `.gameatelier/` |
| Logical capability profiles | Expresses responsibility and capability without embedding concrete model IDs |
| Workflow gates | Provides monotonic `manual`, `standard`, and `strict` modes, plus required CI and optional Git-hook feedback |

## How it works

```text
User request
    |
    v
Codex + develop-godot-game Skill
    |-- judgment, planning, bounded collaboration, review
    |
    v
Bundled deterministic CLI
    |-- safety and workflow gates
    |-- structured results and exit codes
    |-- .gameatelier state and evidence
    |
    v
Godot 4.7.2 standard/GDScript
    |-- headless validation and fixed tests
    |-- Debug/Release technical export
    `-- Apple Silicon target smoke
```

The CLI never chooses a model or acts as a hidden planner. External publication, arbitrary scripts, raw-log access, dependency installation, signing, and notarization are outside the v1 Skill.

## Verified workflow

The v1.0 release verifies the following path on macOS Apple Silicon:

1. Create the embedded Starter in a new directory, or inspect an existing Godot/GDScript project.
2. Detect the project and supported Godot installation.
3. Run `doctor`, including matching export-template checks when export readiness is requested.
4. Initialize `.gameatelier/project.json` without overwriting valid existing state.
5. Run static or explicitly authorized Godot Headless validation.
6. Run the fixed GDScript test protocol.
7. Produce Debug or Release macOS technical exports with manifests and hashes.
8. Verify Universal 2 binary slices and perform an Apple Silicon startup/exit smoke.
9. Aggregate already-recorded facts with a read-only release check.

Failures remain distinguishable as `FAIL`, `BLOCKED`, `SKIPPED`, or `NOT RUN`; missing evidence is never treated as success.

## Support matrix

| Area | v1.0 commitment |
| --- | --- |
| Development host | macOS Apple Silicon |
| Engine | Godot `4.7.2-stable` standard build |
| Project language | GDScript |
| Headless validation | Supported and release-verified |
| Game export | Unsigned, unnotarized macOS technical ZIP |
| Export architecture | Universal 2 generated; runtime smoke verified only on Apple Silicon |
| Windows/Linux CLI artifacts | Cross-build inventory only; not native support |

The full version, host, export, upgrade, and retirement policy is in the [Godot v1.0 Support Matrix](../support-matrix.md).

## Safety and product boundaries

Codex Game Atelier does not:

- install Godot, SDKs, or large dependencies automatically;
- enable telemetry or hidden external writes;
- install Git hooks unless the user explicitly requests it;
- expose arbitrary shell, script, or engine-eval execution as a core workflow;
- hard-code concrete model IDs in distributed prompts, Skills, templates, or runtime code;
- claim native Windows, Linux, macOS Intel, Godot .NET, mobile, web, or console support in v1.0;
- ship a standalone CLI archive, npm package, DMG, or PKG in v1.0;
- require Apple signing or notarization for the Plugin-only installation path.

Headless validation and tests can execute GDScript from the selected project. Use them only with a project you own or have reviewed; v1.0 does not claim to sandbox project code.

## Workflow modes

| Mode | Intended use | Behavior |
| --- | --- | --- |
| `manual` | Advanced, explicit control | Keeps mandatory safety checks while omitting only documented workflow expansions |
| `standard` | Normal development | Automatically runs required Headless and fixed-test gates before build/export |
| `strict` | Release preparation | Requires the complete verified release evidence set; missing facts remain explicit blockers |

Git hooks are optional convenience feedback. CLI command gates and required CI remain authoritative even when no hook is installed.

## Uninstall

```sh
codex plugin remove codex-game-atelier@codex-game-atelier --json
codex plugin marketplace remove codex-game-atelier --json
```

Uninstalling removes the Codex Plugin and its Marketplace registration. It does not delete user-created Godot projects.

## Repository map

```text
plugin/codex-game-atelier/   Plugin and top-level Skill source
packages/cli/                Go CLI and private runner
starter-template/            Atelier Spark embedded Starter source
schemas/                     Versioned state, result, and evidence contracts
examples/                    Reference game material
tools/                       Packaging and validation tools
docs/                        Architecture, ADRs, support policy, and evidence
```

The repository-level `AGENTS.md` governs development of Codex Game Atelier only. It is deliberately excluded from the Plugin, Starter Template, and generated game projects.

## Project status

`v1.0.0` is the current stable Plugin-only release. It passed reproducible packaging, isolated remote installation, install/upgrade/rollback/uninstall lifecycle checks, special-path Godot end-to-end validation, required CI, strict release aggregation, and an independent final read-only audit with no remaining findings.

The project is now in post-release maintenance. New engines, native platforms, export targets, runtimes, or distribution channels require a separate scope decision and new evidence.

## Documentation

| Need | Start here |
| --- | --- |
| Product goal and boundaries | [Project brief](../project-brief.md) |
| System design | [Architecture](../architecture.md) |
| Supported Godot and platform matrix | [Support Matrix](../support-matrix.md) |
| Release acceptance evidence | [v1 Acceptance](../v1-acceptance.md) |
| Decisions and tradeoffs | [Architecture Decision Records](../adr/) |
| Sources and clean-room record | [Provenance](../provenance.md) |
| Security reports | [Security policy](../../SECURITY.md) |

## Inspiration and provenance

Early design research examined [merlinhu1/codex-game-studio](https://github.com/merlinhu1/codex-game-studio) and [Donchitos/Claude-Code-Game-Studios](https://github.com/Donchitos/Claude-Code-Game-Studios). Codex Game Atelier independently reworked the relevant ideas around a smaller Plugin-only surface, native bounded collaboration, deterministic Godot operations, and evidence-backed release gates.

No source code, prompt collection, template, or substantial documentation from those repositories is included. See [provenance.md](../provenance.md) for the audited commits, licenses, researched ideas, and clean-room distinctions.

## License

Codex Game Atelier is released under the [MIT License](../../LICENSE). See [NOTICE](../../NOTICE) and [THIRD_PARTY_NOTICES](../../THIRD_PARTY_NOTICES) for attribution and the notices carried with prebuilt Go binaries.

Security-sensitive reports should use GitHub Private Vulnerability Reporting as described in [SECURITY.md](../../SECURITY.md), not a public issue.
