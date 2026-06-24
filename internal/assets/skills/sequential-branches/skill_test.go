package sequentialbranches_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSequentialBranchesSkillContent verifies that the SKILL.md contains all
// required elements after the Slice 4 rewrite: gate names, three-state model
// terms, sentinel path, and the advisory notice for non-Claude agents.
func TestSequentialBranchesSkillContent(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	skillPath := filepath.Join(filepath.Dir(thisFile), "SKILL.md")

	raw, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	content := string(raw)

	// All three gate names must be present.
	for _, gate := range []string{"branch-base", "orphan-upstream", "sequential-pr"} {
		if !strings.Contains(content, gate) {
			t.Errorf("SKILL.md missing gate name %q", gate)
		}
	}

	// Three-state model terms must be present.
	for _, state := range []string{"enforce", "warn", "off"} {
		if !strings.Contains(content, state) {
			t.Errorf("SKILL.md missing three-state term %q", state)
		}
	}

	// Sentinel path must be documented (break-glass protocol).
	if !strings.Contains(content, "git-gate-override") {
		t.Errorf("SKILL.md missing sentinel path keyword %q", "git-gate-override")
	}

	// Advisory notice for non-Claude agents must be present.
	if !strings.Contains(content, "advisory") {
		t.Errorf("SKILL.md missing advisory notice for non-Claude agents")
	}

	// Break-glass section must be present.
	if !strings.Contains(content, "Break-Glass") && !strings.Contains(content, "break-glass") {
		t.Errorf("SKILL.md missing break-glass protocol section")
	}

	// consumed-once behavior must be documented.
	if !strings.Contains(content, "consumed-once") {
		t.Errorf("SKILL.md missing consumed-once documentation for sentinel")
	}
}
