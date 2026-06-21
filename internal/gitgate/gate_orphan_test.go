package gitgate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pushStdin returns a JSON PreToolUse stdin payload for git push.
func pushStdin() string {
	return `{"tool_name":"Bash","tool_input":{"command":"git push"}}`
}

// pushWithFlagsStdin returns a stdin payload for git push with explicit remote:branch.
func pushWithFlagsStdin() string {
	return `{"tool_name":"Bash","tool_input":{"command":"git push -u origin HEAD"}}`
}

// nonPushStdin returns a git command that is NOT a push operation.
func nonPushStdin() string {
	return `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
}

// initBranchWithUpstream sets up git config to record a branch's upstream.
func setBranchUpstream(t *testing.T, dir, branch, remote, merge string) {
	t.Helper()
	run(t, dir, "git", "config", fmt.Sprintf("branch.%s.remote", branch), remote)
	run(t, dir, "git", "config", fmt.Sprintf("branch.%s.merge", branch), "refs/heads/"+merge)
}

func TestCheckOrphanUpstream(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires git binary")
	}

	tests := []struct {
		name           string
		stdinPayload   string
		cfg            Config
		setupFn        func(t *testing.T, dir, branch string)
		wantAllowed    bool
		wantMsgContain string
	}{
		{
			name:         "non-push tool call: gate not applicable → allow",
			stdinPayload: nonPushStdin(),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"orphan-upstream": ModeEnforce},
			},
			wantAllowed: true,
		},
		{
			name:         "push with upstream = origin → allow",
			stdinPayload: pushStdin(),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"orphan-upstream": ModeEnforce},
			},
			setupFn: func(t *testing.T, dir, branch string) {
				setBranchUpstream(t, dir, branch, "origin", branch)
			},
			wantAllowed: true,
		},
		{
			name:         "push with no upstream configured → deny",
			stdinPayload: pushStdin(),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"orphan-upstream": ModeEnforce},
			},
			// no setupFn: no upstream configured
			wantAllowed:    false,
			wantMsgContain: "no upstream",
		},
		{
			name:         "push with upstream = fork (not origin) → deny",
			stdinPayload: pushStdin(),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"orphan-upstream": ModeEnforce},
			},
			setupFn: func(t *testing.T, dir, branch string) {
				setBranchUpstream(t, dir, branch, "fork", branch)
			},
			wantAllowed:    false,
			wantMsgContain: "not origin",
		},
		{
			name:         "push in warn mode with no upstream → deny (raw domain result; warn applied by Check)",
			stdinPayload: pushStdin(),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"orphan-upstream": ModeWarn},
			},
			// The domain check always returns the raw truth: no upstream = not allowed.
			// The warn → allow translation is applied by Check/CheckWithStdin, not here.
			wantAllowed:    false,
			wantMsgContain: "no upstream",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Fresh dir + git init per sub-test to ensure isolation.
			dir := t.TempDir()
			run(t, dir, "git", "init", ".")
			run(t, dir, "git", "config", "user.email", "test@example.com")
			run(t, dir, "git", "config", "user.name", "Test")
			run(t, dir, "git", "commit", "--allow-empty", "-m", "init")
			run(t, dir, "git", "checkout", "-B", "feat/test-branch")

			branch := "feat/test-branch"
			if tt.setupFn != nil {
				tt.setupFn(t, dir, branch)
			}

			result, err := CheckOrphanUpstream(dir, tt.cfg, strings.NewReader(tt.stdinPayload))
			if err != nil {
				t.Fatalf("CheckOrphanUpstream: %v", err)
			}

			if result.Allowed != tt.wantAllowed {
				t.Errorf("Allowed = %v, want %v (message: %q)", result.Allowed, tt.wantAllowed, result.Message)
			}
			if tt.wantMsgContain != "" && !strings.Contains(strings.ToLower(result.Message), strings.ToLower(tt.wantMsgContain)) {
				t.Errorf("Message = %q, want it to contain %q", result.Message, tt.wantMsgContain)
			}
		})
	}
}

// TestCheckOrphanUpstreamStdinEdgeCases covers stdin parsing edge cases.
func TestCheckOrphanUpstreamStdinEdgeCases(t *testing.T) {
	cfg := Config{
		StrictWorkflow: true,
		Gates:          map[string]GateMode{"orphan-upstream": ModeEnforce},
	}

	t.Run("empty stdin → fail-open (not applicable)", func(t *testing.T) {
		result, err := CheckOrphanUpstream(t.TempDir(), cfg, strings.NewReader(""))
		if err != nil {
			t.Fatalf("CheckOrphanUpstream: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected fail-open on empty stdin, got deny: %q", result.Message)
		}
	})

	t.Run("non-JSON stdin → fail-open (not applicable)", func(t *testing.T) {
		result, err := CheckOrphanUpstream(t.TempDir(), cfg, strings.NewReader("not json"))
		if err != nil {
			t.Fatalf("CheckOrphanUpstream: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected fail-open on non-JSON stdin, got deny: %q", result.Message)
		}
	})
}

// TestOrphanUpstreamIntegrationViaCheck exercises the full Check() pipeline for
// orphan-upstream under enforce and warn modes.
func TestOrphanUpstreamIntegrationViaCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires git binary")
	}

	writeOrphanConfig := func(t *testing.T, dir string, mode GateMode) {
		t.Helper()
		cfgDir := filepath.Join(dir, "openspec")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("strict_workflow: true\nstrict_workflow_gates:\n  orphan-upstream: %s\n", mode)
		if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("enforce + no upstream → deny JSON", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "git", "init", ".")
		run(t, dir, "git", "config", "user.email", "test@example.com")
		run(t, dir, "git", "config", "user.name", "Test")
		run(t, dir, "git", "commit", "--allow-empty", "-m", "init")
		run(t, dir, "git", "checkout", "-B", "feat/orphan")
		writeOrphanConfig(t, dir, ModeEnforce)

		var out strings.Builder
		err := CheckWithStdin("orphan-upstream", dir, strings.NewReader(pushStdin()), &out)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "deny" {
			t.Errorf("want deny, got %q (output=%s)", decision, out.String())
		}
	})

	t.Run("enforce + upstream = origin → allow JSON", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "git", "init", ".")
		run(t, dir, "git", "config", "user.email", "test@example.com")
		run(t, dir, "git", "config", "user.name", "Test")
		run(t, dir, "git", "commit", "--allow-empty", "-m", "init")
		run(t, dir, "git", "checkout", "-B", "feat/tracked")
		setBranchUpstream(t, dir, "feat/tracked", "origin", "feat/tracked")
		writeOrphanConfig(t, dir, ModeEnforce)

		var out strings.Builder
		err := CheckWithStdin("orphan-upstream", dir, strings.NewReader(pushStdin()), &out)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("want allow, got %q (output=%s)", decision, out.String())
		}
	})

	t.Run("warn + no upstream → allow JSON + log entry", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "git", "init", ".")
		run(t, dir, "git", "config", "user.email", "test@example.com")
		run(t, dir, "git", "config", "user.name", "Test")
		run(t, dir, "git", "commit", "--allow-empty", "-m", "init")
		run(t, dir, "git", "checkout", "-B", "feat/warn-orphan")
		writeOrphanConfig(t, dir, ModeWarn)

		var out strings.Builder
		err := CheckWithStdin("orphan-upstream", dir, strings.NewReader(pushStdin()), &out)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("warn mode should allow, got %q", decision)
		}
		logPath := filepath.Join(dir, ".gentle-ai", "git-gate.log")
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("log file not found: %v", err)
		}
		if !strings.Contains(string(data), "orphan-upstream") {
			t.Errorf("log missing orphan-upstream entry: %q", data)
		}
	})
}
