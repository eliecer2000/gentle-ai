package gitgate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LogEntry holds the data for one gate decision log line.
type LogEntry struct {
	// Timestamp is set by AppendLog when zero.
	Timestamp time.Time
	// Gate is the gate name (e.g. "branch-base").
	Gate string
	// Mode is the effective mode that produced the result.
	Mode GateMode
	// Override describes how the mode was modified ("sentinel", "config", or "").
	Override string
	// Result is the final outcome ("deny", "warn", "allow").
	Result string
	// Reason is a human-readable explanation.
	Reason string
}

// AppendLog writes one decision log line to .gentle-ai/git-gate.log under cwd.
// The log is append-only; each line follows the format:
//
//	<ISO8601> <gate> <mode> <override> <result> <reason>
//
// The directory is created if it does not exist.
func AppendLog(cwd, gate string, entry LogEntry) error {
	logDir := filepath.Join(cwd, ".gentle-ai")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("gitgate log: create dir: %w", err)
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	ts := entry.Timestamp.Format(time.RFC3339)
	line := fmt.Sprintf("%s %s %s %s %s %s\n",
		ts,
		entry.Gate,
		string(entry.Mode),
		entry.Override,
		entry.Result,
		entry.Reason,
	)

	logPath := filepath.Join(logDir, "git-gate.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("gitgate log: open: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
