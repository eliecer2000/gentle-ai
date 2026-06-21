# git-gate-model Specification

## Purpose

Specifies the three-state enforcement model, the two override paths
(config and sentinel), the mandatory warning/logging contract, and the
cross-agent advisory fallback that applies where blocking hooks are unavailable.

## Requirements

### Requirement: Three-State Gate Model

Every git gate MUST operate in exactly one of three states:

- `enforce` — the operation is BLOCKED; the blocking JSON output MUST be
  returned to the hook with `permissionDecision: "deny"`.
- `warn` — the operation is ALLOWED; a visible warning MUST be emitted to
  stderr and an entry MUST be written to the gate log.
- `off` — no check is performed; the gate is a silent no-op.

When `strict_workflow: true` is set globally, every gate whose per-gate config
entry is absent MUST default to `enforce`.

When `strict_workflow: false` (or absent), every gate MUST default to `off`
regardless of per-gate config.

#### Scenario: Enforce blocks the operation

- GIVEN a gate in `enforce` state
- WHEN `gentle-ai git-gate check --gate <name>` is invoked by the hook
- THEN the command MUST exit non-zero
- AND MUST output blocking JSON with `permissionDecision: "deny"`
- AND MUST include a human-readable `message` explaining which gate fired and why

#### Scenario: Warn allows with visible warning

- GIVEN a gate in `warn` state
- WHEN `gentle-ai git-gate check --gate <name>` is invoked
- THEN the command MUST exit zero (allow)
- AND MUST emit a visible warning to stderr naming the gate, the violation, and that it was overridden
- AND MUST write a log entry to `.gentle-ai/git-gate.log` with timestamp, gate name, and reason

#### Scenario: Off is a silent pass

- GIVEN a gate in `off` state
- WHEN `gentle-ai git-gate check --gate <name>` is invoked
- THEN the command MUST exit zero with no output and no log entry

#### Scenario: Missing per-gate config defaults to enforce under strict_workflow

- GIVEN `strict_workflow: true` in `state.json`
- AND no entry for a gate under `strict_workflow_gates:` in `openspec/config.yaml`
- WHEN that gate is checked
- THEN the gate MUST behave as `enforce`

---

### Requirement: Config Override (Durable)

The system MUST support a per-gate mode override in `openspec/config.yaml`
under the `strict_workflow_gates:` key. Valid per-gate values are `enforce`,
`warn`, and `off`. The config file is the authoritative per-project, version-
controlled override surface.

Every gate whose config entry is set to `warn` or `off` MUST still emit a
visible warning on each invocation documenting that the gate is not enforcing,
so the user is always informed.

#### Scenario: Per-gate config overrides default enforce

- GIVEN `strict_workflow: true` in `state.json`
- AND `strict_workflow_gates: { branch-base: warn }` in `openspec/config.yaml`
- WHEN the branch-base gate is checked
- THEN the gate MUST behave as `warn` (allow + warn + log)
- AND the warning MUST state the gate name and that it was config-overridden

#### Scenario: Gate off via config emits informational notice

- GIVEN `strict_workflow: true` in `state.json`
- AND `strict_workflow_gates: { orphan-upstream: off }` in `openspec/config.yaml`
- WHEN the orphan-upstream gate is checked
- THEN the gate MUST be a no-op (exit zero, no block)
- AND the command MUST NOT produce any warning or log entry (off = silent pass)

---

### Requirement: Sentinel Override (One-Shot Break-Glass)

The system MUST support a consumed-once sentinel file mechanism. When the skill
instructs the agent to create `.gentle-ai/git-gate-override/<gate>`, the hook
MUST:

1. Detect the sentinel file's presence before evaluating the gate.
2. Degrade the gate state from `enforce` to `warn` for that single invocation.
3. Delete the sentinel file immediately after reading it.
4. Emit a visible warning naming the gate, that a sentinel override was consumed,
   and log the event.

The sentinel MUST NOT be effective when the gate is already `warn` or `off`
(no double-degradation; the gate simply runs at its configured state).

#### Scenario: Sentinel degrades enforce to warn for one operation

- GIVEN a gate in `enforce` state
- AND a sentinel file `.gentle-ai/git-gate-override/<gate>` exists
- WHEN the gate check runs
- THEN the gate MUST behave as `warn` (allow + warn + log)
- AND the sentinel file MUST be deleted before the command exits
- AND the warning MUST state "sentinel override consumed"

