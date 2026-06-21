package gitgate

import (
	"encoding/json"
	"io"
	"strings"
)

// toolCallInput represents the Claude PreToolUse hook stdin JSON structure.
// Claude passes tool call details as JSON on stdin when invoking the hook.
type toolCallInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// parseToolCallCommand reads a Claude PreToolUse JSON payload from r and
// returns the Bash command string. Returns ("", nil) when stdin is empty,
// non-JSON, or when the tool is not Bash — all treated as not-applicable
// (fail-open: a parsing failure must never wedge the agent).
func parseToolCallCommand(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil // fail-open: treat read error as not-applicable
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return "", nil // empty stdin: not applicable
	}

	var tc toolCallInput
	if err := json.Unmarshal(data, &tc); err != nil {
		return "", nil // non-JSON: not applicable
	}

	// Only Bash tool calls are relevant for git gate enforcement.
	if tc.ToolName != "Bash" {
		return "", nil
	}

	return tc.ToolInput.Command, nil
}

// isNewBranchCommand reports whether cmd is a git command that creates a new
// branch (git checkout -b or git branch <name>).
func isNewBranchCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 3 {
		return false
	}
	// Must start with "git".
	if fields[0] != "git" {
		return false
	}
	switch fields[1] {
	case "checkout":
		// git checkout -b <name> [base]
		for _, f := range fields[2:] {
			if f == "-b" || f == "-B" {
				return true
			}
		}
		return false
	case "branch":
		// git branch <name> [base] — creation only (not -d, -D, -m, --list, etc.)
		if len(fields) >= 3 {
			arg := fields[2]
			// Reject flag-like arguments that are branch management, not creation.
			if strings.HasPrefix(arg, "-") {
				return false
			}
			return true
		}
		return false
	}
	return false
}

// isPushCommand reports whether cmd is a git push command.
func isPushCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return false
	}
	return fields[0] == "git" && fields[1] == "push"
}

// pushSetsOriginUpstream reports whether cmd is a git push that explicitly
// sets an origin upstream inline via -u / --set-upstream followed by the
// remote name "origin". When true, the push command itself establishes correct
// origin tracking, so the orphan-upstream gate must allow it even if
// branch.<name>.remote is not yet configured.
func pushSetsOriginUpstream(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "git" || fields[1] != "push" {
		return false
	}
	args := fields[2:]
	setUpstream := false
	for i, f := range args {
		if f == "-u" || f == "--set-upstream" {
			setUpstream = true
			continue
		}
		// The first non-flag argument after -u/--set-upstream is the remote name.
		if setUpstream && !strings.HasPrefix(f, "-") {
			_ = i
			return f == "origin"
		}
	}
	return false
}

// parseExplicitBranchBase returns the explicit start-point argument from a
// git new-branch command, or "" when none is provided.
//
// Recognised forms:
//   - git checkout -b <new> <base>
//   - git checkout -B <new> <base>
//   - git branch <new> <base>
func parseExplicitBranchBase(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return ""
	}
	if fields[0] != "git" {
		return ""
	}
	switch fields[1] {
	case "checkout":
		// Find -b/-B flag position; the argument after it is <new>, the one after
		// that (if present and non-flag) is the explicit <base>.
		for i, f := range fields[2:] {
			idx := i + 2 // index in fields
			if f == "-b" || f == "-B" {
				// fields[idx+1] = <new>, fields[idx+2] = <base> (if present)
				if idx+2 < len(fields) && !strings.HasPrefix(fields[idx+2], "-") {
					return fields[idx+2]
				}
				return ""
			}
		}
	case "branch":
		// git branch <new> [base] — skip flag arguments.
		nonFlag := []string{}
		for _, f := range fields[2:] {
			if !strings.HasPrefix(f, "-") {
				nonFlag = append(nonFlag, f)
			}
		}
		// nonFlag[0] = <new>, nonFlag[1] = <base> (if present)
		if len(nonFlag) >= 2 {
			return nonFlag[1]
		}
	}
	return ""
}
