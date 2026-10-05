package work

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v4/internal/value"
)

// work list's lines keep the registry's order, line up the id, kind and
// status as wide as their widest (counted in runes), and show "-" for a kind
// or status an item does not give, nothing for a title it does not, and
// (deferred) after a deferred item's title.
func TestPrintList(t *testing.T) {
	var b strings.Builder
	PrintList(&b, []*value.Map{
		value.NewMap("id", "T-001", "title", "Done", "status", "done", "kind", "task"),
		value.NewMap("id", "slice-12", "title", "Ünïcode", "status", "doing"),
		value.NewMap("id", "é", "kind", "idea"),
		value.NewMap("id", "later", "title", "Later", "status", "todo", "deferred", "not now"),
		value.NewMap("id", "x", "status", "todo", "deferred", "not now"),
	})
	want := "T-001     task  done   Done\n" +
		"slice-12  -     doing  Ünïcode\n" +
		"é         idea  -\n" +
		"later     -     todo   Later  (deferred)\n" +
		"x         -     todo   (deferred)\n"
	if b.String() != want {
		t.Errorf("got\n%s\nwant\n%s", b.String(), want)
	}
}

// work list's default is the open items, todo and doing, in their order;
// done, dropped, blocked and no status are --all's alone (slice 79).
func TestOpen(t *testing.T) {
	var ids []string
	for _, item := range Open([]*value.Map{
		value.NewMap("id", "a", "status", "done"),
		value.NewMap("id", "b", "status", "doing"),
		value.NewMap("id", "c", "status", "dropped"),
		value.NewMap("id", "d", "status", "todo"),
		value.NewMap("id", "e", "status", "blocked"),
		value.NewMap("id", "f"),
	}) {
		ids = append(ids, value.String(item.At("id")))
	}
	if strings.Join(ids, " ") != "b d" {
		t.Errorf("open: %v, want b d", ids)
	}
}
