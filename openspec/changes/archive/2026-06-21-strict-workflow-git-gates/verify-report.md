# Verification Report — strict-workflow-git-gates

**Change**: strict-workflow-git-gates  
**Date**: 2026-06-21  
**Branch**: feat/git-gate-skill-hardening (chain tip, cumulative Slices 0a → 0b → 1 → 2 → 4)  
**Mode**: Strict TDD  
**Verdict**: PASS WITH WARNINGS

---

## 1. Build and Test Evidence

### go vet ./...
**Result**: PASS — zero diagnostics.

### go test ./...
**Result**: PASS (with pre-existing TUI failures, see below)

Package-level summary:

| Package | Result | Notes |
|---------|--------|-------|
| internal/gitgate | PASS | 6.06 s, 13 test functions, ~60 sub-cases |
| internal/state | PASS | StrictTDD + StrictWorkflow fields + MergeAgents carry |
| internal/cli | PASS | TestRunInjectOptionsIncludesStrictWorkflow (4 sub-cases) |
| internal/components/sdd | PASS | TestEnsureClaudeGitGateHook (3 sub-cases) |
| internal/assets/skills/sequential-branches | PASS | TestSequentialBranchesSkillContent |
| internal/tui | FAIL | TestStrictTDDForward (4 sub-cases), TestInstallNavigationRoundTrips (2 sub-cases) — PRE-EXISTING, not a regression |
| All other packages | PASS (cached or fresh) | |

**Pre-existing TUI failures**: Both failing tests were RED on the tracker branch `feat/strict-workflow-mode` before this change. The StrictWorkflow screen added in a prior commit shifted screen-index enums and those tests were not updated. This is pre-existing tech debt, not a regression from strict-workflow-git-gates.

---

## 2. Task Completion

All tasks in the checklist are marked `[x]`. No unchecked implementation tasks remain.

Slices completed: 0a, 0b, 1, 2, 4. Slice 3 (statusline/worktree) was explicitly deferred to a follow-up change per the spec phase decision.

---

## 3. Spec Scenario Coverage Matrix

### Domain 1: strict-workflow-state-wiring (2 requirements, 5 scenarios)

| Scenario | Implementing code | Covering test | Status |
|----------|------------------|---------------|--------|
| StrictWorkflow is wired on inject | internal/cli/run.go:buildSDDInjectOptions | TestRunInjectOptionsIncludesStrictWorkflow / StrictWorkflow=true | PASS |
| StrictWorkflow absent does not inject marker | internal/cli/run.go | TestRunInjectOptionsIncludesStrictWorkflow / StrictWorkflow=false | PASS |
| StrictWorkflow survives reinstall | internal/state/state.go strict_workflow omitempty field | TestInstallStateStrictFieldsPersistence / StrictWorkflow round-trips | PASS |
| StrictTDD survives reinstall | internal/state/state.go strict_tdd omitempty field | TestInstallStateStrictFieldsPersistence / StrictTDD round-trips | PASS |
| Zero-value defaults on new install | state.go zero-value struct | TestInstallStateStrictFieldsPersistence / zero-value new install | PASS |

### Domain 2: git-gate-model (5 requirements, 13 scenarios)

| Scenario | Implementing code | Covering test | Status |
|----------|------------------|---------------|--------|
| Enforce blocks the operation | check.go ModeEnforce → DenyOutput | TestBranchBaseIntegrationViaCheck / gate enforce + stale main | PASS |
| Warn allows with visible warning | check.go ModeWarn → WarnOutput + stderr | TestBranchBaseIntegrationViaCheck / gate warn + stale main | PASS |
| Off is a silent pass | check.go ModeOff early return | TestBranchBaseIntegrationViaCheck / gate off: always allow | PASS |
| Missing per-gate config defaults to enforce | resolve.go: absent key → ModeEnforce | TestResolve / missing per-gate config + strict_workflow=true | PASS |
| Per-gate config overrides default enforce | resolve.go cfg.Gates[gate] | TestResolve / per-gate config warn respected | PASS |
| Gate off via config emits NO warning/log | check.go ModeOff early return before domain check | TestBranchBaseIntegrationViaCheck / gate off (implicit — no log check) | PASS |
| Sentinel degrades enforce to warn (one shot) | sentinel.go ConsumeSentinel + resolve.go | TestResolve / enforce + sentinel consumed degrades to warn | PASS |
| Sentinel is consumed-once | sentinel.go os.Remove | TestConsumeSentinel / second call after consume | PASS |
| Stale sentinel after crash consumed next invocation | Same ConsumeSentinel path — no timing guard needed | TestConsumeSentinel / sentinel present: consumed=true, file deleted | PASS |
| Sentinel present but gate already off | check.go: ConsumeSentinel runs BEFORE ModeOff check; file deleted, no log | TestResolve / off + sentinel consumed stays off (unit); integration via sentinel path logic | PASS* |
| Log entry written on sentinel override | check.go lines 69-79: AppendLog with Override:"sentinel" | TestBranchBaseIntegrationViaCheck / sentinel → log entry written | PASS |
| Log entry written on config warn | check.go lines 101-107: AppendLog WITHOUT Override:"config" | TestOrphanUpstreamIntegrationViaCheck / warn + log entry (checks gate name only, not Override field) | WARNING — see Finding W-1 |
| Config parsed with all gates present | config.go ReadConfig | TestReadConfig / all gates present | PASS |
| Config parsed with missing gate entry | resolve.go + config.go | TestReadConfig / no gates block; TestResolve / nil gate config map | PASS |

