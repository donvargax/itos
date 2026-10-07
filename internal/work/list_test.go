package work

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/value"
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

// work list's default is the open items in their order: every one neither
// done nor dropped, blocked, a status itos does not know and no status
// among them (bug 25); done and dropped are --all's alone (slice 79).
func TestOpen(t *testing.T) {
	var ids []string
	for _, item := range Open([]*value.Map{
		value.NewMap("id", "a", "status", "done"),
		value.NewMap("id", "b", "status", "doing"),
		value.NewMap("id", "c", "status", "dropped"),
		value.NewMap("id", "d", "status", "todo"),
		value.NewMap("id", "e", "status", "blocked"),
		value.NewMap("id", "f"),
		value.NewMap("id", "g", "status", "review"),
	}) {
		ids = append(ids, value.String(item.At("id")))
	}
	if strings.Join(ids, " ") != "b d e f g" {
		t.Errorf("open: %v, want b d e f g", ids)
	}
}

// work list --tag keeps the items carrying the tag, in their order, and
// itos work and itos status print an item's tags beside its id (slice 97).
func TestTagged(t *testing.T) {
	items := []*value.Map{
		value.NewMap("id", "a", "tags", []any{"plugin", "json"}),
		value.NewMap("id", "b", "tags", []any{}),
		value.NewMap("id", "c", "tags", []any{"plugin"}),
	}
	var ids []string
	for _, item := range Tagged(items, "plugin") {
		ids = append(ids, TaggedID(item))
	}
	if got := strings.Join(ids, " | "); got != "a [plugin, json] | c [plugin]" {
		t.Errorf("got %q", got)
	}
	if got := TaggedID(items[1]); got != "b" {
		t.Errorf("an item with no tags: %q", got)
	}
}
