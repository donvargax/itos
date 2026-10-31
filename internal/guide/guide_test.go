package guide

import (
	"slices"
	"strings"
	"testing"
)

// Each role has a template of its own, LF-terminated and no guide or
// procedure name: the implementer's is the brief procedure's quoted block,
// less the quote marks, and the others ship beside it (slice-204).
func TestTemplates(t *testing.T) {
	if !slices.Equal(Roles, []string{"drafter", "implementer", "reviewer"}) {
		t.Errorf("Roles = %v", Roles)
	}
	for role, first := range map[string]string{
		"drafter":     "Specify <item> of <project>, in the repository at <absolute path>: <one\nline>.",
		"implementer": "Implement <item> of <project>, in the repository at <absolute path>:\n<one line>.",
		"reviewer":    "Review <item> of <project>, in the repository at <absolute path>: <one\nline>.",
	} {
		text, ok := Template(role)
		if !ok || !strings.HasPrefix(text, first) || !strings.HasSuffix(text, "\n") || strings.Contains(text, "\r") || strings.Contains(text, "\n>") {
			t.Errorf("Template(%q) = %.60q, %t", role, text, ok)
		}
		if _, isText := Text(role); isText || IsProcedure(role) {
			t.Errorf("%q is a guide or a procedure", role)
		}
	}
	if text, ok := Template("tester"); ok || text != "" {
		t.Errorf("Template(tester) = %q, %t", text, ok)
	}
	// The implementer's template is every quoted line of the brief
	// procedure, and the procedure holds no other quoted block.
	brief, _ := Text("brief")
	var quotedLines int
	for _, line := range strings.Split(brief, "\n") {
		if strings.HasPrefix(line, ">") {
			quotedLines++
		}
	}
	implementer, _ := Template("implementer")
	if got := strings.Count(implementer, "\n"); got != quotedLines {
		t.Errorf("the implementer's template has %d lines, the brief procedure %d quoted ones", got, quotedLines)
	}
	if !strings.Contains(implementer, "\n\nRed first:") {
		t.Errorf("the quote marks of a blank line are not stripped: %q", implementer)
	}
	if got := quoted("# A\n\n## The brief\n\n> one\n>\n> two\n"); got != "one\n\ntwo\n" {
		t.Errorf("quoted = %q", got)
	}
}

// Each guide and procedure is there by its name and opens with a heading of
// its own; a name with neither has none.
func TestGuides(t *testing.T) {
	if got := Names(); !slices.Equal(got, []string{"coordinate", "work"}) {
		t.Errorf("Names() = %v", got)
	}
	if got := Procedures(); !slices.Equal(got, []string{"brief", "commit", "decide", "land", "push", "repair"}) {
		t.Errorf("Procedures() = %v", got)
	}
	for name, heading := range map[string]string{
		"coordinate": "# Coordinating with itos\n",
		"work":       "# Working with itos\n",
		"brief":      "# Specifying and briefing an item\n",
		"land":       "# Landing a result\n",
		"repair":     "# Repairing what is red\n",
		"decide":     "# Deciding with the person\n",
		"commit":     "# Committing with itos\n",
		"push":       "# Pushing with itos\n",
	} {
		text, ok := Text(name)
		if !ok || !strings.HasPrefix(text, heading) || !strings.HasSuffix(text, "\n") {
			t.Errorf("Text(%q) = %.40q, %t", name, text, ok)
		}
		if IsProcedure(name) == slices.Contains(Names(), name) {
			t.Errorf("IsProcedure(%q) = %t", name, IsProcedure(name))
		}
	}
	if _, ok := Text("review"); ok || IsProcedure("review") {
		t.Error(`"review" has a guide or a procedure`)
	}
}

// Nothing was lost when the guides shrank to their essentials (slice 119):
// every section of the two guides before it is in exactly one of the texts.
func TestEverySectionInExactlyOneText(t *testing.T) {
	before := []string{
		// coordinate.md
		"Start of a session", "The loop", "Specifying", "One agent at a time",
		"Pushing and waiting", "Checking a result", "When main is red",
		"Talking to the person", "Decisions", "Reporting a problem with itos",
		"The brief", "Lessons",
		// work.md
		"The item", "What drives a change", "Commit", "The gates run themselves",
		"Push", "Never", "Report",
	}
	for _, heading := range before {
		var in []string
		for _, name := range slices.Concat(Names(), Procedures()) {
			text, _ := Text(name)
			if slices.Contains(strings.Split(text, "\n"), "## "+heading) {
				in = append(in, name)
			}
		}
		if len(in) != 1 {
			t.Errorf("## %s is in %v", heading, in)
		}
	}
}

// Each procedure is named, as the command that prints it, in exactly one
// guide's map: the role that reads it before its step.
func TestEveryProcedureInOneGuidesMap(t *testing.T) {
	for _, p := range Procedures() {
		var in []string
		for _, name := range Names() {
			text, _ := Text(name)
			if strings.Contains(text, "- `itos guide "+p+"`") {
				in = append(in, name)
			}
		}
		if len(in) != 1 {
			t.Errorf("itos guide %s is mapped in %v", p, in)
		}
	}
}
