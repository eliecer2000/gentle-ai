# Design: strict-workflow-git-gates

## Technical Approach

Move git invariants from prompt text into a deterministic **PreToolUse(Bash) hook** that
calls a new Go subcommand `gentle-ai git-gate check`. The subcommand resolves a 3-state
mode (`enforce|warn|off`) per gate, inspects live git state, honors a consumed-once sentinel
override, and emits Claude hook JSON. Slice 0 lands the foundation + the two confirmed bug
fixes; Slices 1/2 add real gate logic; Slice 4 hardens the skill. Statusline/worktree is
split to a follow-up. Scope: Slices 0,1,2,4.

## Architecture Decisions

| Decision | Choice | Alternatives rejected | Rationale |
|---|---|---|---|
| Validator form | Go subcommand `internal/gitgate/` | Embedded shell script | One binary on PATH, cross-platform, reuses Go types, atomic updates. Shell breaks on Windows + separate deploy path. |
| Override channel | Sentinel file `.gentle-ai/git-gate-override/<gate>` | git-config entry; env var via `CLAUDE_ENV_FILE` | Hook sees no prompt text. Env-file only works from hook scripts, not prompts. Filename **is** the gate (self-documenting), gitignored, no git plumbing. |
| Per-gate config home | `openspec/config.yaml` (`strict_workflow_gates:`) | state.json (global) | Gate modes are per-project; config.yaml is version-controlled + already agent-referenced. Global `strict_workflow` bool stays in state.json. |
| YAML reader | **Minimal scoped reader** (no dep) for Slice 0 | `gopkg.in/yaml.v3` | go.mod has no YAML dep. The surface is one flat `key: value` block under `strict_workflow_gates:`. A bounded line-scanner avoids a new third-party dependency. Adopt `yaml.v3` only if the config surface grows (documented fallback). |
| Hook decision shape | JSON `permissionDecision:"deny"` | exit code 2 | JSON carries a structured `permissionDecisionReason` (the visible warning) and is unambiguous; exit-2 conflates errors with denials. |
| Cross-agent boundary | Claude: deterministic PreToolUse. Codex/others: advisory SessionStart/prompt only | Block everywhere | Only Claude exposes per-tool blocking. Documented limitation, not a defect. |
| state.json back-compat | Add `omitempty` fields; absent = false | Versioned migration | Zero-value bool is correct default; existing state.json decodes unchanged. |

## 3-State Resolution Algorithm

```
resolve(gate, cwd):
  if state.json strict_workflow == false: return off          # global kill switch
  mode = config.strict_workflow_gates[gate]  (default: enforce)
  if mode == off:  return off
  if sentinel exists at .gentle-ai/git-gate-override/<gate>:
      delete sentinel (consumed-once)                          # break-glass
      log("override consumed", gate)
      if mode == enforce: mode = warn                          # degrade one op
  return mode
```

`enforce` → run gate check; on fail emit `deny`. `warn` → run check; on fail emit `allow` +
visible reason + log. `off` → emit `allow` silently.

## Sentinel Override Protocol

- Path: `.gentle-ai/git-gate-override/<gate>` (empty file; name = gate).
- Writer: the agent, instructed by the hardened skill (Slice 4), before the gated Bash call.
- Deleter: `git-gate check` (consumed-once) on the next hook invocation.
- Gitignore: install MUST ensure `.gentle-ai/` is gitignored (sentinel must never commit).
- Crash recovery: if the agent crashes after writing, the orphan sentinel degrades exactly
  one future op to `warn`, is then deleted, and is logged — harmless and self-healing.

## Hook Installation

New `ensureClaudeGitGateHook(settingsPath)`, sibling to `ensureClaudeSkillRegistryHook`,
wired from `installSkillRegistryAutomation` (or a renamed sibling). Idempotent via
`claudeHookExists`. PreToolUse entry:

```json
{ "matcher": "Bash",
  "hooks": [{ "type": "command",
    "command": "gentle-ai git-gate check --gate branch-base --cwd \"${CLAUDE_PROJECT_DIR:-$PWD}\"",
    "timeout": 30 }] }
```

Slice 0 installs a single no-op pass gate to prove the path end-to-end; later slices add
gate-specific matchers/commands. Codex: advisory `SessionStart` note only (no blocking).

## Data Flow

```
Claude Bash tool-call ──> PreToolUse hook ──> gentle-ai git-gate check
                                                   │
            ┌──────── reads strict_workflow (state.json, global) ◄──┘
            ├──────── reads strict_workflow_gates[gate] (openspec/config.yaml)
            ├──────── consumes sentinel (.gentle-ai/git-gate-override/<gate>)
            ├──────── inspects git (branch, upstream, base) + gh (open PRs)
            └──────── stdout JSON {permissionDecision} + stderr log ──> Claude
```

