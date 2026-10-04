package work

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v3/internal/value"
)

// work list's lines keep the registry's order, line up the id, kind and
// status as wide as their widest (counted in runes), and show "-" for a kind
// or status an item does not give, nothing for a title it does not.
func TestPrintList(t *testing.T) {
	var b strings.Builder
	PrintList(&b, []*value.Map{
		value.NewMap("id", "T-001", "title", "Done", "status", "done", "kind", "task"),
		value.NewMap("id", "slice-12", "title", "Ünïcode", "status", "doing"),
		value.NewMap("id", "é", "kind", "idea"),
	})
	want := "T-001     task  done   Done\n" +
		"slice-12  -     doing  Ünïcode\n" +
		"é         idea  -\n"
	if b.String() != want {
		t.Errorf("got\n%s\nwant\n%s", b.String(), want)
	}
}
