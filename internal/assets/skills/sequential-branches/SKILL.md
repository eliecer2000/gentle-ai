---
name: sequential-branches
description: "Trigger: strict_workflow: true in openspec/config.yaml. Sequential PR gate, atomic commits, SDD phases. Deterministic on Claude; advisory on other agents."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "2.0"
---

## Activation Contract

Load this skill when `strict_workflow: true` is present in `openspec/config.yaml`. It activates automatically via the skill registry when StrictWorkflow mode was enabled at install time.

## Enforcement Model

On **Claude Code**, gates are **deterministic**: a `PreToolUse(Bash)` hook calls `gentle-ai git-gate check --gate <name> --cwd <dir>` before every Bash tool use. A gate in `enforce` state blocks the operation outright (the tool call never executes). The agent cannot bypass enforcement silently — the only sanctioned override is the break-glass protocol documented below.

On **other agents** (OpenCode, Cursor, Windsurf, Gemini, Kiro, and others without blocking PreToolUse hooks), gates are **advisory**: this skill text is the primary enforcement surface. The agent reads these rules and must honor them. No binary check runs, so compliance depends on the agent's skill-following discipline.

## Gate Reference

Three gates are registered. The exact names used in `--gate` flags, sentinel filenames, and `strict_workflow_gates:` config keys are:

| Gate name | `--gate` value | Trigger condition |
|-----------|---------------|-------------------|
| Branch-base | `branch-base` | `git checkout -b` or `git branch` — any new branch creation |
| Orphan/upstream | `orphan-upstream` | `git push` — push to remote |
| Sequential-PR | `sequential-pr` | New task-branch creation when the task set has open PRs |

## Three-State Model

Each gate has one of three states, configured per gate in `openspec/config.yaml` under `strict_workflow_gates:`.

| State | Observable behavior |
|-------|---------------------|
| `enforce` | Gate failure **blocks** the operation. On Claude: the Bash tool call is denied with a visible reason. On other agents: STOP and surface the violation before proceeding. |
| `warn` | Gate failure **allows** the operation but emits a visible warning to stderr and appends an entry to `.gentle-ai/git-gate.log`. The flow never continues silently — the warning is always surfaced. |
| `off` | Gate is inactive. The operation proceeds without inspection or logging. |

Default when `strict_workflow: true` and no per-gate override is configured: `enforce`.

When `strict_workflow: false` or the key is absent: all gates are `off` regardless of per-gate config.

## Config Location and Keys

Gates are configured in `openspec/config.yaml`:

```yaml
strict_workflow: true
strict_workflow_gates:
  branch-base: enforce      # enforce | warn | off
  orphan-upstream: enforce
  sequential-pr: warn

sdd:
  delivery_strategy: chained
  chain_strategy: feature-branch-chain
  current_parent_branch: ""   # branch-base gate reads this at runtime
  task_branches: []           # sequential-pr gate reads this list
```

`sdd.current_parent_branch` is the declared parent for the current task. If non-empty, `branch-base` allows new branches off that branch in addition to an up-to-date `main`. The orchestrator sets this when spawning task branches.

`sdd.task_branches` is the active task-branch list. The `sequential-pr` gate calls `gh pr list` and blocks if any listed branch has an open PR. The orchestrator appends branch names here as it creates them. An absent or empty list means no task set is defined — the gate passes.

`delivery_strategy` and `chain_strategy` are read by the orchestrator on session start. Persisting them here avoids re-asking across compactions.

## Break-Glass Protocol (informed override — never silent)

The break-glass override exists for situations where a human engineer has evaluated the risk and accepts it for a specific one-time operation. It does NOT disable the gate — it degrades `enforce` to `warn` for exactly one operation and logs the downgrade.

### When to use

Use break-glass only when:
- The gate is blocking a legitimate operation that the configured rules do not account for.
- The engineer understands the risk and accepts it explicitly.

Do NOT use break-glass to silently work around gate logic. The override is always logged, always surfaces a warning, and the sentinel is deleted immediately after one use. There is no way to make a gate violation invisible.

### How to invoke (on Claude Code)

Before issuing the blocked git command, create the sentinel file for the relevant gate:

```
.gentle-ai/git-gate-override/<gate-name>
```

