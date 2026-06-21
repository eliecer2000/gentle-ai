package gitgate

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// CheckOrphanUpstream validates that the current branch has a correctly
// configured upstream before a git push. It is triggered by git push commands.
//
// Rules:
//   - If the stdin payload is not a git push command: return allowed (not applicable).
//   - If the push command itself sets an origin upstream inline via -u /
//     --set-upstream followed by "origin": return allowed. The command is
//     establishing correct tracking, so blocking it would create a catch-22.
//   - The branch must have branch.<name>.remote set to "origin".
//   - Any deviation from the above blocks the push under enforce.
//
// Fail-open contract: any git execution error returns allowed.
func CheckOrphanUpstream(cwd string, cfg Config, stdin io.Reader) (GateResult, error) {
	cmd, err := parseToolCallCommand(stdin)
	if err != nil || cmd == "" {
		// Empty or unparseable stdin: not applicable, fail-open.
		return GateResult{Allowed: true}, nil
	}

	if !isPushCommand(cmd) {
		// Not a push command: gate not applicable.
		return GateResult{Allowed: true}, nil
	}

	// If the push itself sets origin as the upstream (-u / --set-upstream origin),
	// allow it unconditionally: the command is establishing correct tracking and
	// blocking it would create a catch-22 (you can't set origin upstream if the
	// gate requires origin upstream to already exist).
	if pushSetsOriginUpstream(cmd) {
		return GateResult{Allowed: true}, nil
	}

	// Determine the current branch.
	branch, err := gitCurrentBranch(cwd)
	if err != nil {
		return GateResult{Allowed: true}, nil // fail-open
	}

	// Read upstream remote config.
	remote, err := gitConfigBranchRemote(cwd, branch)
	if err != nil {
		// Config lookup error: treat as not configured (blocked under enforce).
		remote = ""
	}

	if remote == "" {
		return GateResult{
			Allowed: false,
			Message: fmt.Sprintf("branch %q has no upstream set; run 'git push -u origin %s' to set it", branch, branch),
		}, nil
	}

	if remote != "origin" {
		return GateResult{
			Allowed: false,
			Message: fmt.Sprintf("branch %q upstream is not origin (got: %q); only origin is allowed", branch, remote),
		}, nil
	}

	return GateResult{Allowed: true}, nil
}

// gitConfigBranchRemote returns the value of branch.<name>.remote from git config.
// Returns an empty string when the key is not set.
func gitConfigBranchRemote(cwd, branch string) (string, error) {
	key := fmt.Sprintf("branch.%s.remote", branch)
	cmd := exec.Command("git", "config", key)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		// git config exits non-zero when the key is absent — that's not an error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("git config %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}
