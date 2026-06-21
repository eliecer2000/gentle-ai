package gitgate

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name             string
		strictWorkflow   bool
		gateConfig       map[string]GateMode
		gate             string
		sentinelConsumed bool
		want             GateMode
	}{
		{
			name:           "strict_workflow=false always off",
			strictWorkflow: false,
			gate:           "branch-base",
			want:           ModeOff,
		},
		{
			name:           "strict_workflow=false ignores per-gate config",
			strictWorkflow: false,
			gateConfig:     map[string]GateMode{"branch-base": ModeEnforce},
			gate:           "branch-base",
			want:           ModeOff,
		},
		{
			name:           "missing per-gate config + strict_workflow=true defaults to enforce",
			strictWorkflow: true,
			gateConfig:     map[string]GateMode{},
			gate:           "branch-base",
			want:           ModeEnforce,
		},
		{
			name:           "per-gate config warn respected",
			strictWorkflow: true,
			gateConfig:     map[string]GateMode{"branch-base": ModeWarn},
			gate:           "branch-base",
			want:           ModeWarn,
		},
		{
			name:           "per-gate config off respected",
			strictWorkflow: true,
			gateConfig:     map[string]GateMode{"branch-base": ModeOff},
			gate:           "branch-base",
			want:           ModeOff,
		},
		{
			name:             "enforce + no sentinel stays enforce",
			strictWorkflow:   true,
			gateConfig:       map[string]GateMode{"branch-base": ModeEnforce},
			gate:             "branch-base",
			sentinelConsumed: false,
			want:             ModeEnforce,
		},
		{
			name:             "enforce + sentinel consumed degrades to warn",
			strictWorkflow:   true,
			gateConfig:       map[string]GateMode{"branch-base": ModeEnforce},
			gate:             "branch-base",
			sentinelConsumed: true,
			want:             ModeWarn,
		},
		{
			name:             "warn + sentinel consumed stays warn (no double-degrade)",
			strictWorkflow:   true,
			gateConfig:       map[string]GateMode{"branch-base": ModeWarn},
			gate:             "branch-base",
			sentinelConsumed: true,
			want:             ModeWarn,
		},
		{
			name:             "off + sentinel consumed stays off",
			strictWorkflow:   true,
			gateConfig:       map[string]GateMode{"branch-base": ModeOff},
			gate:             "branch-base",
			sentinelConsumed: true,
			want:             ModeOff,
		},
		{
			name:           "nil gate config map with strict_workflow=true defaults to enforce",
			strictWorkflow: true,
			gateConfig:     nil,
			gate:           "orphan-upstream",
			want:           ModeEnforce,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				StrictWorkflow: tt.strictWorkflow,
				Gates:          tt.gateConfig,
			}
			got := Resolve(cfg, tt.gate, tt.sentinelConsumed)
			if got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}
