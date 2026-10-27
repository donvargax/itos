package adr

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// FuzzStrictYAMLStatus holds Record.Status, the reader of a record's
// frontmatter, to value.Parse, itos's strict reading (T-329): frontmatter
// Parse refuses (a key written twice at any depth, an alias that never
// ends) is read as no status, and frontmatter it accepts gives the status
// Parse reads, trimmed, "" when it is no string.
//
// The frontmatter that is not YAML is read as none by design, not refused
// (MADR makes it optional, bug 19), so a refusal is an empty status: a
// record whose frontmatter Parse refuses is live. The frontmatter ends at
// its first closing fence, so it is one document.
func FuzzStrictYAMLStatus(f *testing.F) {
	f.Fuzz(func(t *testing.T, front string) {
		r := Record{Text: "---\n" + front + "\n---\n# A record\n"}
		lines, _, ok := frontmatter(strings.Split(r.Text, "\n"))
		if !ok {
			t.Fatalf("%q has no frontmatter", r.Text)
		}
		yamlText := strings.Join(lines, "\n")
		parsed, parseErr := value.Parse(yamlText)
		status := r.Status()
		if parseErr != nil {
			if status != "" {
				t.Fatalf("value.Parse refuses %q (%v), Status reads %q", yamlText, parseErr, status)
			}
			return
		}
		want, _ := value.Prop(parsed, "status").(string)
		if want = strings.TrimSpace(want); status != want {
			t.Fatalf("value.Parse reads %q as %s, Status reads %q", yamlText, value.JSON(parsed), status)
		}
	})
}
