# Tasks: strict-workflow-git-gates (Slices 0, 1, 2, 4)

> STRICT TDD IS ACTIVE: every behavioral unit follows RED → GREEN → REFACTOR order.
> Test runner: `go test ./...` | Type check: `go vet ./...` | Format: `gofmt`
> Statusline/worktree (Slice 3) is deferred to change `strict-workflow-statusline-worktree`.

---

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Slice 0 — Foundation + bug fixes | ~300–380 lines |
| Slice 1 — Branch-base + orphan-upstream gates | ~200–260 lines |
| Slice 2 — Sequential-PR gate | ~120–160 lines |
| Slice 4 — Skill hardening + strategy persistence | ~80–120 lines |
| **Total across all slices** | **~700–920 lines** |
| 400-line budget risk | High (total); per-slice varies — see per-slice notes |
| Chained PRs recommended | Yes |
| Suggested split | PR 0 → PR 1 → PR 2 → PR 4 (each slice = one PR, dependency-ordered) |
| Delivery strategy | chained PRs (stacked-to-main) |

Decision needed before apply: No (already resolved — chained PRs, stacked-to-main)
Chained PRs recommended: Yes
Chain strategy: stacked-to-main

### Per-Slice Budget Notes

| Slice | PR | Estimated lines | Budget risk |
|-------|----|----------------|-------------|
| 0 | PR 1 | ~300–380 | **High** — owns all new package scaffolding; mandatory own PR |
| 1 | PR 2 | ~200–260 | Medium — two gate implementations + tests |
| 2 | PR 3 | ~120–160 | Low |
| 4 | PR 4 | ~80–120 | Low |

---

## Resolved Design Open Items

### Gate Names (finalized)

Three gate names used verbatim in `--gate` flag, sentinel filenames, and `strict_workflow_gates:` config keys:

| Gate | --gate value | Trigger condition |
|------|-------------|------------------|
| Branch-base | `branch-base` | `git checkout -b` or `git branch` (new branch creation) |
| Orphan/upstream | `orphan-upstream` | `git push` (push to remote) |
| Sequential-PR | `sequential-pr` | New task branch creation when task set has open PRs |

### Parent-Branch Storage Location (Slice 1)

The declared parent branch for the current task is stored in `openspec/config.yaml` under the `sdd:` block, key `current_parent_branch`. The gate reads this at runtime. If the key is absent, `main` is the only allowed base. Example:

```yaml
sdd:
  current_parent_branch: feat/tracker
  delivery_strategy: auto-chain
  chain_strategy: stacked-to-main
```

### Task-Set Association (Slice 2)

The "current task set" for the `sequential-pr` gate is defined as all branches whose names appear in `openspec/config.yaml` `sdd.task_branches[]` list. The orchestrator appends to this list when spawning a new task branch. The gate calls `gh pr list --json headRefName,number,state` and filters by that list. If `sdd.task_branches` is absent or empty, the gate treats the task set as empty and allows the operation (safe default for non-SDD workflows).

---

## Slice 0 — Foundation + Bug Fixes

**Spec domains**: `strict-workflow-state-wiring`, `git-gate-model`
**PR**: `fix/strict-workflow-foundation` → targets `main`
**Depends on**: nothing (first in chain)
**Estimated lines**: ~300–380
**Budget risk**: High — mandatory own PR

### Phase 1 — Infrastructure

#### 1.1 openspec/config.yaml — Add strict_workflow schema

- [x] **1.1.1** Add `strict_workflow: true` and `strict_workflow_gates:` skeleton to `openspec/config.yaml` after the `strict_tdd: true` key.

  ```yaml
  strict_workflow: true
  strict_workflow_gates:
    branch-base: enforce
    orphan-upstream: enforce
    sequential-pr: warn
  ```

- [x] **1.1.2** Add `sdd:` block with empty `delivery_strategy`, `chain_strategy`, `current_parent_branch`, and `task_branches` keys.

  ```yaml
  sdd:
    delivery_strategy: ""
    chain_strategy: ""
    current_parent_branch: ""
    task_branches: []
  ```

