---
name: sequential-branches
description: "Trigger: strict_workflow: true in openspec/config.yaml. Enforce sequential PR gate, atomic commits, and mandatory SDD phases."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Load this skill when `strict_workflow: true` is present in `openspec/config.yaml`. It activates automatically via the skill registry when StrictWorkflow mode was enabled at install time.

## Hard Rules

| Rule | Requirement |
|---|---|
| Sequential PR gate | Before creating any branch, verify zero open PRs exist from the current task set. Block if unmerged PRs are found. |
| Atomic commits | Each commit MUST contain one deliverable behavior: the implementation, its tests, and any related docs — together, never split by file type. |
| Mandatory SDD phases | Every phase (explore → propose → spec → design → tasks → apply → verify → archive) MUST complete before the next begins. No phase may be skipped without explicit user confirmation and a recorded reason. |
| Branch from latest main | Always run `git fetch && git checkout main && git merge upstream/main` before creating a new branch. |

## Decision Gates

| Condition | Action |
|---|---|
| Open PRs detected before branching | STOP. List open PRs. Ask user to merge or explicitly override with rationale. |
| Commit contains mixed concerns | STOP. Split into separate atomic commits before proceeding. |
| User asks to skip a phase | Ask for rationale. If accepted, record it in engram via `mem_save`. |
| `strict_workflow: false` or key absent | This skill is inactive — defer to standard `work-unit-commits` and `branch-pr` skills. |

## Execution Steps

1. On task start: run `gh pr list --state open` to check for unmerged PRs.
2. If any open PRs exist: surface them and block branch creation until resolved.
3. Create branch with `git checkout -b <type>/<short-description>` from updated main.
4. Implement using atomic work-unit commits (code + tests + docs per commit).
5. Before advancing to the next SDD phase: confirm the current phase artifact is complete.
6. On PR open: verify all prior task PRs are merged; reject if any are still open.

## Output Contract

On gate failure: report which PRs are open, which rule was violated, and what action unblocks progress. Never silently continue past a blocked gate.

## References

- [work-unit-commits/SKILL.md](../work-unit-commits/SKILL.md) — atomic commit rules
- [branch-pr/SKILL.md](../branch-pr/SKILL.md) — branch naming and PR creation
