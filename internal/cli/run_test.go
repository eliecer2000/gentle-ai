package cli

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/model"
)

// TestRunInjectOptionsIncludesStrictWorkflow verifies that buildSDDInjectOptions
// maps selection.StrictWorkflow through to the returned sdd.InjectOptions.
// This is a regression guard for the confirmed wiring bug where StrictWorkflow
// was omitted from the sdd.InjectOptions literal in componentApplyStep.Run().
func TestRunInjectOptionsIncludesStrictWorkflow(t *testing.T) {
	tests := []struct {
		name               string
		selection          model.Selection
		wantStrictTDD      bool
		wantStrictWorkflow bool
	}{
		{
			name: "StrictWorkflow=true flows into InjectOptions.StrictWorkflow",
			selection: model.Selection{
				StrictWorkflow: true,
				StrictTDD:      false,
			},
			wantStrictTDD:      false,
			wantStrictWorkflow: true,
		},
		{
			name: "StrictWorkflow=false is not injected",
			selection: model.Selection{
				StrictWorkflow: false,
				StrictTDD:      true,
			},
			wantStrictTDD:      true,
			wantStrictWorkflow: false,
		},
		{
			name: "both true flows through",
			selection: model.Selection{
				StrictWorkflow: true,
				StrictTDD:      true,
			},
			wantStrictTDD:      true,
			wantStrictWorkflow: true,
		},
		{
			name:               "both false is zero value",
			selection:          model.Selection{},
			wantStrictTDD:      false,
			wantStrictWorkflow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildSDDInjectOptions(tt.selection, "")
			if opts.StrictTDD != tt.wantStrictTDD {
				t.Errorf("InjectOptions.StrictTDD = %v, want %v", opts.StrictTDD, tt.wantStrictTDD)
			}
			if opts.StrictWorkflow != tt.wantStrictWorkflow {
				t.Errorf("InjectOptions.StrictWorkflow = %v, want %v", opts.StrictWorkflow, tt.wantStrictWorkflow)
			}
		})
	}
}