- [x] **1.1.3** Add `.gentle-ai/` entry to `.gitignore` if not already present (required for sentinel safety).

#### 1.2 internal/gitgate/ — New package skeleton

- [x] **1.2.1** Create `internal/gitgate/` directory with `doc.go` package declaration comment.

  ```
  // Package gitgate implements the three-state git gate enforcement model
  // for the strict-workflow feature. It is invoked by the gentle-ai
  // git-gate subcommand and consumed by Claude Code PreToolUse hooks.
  package gitgate
  ```

- [x] **1.2.2** Create `internal/gitgate/model.go` — define:
  - `GateMode` type (`string`) with constants `ModeEnforce`, `ModeWarn`, `ModeOff`
  - `GateResult` struct: `Allowed bool`, `Mode GateMode`, `Message string`, `SentinelConsumed bool`
  - `CheckResult` struct: `hookSpecificOutput` with `hookEventName`, `permissionDecision`, `permissionDecisionReason`, `message` (for JSON serialization)

- [x] **1.2.3** Create `internal/gitgate/config.go` — minimal scoped YAML reader:
  - `Config` struct: `StrictWorkflow bool`, `Gates map[string]GateMode`, `SDD SddConfig`
  - `SddConfig` struct: `DeliveryStrategy string`, `ChainStrategy string`, `CurrentParentBranch string`, `TaskBranches []string`
  - `ReadConfig(path string) (Config, error)` — line-by-line reader for `strict_workflow`, `strict_workflow_gates:`, and `sdd:` blocks; tolerates absent keys; no third-party YAML dependency

- [x] **1.2.4** Create `internal/gitgate/sentinel.go` — sentinel file logic:
  - `SentinelPath(cwd, gate string) string` — returns `.gentle-ai/git-gate-override/<gate>` under cwd
  - `Consumesentinel(path string) (bool, error)` — reads existence, deletes file atomically, returns consumed=true; if absent returns consumed=false; if delete fails returns error
  - `EnsureGitignored(cwd string) error` — appends `.gentle-ai/` to `<cwd>/.gitignore` if absent

- [x] **1.2.5** Create `internal/gitgate/log.go` — gate log appender:
  - `LogEntry` struct: `Timestamp`, `Gate`, `Mode`, `Override` (`"config"|"sentinel"|""`), `Result`, `Reason`
  - `AppendLog(cwd, gate string, entry LogEntry) error` — opens `.gentle-ai/git-gate.log` in append mode, writes one line: `<ISO8601> <gate> <mode> <override> <result> <reason>`

- [x] **1.2.6** Create `internal/gitgate/resolve.go` — 3-state resolution:
  - `Resolve(cfg Config, gate string, sentinelConsumed bool) GateMode` — implements the resolution matrix:
    - `strict_workflow=false` → `ModeOff`
    - per-gate config absent and `strict_workflow=true` → `ModeEnforce`
    - per-gate config present → use that value
    - if `sentinelConsumed` and resolved mode == `ModeEnforce` → degrade to `ModeWarn`
    - if `sentinelConsumed` and resolved mode != `ModeEnforce` → no change (sentinel consumed but no-op for warn/off)

