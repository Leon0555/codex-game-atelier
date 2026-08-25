# CLI runtime comparison spike

This directory is disposable Phase 1 research. It is not the production CLI and must not be packaged or published.

Both implementations expose:

- `--version`
- `doctor --godot <executable> --project <directory> [--timeout-ms <milliseconds>]`
- JSON matching the Phase 1 `command-result` shape
- exit `0` for PASS, `2` for usage, `4` for a missing prerequisite, `5` for engine failure, and `6` for timeout

The comparison records build time, release binary size, cold/warm startup, process control, Unicode/space paths, dependency tree, and resulting package behavior. The samples intentionally accept an explicit Godot path for test isolation; that is not a frozen public CLI design.

The current samples validate bounded output and same-Unix-process-group cleanup only. Windows Job Objects and descendants that deliberately leave the process group remain future work. Atomic evidence-file writing from the approved spike plan is still `NOT RUN`; an empty `evidence` array is not a passing evidence implementation.
