# Bounded Native Collaboration

Use Codex's native delegation only when a task has independent work that benefits from another agent. A direct single-agent workflow remains valid. This reference does not create a planner, background service, or new authority.

## Boundaries

- Run no more than three delegated agents concurrently. The coordinating agent remains responsible for scope, synthesis, and user communication.
- Give every delegated task a bounded scope, acceptance conditions, required inputs, expected output, and a logical capability profile from `capability-profiles.json`.
- Assign exactly one active write owner to every writable path. Parallel tasks may write only to disjoint path sets. Read-only agents receive no write-owned paths.
- A delegation inherits only the permissions already granted for its task. Destructive work, installation, external publication, account operations, and scope expansion still require the user's approval.
- Use native Codex agent/thread facilities. Do not start a daemon, hidden model loop, or ad hoc prompt runner. Do not persist a concrete model identifier in distributed state.

If native delegation is unavailable, execute the same task serially, record the degraded mode in the task or handoff, and preserve the same ownership and acceptance rules.

## Establish the write owner

Before work starts, persist a task file under `.gameatelier/tasks/` using the task contract. It must identify the task, scope, acceptance conditions, current owner, allowed paths, status, dependencies, evidence references, and any handoff reference needed for recovery.

Check all active task files before assigning writes. Compare ownership conservatively from the project root:

1. Require a safe project-relative path with `/` separators and reject absolute paths, empty segments, `.`, `..`, backslashes, control characters, and paths outside the project root.
2. Inspect every existing path component without following links. If any component is a symbolic link, return `BLOCKED`; do not use link resolution to claim a distinct write scope. Also block existing paths that identify the same filesystem object.
3. Normalize every segment to Unicode NFC, case-fold it, and normalize the folded value to NFC again for the comparison key. If distinct spellings produce the same key, treat them as the same path.
4. Compare keys by complete segments. Two claims conflict when their keys are equal or either key is an ancestor of the other. A directory claim therefore owns its entire subtree.

The following pairs are negative examples and must return `BLOCKED`:

- `game` and `game/scenes/player.gd` (ancestor/descendant overlap).
- `Scripts/Player.gd` and `scripts/player.gd` (case-folding collision).
- `art/café.png` and `art/café.png` (Unicode-normalization collision).
- `linked/player.gd` and `src/player.gd` when `linked` is a symbolic link to `src` (symbolic-link alias).

A path may belong to only one active write owner after these checks. An ownership transfer is complete only after the previous owner has stopped writing and a handoff names the new owner. If ownership is missing, overlapping, stale, aliased, or cannot be resolved within a bounded project-root walk, return `BLOCKED`; do not guess or silently take the path.

The implementation owner may modify only the authorized paths and must preserve unrelated user changes. Explorers, testers, and reviewers stay read-only unless a later persisted task explicitly grants them a disjoint write scope.

## Independent audit

Use the `independent-audit` profile in a genuinely separate read-only context for architecture, security, compatibility, or release review. The auditor reads the task, diff, tests, handoff, and referenced evidence, then reports findings and evidence references without editing the reviewed paths.

The implementation owner cannot approve their own work. The auditor cannot fix a finding and approve that fix in the same audit step. The owner performs any authorized repair, records new evidence, and a fresh independent read-only audit reviews the resulting state. If an independent context or required capability is unavailable, record `BLOCKED` instead of weakening the gate.

## Handoff and recovery

Before ownership changes or the current context stops, write a handoff under `.gameatelier/handoffs/`. Include the task ID, previous and next owner, completed work, remaining work, blockers, and evidence references. Record decisions needed to continue, but do not store hidden reasoning, credentials, or raw logs in the handoff.

Anchor recovery contracts at `../../schemas/v1/` relative to the installed `develop-godot-game` Skill directory. Before interpreting recovery state, validate the task with `task.schema.json`, the handoff with `handoff.schema.json`, and every evidence record with `evidence.schema.json`; resolve their `common.schema.json` and `error.schema.json` references only from that same packaged directory. If the contract directory or any required record is missing or invalid, return `BLOCKED` before assigning ownership or following an evidence path.

A fresh agent resumes without relying on conversation history:

1. Read the task file and confirm its ID, scope, acceptance conditions, status, owner, and allowed paths.
2. Read the referenced handoff and verify that its task and destination owner match the task state.
3. Read every referenced evidence record under `.gameatelier/evidence/`; follow only validated project-relative artifact or log paths.
4. Inspect the current files and repository state, then reconcile them with completed work, remaining work, blockers, and evidence. Existing files are not proof that an acceptance condition passed.
5. If required files are missing, inconsistent, unsafe, or insufficient to establish ownership, record the discrepancy and return `BLOCKED`. Otherwise continue only the persisted remaining work.

Task, handoff, and evidence files are the recovery source of truth. Conversation summaries may help explain them but cannot replace missing durable state.

## Evidence and completion

Keep large logs and artifacts in evidence rather than agent messages. Evidence should record the actual operation or command, environment and version, exit status, relevant bounded paths or digests, outcome, and uncovered risk. Distinguish `PASS`, `FAIL`, `BLOCKED`, `SKIPPED`, and `NOT_RUN`; only actual execution can produce `PASS`.

The coordinating agent may mark the task complete only after acceptance conditions are satisfied by referenced evidence and any required independent audit has passed. Final reporting must state changed paths, validations run, validations not run, remaining risks, and decisions still requiring the user.
