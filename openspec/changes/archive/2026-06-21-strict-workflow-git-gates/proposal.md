# Proposal: strict-workflow-git-gates

## Problem Statement

The entire git strategy in `gentle-ai` is **prompt-only**. Every git invariant lives
in advisory `SKILL.md` text (`internal/assets/skills/{branch-pr,chained-pr,sequential-branches,work-unit-commits}`).
Prompt instructions are probabilistic: they drift on compaction, get silently
overridden by user instructions, and have **zero deterministic enforcement**.

Observed symptoms (reported by the user, mapped to root cause below):

| Symptom | Root cause |
| --- | --- |
| Orphan branches | No gate ensures a new branch sets a correct upstream. |
| Per-task branches cut from stale `main` → desync/conflicts | No gate validates branch base against updated `main` / declared parent. |
| Parallel conflicting branches from the same task set | Sequential-PR discipline is advisory only. |
| Invisible active branch / worktree | No statusline, no worktree-to-change binding. |
| The intended flow is silently overridden | User/prompt instructions win over the workflow with no warning or log. |

Three concrete defects make even the *existing* advisory machinery unreliable:

1. **`strict_workflow` is read by zero Go code.** `openspec/config.yaml` has
   `strict_tdd: true` but no `strict_workflow` key, and no Go struct reads it.
2. **Injection bug (confirmed):** `internal/cli/run.go` (~line 790) passes
   `StrictTDD` to `sdd.InjectOptions` but **not** `StrictWorkflow` — even though
   the field exists on `InjectOptions` (`inject.go:49`) and on `model.Selection`.
   The strict-workflow marker is therefore **never injected**, even when the TUI
   screen enables it.
3. **Persistence gap (confirmed):** `internal/state/state.go` `InstallState`
   persists neither `StrictTDD` nor `StrictWorkflow` → both are lost on
   sync/reinstall.

## Goals

- Move **non-negotiable git invariants** out of prompt text and into
  **deterministic harness enforcement** for agents that support blocking hooks.
- Make every enforcement decision **visible and logged** — the workflow must
  **never change silently**, even when overridden.
- Provide an **informed-override ("break-glass")** escape hatch so the user
  stays in control without disabling the whole system.
- Fix the three confirmed defects so the feature is testable across
  install/sync/reinstall.
- Persist enforcement configuration **per project** in a machine-readable form.

## Non-Goals

- Enforcing git gates on agents that have **no blocking-hook mechanism**
  (OpenCode, Cursor, Windsurf, Gemini, Kiro). For those, enforcement stays
  prompt-advisory. This is a documented architectural limitation, not a defect.
- Rewriting the existing skills' content model. Slice 4 only *hardens* the
  `sequential-branches` skill and persists chain/delivery strategy; it does not
  redesign skills.
- Building a general-purpose policy engine. The gate set is fixed and small.
- Auto-resolving merge conflicts or rebasing on the agent's behalf.

## Architecture: Informed-Override (Break-Glass)

Each git invariant becomes a **gate** with **three states**:

- `enforce` — block the operation (default under `strict_workflow: true`).
- `warn` — allow the operation, emit a visible warning, and log it.
- `off` — no check.

Enforcement is implemented as a **deterministic PreToolUse hook** on agents that
support blocking (Claude Code; Codex SessionStart is advisory-only). The hook
does **not** see the conversation/prompt — only the tool-call JSON on stdin — so
overrides are communicated **out of band**.

Two override paths, both informed:

- **Config override (durable):** per-gate mode in `openspec/config.yaml`
  (`strict_workflow_gates:`). This is per-project, version-controlled, and
  auditable in git history.
- **Prompt override (one-shot break-glass):** the skill drops a
  **consumed-once sentinel file** at `.gentle-ai/git-gate-override/<gate>`. The
  hook reads it, degrades `enforce → warn` for **one operation**, then deletes
  it. The sentinel file name **is** the gate name (self-documenting).

**Invariant: the flow never changes silently.** Every override — config or
sentinel — emits a visible warning and writes a log entry. The user assumes the
risk, but always informed.

