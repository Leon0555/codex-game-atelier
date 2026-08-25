# Codex Game Atelier CLI

This directory contains the Go production implementation of the deterministic Codex Game Atelier CLI. It is an early Phase 1 vertical slice, not a v1.0 release artifact.

Implemented commands:

- `clean --list`: strictly scans at most 512 run directories, 2,048 closure files, and 256 MiB without writing; it cooperatively honors caller cancellation between 64 KiB read chunks, lists only incomplete/orphan cleanup previews, and protects corrupt closures. It does not delete, recover, repair, or create locks.
- `detect`: read-only project, Godot candidate, and host discovery; it does not start Godot.
- `doctor`: read-only prerequisite checks plus the fixed external call `Godot --version`.
- `initialize`: on the currently verified macOS Apple Silicon host, atomically creates the first strict `.gameatelier/project.json` for an existing supported Godot/GDScript project; valid reruns are byte-for-byte no-ops. Linux x64 and Windows x64 return `INITIALIZE_HOST_NOT_VERIFIED` until their native transaction matrices pass.
- `status`: strict read-only parsing of `.gameatelier/project.json`.
- `validate`: records the static baseline by default. With `--headless`, it requires explicit `--allow-engine-user-data`, requires the selected executable to self-report the accepted Godot 4.7.2 standard-build identifier, and runs the fixed main-scene one-frame check from pinned project and engine identities with bounded output, timeout/cancellation, and recorded evidence.

All subcommands emit one structured JSON result to stdout. `clean --list` is zero-write and its candidates are previews, not deletion authorization: an active run can be observed as orphan or incomplete. Future deletion is forbidden until writer, cleaner, and recovery share one per-run coordination protocol; targets must then be locked and revalidated. `initialize` writes only project state. `validate` creates a self-contained immutable run whose `result.json` is published last and references a hashed validation-report payload; incomplete/orphan runs never count as PASS. Headless does not accept arbitrary Godot arguments. It pins the paired private runner and selected engine as open source descriptors, then creates separate runner/engine snapshots for version and scene; the project directory is inherited as a pinned descriptor. Every transient snapshot must be removed before result commit, otherwise the run remains incomplete. The authorized standard `user://` side effect is recorded symbolically without persisting the user's absolute path. Version text is a process observation, not proof of binary provenance; repository-local official binary identity is verified separately during maintainer setup. Full GDScript test suites, recovery/deletion, packaging, and target-host release verification remain outside this slice.

`validate --headless` executes the selected project's main-scene GDScript. Use it only for a project you own or have reviewed. This Phase 1 slice does not sandbox network or absolute-path operations initiated by project code, so it does not claim that an arbitrary project has no external side effects.

Maintainer verification with the repository-local Go toolchain:

```sh
GOCACHE="$PWD/../../.tools/go-cache" \
GOMODCACHE="$PWD/../../.tools/go-mod-cache" \
../../.tools/go/1.27.0/bin/go test ./...
```

End users will receive a prebuilt artifact bundle containing the public `codex-game-atelier` CLI and its sibling private `codex-game-atelier-runner`; they will not need Go or a source build. The runner is an internal component, not a second user-facing command.
