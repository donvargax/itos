package work

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// hex is a revision's 64 digits.
const hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// Every line Parse refuses is a usage error, word for word what each
// subcommand said before it was read here.
func TestParseRefuses(t *testing.T) {
	for line, want := range map[string]string{
		"--as":                               "work --as needs a handle",
		"--as=":                              "work --as needs a handle",
		"--as ":                              "work --as needs a handle",
		"--as=me -x":                         "work does not take -x: " + takes,
		"check x.yaml more":                  "work check takes at most one file, not more",
		"nosuch":                             "work has no subcommand nosuch: " + takes,
		"--as a extra":                       "work has no subcommand extra: " + takes,
		"list nosuch":                        "work list takes only --all and --tag <tag>: nosuch",
		"list nosuch other":                  "work list takes only --all and --tag <tag>: nosuch other",
		"list --tag=":                        "work list takes one --tag with a value",
		"list --bogus":                       "work list does not take --bogus",
		"show":                               "work show needs one <id>",
		"show a b":                           "work show needs one <id>",
		"show a -x":                          "work show does not take -x",
		"take":                               "work take needs one <id>",
		"take a b":                           "work take needs one <id>",
		"take a --bogus":                     "work take does not take --bogus",
		"take a --as":                        "work take --as needs a value",
		"take a --as=":                       "work take takes one --as with a value",
		"take a --as x --as y":               "work take takes one --as with a value",
		"done":                               "work done needs one <id>",
		"done a --as x":                      "work done does not take --as",
		"take a --expect":                    "work take --expect needs a value",
		"take a --expect=":                   "work take takes one --expect with a value",
		"take a --expect abc":                "work take --expect takes the revision itos next gives, \"sha256:\" and 64 lowercase hex digits, not abc",
		"take a --expect sha256:" + hex[:63]: "work take --expect takes the revision itos next gives, \"sha256:\" and 64 lowercase hex digits, not sha256:" + hex[:63],
		"take a --expect sha256:" + strings.ToUpper(hex):                   "work take --expect takes the revision itos next gives, \"sha256:\" and 64 lowercase hex digits, not sha256:" + strings.ToUpper(hex),
		"done a --expect sha1:" + hex:                                      "work done --expect takes the revision itos next gives, \"sha256:\" and 64 lowercase hex digits, not sha1:" + hex,
		"done a --expect x --expect y":                                     "work done takes one --expect with a value",
		"promote":                                                          "work promote needs one <id>",
		"promote a b --kind slice":                                         "work promote needs one <id>",
		"promote a --bogus x":                                              "work promote does not take --bogus",
		"promote a --kind":                                                 "work promote --kind needs a value",
		"promote a":                                                        "work promote needs --kind slice|task",
		"promote a --kind idea":                                            "work promote needs --kind slice|task",
		"promote a --kind slice --kind slice":                              "work promote takes one --kind with a value",
		"promote a --kind=":                                                "work promote takes one --kind with a value",
		"promote a --kind=\t":                                              "work promote takes one --kind with a value",
		"promote a --kind slice --id x":                                    "work promote mints the slice id; do not pass --id",
		"promote a --kind slice --why w":                                   "work promote --kind slice does not take --why, which only a task's promotion takes",
		"promote a --kind slice --check c":                                 "work promote --kind slice does not take --check, which only a task's promotion takes",
		"promote a --kind task":                                            "work promote --kind task needs --type <type> and a --check <command>",
		"promote a --kind task --type chore":                               "work promote --kind task needs --type <type> and a --check <command>",
		"promote a --kind task --check c":                                  "work promote --kind task needs --type <type> and a --check <command>",
		"promote a --kind task --type t --timeout 3":                       "work promote takes one --timeout <seconds> after a --check, a number above 0, for that check",
		"promote a --kind task --type t --check c --timeout 0":             "work promote takes one --timeout <seconds> after a --check, a number above 0, for that check",
		"promote a --kind task --type t --check c --timeout 3 --timeout 4": "work promote takes one --timeout <seconds> after a --check, a number above 0, for that check",
		"promote a --kind task --type t --check=":                          "work promote --check needs a command",
		"add":                                  "work add needs --title <title> and --why <why>",
		"add a b --title t --why w":            "work add takes at most one <id>",
		"add a --title t":                      "work add needs --title <title> and --why <why>",
		"add a --why w":                        "work add needs --title <title> and --why <why>",
		"add a --title t --why=\t":             "work add needs --title <title> and --why <why>",
		"add a --title t --why w --kind nope":  "work add takes --kind idea|slice|task|bug",
		"add --title t --why w":                "work add needs <idea-id> when --kind is idea",
		"add --title t --why w --kind idea":    "work add needs <idea-id> when --kind is idea",
		"add a --title t --why w --kind slice": "work add mints the slice id; do not pass one",
		"add a --title t --why w --bogus x":    "work add does not take --bogus",
		"add a --title t --why w --kind":       "work add --kind needs a value",
		"edit":                                 "work edit needs one <id> or more",
		"edit a":                               "work edit needs --title, --depends-on, --refs, --tags or --note",
		"edit a a --note n":                    "work edit names a twice",
		"edit a b --title t":                   "work edit --title is one item's own: give it one <id>, not 2",
		"edit a --title=":                      "work edit takes one --title with a value",
		"edit a --note n --note m":             "work edit takes one --note with a value",
		"queue":                                "work queue needs one <id> or more",
		"queue --top":                          "work queue needs one <id> or more",
		"queue a":                              "work queue needs one of --top, --before <id>, --after <id> or --remove",
		"queue a --top --remove":               "work queue needs one of --top, --before <id>, --after <id> or --remove",
		"queue a --before b --after c":         "work queue needs one of --top, --before <id>, --after <id> or --remove",
		"queue a --top --before b":             "work queue needs one of --top, --before <id>, --after <id> or --remove",
		"queue a --remove --after b":           "work queue needs one of --top, --before <id>, --after <id> or --remove",
		"queue a b a --top":                    "work queue names a twice",
		"queue a --bogus":                      "work queue does not take --bogus",
		"drop":                                 "work drop needs one <id> or more",
		"drop a":                               "work drop needs --why <reason>, the commit's record of why the item is dropped",
		"drop a --why=\t":                      "work drop needs --why <reason>, the commit's record of why the item is dropped",
		"drop a a --why w":                     "work drop names a twice",
		"drop a --as x --why w":                "work drop does not take --as",
		"defer a":                              "work defer needs --why <reason>, why the item is put off, which its deferred key holds",
		"defer a --why=":                       "work defer takes one --why with a value",
		"resume":                               "work resume needs one <id> or more",
		"resume a a":                           "work resume names a twice",
		"resume a --why w":                     "work resume does not take --why",
		"give-back":                            "work give-back needs one <id> or more",
		"give-back a":                          "work give-back needs --why <reason>, the commit's record of why the item is given back",
		"give-back a --as x":                   "work give-back needs --why <reason>, the commit's record of why the item is given back",
		"give-back a --why w --as":             "work give-back --as needs a value",
		"give-back a --bogus":                  "work give-back does not take --bogus",
	} {
		_, err := Parse(strings.Split(line, " "), "", false, false)
		var usage out.UsageError
		if !errors.As(err, &usage) || err.Error() != want {
			t.Errorf("Parse(%q) = %v, want the usage error %q", line, err, want)
		}
	}
}