### Recommended implementation (decided)

- **Validator = Go subcommand** (`gentle-ai git-gate check --gate <name> --cwd <dir>`)
  in a new `internal/gitgate/` package, dispatched from `internal/app/app.go`
  following the existing subcommand pattern. The binary is already on PATH, so
  the installed hook calls it directly and gets blocking JSON
  (`hookSpecificOutput.permissionDecision: "deny"`) back. One binary, atomic
  updates, cross-platform, reuses Go types. (Rejected: embedded shell validator
  — separate deploy path, Windows portability, can't share Go types.)
- **Override = sentinel file** at `.gentle-ai/git-gate-override/<gate>`. Simpler
  than git-config: no git plumbing, self-documenting, gitignored. (Requires
  `.gentle-ai/` to be gitignored.)
- **Per-gate config = `openspec/config.yaml`** read by a **minimal Go reader**
  scoped to the `strict_workflow_gates:` section. `config.yaml` is the natural
  per-project home; agents already reference it. The **global** `strict_workflow`
  bool lives in `state.json` (`InstallState`).

## Gate Inventory (mapped to symptoms)

1. **Branch-base validation** — a new branch MUST be cut from an updated `main`
   or the declared parent. Kills desynced per-task branches.
2. **Orphan / upstream gate** — a new branch MUST set the correct upstream.
   Kills orphan branches.
3. **Sequential-PR gate** — block a new branch if open PRs from the task set
   already exist. Kills parallel conflicting branches.
4. **Statusline + worktree binding** — show the active branch + worktree and
   bind one worktree per change. Kills the invisible-branch/worktree problem.
   **Fully net-new — zero existing plumbing.**

## Slice Plan (dependency-ordered, chained PRs)

Delivery is a **chain of PRs**, ordered by dependency. Keep the first slice tight.

- **Slice 0 — Foundation + bug fixes (first PR).**
  - Fix the two confirmed bugs: wire `StrictWorkflow` in `run.go`; persist
    `StrictWorkflow` (and `StrictTDD`) in `InstallState`/`state.json`.
  - Add `strict_workflow: true` and the `strict_workflow_gates:` skeleton to
    `openspec/config.yaml`; add the minimal Go reader.
  - Implement the **3-state model** + **sentinel override mechanism** +
    visible-warning/logging plumbing, and the `internal/gitgate/` package +
    `gentle-ai git-gate check` subcommand scaffold (no gate logic yet beyond a
    no-op/pass gate to prove the hook path end-to-end).
  - Install the PreToolUse hook (Claude) / advisory SessionStart (Codex).
- **Slice 1 — Branch-base + orphan/upstream gates.**
- **Slice 2 — Sequential-PR gate.**
- **Slice 3 — Statusline + worktree binding.**
  **Recommendation: split into a follow-up change.** It is entirely net-new
  (no statusline asset, no worktree plumbing, no `EnterWorktree` tool), depends
  on none of the enforcement primitives, and carries the largest design
  uncertainty. Including it would balloon this change and delay the gates that
  directly fix the reported branch defects. Keep this change focused on Slices
  0–2 (+4); track Slice 3 as `strict-workflow-statusline-worktree`.
- **Slice 4 — Skill hardening + strategy persistence.**
  Harden `sequential-branches/SKILL.md` to reference the gates and the
  break-glass protocol; persist chain/delivery strategy outside the session
  cache so it survives compaction.

## Key Decisions

1. **YAML dependency vs. minimal parser.** `go.mod` has **no YAML dependency**.
   - *Recommendation:* add a **minimal scoped reader** (or `gopkg.in/yaml.v3`)
     for **only** the `strict_workflow_gates:` section. A bounded minimal reader
     avoids a new third-party dependency; `yaml.v3` is the fallback if the config
     surface grows. Decide in design; default to minimal-reader for slice 0.
2. **Statusline scope: in or out.** *Recommendation:* **out** of this change —
   move Slice 3 to a follow-up (`strict-workflow-statusline-worktree`). Rationale
   above.
3. **Validator: Go subcommand vs. shell.** *Recommendation:* **Go subcommand**
   (`internal/gitgate/`) per exploration A2. One binary, cross-platform, reuses
   Go types.
4. **Config truth: state.json vs. openspec/config.yaml.** *Recommendation:*
   **both, split by scope** — global `strict_workflow` bool in `state.json`;
   per-gate `enforce/warn/off` mode in per-project `openspec/config.yaml`.

## Affected internal/ Packages

- `internal/cli/run.go` — fix missing `StrictWorkflow` wiring (~line 790).
- `internal/state/state.go` — add `StrictWorkflow` (and `StrictTDD`) to
  `InstallState` JSON; persist across sync/reinstall.
- `internal/components/sdd/inject.go` — add the git-gate hook installer
  (PreToolUse for Claude, advisory SessionStart for Codex), reusing the
  `ensureClaudeSkillRegistryHook` / `ensureCodexSkillRegistryHook` pattern.
- `internal/app/app.go` — add the `git-gate` case to subcommand dispatch.
- `internal/gitgate/` — **new package**: gate checks, 3-state model, sentinel
  consume/log, config reader, blocking-JSON output.
- `openspec/config.yaml` — add `strict_workflow` + `strict_workflow_gates:`.
- `internal/assets/skills/sequential-branches/SKILL.md` — hardened in Slice 4.
- `internal/model/selection.go` — no change (`StrictWorkflow` already present).

## Rollback Plan

- **Per-gate kill switch:** set any gate to `off` in
  `openspec/config.yaml` → that gate is fully disabled with no code change.
- **Global kill switch:** set `strict_workflow: false` (or the `state.json`
  bool) → all gates become no-ops; the harness falls back to today's
  prompt-advisory behavior.
- **Hook removal:** the hook installer is idempotent (`claudeHookExists`
  pattern). Uninstall removes the PreToolUse entry; no residual blocking.
- **Bug-fix isolation:** the Slice 0 wiring/persistence fixes are independent
  and safe to keep even if gate enforcement is reverted — they only correct
  values that should always have flowed through.
- **Sentinel safety:** if the agent crashes mid-override, the sentinel is a
  plain file under gitignored `.gentle-ai/`; it is consumed-once and harmless if
  orphaned (next hook invocation deletes it).
- Each slice ships as its own PR, so any single slice can be reverted without
  unwinding the others.

## Risks

- **Non-Claude advisory-only enforcement (primary risk).** Only Claude Code
  (and partially Codex) can block via hooks. OpenCode, Cursor, Windsurf, Gemini,
  and Kiro stay prompt-advisory — gates do not deterministically enforce there.
  This is an architectural limitation to **document clearly**, not a defect.
- **Hook receives no prompt text.** Any override must be out-of-band (sentinel
  file). The `CLAUDE_ENV_FILE` mechanism does **not** work from agent prompts,
  only from hook scripts — so an env-var override is not viable.
- **Sentinel race.** If the agent crashes between writing the sentinel and the
  Bash call, a stale override may persist for one operation. Mitigated by
  consumed-once delete + visible logging.
- **YAML reading in Go is net-new.** Introducing parsing (minimal or `yaml.v3`)
  is a new surface in a codebase that currently reads `config.yaml` only from
  the prompt layer. Keep the reader scoped to `strict_workflow_gates:`.
- **`.gentle-ai/` must be gitignored** for the sentinel mechanism to be safe;
  installation must ensure this.
- **Slice 3 (statusline/worktree) uncertainty** justifies deferring it to a
  follow-up change.

## Open Questions for Spec/Design

- Per-gate vs. global override granularity for v1 (default: per-gate).
- Exact blocking-JSON contract and exit-code behavior for the hook (design).
- Whether the minimal YAML reader or `gopkg.in/yaml.v3` is adopted (design;
  default minimal reader for Slice 0).
- Confirm Slice 3 is split into `strict-workflow-statusline-worktree`.
