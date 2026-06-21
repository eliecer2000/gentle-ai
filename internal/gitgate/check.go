package gitgate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Check runs the gate named gate in the directory cwd.
//
// In Slice 0 the gate domain check is a no-op stub that always returns allow.
// The full resolution pipeline (config read → sentinel consume → mode resolve →
// domain check → output) is exercised so the wiring is proven end-to-end.
// Slices 1 and 2 replace the stub with real git/gh inspection.
//
// Fail-open contract: any internal error must never block the agent.
// On internal error, allow JSON is written and the error is returned to the
// caller for stderr logging.
func Check(gate, cwd string, stdout io.Writer) error {
	if stdout == nil {
		stdout = os.Stdout
	}

	// Resolve config path: openspec/config.yaml under cwd, or fallback to
	// CLAUDE_PROJECT_DIR env var when cwd is not set.
	projectDir := cwd
	if projectDir == "" {
		projectDir = os.Getenv("CLAUDE_PROJECT_DIR")
	}
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			// Fail-open: cannot determine project dir.
			_, _ = stdout.Write(AllowOutput())
			return fmt.Errorf("gitgate: resolve cwd: %w", err)
		}
	}

	configPath := filepath.Join(projectDir, "openspec", "config.yaml")
	cfg, err := ReadConfig(configPath)
	if err != nil {
		// Fail-open on config read error.
		_, _ = stdout.Write(AllowOutput())
		return fmt.Errorf("gitgate: read config: %w", err)
	}

	// Consume the break-glass sentinel for this gate.
	sentinelPath := SentinelPath(projectDir, gate)
	consumed, err := ConsumeSentinel(sentinelPath)
	if err != nil {
		// Sentinel delete failure: treat as not consumed, continue.
		consumed = false
	}

	mode := Resolve(cfg, gate, consumed)

	if mode == ModeOff {
		_, _ = stdout.Write(AllowOutput())
		return nil
	}

	// In Slice 0, the domain check is always allow.
	// Real gate logic is added in Slices 1 and 2.
	result := GateResult{Allowed: true, Mode: mode, SentinelConsumed: consumed}

	// When sentinel was consumed under enforce (now degraded to warn), log it.
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

	if result.Allowed {
		_, _ = stdout.Write(AllowOutput())
		return nil
	}

	// Domain check failed (unreachable in Slice 0 stub, but wired for completeness).
	if mode == ModeEnforce {
		_, _ = stdout.Write(DenyOutput(gate, result.Message))
		return nil
	}
	// mode == ModeWarn: allow + emit warning.
	_, _ = stdout.Write(WarnOutput(gate, result.Message))
	_, _ = fmt.Fprintf(os.Stderr, "WARNING [git-gate/%s]: %s\n", gate, result.Message)
	entry := LogEntry{
		Gate:   gate,
		Mode:   mode,
		Result: "warn",
		Reason: result.Message,
	}
	_ = AppendLog(projectDir, gate, entry)
	return nil
}
