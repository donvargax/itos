package guide

import (
	"slices"
	"strings"
	"testing"
)

// Each guide is there by its name and opens with the heading a session
// recognises it by; a name with no guide has none.
func TestGuides(t *testing.T) {
	if got := Names(); !slices.Equal(got, []string{"coordinate", "work"}) {
		t.Errorf("Names() = %v", got)
	}
	for name, heading := range map[string]string{
		"coordinate": "# Coordinating with itos\n",
		"work":       "# Working with itos\n",
	} {
		text, ok := Text(name)
		if !ok || !strings.HasPrefix(text, heading) || !strings.HasSuffix(text, "\n") {
			t.Errorf("Text(%q) = %.40q, %t", name, text, ok)
		}
	}
	if _, ok := Text("review"); ok {
		t.Error(`Text("review") has a guide`)
	}
}
