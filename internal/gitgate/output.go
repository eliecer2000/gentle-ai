package gitgate

import (
	"encoding/json"
	"fmt"
)

type hookSpecificOutput struct {
	HookEventName            string `json:"hookEventName,omitempty"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

type hookOutputEnvelope struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

// DenyOutput returns blocking Claude hook JSON for a gate denial.
// The permissionDecisionReason includes the gate name and reason so Claude
// surfaces it to the user as the block explanation.
func DenyOutput(gate, reason string) []byte {
	env := hookOutputEnvelope{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: fmt.Sprintf("[%s] blocked: %s", gate, reason),
		},
	}
	b, _ := json.Marshal(env)
	return b
}

// AllowOutput returns permitting Claude hook JSON.
func AllowOutput() []byte {
	env := hookOutputEnvelope{
		HookSpecificOutput: hookSpecificOutput{
			PermissionDecision: "allow",
		},
	}
	b, _ := json.Marshal(env)
	return b
}

// WarnOutput returns allowing Claude hook JSON. The caller is responsible for
// emitting the human-readable warning to stderr.
func WarnOutput(gate, reason string) []byte {
	// Warn allows the operation; warning is emitted to stderr by the caller.
	_ = gate
	_ = reason
	return AllowOutput()
}
