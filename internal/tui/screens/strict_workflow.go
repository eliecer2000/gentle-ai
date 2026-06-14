package screens

import (
	"strings"

	"github.com/gentleman-programming/gentle-ai/internal/tui/styles"
)

const (
	StrictWorkflowOptionEnable  = 0
	StrictWorkflowOptionDisable = 1
)

func StrictWorkflowOptions() []string {
	return []string{"Enable", "Disable"}
}

func RenderStrictWorkflow(enabled bool, cursor int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("STRICT WORKFLOW MODE"))
	b.WriteString("\n\n")
	b.WriteString(styles.SubtextStyle.Render("Should agents enforce sequential PR gates and atomic commits?"))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("When enabled: new branches require all previous task PRs to be merged."))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("Commits must be atomic work units. All SDD phases are mandatory."))
	b.WriteString("\n\n")

	options := StrictWorkflowOptions()
	for idx, opt := range options {
		isSelected := (idx == StrictWorkflowOptionEnable && enabled) || (idx == StrictWorkflowOptionDisable && !enabled)
		focused := idx == cursor
		b.WriteString(renderRadio(opt, isSelected, focused))
	}

	b.WriteString("\n")
	b.WriteString(renderOptions([]string{"Back"}, cursor-len(options)))
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • enter: select • esc: back"))

	return b.String()
}