// What Parse reads of every subcommand's line, each flag in both of its
// forms.
func TestParseReads(t *testing.T) {
	title := "T"
	none, deps := []string{}, []string{"a", "b"}
	three := 3.0
	for _, c := range []struct {
		line string
		want Query
	}{
		{"", Query{}},
		{"--as a", Query{As: "a"}},
		{"--as=a", Query{As: "a"}},
		{"--as a --as=b", Query{As: "b"}},
		{"list", Query{Sub: "list"}},
		{"list --all --tag=cli", Query{Sub: "list", All: true, Tag: "cli"}},
		{"check", Query{Sub: "check"}},
		{"show a", Query{Sub: "show", ID: "a"}},
		{"show --patch a", Query{Sub: "show", ID: "a", Patch: true}},
		{"take a", Query{Sub: "take", ID: "a"}},
		{"take --as=b a", Query{Sub: "take", ID: "a", As: "b"}},
		{"take a --expect sha256:" + hex, Query{Sub: "take", ID: "a", Expect: "sha256:" + hex}},
		{"done a", Query{Sub: "done", ID: "a"}},
		{"done a --expect=sha256:" + hex, Query{Sub: "done", ID: "a", Expect: "sha256:" + hex}},
		{"promote a --kind slice --title=New", Query{Sub: "promote", Promotion: Promotion{ID: "a", Kind: "slice", Title: "New"}}},
		{"promote a --kind=task --type chore --why W --check c --timeout=3 --check d", Query{Sub: "promote",
			Promotion: Promotion{ID: "a", Kind: "task", Type: "chore", Why: "W", Checks: []ledger.NewCheck{{Run: "c", Timeout: &three}, {Run: "d"}}}}},
		{"add a --title T --why=W", Query{Sub: "add", New: domain.New{ID: "a", Title: "T", Why: "W", Kind: "idea", DependsOn: none, Refs: none, Tags: none}}},
		{"add --kind bug --title T --why W --phase 2 --owner o --depends-on a,,b --refs=r --tags x", Query{Sub: "add",
			New: domain.New{Title: "T", Why: "W", Kind: "bug", Phase: "2", Owner: "o", DependsOn: deps, Refs: []string{"r"}, Tags: []string{"x"}}}},
		{"edit a --title T --note N", Query{Sub: "edit", IDs: []string{"a"}, Edits: domain.Edits{Title: &title, Note: "N"}}},
		{"edit a b --depends-on=a,b --refs= --tags ,", Query{Sub: "edit", IDs: []string{"a", "b"},
			Edits: domain.Edits{DependsOn: &deps, Refs: &none, Tags: &none}}},
		{"queue a b --top", Query{Sub: "queue", IDs: []string{"a", "b"}, Place: domain.Place{Top: true}}},
		{"queue a --remove", Query{Sub: "queue", IDs: []string{"a"}, Place: domain.Place{Remove: true}}},
		{"queue a --before=b", Query{Sub: "queue", IDs: []string{"a"}, Place: domain.Place{Before: "b"}}},
		{"queue a --after b", Query{Sub: "queue", IDs: []string{"a"}, Place: domain.Place{After: "b"}}},
		{"drop a b --why W", Query{Sub: "drop", IDs: []string{"a", "b"}, Why: "W"}},
		{"defer a --why=W", Query{Sub: "defer", IDs: []string{"a"}, Why: "W"}},
		{"resume a b", Query{Sub: "resume", IDs: []string{"a", "b"}}},
		{"give-back a --why W --as=b", Query{Sub: "give-back", IDs: []string{"a"}, Why: "W", As: "b"}},
	} {
		args := strings.Fields(c.line)
		got, err := Parse(args, "", false, false)
		if err != nil || !reflect.DeepEqual(got, CLI{Query: c.want}) {
			t.Errorf("Parse(%q) = %+v, %v, want %+v", c.line, got, err, c.want)
		}
	}
	got, err := Parse([]string{"show", "a"}, "", true, true)
	if err != nil || !reflect.DeepEqual(got, CLI{Query: Query{Sub: "show", ID: "a", JSON: true}, JSON: true, Quiet: true}) {
		t.Errorf("Parse(show a) under --json -q = %+v, %v", got, err)
	}
}

