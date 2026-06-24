package gitgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendLog(t *testing.T) {
	t.Run("log file created on first entry", func(t *testing.T) {
		dir := t.TempDir()
		entry := LogEntry{
			Gate:   "branch-base",
			Mode:   ModeEnforce,
			Result: "deny",
			Reason: "stale base",
		}
		if err := AppendLog(dir, "branch-base", entry); err != nil {
			t.Fatalf("AppendLog() error = %v", err)
		}
		logPath := filepath.Join(dir, ".gentle-ai", "git-gate.log")
		if _, err := os.Stat(logPath); err != nil {
			t.Fatalf("log file not created: %v", err)
		}
	})

	t.Run("entries are appended, not overwritten", func(t *testing.T) {
		dir := t.TempDir()
		e1 := LogEntry{Gate: "branch-base", Mode: ModeEnforce, Result: "deny", Reason: "first"}
		e2 := LogEntry{Gate: "orphan-upstream", Mode: ModeWarn, Result: "warn", Reason: "second"}
		if err := AppendLog(dir, "branch-base", e1); err != nil {
			t.Fatal(err)
		}
		if err := AppendLog(dir, "orphan-upstream", e2); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".gentle-ai", "git-gate.log"))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 log lines, got %d:\n%s", len(lines), data)
		}
	})

	t.Run("entry contains timestamp, gate, mode, result, reason", func(t *testing.T) {
		dir := t.TempDir()
		entry := LogEntry{
			Gate:     "sequential-pr",
			Mode:     ModeWarn,
			Override: "sentinel",
			Result:   "allow",
			Reason:   "sentinel override consumed",
		}
		if err := AppendLog(dir, "sequential-pr", entry); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".gentle-ai", "git-gate.log"))
		if err != nil {
			t.Fatal(err)
		}
		line := string(data)
		for _, want := range []string{"sequential-pr", "warn", "sentinel", "allow", "sentinel override consumed"} {
			if !strings.Contains(line, want) {
				t.Errorf("log line missing %q:\n%s", want, line)
			}
		}
	})
}
