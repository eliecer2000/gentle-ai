# strict-workflow-state-wiring Specification

## Purpose

Specifies the required wiring and persistence behavior for `StrictWorkflow` and
`StrictTDD` flags, correcting two confirmed bugs: missing injection in
`run.go` and missing persistence in `InstallState`.

## Requirements

### Requirement: StrictWorkflow Injection

The system MUST pass `StrictWorkflow` from `model.Selection` into
`sdd.InjectOptions` when building the inject options for any agent session. This
MUST happen at the same call site where `StrictTDD` is already wired (~line 790
of `internal/cli/run.go`).

The strict-workflow marker MUST appear in the injected prompt context whenever
the user's selection has `StrictWorkflow: true`.

#### Scenario: StrictWorkflow is wired on inject

- GIVEN a `model.Selection` with `StrictWorkflow: true`
- WHEN `internal/cli/run.go` builds `sdd.InjectOptions` for the agent session
- THEN the resulting `InjectOptions.StrictWorkflow` field MUST equal `true`
- AND the injected agent context MUST contain the strict-workflow marker

#### Scenario: StrictWorkflow absent does not inject marker

- GIVEN a `model.Selection` with `StrictWorkflow: false`
- WHEN `internal/cli/run.go` builds `sdd.InjectOptions`
- THEN `InjectOptions.StrictWorkflow` MUST equal `false`
- AND no strict-workflow marker is injected

---

### Requirement: InstallState Persistence

The system MUST persist both `StrictWorkflow` and `StrictTDD` booleans in
`InstallState` (the JSON structure written to `state.json`). After a
sync/reinstall cycle both values MUST be recoverable from `state.json` without
re-prompting the user.

#### Scenario: StrictWorkflow survives reinstall

- GIVEN an installed project with `StrictWorkflow: true` in `InstallState`
- WHEN a sync or reinstall writes `state.json`
- THEN `state.json` MUST contain `"strict_workflow": true`
- AND loading `InstallState` after reinstall MUST yield `StrictWorkflow: true`

#### Scenario: StrictTDD survives reinstall

- GIVEN an installed project with `StrictTDD: true` in `InstallState`
- WHEN a sync or reinstall writes `state.json`
- THEN `state.json` MUST contain `"strict_tdd": true`
- AND loading `InstallState` after reinstall MUST yield `StrictTDD: true`

#### Scenario: Zero-value defaults on new install

- GIVEN a fresh install with no prior `state.json`
- WHEN `InstallState` is initialized
- THEN `StrictWorkflow` MUST default to `false`
- AND `StrictTDD` MUST default to `false`