- [x] **1.2.7** Create `internal/gitgate/output.go` — JSON output helpers:
  - `DenyOutput(gate, reason string) []byte` — returns blocking hook JSON: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"[gate] blocked: <reason>"}}`
  - `AllowOutput() []byte` — returns permitting hook JSON with `permissionDecision: "allow"`
  - `WarnOutput(gate, reason string) []byte` — returns allow JSON; warning is emitted to stderr separately

- [x] **1.2.8** Create `internal/gitgate/check.go` — `Check(gate, cwd string) error` orchestrator function (Slice 0: no-op pass gate, always returns allow):
  - Reads config from `openspec/config.yaml` (path resolved relative to cwd or `CLAUDE_PROJECT_DIR`)
  - Checks sentinel via `ConsumeSentinel`
  - Resolves mode via `Resolve`
  - For Slice 0: skips domain check and always emits allow JSON to stdout; mode==off: exits immediately
  - Calls `AppendLog` when mode is warn (Slice 0: only triggered by sentinel downgrade)

#### 1.3 internal/app/app.go — git-gate subcommand dispatch

- [x] **1.3.1** Add `"git-gate"` case to the early dispatch switch in `RunArgs` (before system detection), following the pattern of existing info-command cases. Route to `cli.RunGitGate(args[1:], stdout)`.

#### 1.4 internal/cli/ — RunGitGate entrypoint

- [x] **1.4.1** Create `internal/cli/gitgate.go` — `RunGitGate(args []string, stdout io.Writer) error`:
  - Parses `--gate <name>` and `--cwd <dir>` flags
  - Returns error on missing `--gate`
  - Calls `gitgate.Check(gate, cwd)`
  - On internal error: emits allow JSON (fail-open), writes error to stderr

#### 1.5 internal/components/sdd/inject.go — Hook installer

- [x] **1.5.1** Add `ensureClaudeGitGateHook(settingsPath, cwd string) (bool, error)` sibling function following the `ensureClaudeSkillRegistryHook` pattern:
  - Command template: `gentle-ai git-gate check --gate <g> --cwd "${CLAUDE_PROJECT_DIR:-$PWD}"` for each of the three gates
  - Installs as a `PreToolUse` hook with `matcher: "Bash"` and `timeout: 30`
  - Idempotent: calls `claudeHookExists` for the command before inserting
  - Installs one hook entry per gate (three entries total)

- [x] **1.5.2** Wire `ensureClaudeGitGateHook` call from `installSkillRegistryAutomation` (Claude Code branch only), after the existing skill-registry hook install, gated on `StrictWorkflow` being enabled. Pass `settingsPath` and `homeDir`.

#### 1.6 Bug fixes — state wiring

- [x] **1.6.1** `internal/cli/run.go` ~line 790: add `StrictWorkflow: s.selection.StrictWorkflow` to the `sdd.InjectOptions` literal (one-line fix, closes confirmed injection bug).

- [x] **1.6.2** `internal/state/state.go` `InstallState` struct: add two fields with `omitempty`:
  ```go
  StrictTDD      bool `json:"strict_tdd,omitempty"`
  StrictWorkflow bool `json:"strict_workflow,omitempty"`
  ```

- [x] **1.6.3** `internal/state/state.go` `MergeAgents`: carry both new fields from `existing` into the returned struct literal.

### Phase 2 — Tests (RED first, then GREEN)

#### 1.7 Tests — Bug fixes (state wiring)

- [x] **1.7.1** `internal/state/state_test.go` — RED: add table-driven test `TestInstallStateStrictFieldsPersistence`:
  - case "StrictWorkflow round-trips via Write/Read"
  - case "StrictTDD round-trips via Write/Read"
  - case "zero-value new install yields false for both fields"
  - case "old state.json without fields deserializes as false (back-compat)"

- [x] **1.7.2** `internal/state/state_test.go` — RED: add test `TestMergeAgentsCarriesStrictFields`: verifies MergeAgents preserves StrictTDD and StrictWorkflow from existing state.

- [x] **1.7.3** Implement 1.6.2 + 1.6.3 to make 1.7.1 + 1.7.2 GREEN.

#### 1.8 Tests — run.go injection

- [x] **1.8.1** `internal/cli/run_test.go` — RED: add test `TestRunInjectOptionsIncludesStrictWorkflow`:
  - case "StrictWorkflow=true flows into InjectOptions.StrictWorkflow"
  - case "StrictWorkflow=false is not injected"

- [x] **1.8.2** Implement 1.6.1 to make 1.8.1 GREEN.

#### 1.9 Tests — gitgate package

- [x] **1.9.1** `internal/gitgate/config_test.go` — RED: table-driven `TestReadConfig`:
  - case "all gates present: each resolves to config value"
  - case "strict_workflow=true, no gates block: all default to enforce"
  - case "strict_workflow=false: all gates resolve to off regardless of config"
  - case "missing file: returns zero Config, no error expected or explicit error per impl"

- [x] **1.9.2** `internal/gitgate/resolve_test.go` — RED: table-driven `TestResolve`:
  - case "enforce + no sentinel → enforce"
  - case "enforce + sentinel consumed → warn"
  - case "warn + sentinel consumed → warn (no double-degrade)"
  - case "off + sentinel consumed → off (sentinel silently consumed)"
  - case "strict_workflow=false → off always"
  - case "missing per-gate config + strict_workflow=true → enforce"

- [x] **1.9.3** `internal/gitgate/sentinel_test.go` — RED: `TestConsumeSentinel` using `t.TempDir()`:
  - case "sentinel present: consumed=true, file deleted"
  - case "sentinel absent: consumed=false, no error"
  - case "second call after consume: consumed=false (truly consumed-once)"
  - `TestSentinelPath`: verifies path shape `.gentle-ai/git-gate-override/<gate>`

- [x] **1.9.4** `internal/gitgate/log_test.go` — RED: `TestAppendLog` using `t.TempDir()`:
  - case "log file created on first entry"
  - case "entries are appended, not overwritten"
  - case "entry contains timestamp, gate, mode, override, result"

- [x] **1.9.5** `internal/gitgate/output_test.go` — RED: `TestDenyOutput`, `TestAllowOutput`, `TestWarnOutput`:
  - Verify `permissionDecision: "deny"` / `"allow"` in JSON shape
  - Verify `hookEventName: "PreToolUse"` present in deny output
  - Verify `permissionDecisionReason` non-empty in deny

- [x] **1.9.6** Implement `internal/gitgate/` package (tasks 1.2.2–1.2.8) to make 1.9.1–1.9.5 GREEN.

#### 1.10 Tests — CLI entrypoint

- [x] **1.10.1** `internal/cli/gitgate_test.go` — RED: `TestRunGitGate`:
  - case "missing --gate returns error"
  - case "valid --gate=branch-base, strict_workflow=false → allow JSON on stdout, exit 0"
  - case "internal error in Check → allow JSON (fail-open), error on stderr"
  - Use `t.TempDir()` for cwd; write minimal `openspec/config.yaml` with `strict_workflow: false`

- [x] **1.10.2** Implement 1.4.1 to make 1.10.1 GREEN.

#### 1.11 Tests — Hook installer

- [x] **1.11.1** `internal/components/sdd/inject_test.go` — RED: `TestEnsureClaudeGitGateHook`:
  - case "fresh settings file: three PreToolUse hook entries inserted (one per gate)"
  - case "idempotent: second call returns changed=false, no duplicate entries"
  - case "malformed JSON: returns parse error"

- [x] **1.11.2** Implement 1.5.1 to make 1.11.1 GREEN.

### Phase 3 — Slice 0 Verification

- [x] **1.12.1** Run `go test ./internal/state/... ./internal/cli/... ./internal/gitgate/... ./internal/components/sdd/...` — all pass.
- [x] **1.12.2** Run `go vet ./...` — no errors.
- [x] **1.12.3** Run `gofmt -l ./internal/gitgate/` — no files listed.
- [x] **1.12.4** Smoke-check: build `go build ./cmd/...` succeeds.

### Slice 0 PR Boundary

- **Branch**: `fix/strict-workflow-foundation`
- **PR type**: `type:feature` (new package + bug fixes)
- **Depends on**: nothing
- **Next**: Slice 1 targets `main` after this merges
- **Out of scope**: gate domain logic (Slices 1, 2), skill hardening (Slice 4)
- **Rollback**: set `strict_workflow: false` in `openspec/config.yaml` → all gates become no-ops; bug fixes (state wiring) are safe to keep independently

---

## Slice 1 — Branch-Base + Orphan/Upstream Gates

**Spec domain**: `git-gates` (Gate 1, Gate 2)
**PR**: `feat/git-gates-branch-base-orphan` → targets `main`
**Depends on**: Slice 0 merged (requires `internal/gitgate/` package)
**Estimated lines**: ~200–260
**Budget risk**: Medium

### Phase 1 — Infrastructure (Gate 1)

#### 2.1 internal/gitgate/ — Branch-base gate implementation

- [x] **2.1.1** Create `internal/gitgate/gate_branchbase.go` — `CheckBranchBase(cwd string, cfg Config) (GateResult, error)`:
  - Reads stdin for tool-call JSON to extract the `git checkout -b` or `git branch` command arguments (Slice 1 parses stdin for arg-aware matching)
  - If the tool call is not a new-branch operation: returns `GateResult{Allowed: true}` immediately (not applicable)
  - Determines the base branch by running `git rev-parse --abbrev-ref HEAD` in cwd
  - Checks if base == `main` or `cfg.SDD.CurrentParentBranch` (if non-empty)
  - For `main` base: runs `git fetch --dry-run origin main` then compares `git rev-parse HEAD` vs `git rev-parse origin/main` to detect staleness
  - For declared parent base: compares local HEAD vs `origin/<parent>` the same way
  - Stale or unknown base under enforce → `GateResult{Allowed: false, Message: "..."}`
  - Fresh base → `GateResult{Allowed: true}`

- [x] **2.1.2** Wire `gate_branchbase.go` into `internal/gitgate/check.go` `Check` function: if gate name is `branch-base`, call `CheckBranchBase`; apply mode resolution from Slice 0 `Resolve`; emit deny/allow/warn JSON; call `AppendLog` on warn/deny.

#### 2.2 internal/gitgate/ — Orphan/upstream gate implementation

- [x] **2.2.1** Create `internal/gitgate/gate_orphan.go` — `CheckOrphanUpstream(cwd string, cfg Config) (GateResult, error)`:
  - Parses stdin tool-call JSON to confirm the command is a `git push` variant (not a push with explicit remote:branch that sets upstream implicitly — treat those as would-set-upstream = OK)
  - Reads current branch name via `git rev-parse --abbrev-ref HEAD`
  - Reads `git config branch.<name>.remote` and `git config branch.<name>.merge`
  - If `remote` is absent or empty → blocked: "branch has no upstream set"
  - If `remote` != `origin` → blocked: "branch upstream is not origin (got: <remote>)"
  - Otherwise → allowed
  - Returns `GateResult`

- [x] **2.2.2** Wire `gate_orphan.go` into `check.go` for gate name `orphan-upstream`; same mode + log pattern as branch-base.

### Phase 2 — Tests (RED first, then GREEN)

#### 2.3 Tests — Branch-base gate

- [x] **2.3.1** `internal/gitgate/gate_branchbase_test.go` — RED: table-driven `TestCheckBranchBase` using `t.TempDir()` and a real git repo initialized in the temp dir:
  - case "non-branch-create tool call: gate not applicable → allow"
  - case "branch from up-to-date main → allow"
  - case "branch from stale main (local behind origin/main by 1 commit) → deny (enforce)"
  - case "branch from declared parent (current, cfg.SDD.CurrentParentBranch set) → allow"
  - case "branch from stale declared parent → deny"
  - Use `git init`, `git remote add origin file://<tmpdir-bare>`, and commit manipulation to create stale state; skip with `testing.Short()` for the git-exec cases