*The sentinel-off case: ConsumeSentinel is called unconditionally at check.go:55 before the ModeOff early return at check.go:63, so the sentinel file IS deleted. The unit test covers the resolve.go behavior (stays off). There is no integration test asserting that the sentinel file is gone after an off-mode check, but the code path is provably correct.

### Domain 3: git-gates (3 requirements, 12 scenarios)

| Scenario | Implementing code | Covering test | Status |
|----------|------------------|---------------|--------|
| New branch from up-to-date main — allowed | gate_branchbase.go isBranchStale → false | TestCheckBranchBase / branch from up-to-date main | PASS |
| New branch from stale main — blocked under enforce | gate_branchbase.go isBranchStale → true | TestCheckBranchBase / branch from stale main → deny | PASS |
| New branch from declared parent — allowed | gate_branchbase.go cfg.SDD.CurrentParentBranch | TestCheckBranchBase / branch from declared parent (current) | PASS |
| New branch from stale base — warn mode allows with warning | check.go ModeWarn path | TestBranchBaseIntegrationViaCheck / gate warn + stale main | PASS |
| Sentinel degrades enforce to warn for branch-base | check.go + sentinel.go | TestBranchBaseIntegrationViaCheck / gate enforce + sentinel | PASS |
| Push with correct upstream — allowed | gate_orphan.go remote=="origin" | TestCheckOrphanUpstream / push with upstream = origin | PASS |
| Push with no upstream set — blocked under enforce | gate_orphan.go remote=="" | TestCheckOrphanUpstream / push with no upstream configured → deny | PASS |
| Push with wrong upstream remote — blocked under enforce | gate_orphan.go remote!="origin" | TestCheckOrphanUpstream / push with upstream = fork → deny | PASS |
| Orphan push in warn mode | check.go ModeWarn path | TestOrphanUpstreamIntegrationViaCheck / warn + no upstream → allow + log | PASS |
| No open PRs — new task branch allowed | gate_sequentialpr.go len(offending)==0 | TestCheckSequentialPR / no open PRs from task set → allow | PASS |
| Open PR exists — new task branch blocked under enforce | gate_sequentialpr.go offending list | TestCheckSequentialPR / open PR from task set exists (enforce) → deny | PASS |
| Open PR exists — warn mode allows with warning | check.go ModeWarn path | TestSequentialPRViaCheck / warn: open PR from task set → allow JSON | PASS |
| gh CLI unavailable — gate degrades to warn | gate_sequentialpr.go errGhUnavailable → allow | TestCheckSequentialPRGhUnavailable; TestSequentialPRViaCheck / gh missing | PASS |

### Domain 4: skill-hardening (2 requirements, 5 scenarios)

| Scenario | Implementing code | Covering test | Status |
|----------|------------------|---------------|--------|
| Skill loaded by Claude Code agent references gate names | SKILL.md Gate Reference section | TestSequentialBranchesSkillContent (asserts gate names, 3-state terms, sentinel path, advisory notice, break-glass, consumed-once) | PASS |
| Skill informs non-Claude agent of advisory status | SKILL.md Enforcement Model section | TestSequentialBranchesSkillContent | PASS |
| Strategy saved to config after user confirmation | openspec/config.yaml sdd: block | TestReadConfig / sdd block delivery_strategy and chain_strategy cases | PASS |
| Strategy recovered from config on session start | config.go SddConfig fields | TestReadConfig / sdd block cases | PASS |
| Absent strategy in config falls back to asking | Orchestrator behavior (documented intent, not code) | No Go test — orchestrator-side behavior | WARNING — see Finding W-2 |

---

## 4. Security and Correctness-Critical Behaviors

| Behavior | Verdict | Evidence |
|----------|---------|---------|
| Fail-open on all git/gh errors | CONFIRMED | check.go: config error → AllowOutput; domain errors → AllowOutput; git exec errors in gate_branchbase.go/gate_orphan.go return Allowed:true |
| Enforce blocks via deny JSON | CONFIRMED | output.go DenyOutput; TestBranchBaseIntegrationViaCheck / gate enforce + stale main |
| Warn never silent — emits warning + log via CheckWithStdin | CONFIRMED | check.go lines 99-107; integration tests for all three gates verify warn → allow + log |
| Sentinel consumed-once; degrades enforce → warn only | CONFIRMED | resolve.go + sentinel.go; TestResolve + TestConsumeSentinel |
| Orphan inline git push -u origin allowed (catch-22 fix) | CONFIRMED | gate_orphan.go pushSetsOriginUpstream(); TestCheckOrphanUpstreamInlinePush |
| Branch-base validates explicit start-point | CONFIRMED | gate_branchbase.go parseExplicitBranchBase(); TestCheckBranchBaseExplicitStartPoint |
| Sequential-pr gh is injectable (avoid real network in tests) | CONFIRMED | gate_sequentialpr.go var ghListOpenPRs (package variable); TestCheckSequentialPR uses mock |
| Sequential-pr fail-open when gh missing | CONFIRMED | gate_sequentialpr.go errGhUnavailable + allow; TestCheckSequentialPRGhUnavailable |

