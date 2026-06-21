package gitgate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Check runs the gate named gate in the directory cwd, reading stdin from
// os.Stdin. This is the main entry point used by the git-gate CLI.
//
// Fail-open contract: any internal error must never block the agent.
// On internal error, allow JSON is written and the error is returned to the
// caller for stderr logging.
func Check(gate, cwd string, stdout io.Writer) error {
	return CheckWithStdin(gate, cwd, os.Stdin, stdout)
}

// CheckWithStdin is the full implementation of Check, with injectable stdin.
// It is used by tests to inject simulated Claude PreToolUse payloads without
// depending on os.Stdin.
func CheckWithStdin(gate, cwd string, stdin io.Reader, stdout io.Writer) error {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stdin == nil {
		stdin = strings.NewReader("")
	}

	// Resolve project directory.
	projectDir := cwd
	if projectDir == "" {
		projectDir = os.Getenv("CLAUDE_PROJECT_DIR")
	}
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			_, _ = stdout.Write(AllowOutput())
			return fmt.Errorf("gitgate: resolve cwd: %w", err)
		}
	}

	configPath := filepath.Join(projectDir, "openspec", "config.yaml")
	cfg, err := ReadConfig(configPath)
	if err != nil {
		_, _ = stdout.Write(AllowOutput())
		return fmt.Errorf("gitgate: read config: %w", err)
	}

	// Consume the break-glass sentinel for this gate.
	sentinelPath := SentinelPath(projectDir, gate)
	consumed, err := ConsumeSentinel(sentinelPath)
	if err != nil {
		consumed = false
	}

	mode := Resolve(cfg, gate, consumed)

	if mode == ModeOff {
		_, _ = stdout.Write(AllowOutput())
		return nil
	}

	// When sentinel was consumed and we degraded from enforce → warn, log it
	// before the domain check so it is always recorded.
	if consumed && mode == ModeWarn {
		entry := LogEntry{
			Gate:     gate,
			Mode:     mode,
			Override: "sentinel",
			Result:   "allow",
			Reason:   "break-glass sentinel consumed; enforced operation degraded to warn",
		}
		_ = AppendLog(projectDir, gate, entry)
		_, _ = fmt.Fprintf(os.Stderr, "WARNING [git-gate/%s]: break-glass sentinel consumed; this operation runs with warnings only\n", gate)
	}

	// Run the domain check for the requested gate.
	result, domainErr := runDomainCheck(gate, projectDir, cfg, stdin)
	if domainErr != nil {
		// Domain check failure: fail-open.
		_, _ = stdout.Write(AllowOutput())
		return fmt.Errorf("gitgate: domain check %s: %w", gate, domainErr)
	}

	if result.Allowed {
		_, _ = stdout.Write(AllowOutput())
		return nil
	}

	// Gate check failed. Apply the resolved mode.
	switch mode {
	case ModeEnforce:
		_, _ = stdout.Write(DenyOutput(gate, result.Message))
	case ModeWarn:
		_, _ = stdout.Write(WarnOutput(gate, result.Message))
		_, _ = fmt.Fprintf(os.Stderr, "WARNING [git-gate/%s]: %s\n", gate, result.Message)
		entry := LogEntry{
			Gate:     gate,
			Mode:     mode,
			Override: "config",
			Result:   "warn",
			Reason:   result.Message,
		}
		_ = AppendLog(projectDir, gate, entry)
	default:
		// ModeOff is handled above; should not reach here.
		_, _ = stdout.Write(AllowOutput())
	}

	return nil
}

// runDomainCheck dispatches to the gate-specific check function.
// Returns GateResult{Allowed: true} for unknown gate names (fail-open).
func runDomainCheck(gate, cwd string, cfg Config, stdin io.Reader) (GateResult, error) {
	switch gate {
	case "branch-base":
		return CheckBranchBase(cwd, cfg, stdin)
	case "orphan-upstream":
		return CheckOrphanUpstream(cwd, cfg, stdin)
	default:
		// Unknown gate: treat as no-op (allow). This preserves backward
		// compatibility if new gate names are added before the binary is updated.
		return GateResult{Allowed: true}, nil
	}
}
