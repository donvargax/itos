package work

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
)

func statusesOf(t *testing.T, text string) Statuses {
	t.Helper()
	file := filepath.Join(t.TempDir(), "work-items.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return ItemStatuses(adapter.Worktree, file)
}

// The first item of an id wins; an item without a status, or with null, has
// none; a registry that cannot be read, or holds a null item, gives none.
func TestItemStatuses(t *testing.T) {
	s := statusesOf(t, "items:\n  - { id: T-1, status: doing }\n  - { id: T-1, status: done }\n  - { id: T-2 }\n  - { id: T-3, status: null }\n")
	if got, ok := s.Of("T-1"); !ok || got != "doing" {
		t.Errorf("T-1: got %v, %v", got, ok)
	}
	for _, id := range []string{"T-2", "T-3", "T-4"} {
		if got, ok := s.Of(id); ok {
			t.Errorf("%s: got %v", id, got)
		}
	}
	for _, text := range []string{"items:\n  - { id: T-1, status: doing }\n  - null\n", "items: {}\n", "items: [\n"} {
		if got, ok := statusesOf(t, text).Of("T-1"); ok {
			t.Errorf("%q: got %v", text, got)
		}
	}
	if _, ok := ItemStatuses(adapter.Worktree, filepath.Join(t.TempDir(), "none.yaml")).Of("T-1"); ok {
		t.Error("a missing registry has a status")
	}
}

// IDsAt reads the working tree for "", else the tree it names, opened by the
// trees it is handed: one that cannot be opened is the error.
func TestIDsAtOpensTheTree(t *testing.T) {
	trees := porttest.Trees{Files: map[string]map[string]string{
		"worktree": {"work-items.yaml": "items:\n  - { id: S-1 }\n"},
	}}
	if ids, err := IDsAt(trees, "work-items.yaml", ""); err != nil || !ids["S-1"] || len(ids) != 1 {
		t.Errorf("the working tree's ids: %v, %v", ids, err)
	}
	if _, err := IDsAt(trees, "work-items.yaml", "abc"); err == nil {
		t.Error("a commit outside a repository is read")
	}
}

// TitleAt is the item's title with each run of space one space and none at
// either end, a line break of a folded YAML text included; "" when the tree
// cannot be opened or lacks the registry, the registry holds no list of
// items or does not read, the item is not there, or its title is no text or
// empty once trimmed. The first item of an id wins.
func TestTitleAt(t *testing.T) {
	registry := `items:
  - { id: S-1, title: "  Add the\n\t widget   (#12) " }
  - { id: S-1, title: Other }
  - { id: S-2, title: 7 }
  - { id: S-3, title: "  \n " }
  - { id: S-4 }
  - id: S-6
    title: >
      Folded over
      two lines
`
	files := map[string]string{
		"ok.yaml":       registry,
		"items.yaml":    "items: 3\n",
		"invalid.yaml":  "items: [\n",
		"scalar.yaml":   "just text\n",
		"folder/x.yaml": "items: []\n",
	}
	trees := porttest.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": files, "abc": files}}
	for _, c := range []struct{ path, tree, id, want string }{
		{"ok.yaml", "", "S-1", "Add the widget (#12)"},
		{"ok.yaml", "abc", "S-1", "Add the widget (#12)"},
		{"ok.yaml", "", "S-2", ""},
		{"ok.yaml", "", "S-3", ""},
		{"ok.yaml", "", "S-4", ""},
		{"ok.yaml", "", "S-6", "Folded over two lines"},
		{"ok.yaml", "", "S-9", ""},
		{"items.yaml", "", "S-1", ""},
		{"invalid.yaml", "", "S-1", ""},
		{"scalar.yaml", "", "S-1", ""},
		{"missing.yaml", "", "S-1", ""},
		{"folder", "", "S-1", ""}, // a folder holds a file but cannot be read
		{"ok.yaml", "def", "S-1", ""},
	} {
		if got := TitleAt(trees, c.path, c.tree, c.id); got != c.want {
			t.Errorf("%s at %q, %s: got %q, want %q", c.path, c.tree, c.id, got, c.want)
		}
	}
	if got := TitleAt(porttest.Trees{Files: map[string]map[string]string{"abc": files}}, "ok.yaml", "abc", "S-1"); got != "" {
		t.Errorf("a tree that cannot be opened has the title %q", got)
	}
}
