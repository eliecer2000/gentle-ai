# Gentle AI — Agent Skills Index

When working on this project, load the relevant skill(s) BEFORE writing any code.

Naming convention: `gentle-ai-*` skills are repo-specific workflow skills. Unprefixed skills are portable writing or work-unit skills and intentionally keep their canonical names.

## How to Use

1. Check the trigger column to find skills that match your current task
2. Load the skill by reading the SKILL.md file at the listed path
3. Follow ALL patterns and rules from the loaded skill
4. Multiple skills can apply simultaneously

## Skills

| Skill | Trigger | Path |
|-------|---------|------|
| `gentle-ai-issue-creation` | When creating a GitHub issue, reporting a bug, or requesting a feature. | [`skills/issue-creation/SKILL.md`](skills/issue-creation/SKILL.md) |
| `gentle-ai-branch-pr` | When creating a pull request, opening a PR, or preparing changes for review. | [`skills/branch-pr/SKILL.md`](skills/branch-pr/SKILL.md) |
| `gentle-ai-chained-pr` | When a change is too large for one review, or when creating chained/stacked pull requests. | [`skills/chained-pr/SKILL.md`](skills/chained-pr/SKILL.md) |
| `cognitive-doc-design` | When writing docs that must reduce cognitive load for readers or reviewers. | [`skills/cognitive-doc-design/SKILL.md`](skills/cognitive-doc-design/SKILL.md) |
| `comment-writer` | When drafting human comments, PR feedback, issue replies, or async updates. | [`skills/comment-writer/SKILL.md`](skills/comment-writer/SKILL.md) |
| `work-unit-commits` | When splitting implementation work into deliverable commits or chained PRs. | [`skills/work-unit-commits/SKILL.md`](skills/work-unit-commits/SKILL.md) |
| `sequential-branches` | When `strict_workflow: true` is set — enforces sequential PR gate, atomic commits, and mandatory SDD phases. | [`internal/assets/skills/sequential-branches/SKILL.md`](internal/assets/skills/sequential-branches/SKILL.md) |

## Code Review Standards

These rules apply to all Go source files in this project. Review each staged `.go` file against these standards.

### Correctness
- Error values must be checked; discarding with `_` requires an explicit comment explaining why.
- Exported identifiers must have doc comments.
- `context.Context` must be the first parameter when used; never stored in structs.
- Use `errors.Is` / `errors.As` for error comparison and unwrapping; never `== err`.
- Nil maps/slices are valid zero values; initialize only when the capacity is known or mutated immediately.

### Testing
- New behavioral code must have at least one test covering the happy path.
- Table-driven tests use `t.Run(tt.name, ...)` to name sub-tests by scenario.
- Filesystem tests use `t.TempDir()` — never a hardcoded temp path or real home directory.
- Integration tests that invoke external commands must be skippable via `testing.Short()`.

### Style
- `gofmt` output must match the committed file exactly (no diff).
- `go vet ./...` must pass with zero warnings.
- Package-level variables used only in tests must be unexported.
- Struct literal fields must be named when the struct has more than one field.

### What does NOT block a PASS
- Pre-existing file size or complexity (e.g. large switch in `run.go`) — flag as a note, not a violation.
- The skills index above — it is not a code rule, it is a workflow guide for agents.
- Missing coverage for code paths not touched by the diff under review.
