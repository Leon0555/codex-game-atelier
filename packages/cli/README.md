# Codex Game Atelier CLI

This directory contains the Go production implementation of the deterministic Codex Game Atelier CLI. It is an early Phase 1 vertical slice, not a v1.0 release artifact.

Implemented commands:

- `detect`: read-only project, Godot candidate, and host discovery; it does not start Godot.
- `doctor`: read-only prerequisite checks plus the fixed external call `Godot --version`.
- `initialize`: on the currently verified macOS Apple Silicon host, atomically creates the first strict `.gameatelier/project.json` for an existing supported Godot/GDScript project; valid reruns are byte-for-byte no-ops. Linux x64 and Windows x64 return `INITIALIZE_HOST_NOT_VERIFIED` until their native transaction matrices pass.
- `status`: strict read-only parsing of `.gameatelier/project.json`.

All subcommands emit one structured JSON result to stdout. `initialize` is the only current write command; it uses an OS-released advisory lock and no-replace publication, writes no evidence, and never modifies `project.godot`. Atomic multi-file evidence, full headless validation, packaging, and target-host release verification remain outside this slice.

Maintainer verification with the repository-local Go toolchain:

```sh
GOCACHE="$PWD/../../.tools/go-cache" \
GOMODCACHE="$PWD/../../.tools/go-mod-cache" \
../../.tools/go/1.27.0/bin/go test ./...
```

End users will receive a prebuilt artifact and will not need Go or a source build.