- [x] **2.3.2** `internal/gitgate/check_test.go` — RED: integration tests for `Check("branch-base", cwd)`:
  - case "gate off: always allow, no log entry"
  - case "gate enforce + stale main → deny JSON on stdout"
  - case "gate enforce + sentinel → warn JSON, sentinel deleted, log entry written"
  - case "gate warn + stale main → allow JSON, warning to stderr, log entry"

#### 2.4 Tests — Orphan/upstream gate

- [x] **2.4.1** `internal/gitgate/gate_orphan_test.go` — RED: table-driven `TestCheckOrphanUpstream`:
  - case "non-push tool call: gate not applicable → allow"
  - case "push with upstream = origin → allow"
  - case "push with no upstream configured → deny"
  - case "push with upstream = fork (not origin) → deny"
  - case "push in warn mode with no upstream → allow + warn + log"
  - Use `t.TempDir()` + `git init` + `git config branch.<name>.remote/merge` manipulation; skip with `testing.Short()` for git-exec cases

- [x] **2.4.2** Extend `check_test.go` with `Check("orphan-upstream", ...)` cases matching the scenarios above.

#### 2.5 Implement and make tests GREEN

- [x] **2.5.1** Implement `gate_branchbase.go` (task 2.1.1) to make 2.3.1 GREEN.
- [x] **2.5.2** Wire branch-base into `check.go` (task 2.1.2) to make 2.3.2 GREEN.
- [x] **2.5.3** Implement `gate_orphan.go` (task 2.2.1) to make 2.4.1 GREEN.
- [x] **2.5.4** Wire orphan-upstream into `check.go` (task 2.2.2) to make 2.4.2 GREEN.

