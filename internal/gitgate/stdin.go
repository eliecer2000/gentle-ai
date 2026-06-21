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
