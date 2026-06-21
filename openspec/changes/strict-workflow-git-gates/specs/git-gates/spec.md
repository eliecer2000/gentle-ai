# git-gates Specification

## Purpose

Specifies the three concrete enforcement gates: branch-base validation,
orphan/upstream validation, and sequential-PR validation. Each gate inherits
the three-state model from `git-gate-model` spec; this spec defines the
domain-specific trigger conditions and enforce/warn outcomes for each.

## Requirements

### Requirement: Gate 1 — Branch-Base Validation

When a new branch is created (via `git checkout -b` or `git branch`), the
system MUST verify that the branch base is either:

- An up-to-date `main` branch (HEAD at or ahead of `origin/main`), OR
- The declared parent branch for the current change as recorded in the project
  state.

Under `enforce`: if neither condition is met, the operation MUST be blocked.
Under `warn`: the operation is allowed and a warning is emitted.
Under `off`: no check is performed.

The gate MUST NOT block if the branch is being created from a branch that is
already up-to-date with its declared parent.

#### Scenario: New branch from up-to-date main — allowed

- GIVEN `strict_workflow: true` and gate `branch-base` in `enforce`
- AND `main` is up-to-date with `origin/main` (no divergence)
- WHEN `git checkout -b feature/my-task` is executed from `main`
- THEN the gate MUST allow the operation (exit zero, no block)

#### Scenario: New branch from stale main — blocked under enforce

- GIVEN `strict_workflow: true` and gate `branch-base` in `enforce`
- AND local `main` is behind `origin/main` by at least one commit
- WHEN `git checkout -b feature/my-task` from `main`
- THEN the gate MUST block with `permissionDecision: "deny"`
- AND the blocking message MUST state that `main` is out of date

#### Scenario: New branch from declared parent — allowed

- GIVEN `strict_workflow: true` and gate `branch-base` in `enforce`
- AND the declared parent for the current task is `feat/tracker`
- AND `feat/tracker` is current
- WHEN `git checkout -b feat/child-task` from `feat/tracker`
- THEN the gate MUST allow the operation

#### Scenario: New branch from stale base — warn mode allows with warning

- GIVEN gate `branch-base` in `warn`
- AND local `main` is stale
- WHEN `git checkout -b feature/my-task` from stale `main`
- THEN the gate MUST allow the operation
- AND MUST emit a visible warning naming the stale base
- AND MUST log the event to `.gentle-ai/git-gate.log`

#### Scenario: Sentinel degrades enforce to warn for branch-base

- GIVEN gate `branch-base` in `enforce` and a stale base
- AND `.gentle-ai/git-gate-override/branch-base` exists
- WHEN `git checkout -b feature/my-task`
- THEN the operation is ALLOWED (warn behavior)
- AND the sentinel file is deleted
- AND a warning is emitted stating sentinel override consumed

---

### Requirement: Gate 2 — Orphan/Upstream Validation

When a branch is pushed to a remote (`git push`), the system MUST verify that
the branch has a correct upstream set (`branch.<name>.remote` and
`branch.<name>.merge` are both configured and point to `origin`).

Under `enforce`: branches without a correct upstream MUST be blocked from
pushing.
Under `warn`: the push is allowed with a visible warning.
Under `off`: no check.

#### Scenario: Push with correct upstream — allowed

- GIVEN gate `orphan-upstream` in `enforce`
- AND the branch has `upstream = origin/<branch>` configured
- WHEN `git push` is executed
- THEN the gate MUST allow the operation

#### Scenario: Push with no upstream set — blocked under enforce

- GIVEN gate `orphan-upstream` in `enforce`
- AND the branch has NO upstream configured
- WHEN `git push` is executed
- THEN the gate MUST block with `permissionDecision: "deny"`
- AND the message MUST state the branch is orphaned and instruct setting upstream

#### Scenario: Push with wrong upstream remote — blocked under enforce

- GIVEN gate `orphan-upstream` in `enforce`
- AND the branch upstream points to a remote other than `origin`
- WHEN `git push` is executed
- THEN the gate MUST block
- AND the message MUST identify the incorrect upstream

#### Scenario: Orphan push in warn mode

- GIVEN gate `orphan-upstream` in `warn`
- AND no upstream is set
- WHEN `git push` is executed
- THEN the push is ALLOWED
- AND a warning is emitted and logged

---

### Requirement: Gate 3 — Sequential-PR Validation

When a new task branch would be created, the system MUST check whether any open
PRs from the current task set already exist (via `gh pr list --json
headRefName`). If open PRs are found targeting the same base as the proposed
new branch, the creation MUST be blocked under `enforce`.

The "current task set" is the set of branches associated with the active SDD
change as recorded in the project state.

Under `enforce`: creating a new task branch while open PRs from the task set
exist MUST be blocked.
Under `warn`: allowed with visible warning.
Under `off`: no check.

#### Scenario: No open PRs — new task branch allowed

- GIVEN gate `sequential-pr` in `enforce`
- AND `gh pr list` returns no open PRs from the current task set
- WHEN a new task branch is created
- THEN the gate MUST allow the operation

#### Scenario: Open PR exists — new task branch blocked under enforce

- GIVEN gate `sequential-pr` in `enforce`
- AND `gh pr list` returns at least one open PR from the current task set
- WHEN a new task branch creation is attempted
- THEN the gate MUST block with `permissionDecision: "deny"`
- AND the message MUST list the conflicting open PR(s) by branch name and PR number

#### Scenario: Open PR exists — warn mode allows with warning

- GIVEN gate `sequential-pr` in `warn`
- AND at least one open PR from the task set exists
- WHEN a new task branch is created
- THEN the operation is ALLOWED
- AND a warning is emitted listing the open PRs
- AND the event is logged

#### Scenario: gh CLI unavailable — gate degrades to warn

- GIVEN gate `sequential-pr` in `enforce`
- AND the `gh` CLI binary is not on PATH
- WHEN the gate check runs
- THEN the gate MUST NOT block (cannot query PRs)
- AND MUST emit a warning that the PR check was skipped due to missing `gh` CLI
- AND MUST log the skip event
