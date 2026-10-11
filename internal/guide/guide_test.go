package guide

import (
	"slices"
	"strings"
	"testing"
)

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
