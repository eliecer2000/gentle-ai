package gitgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// errGhUnavailable is the sentinel error returned by ghListOpenPRs when the gh
// CLI is not on PATH. The sequential-pr gate treats it as fail-open (allow).
var errGhUnavailable = errors.New("gh CLI unavailable")

// openPR is the subset of `gh pr list --json headRefName,number` output the
// sequential-pr gate needs.
type openPR struct {
	HeadRefName string `json:"headRefName"`
	Number      int    `json:"number"`
}

// ghListOpenPRs returns the open pull requests for the repo in cwd via the gh
// CLI. It is a package variable so tests can inject a fake without invoking gh
// or the network. Returns errGhUnavailable when gh is not installed.
var ghListOpenPRs = func(cwd string) ([]openPR, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "list", "--state", "open", "--json", "headRefName,number")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}
	var prs []openPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("parse gh pr list output: %w", err)
	}
	return prs, nil
}

// checkSequentialPRWith validates that no open PR from the current task set
// exists before a new branch is created. It is triggered by git checkout -b /
// git branch commands.
//
// Like the other gate validators it returns the RAW truth: Allowed=false when a
// blocking PR exists, regardless of the configured mode. The enforce/warn/off
// translation (and the non-silent warning + log) is applied by CheckWithStdin.
//
// Fail-open contract: a missing gh CLI, any gh error, or an empty task set
// returns Allowed=true so enforcement never wedges the agent.
func checkSequentialPRWith(cwd string, cfg Config, stdin io.Reader, ghFunc func(cwd string) ([]openPR, error)) (GateResult, error) {
	cmd, err := parseToolCallCommand(stdin)
	if err != nil || cmd == "" {
		// Empty or unparseable stdin: not applicable, fail-open.
		return GateResult{Allowed: true}, nil
	}
	if !isNewBranchCommand(cmd) {
		// Not a branch-creation command: gate not applicable.
		return GateResult{Allowed: true}, nil
	}

	// No task set declared: nothing to gate against. Skip without calling gh.
	if len(cfg.SDD.TaskBranches) == 0 {
		return GateResult{Allowed: true}, nil
	}

	prs, err := ghFunc(cwd)
	if err != nil {
		if errors.Is(err, errGhUnavailable) {
			return GateResult{
				Allowed: true,
				Message: "sequential-pr gate skipped: gh CLI unavailable",
			}, nil
		}
		// Any other gh failure (auth, network, non-zero exit): fail-open.
		return GateResult{Allowed: true}, nil
	}

	taskSet := make(map[string]bool, len(cfg.SDD.TaskBranches))
	for _, b := range cfg.SDD.TaskBranches {
		taskSet[b] = true
	}

	var offending []openPR
	for _, pr := range prs {
		if taskSet[pr.HeadRefName] {
			offending = append(offending, pr)
		}
	}
	if len(offending) == 0 {
		return GateResult{Allowed: true}, nil
	}

	parts := make([]string, 0, len(offending))
	for _, pr := range offending {
		parts = append(parts, fmt.Sprintf("#%d (%s)", pr.Number, pr.HeadRefName))
	}
	return GateResult{
		Allowed: false,
		Message: fmt.Sprintf("open PR(s) from the current task set must be merged before branching: %s", strings.Join(parts, ", ")),
	}, nil
}