// work check's file is read from where itos was started, unless it is
// absolute or stdin.
func TestParseReadsCheckFromTheOrigin(t *testing.T) {
	abs, err := filepath.Abs("x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ origin, file, want string }{
		{"sub", "x.yaml", filepath.Join("sub", "x.yaml")},
		{"", "x.yaml", "x.yaml"},
		{"sub", "-", "-"},
		{"sub", abs, abs},
	} {
		got, err := Parse([]string{"check", c.file}, c.origin, false, false)
		if err != nil || got.File != c.want || !got.Named {
			t.Errorf("Parse(check %s) from %q = %+v, %v, want %s", c.file, c.origin, got, err, c.want)
		}
	}
	if got, err := Parse([]string{"check"}, "sub", false, false); err != nil || got.File != "" || got.Named {
		t.Errorf("Parse(check) from sub = %+v, %v", got, err)
	}
}

// AddLine and PromoteLine are what draft reads a drafted line with.
func TestAddLineAndPromoteLine(t *testing.T) {
	n, err := AddLine([]string{"--kind", "task", "--title", "T", "--why", "W"})
	if err != nil || n.Kind != "task" || n.ID != "" {
		t.Errorf("AddLine = %+v, %v", n, err)
	}
	// An empty argument is an idea's id still, for work.Add to judge.
	n, err = AddLine([]string{"", "--title", "T", "--why", "W"})
	if err != nil || n.Kind != "idea" || n.ID != "" {
		t.Errorf("AddLine of an empty id = %+v, %v", n, err)
	}
	p, err := PromoteLine([]string{"a", "--kind", "slice"})
	if err != nil || p.Kind != "slice" || p.ID != "a" {
		t.Errorf("PromoteLine = %+v, %v", p, err)
	}
}