### Phase 3 — Slice 1 Verification

- [x] **2.6.1** Run `go test ./internal/gitgate/...` — all pass.
- [x] **2.6.2** Run `go vet ./internal/gitgate/...` — no errors.
- [x] **2.6.3** Run `gofmt -l ./internal/gitgate/` — no files listed.
- [x] **2.6.4** Build `go build ./cmd/...` succeeds.

### Slice 1 PR Boundary

- **Branch**: `feat/git-gates-branch-base-orphan`
- **PR type**: `type:feature`
- **Depends on**: `fix/strict-workflow-foundation` merged
- **Next**: Slice 2 targets `main` after this merges
- **Out of scope**: sequential-PR gate (Slice 2), skill hardening (Slice 4)
- **Rollback**: set `branch-base: off` and `orphan-upstream: off` in `openspec/config.yaml`

---

## Slice 2 — Sequential-PR Gate

**Spec domain**: `git-gates` (Gate 3)
**PR**: `feat/git-gate-sequential-pr` → targets `main`
**Depends on**: Slice 1 merged
**Estimated lines**: ~120–160
**Budget risk**: Low

### Phase 1 — Infrastructure

#### 3.1 internal/gitgate/ — Sequential-PR gate implementation

- [x] **3.1.1** Create `internal/gitgate/gate_sequentialpr.go` — `CheckSequentialPR(cwd string, cfg Config) (GateResult, error)`:
  - Parses stdin tool-call JSON to confirm this is a new-branch-creation operation (same check as branch-base)
  - If `cfg.SDD.TaskBranches` is empty or nil: returns `GateResult{Allowed: true}` (no task set, gate not applicable)
  - Checks `gh` binary availability via `exec.LookPath("gh")`; if absent: returns `GateResult{Allowed: true, Message: "gh CLI unavailable — sequential-pr gate skipped"}` with mode degraded to warn regardless of configured state (spec: gh CLI missing degrades to warn)
  - Runs `gh pr list --state open --json headRefName,number` in cwd
  - Parses output: if any returned `headRefName` is in `cfg.SDD.TaskBranches` → conflict found
  - No conflicts → `GateResult{Allowed: true}`
  - Conflict found under enforce → `GateResult{Allowed: false, Message: "open PRs from task set: <list of PR numbers + branches>"}`

