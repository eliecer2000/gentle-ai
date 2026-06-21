package gitgate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initBareRepo initialises a bare git repo at path and returns the file:// URL.
func initBareRepo(t *testing.T, path string) string {
	t.Helper()
	run(t, path, "git", "init", "--bare", ".")
	return "file://" + path
}

// initWorkingRepo initialises a working copy with an initial commit and ties it
// to a bare repo as origin.
func initWorkingRepo(t *testing.T, dir, originURL string) {
	t.Helper()
	run(t, dir, "git", "init", ".")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test")
	run(t, dir, "git", "commit", "--allow-empty", "-m", "initial")
	run(t, dir, "git", "remote", "add", "origin", originURL)
	run(t, dir, "git", "push", "-u", "origin", "HEAD:main")
	// Ensure we are on a branch called main.
	run(t, dir, "git", "checkout", "-B", "main")
	run(t, dir, "git", "branch", "--set-upstream-to=origin/main", "main")
}

// run executes a command in dir and fails the test on error.
func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %v: %v\n%s", args, err, out)
	}
}

// parseJSON decodes JSON from b and returns the permissionDecision string.
func parseDecision(t *testing.T, b []byte) string {
	t.Helper()
	var env struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("parse hook JSON %q: %v", b, err)
	}
	return env.HookSpecificOutput.PermissionDecision
}

// parseReason decodes permissionDecisionReason from hook JSON.
func parseReason(t *testing.T, b []byte) string {
	t.Helper()
	var env struct {
		HookSpecificOutput struct {
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("parse hook JSON %q: %v", b, err)
	}
	return env.HookSpecificOutput.PermissionDecisionReason
}

// stdinPayload returns a JSON PreToolUse stdin payload for git checkout -b <name>.
func checkoutBStdin(name string) string {
	return fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"git checkout -b %s"}}`, name)
}

// stdinPayload for git branch <name>.
func branchStdin(name string) string {
	return fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"git branch %s"}}`, name)
}

