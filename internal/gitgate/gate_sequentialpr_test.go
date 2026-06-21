package gitgate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGHOutput builds JSON that looks like `gh pr list --state open --json headRefName,number` output.
func fakeGHOutput(prs []openPR) string {
	b, _ := json.Marshal(prs)
	return string(b)
}

// newCheckoutBStdin returns a JSON PreToolUse stdin for git checkout -b <name>.
// (Re-declared here to avoid test-file cross-dependencies within the package.)
func newCheckoutBStdin(name string) string {
	return fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"git checkout -b %s"}}`, name)
}

// newPushStdin returns a JSON PreToolUse stdin for git push.
func newPushStdin() string {
	return `{"tool_name":"Bash","tool_input":{"command":"git push"}}`
}

func TestCheckSequentialPR(t *testing.T) {
	tests := []struct {
		name           string
		stdinPayload   string
		cfg            Config
		ghFunc         func(cwd string) ([]openPR, error)
		wantAllowed    bool
		wantMsgContain string
	}{
		{
			name:         "non-branch-create tool call: gate not applicable → allow",
			stdinPayload: newPushStdin(),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{{HeadRefName: "feat/task-1", Number: 99}}, nil
			},
			wantAllowed: true,
		},
		{
			name:         "empty task_branches: gate not applicable → allow",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{{HeadRefName: "feat/task-1", Number: 10}}, nil
			},
			wantAllowed: true,
		},
		{
			name:         "nil task_branches: gate not applicable → allow",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: nil},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{{HeadRefName: "feat/task-1", Number: 10}}, nil
			},
			wantAllowed: true,
		},
		{
			name:         "no open PRs from task set → allow",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1", "feat/task-2"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				// Open PRs exist but NOT from our task set.
				return []openPR{{HeadRefName: "feat/unrelated", Number: 5}}, nil
			},
			wantAllowed: true,
		},
		{
			name:         "open PR from task set exists (enforce) → deny with PR list",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1", "feat/task-2"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{
					{HeadRefName: "feat/task-1", Number: 42},
				}, nil
			},
			wantAllowed:    false,
			wantMsgContain: "#42",
		},
		{
			name:         "open PR from task set lists branch in deny message",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1", "feat/task-2"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{
					{HeadRefName: "feat/task-1", Number: 42},
				}, nil
			},
			wantAllowed:    false,
			wantMsgContain: "feat/task-1",
		},
		{
			name:         "open PR from task set exists (warn) → deny raw (warn applied by Check pipeline)",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeWarn},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{{HeadRefName: "feat/task-1", Number: 7}}, nil
			},
			// Domain check returns raw truth (a blocking PR exists). The warn→allow
			// translation and the non-silent warning happen in CheckWithStdin,
			// exercised by TestSequentialPRViaCheck.
			wantAllowed:    false,
			wantMsgContain: "#7",
		},
		{
			name:         "gh returns non-zero exit → fail-open (allow)",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return nil, fmt.Errorf("gh: exit status 1")
			},
			wantAllowed: true,
		},
		{
			name:         "gh returns empty list → allow",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{}, nil
			},
			wantAllowed: true,
		},
		{
			name:         "multiple open PRs from task set → deny listing all",
			stdinPayload: newCheckoutBStdin("feat/new-task"),
			cfg: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
				SDD:            SddConfig{TaskBranches: []string{"feat/task-1", "feat/task-2"}},
			},
			ghFunc: func(cwd string) ([]openPR, error) {
				return []openPR{
					{HeadRefName: "feat/task-1", Number: 11},
					{HeadRefName: "feat/task-2", Number: 12},
				}, nil
			},
			wantAllowed:    false,
			wantMsgContain: "#11",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			result, err := checkSequentialPRWith(dir, tt.cfg, strings.NewReader(tt.stdinPayload), tt.ghFunc)
			if err != nil {
				t.Fatalf("checkSequentialPRWith: %v", err)
			}
			if result.Allowed != tt.wantAllowed {
				t.Errorf("Allowed = %v, want %v (message: %q)", result.Allowed, tt.wantAllowed, result.Message)
			}
			if tt.wantMsgContain != "" && !strings.Contains(result.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want it to contain %q", result.Message, tt.wantMsgContain)
			}
		})
	}
}

// TestCheckSequentialPRGhUnavailable verifies that when gh is not on PATH, the
// gate degrades to allow (fail-open) regardless of the configured mode.
func TestCheckSequentialPRGhUnavailable(t *testing.T) {
	cfg := Config{
		StrictWorkflow: true,
		Gates:          map[string]GateMode{"sequential-pr": ModeEnforce},
		SDD:            SddConfig{TaskBranches: []string{"feat/task-1"}},
	}
	stdin := newCheckoutBStdin("feat/new-task")

	// Simulate gh unavailable by injecting a function that returns the sentinel error.
	result, err := checkSequentialPRWith(t.TempDir(), cfg, strings.NewReader(stdin), func(cwd string) ([]openPR, error) {
		return nil, errGhUnavailable
	})
	if err != nil {
		t.Fatalf("checkSequentialPRWith: %v", err)
	}
	if !result.Allowed {
		t.Errorf("gh unavailable should fail-open (allow), got deny: %q", result.Message)
	}
	if !strings.Contains(result.Message, "gh CLI unavailable") {
		t.Errorf("message should mention gh CLI unavailable, got %q", result.Message)
	}
}

// TestSequentialPRViaCheck exercises the full CheckWithStdin pipeline for the
// sequential-pr gate by writing a real openspec/config.yaml and using the
// injectable ghListOpenPRs var.
func TestSequentialPRViaCheck(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "openspec")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSeqConfig := func(t *testing.T, mode string, branches ...string) {
		t.Helper()
		var sb strings.Builder
		sb.WriteString("strict_workflow: true\n")
		sb.WriteString("strict_workflow_gates:\n")
		sb.WriteString(fmt.Sprintf("  sequential-pr: %s\n", mode))
		sb.WriteString("sdd:\n")
		sb.WriteString("  task_branches:\n")
		for _, b := range branches {
			sb.WriteString(fmt.Sprintf("    - %s\n", b))
		}
		if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("enforce: open PR from task set → deny JSON", func(t *testing.T) {
		writeSeqConfig(t, "enforce", "feat/task-1")

		// Inject fake gh that returns one open PR from our task set.
		old := ghListOpenPRs
		defer func() { ghListOpenPRs = old }()
		ghListOpenPRs = func(cwd string) ([]openPR, error) {
			return []openPR{{HeadRefName: "feat/task-1", Number: 55}}, nil
		}

		var out strings.Builder
		err := CheckWithStdin("sequential-pr", dir, strings.NewReader(newCheckoutBStdin("feat/new")), &out)
		if err != nil {
			t.Fatalf("CheckWithStdin: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "deny" {
			t.Errorf("want deny, got %q (output=%s)", decision, out.String())
		}
	})

	t.Run("warn: open PR from task set → allow JSON", func(t *testing.T) {
		writeSeqConfig(t, "warn", "feat/task-1")

		old := ghListOpenPRs
		defer func() { ghListOpenPRs = old }()
		ghListOpenPRs = func(cwd string) ([]openPR, error) {
			return []openPR{{HeadRefName: "feat/task-1", Number: 56}}, nil
		}

		var out strings.Builder
		err := CheckWithStdin("sequential-pr", dir, strings.NewReader(newCheckoutBStdin("feat/new2")), &out)
		if err != nil {
			t.Fatalf("CheckWithStdin: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("warn mode should allow, got %q", decision)
		}
	})

	t.Run("gh missing: always allow regardless of enforce", func(t *testing.T) {
		writeSeqConfig(t, "enforce", "feat/task-1")

		old := ghListOpenPRs
		defer func() { ghListOpenPRs = old }()
		ghListOpenPRs = func(cwd string) ([]openPR, error) {
			return nil, errGhUnavailable
		}

		var out strings.Builder
		err := CheckWithStdin("sequential-pr", dir, strings.NewReader(newCheckoutBStdin("feat/new3")), &out)
		if err != nil {
			t.Fatalf("CheckWithStdin: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("gh missing should fail-open (allow), got %q", decision)
		}
	})

	t.Run("empty task_branches: allow without calling gh", func(t *testing.T) {
		writeSeqConfig(t, "enforce") // no branches listed

		called := false
		old := ghListOpenPRs
		defer func() { ghListOpenPRs = old }()
		ghListOpenPRs = func(cwd string) ([]openPR, error) {
			called = true
			return []openPR{{HeadRefName: "feat/anything", Number: 1}}, nil
		}

		var out strings.Builder
		err := CheckWithStdin("sequential-pr", dir, strings.NewReader(newCheckoutBStdin("feat/new4")), &out)
		if err != nil {
			t.Fatalf("CheckWithStdin: %v", err)
		}
		decision := parseDecision(t, []byte(out.String()))
		if decision != "allow" {
			t.Errorf("empty task set should allow, got %q", decision)
		}
		if called {
			t.Error("gh should not be called when task_branches is empty")
		}
	})
}
