package gitgate

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// CheckBranchBase validates that the current HEAD branch is an acceptable base
// for a new branch creation. It is triggered by git checkout -b or git branch
// commands.
//
// Rules:
//   - If the stdin payload is not a new-branch command: return allowed (not applicable).
//   - Acceptable bases are:
//     1. main — and local main must not be behind origin/main.
//     2. cfg.SDD.CurrentParentBranch (if non-empty) — and it must not be behind
//     its origin counterpart.
//   - Any other base, or an out-of-date acceptable base: return not allowed.
//
// Fail-open contract: any git execution error returns allowed so a transient
// git failure never wedges the agent.
func CheckBranchBase(cwd string, cfg Config, stdin io.Reader) (GateResult, error) {
	cmd, err := parseToolCallCommand(stdin)
	if err != nil || cmd == "" {
		// Empty or unparseable stdin: not applicable, fail-open.
		return GateResult{Allowed: true}, nil
	}

	if !isNewBranchCommand(cmd) {
		// Not a branch-creation command: gate not applicable.
		return GateResult{Allowed: true}, nil
	}

	// If the command supplies an explicit start-point, validate that instead of
	// current HEAD. This prevents git checkout -b feat/x <wrong-base> from
	// passing because current HEAD happens to be an allowed branch.
	var baseBranch string
	if explicit := parseExplicitBranchBase(cmd); explicit != "" {
		baseBranch = explicit
	} else {
		// No explicit base: fall back to current HEAD (original behavior).
		var err2 error
		baseBranch, err2 = gitCurrentBranch(cwd)
		if err2 != nil {
			// Cannot determine base: fail-open.
			return GateResult{Allowed: true}, nil
		}
	}

	// Determine which base branches are acceptable.
	allowedBases := []string{"main"}
	if cfg.SDD.CurrentParentBranch != "" {
		allowedBases = append(allowedBases, cfg.SDD.CurrentParentBranch)
	}

	// Check if the current base is in the allowed list.
	baseAllowed := false
	for _, b := range allowedBases {
		if baseBranch == b {
			baseAllowed = true
			break
		}
	}
	if !baseAllowed {
		return GateResult{
			Allowed: false,
			Message: fmt.Sprintf("base branch %q is not allowed; must branch from main or the declared parent branch", baseBranch),
		}, nil
	}

	// Check that the accepted base is up-to-date with its origin counterpart.
	stale, reason, err := isBranchStale(cwd, baseBranch)
	if err != nil {
		// Git execution error: fail-open.
		return GateResult{Allowed: true}, nil
	}
	if stale {
		return GateResult{
			Allowed: false,
			Message: fmt.Sprintf("base branch %q is out of date with origin/%s: %s; pull or rebase before branching", baseBranch, baseBranch, reason),
		}, nil
	}

	return GateResult{Allowed: true}, nil
}

// gitCurrentBranch returns the short name of the current branch in cwd.
func gitCurrentBranch(cwd string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --abbrev-ref HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// isBranchStale reports whether the local branch is behind its origin
// counterpart by at least one commit.
//
// Strategy:
//  1. Fetch origin/<branch> (dry-run to avoid mutating state in tests that
//     don't have a real remote — but we actually do a real fetch here because
//     --dry-run for fetch does not update FETCH_HEAD). Instead we use
//     git fetch origin <branch> to refresh the remote-tracking ref.
//  2. Compare local HEAD vs origin/<branch> using git rev-parse.
//  3. Count commits origin/<branch> is ahead using git rev-list.
func isBranchStale(cwd, branch string) (stale bool, reason string, err error) {
	// Fetch the remote-tracking ref to get the latest origin state.
	fetchCmd := exec.Command("git", "fetch", "origin", branch)
	fetchCmd.Dir = cwd
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		// Fetch failure is non-fatal: we fall back to comparing with the
		// existing remote-tracking ref, which may be stale but is safe.
		_ = out
	}

	// Get local HEAD SHA.
	localSHA, err := gitRevParse(cwd, "HEAD")
	if err != nil {
		return false, "", err
	}

	// Get origin/<branch> SHA.
	originRef := "origin/" + branch
	originSHA, err := gitRevParse(cwd, originRef)
	if err != nil {
		// Remote ref absent: treat as not stale (branch may be new).
		return false, "", nil
	}

	if localSHA == originSHA {
		return false, "", nil
	}

	// Count how many commits origin is ahead of local.
	countCmd := exec.Command("git", "rev-list", "--count", "HEAD.."+originRef)
	countCmd.Dir = cwd
	countOut, err := countCmd.Output()
	if err != nil {
		return false, "", nil // fail-open
	}
	ahead := strings.TrimSpace(string(countOut))
	if ahead == "0" {
		// Local is ahead of or equal to origin — not stale.
		return false, "", nil
	}

	return true, fmt.Sprintf("local is %s commit(s) behind origin/%s", ahead, branch), nil
}

// gitRevParse returns the full SHA for a git ref.
func gitRevParse(cwd, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}
