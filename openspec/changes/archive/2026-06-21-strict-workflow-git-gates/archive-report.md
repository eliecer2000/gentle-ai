# Archive Report — strict-workflow-git-gates

**Archived:** 2026-06-21
**Status:** Delivered and merged into `feat/strict-workflow-mode`.

## Outcome

Moved the git workflow strategy from prompt-only skills into **deterministic enforcement** via a Claude `PreToolUse(Bash)` hook calling a new `gentle-ai git-gate check` subcommand, with a 3-state-per-gate model (`enforce` / `warn` / `off`) and an **informed-override** (break-glass) escape hatch that never changes the flow silently.

## Delivered slices (feature-branch-chain → tracker)

| Slice | PR | Content |
|-------|----|---------|
| 0a | #5 | Wire `StrictWorkflow` into `sdd.InjectOptions`; persist `StrictTDD`/`StrictWorkflow` in `state.json` (omitempty, MergeAgents carry) |
| 0b | (in #6 branch) | `internal/gitgate/` package: 3-state resolution, sentinel override, log, output, check pipeline, `git-gate` subcommand, `ensureClaudeGitGateHook`, config schema, `.gitignore` |
| 1 | #6 | `branch-base` + `orphan-upstream` validators (+ catch-22 fix for `git push -u origin`, + explicit start-point validation, + W-1 config-override logging) |
| 2 | #7 | `sequential-pr` validator (injectable `ghListOpenPRs`, fail-open) |
| 4 | #8 | Hardened `sequential-branches` SKILL.md; GGA Code Review Standards in AGENTS.md; persisted chain/delivery strategy in config |

## Verification

- `sdd-verify`: 0 CRITICAL, 3 WARNING (W-1 fixed, W-2 orchestrator-side, W-3 pre-existing TUI tests — fixed), 3 SUGGESTION.
- Merged tracker `feat/strict-workflow-mode` (`05a3ca6`): `go build ./cmd/...` clean, `go test ./...` GREEN, `go vet` clean.

## Bugs caught and fixed during implementation

- **Orphan catch-22**: the `orphan-upstream` gate would have blocked `git push -u origin <branch>` — the very command that sets the upstream. Fixed via `pushSetsOriginUpstream`.
- **W-1**: config-warn log entries lacked the `Override: "config"` audit field. Fixed.
- **W-3**: pre-existing `internal/tui` nav tests were stale after the StrictWorkflow screen insertion (not a nav bug). Fixed test expectations.

## Spec capabilities promoted to `openspec/specs/`

- `git-gate-model`, `git-gates`, `skill-hardening`, `strict-workflow-state-wiring`.

## Follow-up (out of scope, separate change)

- `strict-workflow-statusline-worktree` — statusline showing active branch/worktree + worktree-per-change binding (split out of the original plan).

## Remaining (delivery, not spec closure)

- End-to-end validation: build + `gentle-ai install` to activate the hook and observe a gate blocking live.
- Release: `feat/strict-workflow-mode → main` at maintainer's discretion.
