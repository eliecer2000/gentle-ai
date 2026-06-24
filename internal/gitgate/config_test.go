package gitgate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadConfig(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    Config
		wantErr bool
	}{
		{
			name: "all gates present: each resolves to config value",
			yaml: `strict_workflow: true
strict_workflow_gates:
  branch-base: enforce
  orphan-upstream: warn
  sequential-pr: off
`,
			want: Config{
				StrictWorkflow: true,
				Gates: map[string]GateMode{
					"branch-base":     ModeEnforce,
					"orphan-upstream": ModeWarn,
					"sequential-pr":   ModeOff,
				},
			},
		},
		{
			name: "strict_workflow=true, no gates block: returns empty gates map",
			yaml: `strict_workflow: true
`,
			want: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{},
			},
		},
		{
			name: "strict_workflow=false: returns false",
			yaml: `strict_workflow: false
strict_workflow_gates:
  branch-base: enforce
`,
			want: Config{
				StrictWorkflow: false,
				Gates: map[string]GateMode{
					"branch-base": ModeEnforce,
				},
			},
		},
		{
			name: "missing strict_workflow key: defaults to false",
			yaml: `strict_tdd: true
`,
			want: Config{
				StrictWorkflow: false,
				Gates:          map[string]GateMode{},
			},
		},
		{
			name: "sdd block: delivery_strategy and chain_strategy are read",
			yaml: `strict_workflow: true
sdd:
  delivery_strategy: auto-chain
  chain_strategy: stacked-to-main
  current_parent_branch: feat/tracker
`,
			want: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{},
				SDD: SddConfig{
					DeliveryStrategy:    "auto-chain",
					ChainStrategy:       "stacked-to-main",
					CurrentParentBranch: "feat/tracker",
				},
			},
		},
		{
			name: "sdd.task_branches list is read correctly when non-empty",
			yaml: `strict_workflow: true
sdd:
  task_branches:
    - feat/slice-1
    - feat/slice-2
`,
			want: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{},
				SDD: SddConfig{
					TaskBranches: []string{"feat/slice-1", "feat/slice-2"},
				},
			},
		},
		{
			name: "absent sdd block returns zero SddConfig (no error)",
			yaml: `strict_workflow: true
`,
			want: Config{
				StrictWorkflow: true,
				Gates:          map[string]GateMode{},
				SDD:            SddConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := ReadConfig(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ReadConfig() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadConfig() error = %v", err)
			}
			if got.StrictWorkflow != tt.want.StrictWorkflow {
				t.Errorf("StrictWorkflow = %v, want %v", got.StrictWorkflow, tt.want.StrictWorkflow)
			}
			if len(got.Gates) != len(tt.want.Gates) {
				t.Errorf("Gates len = %d, want %d; got %v", len(got.Gates), len(tt.want.Gates), got.Gates)
			}
			for gate, wantMode := range tt.want.Gates {
				if got.Gates[gate] != wantMode {
					t.Errorf("Gates[%q] = %q, want %q", gate, got.Gates[gate], wantMode)
				}
			}
			if got.SDD.DeliveryStrategy != tt.want.SDD.DeliveryStrategy {
				t.Errorf("SDD.DeliveryStrategy = %q, want %q", got.SDD.DeliveryStrategy, tt.want.SDD.DeliveryStrategy)
			}
			if got.SDD.ChainStrategy != tt.want.SDD.ChainStrategy {
				t.Errorf("SDD.ChainStrategy = %q, want %q", got.SDD.ChainStrategy, tt.want.SDD.ChainStrategy)
			}
			if got.SDD.CurrentParentBranch != tt.want.SDD.CurrentParentBranch {
				t.Errorf("SDD.CurrentParentBranch = %q, want %q", got.SDD.CurrentParentBranch, tt.want.SDD.CurrentParentBranch)
			}
			if len(got.SDD.TaskBranches) != len(tt.want.SDD.TaskBranches) {
				t.Errorf("SDD.TaskBranches len = %d, want %d; got %v", len(got.SDD.TaskBranches), len(tt.want.SDD.TaskBranches), got.SDD.TaskBranches)
			}
			for i, branch := range tt.want.SDD.TaskBranches {
				if i < len(got.SDD.TaskBranches) && got.SDD.TaskBranches[i] != branch {
					t.Errorf("SDD.TaskBranches[%d] = %q, want %q", i, got.SDD.TaskBranches[i], branch)
				}
			}
		})
	}
}

func TestReadConfigMissingFile(t *testing.T) {
	cfg, err := ReadConfig("/nonexistent/path/config.yaml")
	// Missing file: returns zero Config, no error (fail-open)
	if err != nil {
		t.Fatalf("ReadConfig(missing) error = %v, want nil", err)
	}
	if cfg.StrictWorkflow != false {
		t.Errorf("StrictWorkflow = %v, want false", cfg.StrictWorkflow)
	}
}
