package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/gentleman-programming/gentle-ai/internal/gitgate"
)

// RunGitGate implements the `gentle-ai git-gate check --gate <name> --cwd <dir>` subcommand.
//
// It parses --gate and --cwd flags, delegates to gitgate.Check, and handles
// the fail-open contract: on any internal error, allow JSON is written to
// stdout and the error is logged to stderr rather than returned (so Claude
// Code never gets wedged by a broken gate).
func RunGitGate(args []string, stdout io.Writer) error {
	if stdout == nil {
		stdout = os.Stdout
	}

	var gate, cwd string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--gate":
			if i+1 >= len(args) {
				return fmt.Errorf("git-gate: --gate requires a value")
			}
			i++
			gate = args[i]
		case "--cwd":
			if i+1 >= len(args) {
				return fmt.Errorf("git-gate: --cwd requires a value")
			}
			i++
			cwd = args[i]
		}
	}

	if gate == "" {
		return fmt.Errorf("git-gate: --gate <name> is required")
	}

	if err := gitgate.Check(gate, cwd, stdout); err != nil {
		// Fail-open: allow JSON was already written by Check.
		// Log the internal error to stderr but do NOT return it — returning an
		// error would cause the CLI to exit non-zero, which Claude Code may
		// interpret differently from a deny JSON response.
		_, _ = fmt.Fprintf(os.Stderr, "git-gate internal error (fail-open): %v\n", err)
	}
	return nil
}