#### Scenario: Sentinel is consumed-once

- GIVEN a sentinel override was consumed in the previous invocation
- WHEN the same gate is checked again
- THEN no sentinel file exists
- AND the gate MUST enforce normally (block)

#### Scenario: Stale sentinel after agent crash is consumed next invocation

- GIVEN an agent crashed after writing `.gentle-ai/git-gate-override/<gate>`
  but before the Bash call ran
- WHEN the gate is next checked (by any operation)
- THEN the sentinel MUST be read, the gate MUST behave as `warn`, and the file
  MUST be deleted
- AND the log MUST record the consumed sentinel with the current timestamp

#### Scenario: Sentinel present but gate already off

- GIVEN `strict_workflow_gates: { branch-base: off }` in config
- AND a sentinel file `.gentle-ai/git-gate-override/branch-base` exists
- WHEN the gate is checked
- THEN the gate MUST behave as `off` (silent pass)
- AND the sentinel file MUST be deleted to avoid accumulation
- AND no warning or log entry is emitted

---

### Requirement: Mandatory Warning and Log on Every Override

The flow MUST NEVER change silently. For every override — whether config-level
(`warn`/`off`) or sentinel — the system MUST:

- Emit at least one visible warning to stderr during the affected operation.
- Write a structured log entry to `.gentle-ai/git-gate.log` containing:
  timestamp (ISO 8601), gate name, resolved state, override path
  (`config` | `sentinel`), and the operation context.

The log file MUST be created if absent. It MUST be append-only per invocation.

#### Scenario: Log entry written on sentinel override

- GIVEN a sentinel override is consumed for gate `branch-base`
- WHEN the gate check exits
- THEN `.gentle-ai/git-gate.log` MUST contain a new line with timestamp, gate
  name `branch-base`, state `warn`, override `sentinel`

#### Scenario: Log entry written on config warn

- GIVEN gate `orphan-upstream` is configured as `warn`
- WHEN the gate check exits after allowing an operation
- THEN `.gentle-ai/git-gate.log` MUST contain a new line with timestamp, gate
  name `orphan-upstream`, state `warn`, override `config`

---

### Requirement: Cross-Agent Advisory Fallback

For agents that do NOT support blocking PreToolUse hooks (OpenCode, Cursor,
Windsurf, Gemini, Kiro), the gates MUST NOT be treated as failing to enforce.

The system MUST document that enforcement is advisory-only on those agents,
delivered through the `sequential-branches` skill text. This is an architectural
boundary, not a defect.

The `git-gate check` subcommand MUST still be callable on these platforms and
MUST return a non-blocking advisory output (exit zero) when invoked without a
hook context, so CI scripts and manual invocations still surface the warning.

#### Scenario: Claude Code enforces deterministically

- GIVEN `strict_workflow: true` and a gate in `enforce` state
- WHEN a Claude Code PreToolUse hook invokes `gentle-ai git-gate check`
- THEN the hook receives blocking JSON and the tool call is denied

#### Scenario: Non-Claude agent receives advisory output only

- GIVEN the same gate in `enforce` state
- AND the invoking agent is OpenCode, Cursor, Windsurf, Gemini, or Kiro
  (no blocking-hook mechanism)
- WHEN the skill text is rendered to that agent
- THEN the gate rules MUST appear as advisory warnings in the skill
- AND no deterministic block occurs
- AND this behavior MUST NOT be reported as a gate defect

---

### Requirement: openspec/config.yaml Schema for Gates

The `openspec/config.yaml` MUST accept the following new keys:

```yaml
strict_workflow: true          # global bool (mirrors state.json)
strict_workflow_gates:
  branch-base: enforce         # enforce | warn | off
  orphan-upstream: enforce
  sequential-pr: enforce
```

A minimal Go reader scoped to these keys MUST parse them. The reader MUST
tolerate absent keys and default missing gates to `enforce` when
`strict_workflow: true`.

#### Scenario: Config parsed with all gates present

- GIVEN a valid `openspec/config.yaml` with all three gates set
- WHEN the Go reader loads the file
- THEN each gate's resolved state MUST match the config value

#### Scenario: Config parsed with missing gate entry

- GIVEN `openspec/config.yaml` with `strict_workflow: true` and no
  `strict_workflow_gates:` block
- WHEN the Go reader loads the file
- THEN all three gates MUST resolve to `enforce`