## Subcommand Contract

`gentle-ai git-gate check --gate <name> --cwd <dir>`

- **Input**: args only (`--gate`, `--cwd`). Tool-call JSON arrives on stdin and MAY be
  parsed for richer matching later; Slice 0 ignores stdin.
- **Output (deny)**: stdout `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"<gate> blocked: <why>"}}`, exit 0.
- **Output (allow)**: stdout `{"hookSpecificOutput":{"permissionDecision":"allow"}}`, exit 0.
- **Warn**: allow JSON + human warning on **stderr** + log entry. Internal errors → exit 0 +
  allow (fail-open: a broken gate must never wedge the agent).

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/gitgate/gitgate.go` | Create | Resolve 3-state, git/gh inspection, sentinel consume, JSON output |
| `internal/gitgate/config.go` | Create | Minimal scoped reader for `strict_workflow_gates:` |
| `internal/gitgate/*_test.go` | Create | Unit tests (resolution, sentinel, JSON shape) |
| `internal/app/app.go` | Modify | Add `case "git-gate"` to subcommand dispatch |
| `internal/cli/run.go` (~790) | Modify | Add `StrictWorkflow: s.selection.StrictWorkflow` to `InjectOptions` |
| `internal/state/state.go` | Modify | Add `StrictTDD`, `StrictWorkflow` (omitempty) to `InstallState`; carry in `MergeAgents` |
| `internal/components/sdd/inject.go` | Modify | Add `ensureClaudeGitGateHook`; wire from install automation |
| `openspec/config.yaml` | Modify | Add `strict_workflow: true` + `strict_workflow_gates:` block |
| `internal/assets/skills/sequential-branches/SKILL.md` | Modify (Slice 4) | Reference gates + break-glass sentinel protocol |

## Config Schema (openspec/config.yaml)

```yaml
strict_workflow: true
strict_workflow_gates:
  branch-base: enforce      # new branch from updated main / declared parent
  orphan-upstream: enforce  # new branch must set correct upstream
  sequential-pr: warn       # block if open PRs from task set exist
```

## state.json Additions

```go
StrictTDD      bool `json:"strict_tdd,omitempty"`
StrictWorkflow bool `json:"strict_workflow,omitempty"`
```

Absent in old files → `false` (safe; gates become no-ops until reinstall persists `true`).

## Sequence: Blocked branch creation (enforce)

```
Agent      Claude        Hook            git-gate          git
 │  Bash    │  PreToolUse │   check        │                 │
 ├─────────►├────────────►│ --gate         │                 │
 │          │             ├ resolve=enforce│                 │
 │          │             ├ no sentinel    │                 │
 │          │             ├───────────────►│ base != main?   │
 │          │             │                ├────────────────►│
 │          │             │                │◄── stale base ──┤
 │          │◄── deny JSON ┤◄── deny ───────┤                 │
 │◄ blocked ┤ (reason)    │                 │                 │
```

## Sequence: Informed override (sentinel → warn + log)

```
Skill→Agent  touch .gentle-ai/git-gate-override/branch-base
Agent  Bash ─► Claude PreToolUse ─► Hook ─► git-gate check
                                              ├ resolve: enforce + sentinel
                                              ├ delete sentinel (consumed-once)
                                              ├ degrade enforce→warn; log override
                                              ├ git check fails
                                              └► allow JSON + stderr warning
Claude ─► tool runs; warning + override recorded in log
```

## Logging / Warning Surface

- **Deny reason**: `permissionDecisionReason` (shown to user by Claude).
- **Warn text**: hook **stderr** (Claude surfaces non-blocking stderr).
- **Override + decision log**: append-only `.gentle-ai/git-gate.log` (gitignored), one line
  per decision: `ts gate mode result reason`. Guarantees the flow never changes silently.

## Testing Strategy

| Layer | What | How |
|---|---|---|
| Unit | resolution matrix, sentinel consume-once, JSON shape, config reader, fail-open | `go test ./internal/gitgate/...` |
| Unit | run.go wiring, state.json round-trip + back-compat | existing patterns in `internal/cli`, `internal/state` |
| Integration | hook installer idempotency (PreToolUse) | `inject.go` test pattern (`ensure*Hook`) |

## Migration / Rollout

No data migration. Back-compat via `omitempty` zero-values. Rollback: per-gate `off`,
global `strict_workflow: false`, or idempotent hook uninstall.

## Open Questions

- [ ] Exact `--gate` names per slice (`orphan-upstream` vs split) — finalize in tasks.
- [ ] Whether `git-gate check` parses stdin tool-call JSON in Slice 1 for arg-aware matching.