- [x] **3.1.2** Wire `gate_sequentialpr.go` into `check.go` for gate name `sequential-pr`; same mode + log pattern. Apply gh-unavailable degradation before mode resolution (always degrade to warn regardless of config when gh is missing).

### Phase 2 — Tests (RED first, then GREEN)

#### 3.2 Tests — Sequential-PR gate

- [x] **3.2.1** `internal/gitgate/gate_sequentialpr_test.go` — RED: table-driven `TestCheckSequentialPR` with a mock `gh` command (use `t.TempDir()` and a fake `gh` binary script on PATH):
  - case "non-branch-create tool call: gate not applicable → allow"
  - case "empty task_branches: gate not applicable → allow"
  - case "gh CLI absent: allow + warning (degrade to warn)"
  - case "no open PRs from task set → allow"
  - case "open PR from task set exists (enforce) → deny with PR list"
  - case "open PR from task set exists (warn) → allow + warning + log"
  - case "gh returns non-zero exit → fail-open (allow) + warning"

- [x] **3.2.2** Extend `check_test.go` with `Check("sequential-pr", ...)` cases for enforce, warn, and gh-missing scenarios.

#### 3.3 Implement and make tests GREEN

- [x] **3.3.1** Implement `gate_sequentialpr.go` (task 3.1.1) to make 3.2.1 GREEN.
- [x] **3.3.2** Wire sequential-pr into `check.go` (task 3.1.2) to make 3.2.2 GREEN.

