# skill-hardening Specification

## Purpose

Specifies the hardening of `internal/assets/skills/sequential-branches/SKILL.md`
to reference the deterministic gates and break-glass protocol, and the
persistence of chain/delivery strategy outside the session cache so it survives
compaction.

## Requirements

### Requirement: Sequential-Branches Skill References Gates

The `sequential-branches/SKILL.md` MUST be updated to:

- Explicitly reference the three git gates (branch-base, orphan-upstream,
  sequential-PR) by name.
- Explain the three-state model (`enforce` / `warn` / `off`) to the agent.
- Document the sentinel-file break-glass protocol: when to use it, how to
  create the file, and the single-operation consequence.
- State clearly that on non-Claude agents the gates are advisory-only and the
  skill text is the enforcement surface.

The skill MUST NOT describe implementation details (Go packages, binary paths,
internal JSON format). It MUST describe observable behavior and user-facing
protocol only.

#### Scenario: Skill loaded by Claude Code agent references gate names

- GIVEN the updated `sequential-branches/SKILL.md` is loaded
- WHEN an agent reads the skill
- THEN the agent MUST see all three gate names listed
- AND MUST see the three-state model described in plain language
- AND MUST see instructions for creating a sentinel file as a break-glass

#### Scenario: Skill informs non-Claude agent of advisory status

- GIVEN the updated skill is loaded by a non-blocking agent (Cursor, OpenCode, etc.)
- WHEN the agent reads the skill
- THEN the agent MUST see a notice that gate enforcement is advisory on this
  platform and MUST follow the rules as best-effort guidance

---

### Requirement: Chain/Delivery Strategy Persisted to openspec/config.yaml

The chain and delivery strategy choices (e.g., `stacked-to-main`,
`feature-branch-chain`, `auto-chain`, `ask-on-risk`) MUST be persisted in
`openspec/config.yaml` under a `sdd:` block after the user confirms them. They
MUST NOT live only in session cache (engram or orchestrator memory).

On session start, the orchestrator MUST read persisted strategy from
`openspec/config.yaml` when available and use it without re-asking the user.

#### Scenario: Strategy saved to config after user confirmation

- GIVEN the user confirms a delivery strategy and chain strategy
- WHEN the SDD orchestrator persists the choice
- THEN `openspec/config.yaml` MUST be updated with the strategy values under
  `sdd.delivery_strategy` and `sdd.chain_strategy`

#### Scenario: Strategy recovered from config on session start

- GIVEN a prior session persisted `delivery_strategy: auto-chain` and
  `chain_strategy: stacked-to-main` in `openspec/config.yaml`
- WHEN a new session starts and the orchestrator reads the config
- THEN the orchestrator MUST use `auto-chain` and `stacked-to-main`
- AND MUST NOT ask the user again unless they explicitly request a change

#### Scenario: Absent strategy in config falls back to asking

- GIVEN no `sdd:` block in `openspec/config.yaml`
- WHEN a new session invokes `/sdd-new` or `/sdd-continue`
- THEN the orchestrator MUST ask the user for delivery and chain strategy
- AND MUST offer to persist the choice to `openspec/config.yaml`
