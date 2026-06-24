package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGitGate(t *testing.T) {
	t.Run("missing --gate returns error", func(t *testing.T) {
		var buf bytes.Buffer
		err := RunGitGate([]string{"check"}, &buf)
		if err == nil {
			t.Fatal("RunGitGate(no --gate) error = nil, want error")
		}
		if !strings.Contains(err.Error(), "--gate") {
			t.Errorf("error %q should mention --gate", err.Error())
		}
	})

	t.Run("valid --gate=branch-base, strict_workflow=false => allow JSON", func(t *testing.T) {
		dir := t.TempDir()
		// Write minimal openspec/config.yaml with strict_workflow=false.
		configDir := filepath.Join(dir, "openspec")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("strict_workflow: false\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		err := RunGitGate([]string{"check", "--gate", "branch-base", "--cwd", dir}, &buf)
		if err != nil {
			t.Fatalf("RunGitGate() error = %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("output not valid JSON: %v\n%s", err, buf.String())
		}
		hso, ok := result["hookSpecificOutput"].(map[string]any)
		if !ok {
			t.Fatalf("missing hookSpecificOutput in: %s", buf.String())
		}
		if hso["permissionDecision"] != "allow" {
			t.Errorf("permissionDecision = %q, want %q", hso["permissionDecision"], "allow")
		}
	})

	t.Run("missing openspec/config.yaml => fail-open allow JSON", func(t *testing.T) {
		dir := t.TempDir()
		// No config file at all — should fail-open.
		var buf bytes.Buffer
		err := RunGitGate([]string{"check", "--gate", "branch-base", "--cwd", dir}, &buf)
		// Fail-open: no error returned to caller (already emitted allow JSON).
		if err != nil {
			t.Fatalf("RunGitGate(missing config) error = %v, want nil (fail-open)", err)
		}
		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("output not valid JSON: %v\n%s", err, buf.String())
		}
		hso, ok := result["hookSpecificOutput"].(map[string]any)
		if !ok {
			t.Fatalf("missing hookSpecificOutput in: %s", buf.String())
		}
		if hso["permissionDecision"] != "allow" {
			t.Errorf("permissionDecision = %q, want %q (fail-open)", hso["permissionDecision"], "allow")
		}
	})
}