### Phase 3 — Slice 2 Verification

- [x] **3.4.1** Run `go test ./internal/gitgate/...` — all pass.
- [x] **3.4.2** Run `go vet ./internal/gitgate/...` — no errors.
- [x] **3.4.3** Build `go build ./cmd/...` succeeds.

### Slice 2 PR Boundary

- **Branch**: `feat/git-gate-sequential-pr`
- **PR type**: `type:feature`
- **Depends on**: `feat/git-gates-branch-base-orphan` merged
- **Next**: Slice 4 targets `main` after this merges
- **Out of scope**: skill hardening (Slice 4)
- **Rollback**: set `sequential-pr: off` in `openspec/config.yaml`

---

## Slice 4 — Skill Hardening + Strategy Persistence

**Spec domain**: `skill-hardening`
**PR**: `feat/strict-workflow-skill-hardening` → targets `main`
**Depends on**: Slice 2 merged (gates must exist before skill documents them)
**Estimated lines**: ~80–120
**Budget risk**: Low

### Phase 1 — Implementation

#### 4.1 internal/assets/skills/sequential-branches/SKILL.md — Rewrite

- [x] **4.1.1** Rewrite `internal/assets/skills/sequential-branches/SKILL.md` to include:
  - Reference to all three gate names (`branch-base`, `orphan-upstream`, `sequential-pr`) by name in a **Gate Reference** section
  - Three-state model table: `enforce` / `warn` / `off` with observable behavior for each
  - **Break-glass protocol** section: when to use it, exact path `.gentle-ai/git-gate-override/<gate>`, that it is consumed-once, and what happens (one-operation warn downgrade + log)
  - **Advisory notice** for non-Claude agents: prominent statement that gates are advisory-only on platforms without blocking PreToolUse hooks (OpenCode, Cursor, Windsurf, Gemini, Kiro) and the skill text is the enforcement surface on those platforms
  - **Do NOT** include Go package names, binary internals, or JSON format details

#### 4.2 openspec/config.yaml — Persist SDD strategy

- [x] **4.2.1** Update `openspec/config.yaml` `sdd:` block to include the confirmed delivery and chain strategy for this project:
  ```yaml
  sdd:
    delivery_strategy: auto-chain
    chain_strategy: stacked-to-main
    current_parent_branch: ""
    task_branches: []
  ```
  (These values represent the confirmed strategy for `gentle-ai` and will be read by the orchestrator on session start to avoid re-asking.)

### Phase 2 — Tests (RED first, then GREEN)

#### 4.3 Tests — SKILL.md content validation

- [x] **4.3.1** `internal/assets/skills/sequential-branches/skill_test.go` — RED: `TestSequentialBranchesSkillContent`:
  - Reads the embedded skill file (or raw file path in test)
  - Asserts all three gate names appear in content: `branch-base`, `orphan-upstream`, `sequential-pr`
  - Asserts the three-state terms `enforce`, `warn`, `off` all appear
  - Asserts `git-gate-override` (sentinel path) appears
  - Asserts advisory notice keyword (`advisory`) appears for non-Claude agents

- [x] **4.3.2** Implement 4.1.1 to make 4.3.1 GREEN.

#### 4.4 Tests — Config strategy persistence

- [x] **4.4.1** `internal/gitgate/config_test.go` — RED: extend `TestReadConfig` with cases:
  - case "sdd.delivery_strategy and chain_strategy are read correctly"
  - case "sdd.task_branches list is read correctly when non-empty"
  - case "absent sdd block returns zero SddConfig (no error)"

- [x] **4.4.2** Implement or extend `ReadConfig` (task 1.2.3) to cover sdd block parsing; make 4.4.1 GREEN.

### Phase 3 — Slice 4 Verification

