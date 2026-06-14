package screens

import (
	"strings"
	"testing"
)

func TestRenderStrictWorkflowContainsTitle(t *testing.T) {
	output := RenderStrictWorkflow(false, 0)
	if !strings.Contains(output, "STRICT WORKFLOW MODE") {
		t.Errorf("RenderStrictWorkflow output missing title %q\ngot: %s", "STRICT WORKFLOW MODE", output)
	}
}

func TestRenderStrictWorkflowContainsEnableOption(t *testing.T) {
	output := RenderStrictWorkflow(false, 0)
	if !strings.Contains(output, "Enable") {
		t.Errorf("RenderStrictWorkflow output missing Enable option\ngot: %s", output)
	}
}

func TestRenderStrictWorkflowContainsDisableOption(t *testing.T) {
	output := RenderStrictWorkflow(false, 0)
	if !strings.Contains(output, "Disable") {
		t.Errorf("RenderStrictWorkflow output missing Disable option\ngot: %s", output)
	}
}

func TestRenderStrictWorkflowContainsBackOption(t *testing.T) {
	output := RenderStrictWorkflow(false, 0)
	if !strings.Contains(output, "Back") {
		t.Errorf("RenderStrictWorkflow output missing Back option\ngot: %s", output)
	}
}

func TestRenderStrictWorkflowEnabledState(t *testing.T) {
	output := RenderStrictWorkflow(true, 0)
	if !strings.Contains(output, "(*) Enable") {
		t.Errorf("RenderStrictWorkflow(enabled=true) should show Enable as selected\ngot: %s", output)
	}
}

func TestRenderStrictWorkflowDisabledState(t *testing.T) {
	output := RenderStrictWorkflow(false, 0)
	if !strings.Contains(output, "(*) Disable") {
		t.Errorf("RenderStrictWorkflow(enabled=false) should show Disable as selected\ngot: %s", output)
	}
}