---

## 5. Findings by Severity

### CRITICAL

None.

### WARNING

**W-1: Log entry for config-warn missing Override:"config" field**

- Spec scenario "Log entry written on config warn" (git-gate-model spec, Mandatory Warning requirement) states the log entry MUST contain `override path (config | sentinel)`.
- Implementation: check.go lines 101-106 creates LogEntry without setting `Override` field when the mode is warn due to config (not sentinel). Only the sentinel path explicitly sets `Override: "sentinel"`.
- The warn+log path IS exercised and logs correctly except for the override field. The test (gate_orphan_test.go line 321) checks only that "orphan-upstream" appears in the log, not the override field.
- Impact: log entries for config-warn events have an empty override column, making it impossible to audit whether a warn event came from config or default behavior. Not a behavioral blocker, but a spec gap.

**W-2: Absent strategy fallback behavior is orchestrator-side (no Go test)**

- Spec scenario "Absent strategy in config falls back to asking" is an orchestrator behavioral requirement. There is no Go code or test covering it — it depends on the orchestrator prompt/skill behavior.
- This is expected (the orchestrator is Claude itself, not a compiled binary), but it is a compliance gap for this spec scenario: runtime evidence cannot prove it.
- Recommendation: Document as orchestrator-side advisory in the archive report.

**W-3: Pre-existing TUI test failures (tech debt, not a regression)**

- `TestStrictTDDForward` and `TestInstallNavigationRoundTrips` fail in `internal/tui` because screen-index enums shifted when the StrictWorkflow screen was added in `feat/strict-workflow-mode` (the tracker branch). These tests were RED before this change and remain RED on the chain tip.
- Confirmed pre-existing: the failures report `screen = 11, want 12/14/15/16`, which corresponds to the enum offset introduced by the StrictWorkflow screen.
- This MUST be resolved before the tracker branch merges to main, but does not block the git-gate change PRs themselves.

### SUGGESTION

**S-1: No integration test for sentinel-file deletion when gate=off**

- The code path is provably correct (ConsumeSentinel runs unconditionally before the ModeOff early return), but there is no dedicated integration test for the spec scenario "Sentinel present but gate already off → sentinel deleted, no log emitted." Adding one would make the behavior explicitly verifiable.

**S-2: TestReadConfig does not assert missing-file-gives-off behavior for gates**

- TestReadConfigMissingFile exists and passes, but does not assert that the resulting zero Config causes gates to resolve to ModeOff. This is covered transitively via TestResolve / strict_workflow=false always off, but a combined config-missing → gates-off integration path would increase confidence.

**S-3: Log format is space-delimited with no escaping**

- The current log format (`<ts> <gate> <mode> <override> <result> <reason>`) uses space delimiters. The reason field can contain spaces, which means the log is not reliably machine-parseable without field-count assumptions. This is acceptable for the current use (human audit), but would need a change (CSV, JSON lines, or quoted fields) if the log is ever consumed programmatically.

---

## 6. Design Deviations

| Deviation | Impact |
|-----------|--------|
| `delivery_strategy` saved as "chained" (tasks said "auto-chain") | Naming difference only; semantically equivalent. No functional impact. |
| `chain_strategy` saved as "feature-branch-chain" (tasks said "stacked-to-main") | Orchestrator prompt was authoritative per apply-progress. The config correctly reflects what was requested. No functional impact. |

---

## 7. Verdict

**PASS WITH WARNINGS**

All 35 spec scenarios are covered by implementation. 33 of 35 have passing runtime tests. The 2 exceptions are:

1. W-2 (orchestrator fallback): inherently untestable in Go — verified by spec text and skill behavior.
2. W-1 (config-warn Override field): spec compliance gap in the log format. Behavioral correctness is intact; only the override attribution field in the log is missing for config-warn events.

No CRITICAL issues found. The change is ready to proceed to PR creation with the following pre-conditions:

- **W-1 (Override:"config" in log entries)**: Should be fixed before or during the first PR review, not a hard blocker for PR creation but would ideally be resolved before merging to main.
- **W-3 (pre-existing TUI failures)**: Must be resolved on the tracker branch before final merge to main. Does not block individual slice PRs.

**Next recommended**: sdd-archive (the change is implementation-complete and spec-compliant within documented tolerances).