// nonBranchStdin returns a git command that is NOT a new-branch operation.
func nonBranchStdin() string {
	return `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
}

func TestCheckBranchBase(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires git binary")
	}

	// ------------------------------------------------------------------
	// shared setup: bare repo + working copy
	// ------------------------------------------------------------------
	bareDir := t.TempDir()
	bareURL := initBareRepo(t, bareDir)

	workDir := t.TempDir()
	initWorkingRepo(t, workDir, bareURL)

	tests := []struct {
		name           string
		stdinPayload   string
		cfg            Config
		setupFn        func(t *testing.T, dir string)
		wantAllowed    bool
		wantMsgContain string
	}{
		{
			name:         "non-branch-create tool call: gate not applicable → allow",
			stdinPayload: nonBranchStdin(),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"branch-base": ModeEnforce},
			},
			wantAllowed: true,
		},
		{
			name:         "branch from up-to-date main → allow",
			stdinPayload: checkoutBStdin("feature/my-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"branch-base": ModeEnforce},
			},
			// no setup: workDir is already up-to-date
			wantAllowed: true,
		},
		{
			name:         "branch from stale main → deny (enforce)",
			stdinPayload: checkoutBStdin("feature/my-task-stale"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"branch-base": ModeEnforce},
			},
			setupFn: func(t *testing.T, dir string) {
				// Push an extra commit to origin/main without pulling locally.
				cloneDir := t.TempDir()
				run(t, cloneDir, "git", "clone", bareURL, ".")
				run(t, cloneDir, "git", "config", "user.email", "test@example.com")
				run(t, cloneDir, "git", "config", "user.name", "Test")
				run(t, cloneDir, "git", "commit", "--allow-empty", "-m", "extra commit")
				run(t, cloneDir, "git", "push", "origin", "main")
			},
			wantAllowed:    false,
			wantMsgContain: "out of date",
		},
		{
			name:         "branch from declared parent (current) → allow",
			stdinPayload: checkoutBStdin("feat/child-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"branch-base": ModeEnforce},
				SDD:            SddConfig{CurrentParentBranch: "feat/tracker"},
			},
			setupFn: func(t *testing.T, dir string) {
				// Create and push feat/tracker; check it out so HEAD is on it.
				run(t, dir, "git", "fetch", "origin")
				run(t, dir, "git", "checkout", "-B", "feat/tracker")
				run(t, dir, "git", "push", "-u", "origin", "feat/tracker")
			},
			wantAllowed: true,
		},
		{
			name:         "branch from stale declared parent → deny",
			stdinPayload: checkoutBStdin("feat/child-stale"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"branch-base": ModeEnforce},
				SDD:            SddConfig{CurrentParentBranch: "feat/tracker-stale"},
			},
			setupFn: func(t *testing.T, dir string) {
				// Create feat/tracker-stale locally and at origin, then add a remote-only commit.
				run(t, dir, "git", "checkout", "-B", "feat/tracker-stale")
				run(t, dir, "git", "push", "-u", "origin", "feat/tracker-stale")

				cloneDir := t.TempDir()
				run(t, cloneDir, "git", "clone", bareURL, ".")
				run(t, cloneDir, "git", "config", "user.email", "test@example.com")
				run(t, cloneDir, "git", "config", "user.name", "Test")
				run(t, cloneDir, "git", "checkout", "feat/tracker-stale")
				run(t, cloneDir, "git", "commit", "--allow-empty", "-m", "remote-only commit on tracker-stale")
				run(t, cloneDir, "git", "push", "origin", "feat/tracker-stale")
			},
			wantAllowed:    false,
			wantMsgContain: "out of date",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Each sub-test uses the shared workDir; setupFn mutates git state.
			// Tests that need isolation should use a separate TempDir.
			dir := workDir
			if tt.setupFn != nil {
				tt.setupFn(t, dir)
			}

			result, err := CheckBranchBase(dir, tt.cfg, strings.NewReader(tt.stdinPayload))
			if err != nil {
				t.Fatalf("CheckBranchBase: %v", err)
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

// TestCheckBranchBaseExplicitStartPoint covers Bug 2: when an explicit base is
// provided in the command (git checkout -b <new> <base> or git branch <new>
// <base>), the gate must validate THAT explicit base, not current HEAD.
func TestCheckBranchBaseExplicitStartPoint(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires git binary")
	}

	bareDir := t.TempDir()
	bareURL := initBareRepo(t, bareDir)

	workDir := t.TempDir()
	initWorkingRepo(t, workDir, bareURL)

	cfg := Config{
		StrictWorkflow: true,
		Gates:          map[string]GateMode{"branch-base": ModeEnforce},
	}

	t.Run("explicit wrong base while on main → deny", func(t *testing.T) {
		// HEAD is on main (allowed), but explicit base is 'some-stale-or-wrong-base'
		// which is not in the allowed list. Bug: before fix this would PASS because
		// it only checked current HEAD (main), ignoring the explicit base argument.
		payload := `{"tool_name":"Bash","tool_input":{"command":"git checkout -b feat/x some-stale-or-wrong-base"}}`
		result, err := CheckBranchBase(workDir, cfg, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("CheckBranchBase: %v", err)
		}
		if result.Allowed {
			t.Errorf("expected deny for explicit disallowed base 'some-stale-or-wrong-base', got allow")
		}
	})

	t.Run("git branch <new> <wrong-base> → deny", func(t *testing.T) {
		payload := `{"tool_name":"Bash","tool_input":{"command":"git branch feat/y some-wrong-base"}}`
		result, err := CheckBranchBase(workDir, cfg, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("CheckBranchBase: %v", err)
		}
		if result.Allowed {
			t.Errorf("expected deny for explicit disallowed base via git branch, got allow")
		}
	})

	t.Run("explicit allowed+fresh base → allow", func(t *testing.T) {
		// Push main to origin so origin/main exists and is up to date.
		// git checkout -b feat/z main (explicit base = main, which is up-to-date).
		payload := `{"tool_name":"Bash","tool_input":{"command":"git checkout -b feat/z main"}}`
		result, err := CheckBranchBase(workDir, cfg, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("CheckBranchBase: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected allow for explicit allowed+fresh base 'main', got deny: %q", result.Message)
		}
	})

	t.Run("no explicit base (git checkout -b feat/q) → uses HEAD as before", func(t *testing.T) {
		// Ensure we're on main (allowed base).
		run(t, workDir, "git", "checkout", "main")
		payload := `{"tool_name":"Bash","tool_input":{"command":"git checkout -b feat/q"}}`
		result, err := CheckBranchBase(workDir, cfg, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("CheckBranchBase: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected allow when on main with no explicit base, got deny: %q", result.Message)
		}
	})
}

// TestCheckBranchBaseStdinEdgeCases covers edge cases around stdin parsing.
func TestCheckBranchBaseStdinEdgeCases(t *testing.T) {
	cfg := Config{
		StrictWorkflow: true,
		Gates:          map[string]GateMode{"branch-base": ModeEnforce},
	}

	t.Run("empty stdin → fail-open (not applicable)", func(t *testing.T) {
		result, err := CheckBranchBase(t.TempDir(), cfg, strings.NewReader(""))
		if err != nil {
			t.Fatalf("CheckBranchBase: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected fail-open (allow) on empty stdin, got deny: %q", result.Message)
		}
	})

	t.Run("non-JSON stdin → fail-open (not applicable)", func(t *testing.T) {
		result, err := CheckBranchBase(t.TempDir(), cfg, strings.NewReader("not json"))
		if err != nil {
			t.Fatalf("CheckBranchBase: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected fail-open (allow) on non-JSON stdin, got deny: %q", result.Message)
		}
	})

	t.Run("git push stdin → not applicable (allow)", func(t *testing.T) {
		payload := `{"tool_name":"Bash","tool_input":{"command":"git push"}}`
		result, err := CheckBranchBase(t.TempDir(), cfg, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("CheckBranchBase: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected allow for non-branch-create command, got deny: %q", result.Message)
		}
	})
}

// TestBranchBaseIntegrationViaCheck exercises the full Check() pipeline for
// branch-base under the four key mode scenarios using a real temp git repo.
func TestBranchBaseIntegrationViaCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires git binary")
	}

	// Setup: bare + working copy. We need a stale state for deny/warn tests.
	bareDir := t.TempDir()
	bareURL2 := initBareRepo(t, bareDir)

	workDir := t.TempDir()
	initWorkingRepo(t, workDir, bareURL2)

	// Write openspec/config.yaml
	writeConfig := func(t *testing.T, dir string, mode GateMode) {
		t.Helper()
		cfgDir := filepath.Join(dir, "openspec")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("strict_workflow: true\nstrict_workflow_gates:\n  branch-base: %s\n", mode)
		if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Make origin/main 1 commit ahead of local.
	makeStale := func(t *testing.T) {
		t.Helper()
		cloneDir := t.TempDir()
		run(t, cloneDir, "git", "clone", bareURL2, ".")
		run(t, cloneDir, "git", "config", "user.email", "test@example.com")
		run(t, cloneDir, "git", "config", "user.name", "Test")
		run(t, cloneDir, "git", "commit", "--allow-empty", "-m", "stale trigger")
		run(t, cloneDir, "git", "push", "origin", "main")
	}

	// Pull to make fresh again.
	makeFresh := func(t *testing.T) {
		t.Helper()
		run(t, workDir, "git", "fetch", "origin")
		run(t, workDir, "git", "checkout", "main")
		run(t, workDir, "git", "reset", "--hard", "origin/main")
	}

	stdin := checkoutBStdin("feature/check-test")
	stdinBytes := strings.NewReader(stdin)
	_ = stdinBytes

	t.Run("gate off: always allow even when stale", func(t *testing.T) {
		writeConfig(t, workDir, ModeOff)
		makeStale(t)
		defer makeFresh(t)

		var out strings.Builder
		err := Check("branch-base", workDir, &out)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		// gate is off: stdout must be allow (mode=off short-circuits before domain check)
		// Note: Check reads stdin from os.Stdin; since stdin is terminal/empty, gate goes to domain
		// but in off mode we never reach domain.
		// We just verify no error and the output is allow JSON.
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("want allow, got %q", decision)
		}
	})

	t.Run("gate enforce + stale main → deny JSON", func(t *testing.T) {
		writeConfig(t, workDir, ModeEnforce)
		makeStale(t)
		defer makeFresh(t)

		// Inject stdin via CheckWithStdin helper to avoid os.Stdin dependency.
		var out strings.Builder
		err := CheckWithStdin("branch-base", workDir, strings.NewReader(checkoutBStdin("feat/stale-deny")), &out)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "deny" {
			t.Errorf("want deny, got %q (output=%s)", decision, out.String())
		}
	})

	t.Run("gate enforce + sentinel → warn JSON, sentinel deleted, log entry written", func(t *testing.T) {
		writeConfig(t, workDir, ModeEnforce)
		makeStale(t)
		defer makeFresh(t)

		// Create sentinel.
		sentinelDir := filepath.Join(workDir, ".gentle-ai", "git-gate-override")
		if err := os.MkdirAll(sentinelDir, 0o755); err != nil {
			t.Fatal(err)
		}
		sentinelFile := filepath.Join(sentinelDir, "branch-base")
		if err := os.WriteFile(sentinelFile, nil, 0o644); err != nil {
			t.Fatal(err)
		}

		var out strings.Builder
		err := CheckWithStdin("branch-base", workDir, strings.NewReader(checkoutBStdin("feat/sentinel-warn")), &out)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("sentinel should degrade to warn → allow, got %q", decision)
		}
		// Sentinel must be gone.
		if _, err := os.Stat(sentinelFile); !os.IsNotExist(err) {
			t.Error("sentinel file should have been deleted")
		}
		// Log entry must exist.
		logPath := filepath.Join(workDir, ".gentle-ai", "git-gate.log")
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("log file not found: %v", err)
		}
		if !strings.Contains(string(data), "branch-base") {
			t.Errorf("log missing branch-base entry: %q", data)
		}
	})

	t.Run("gate warn + stale main → allow JSON, warning to stderr, log entry", func(t *testing.T) {
		writeConfig(t, workDir, ModeWarn)
		makeStale(t)
		defer makeFresh(t)

		var out strings.Builder
		err := CheckWithStdin("branch-base", workDir, strings.NewReader(checkoutBStdin("feat/warn-test")), &out)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("warn mode should allow, got %q", decision)
		}
		logPath := filepath.Join(workDir, ".gentle-ai", "git-gate.log")
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("log file not found: %v", err)
		}
		if !strings.Contains(string(data), "warn") {
			t.Errorf("log missing warn entry: %q", data)
		}
	})
}