- [x] **4.5.1** Run `go test ./internal/gitgate/... ./internal/assets/...` — all pass.
- [x] **4.5.2** Run `go vet ./...` — no errors.
- [x] **4.5.3** Run `gofmt -l ./internal/...` — no new files listed (pre-existing issues only, not introduced by Slice 4).
- [x] **4.5.4** Build `go build ./cmd/...` succeeds.

### Slice 4 PR Boundary

- **Branch**: `feat/strict-workflow-skill-hardening`
- **PR type**: `type:feature`
- **Depends on**: `feat/git-gate-sequential-pr` merged
- **Next**: no further slices in this change (Slice 3 tracked separately as `strict-workflow-statusline-worktree`)
- **Out of scope**: statusline, worktree binding (deferred change)
- **Rollback**: revert `sequential-branches/SKILL.md` to previous content; clear `sdd:` block in `openspec/config.yaml`

---

## Chained PR Dependency Diagram

```
[fix/strict-workflow-foundation]   PR 1 (Slice 0) ← 📍 start here
            |
            v
[feat/git-gates-branch-base-orphan]  PR 2 (Slice 1)
            |
            v
[feat/git-gate-sequential-pr]        PR 3 (Slice 2)
            |
            v
[feat/strict-workflow-skill-hardening]  PR 4 (Slice 4)
            |
            v
          main
```

Each PR must merge before the next branch is created. No parallel PR creation.

---

## Files Affected Summary

| File | Slice | Action |
|------|-------|--------|
| `internal/cli/run.go` | 0 | Fix StrictWorkflow injection (1-line) |
| `internal/state/state.go` | 0 | Add StrictTDD + StrictWorkflow fields + MergeAgents carry |
| `internal/state/state_test.go` | 0 | New tests for field persistence + MergeAgents |
| `internal/cli/run_test.go` | 0 | New test for InjectOptions wiring |
| `internal/gitgate/doc.go` | 0 | New package |
| `internal/gitgate/model.go` | 0 | New: GateMode, GateResult, CheckResult types |
| `internal/gitgate/config.go` | 0, 4 | New: minimal YAML reader |
| `internal/gitgate/config_test.go` | 0, 4 | New: ReadConfig tests |
| `internal/gitgate/sentinel.go` | 0 | New: sentinel consume logic |
| `internal/gitgate/sentinel_test.go` | 0 | New |
| `internal/gitgate/log.go` | 0 | New: append-only log |
| `internal/gitgate/log_test.go` | 0 | New |
| `internal/gitgate/resolve.go` | 0 | New: 3-state resolution |
| `internal/gitgate/resolve_test.go` | 0 | New |
| `internal/gitgate/output.go` | 0 | New: deny/allow/warn JSON |
| `internal/gitgate/output_test.go` | 0 | New |
| `internal/gitgate/check.go` | 0, 1, 2 | New + extended per slice |
| `internal/gitgate/check_test.go` | 0, 1, 2 | New + extended |
| `internal/cli/gitgate.go` | 0 | New: RunGitGate entrypoint |
| `internal/cli/gitgate_test.go` | 0 | New |
| `internal/app/app.go` | 0 | Add git-gate dispatch case |
| `internal/components/sdd/inject.go` | 0 | Add ensureClaudeGitGateHook + wire |
| `internal/components/sdd/inject_test.go` | 0 | New hook installer tests |
| `openspec/config.yaml` | 0, 4 | Add strict_workflow + gates + sdd: block |
| `.gitignore` | 0 | Add .gentle-ai/ entry |
| `internal/gitgate/gate_branchbase.go` | 1 | New: branch-base gate |
| `internal/gitgate/gate_branchbase_test.go` | 1 | New |
| `internal/gitgate/gate_orphan.go` | 1 | New: orphan-upstream gate |
| `internal/gitgate/gate_orphan_test.go` | 1 | New |
| `internal/gitgate/gate_sequentialpr.go` | 2 | New: sequential-PR gate |
| `internal/gitgate/gate_sequentialpr_test.go` | 2 | New |
| `internal/assets/skills/sequential-branches/SKILL.md` | 4 | Rewrite with gate references + break-glass |
| `internal/assets/skills/sequential-branches/skill_test.go` | 4 | New content assertion test |
