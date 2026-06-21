package gitgate

import (
	"encoding/json"
	"testing"
)

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

func TestDenyOutput(t *testing.T) {
	out := DenyOutput("branch-base", "stale main branch")
	var h hookOutput
	if err := json.Unmarshal(out, &h); err != nil {
		t.Fatalf("DenyOutput() not valid JSON: %v\n%s", err, out)
	}
	if h.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want %q", h.HookSpecificOutput.PermissionDecision, "deny")
	}
	if h.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q, want %q", h.HookSpecificOutput.HookEventName, "PreToolUse")
	}
	if h.HookSpecificOutput.PermissionDecisionReason == "" {
		t.Error("permissionDecisionReason is empty, want non-empty")
	}
}

func TestAllowOutput(t *testing.T) {
	out := AllowOutput()
	var h hookOutput
	if err := json.Unmarshal(out, &h); err != nil {
		t.Fatalf("AllowOutput() not valid JSON: %v\n%s", err, out)
	}
	if h.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision = %q, want %q", h.HookSpecificOutput.PermissionDecision, "allow")
	}
}

func TestWarnOutput(t *testing.T) {
	out := WarnOutput("orphan-upstream", "no upstream set")
	var h hookOutput
	if err := json.Unmarshal(out, &h); err != nil {
		t.Fatalf("WarnOutput() not valid JSON: %v\n%s", err, out)
	}
	// Warn: allow JSON (warning emitted to stderr separately)
	if h.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision = %q, want %q (warn allows the operation)", h.HookSpecificOutput.PermissionDecision, "allow")
	}
}