For example, to break-glass the `branch-base` gate:

```bash
mkdir -p .gentle-ai/git-gate-override
touch .gentle-ai/git-gate-override/branch-base
```

Then immediately run the blocked git command. The `git-gate check` hook will:
1. Detect the sentinel file.
2. Delete it (consumed-once — a second command will not be overridden).
3. Degrade the gate from `enforce` to `warn` for that one operation.
4. Allow the operation to proceed, emitting a visible warning to stderr.
5. Append a log entry to `.gentle-ai/git-gate.log` recording the gate name, timestamp, original mode, override source, result, and reason.

### On other agents (advisory)

There is no sentinel mechanism. To apply the equivalent informed override: surface the violation to the user, record the rationale, and proceed only with explicit user confirmation. Log the override decision in engram via `mem_save`.

### Invariant

The flow never changes silently. Every deviation — whether a sentinel-triggered downgrade, a `warn`-mode continuation, or a `gh`-absent degradation — is surfaced and logged. The user's risk is always informed, never hidden.

## Hard Rules

| Rule | Requirement |
|------|-------------|
| Sequential PR gate | Before creating any task branch, verify zero open PRs exist from `sdd.task_branches`. Block (`enforce`) or warn if any unmerged PRs are found. |
| Atomic commits | Each commit MUST contain one deliverable behavior: the implementation, its tests, and any related docs — together, never split by file type. |
| Mandatory SDD phases | Every phase (explore → propose → spec → design → tasks → apply → verify → archive) MUST complete before the next begins. No phase may be skipped without explicit user confirmation and a recorded reason. |
| Branch from valid base | New branches must originate from an up-to-date `main` or the declared `sdd.current_parent_branch`. Stale base is blocked under `enforce`. |
| Upstream on push | Every push must target `origin` with the upstream configured (`branch.<name>.remote = origin`). Inline `-u origin <branch>` sets the upstream and is accepted. |

## Decision Gates

| Condition | Action |
|-----------|--------|
| Open PRs detected before branching (enforce) | STOP. List open PRs. Block until resolved or break-glass used with documented rationale. |
| Open PRs detected (warn) | Surface the warning. Log. Allow the operation. |
| New branch from stale base (enforce) | STOP. Run `git fetch` and rebase/reset to current `origin/main` (or declared parent). |
| Push without `origin` upstream (enforce) | STOP. Use `git push -u origin <branch>` to set upstream, then re-push. |
| `sequential-pr` gate + `gh` CLI absent | Degrade to `warn` regardless of configured state. Surface advisory. Allow. |
| Commit contains mixed concerns | STOP. Split into separate atomic commits before proceeding. |
| User asks to skip a phase | Ask for rationale. If accepted, record it in engram via `mem_save`. |
| `strict_workflow: false` or key absent | This skill is inactive — all gates are `off`. Defer to standard `work-unit-commits` and `branch-pr` skills. |

## Execution Steps

1. On task start: verify `strict_workflow: true` in `openspec/config.yaml`. If false, this skill is inactive.
2. Before creating any branch: check `sdd.task_branches` for open PRs (Claude: automated via hook; other agents: run `gh pr list --state open` manually and inspect).
3. If any open PRs from the task set exist under `enforce`: surface them and block branch creation.
4. Fetch and update `main` (or declared parent) to ensure the base is current.
5. Create branch with `git checkout -b <type>/<short-description>` from the valid, up-to-date base.
6. Implement using atomic work-unit commits (code + tests + docs per commit).
7. Before pushing: confirm `branch.<name>.remote` is set to `origin`; use `git push -u origin <branch>` if not.
8. Before advancing to the next SDD phase: confirm the current phase artifact is complete.

## Output Contract

On gate failure (`enforce`): report the gate name, which rule was violated, the exact condition that triggered it, and what action unblocks progress. Never silently continue past a blocked gate.

On gate warning (`warn`): emit the warning visibly and log it. Note that the operation proceeded. The agent must not suppress the warning.

## References

- [work-unit-commits/SKILL.md](../work-unit-commits/SKILL.md) — atomic commit rules
- [branch-pr/SKILL.md](../branch-pr/SKILL.md) — branch naming and PR creation
