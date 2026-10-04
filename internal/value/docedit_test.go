package value

import (
	"strings"
	"testing"
)

func TestWrap(t *testing.T) {
	if got := Wrap("one two three four", 9); got != "one two\nthree\nfour" {
		t.Errorf("%q", got)
	}
}

// edited is the text after the edits, each made on a Doc of text.
func edited(t *testing.T, text string, edits ...func(d *Doc) error) (string, *Doc) {
	t.Helper()
	d, err := OpenDoc(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edits {
		if err := e(d); err != nil {
			t.Fatal(err)
		}
	}
	got, err := d.Text()
	if err != nil {
		t.Fatal(err)
	}
	return got, d
}

func TestDocAppend(t *testing.T) {
	item := NewMap("id", "p1-new", "title", "A new: thing", "phase", float64(1), "owner", nil,
		"status", "todo", "depends_on", []any{"a", "b, c"}, "kind", "idea",
		"why", "Because "+strings.Repeat("it is missing ", 10)+"still.")
	add := func(d *Doc) error { return d.Append([]any{"items"}, item, "why") }

	// Blank lines part the items, so one parts the new one; it goes before
	// the comment that heads what follows, after the last item's last line.
	block := "items:\n  - id: a\n    title: A\n    why: >\n      a\n      reason\n\n  - id: b # b\n    title: B\n\n  # ---- the end\nphases: { 1: x }\n"
	got, d := edited(t, block, add)
	want := "items:\n  - id: a\n    title: A\n    why: >\n      a\n      reason\n\n  - id: b # b\n    title: B\n\n" +
		"  - id: p1-new\n    title: \"A new: thing\"\n    phase: 1\n    owner: null\n    status: todo\n" +
		"    depends_on: [a, \"b, c\"]\n    kind: idea\n    why: >\n" +
		"      Because it is missing it is missing it is missing it is missing it is missing it is missing it\n" +
		"      is missing it is missing it is missing it is missing still.\n" +
		"\n  # ---- the end\nphases: { 1: x }\n"
	if got != want {
		t.Errorf("a block list:\n got %q\nwant %q", got, want)
	}
	if why := Prop(Prop(d.Want, "items").([]any)[2], "why"); why != words(item.At("why").(string))+"\n" {
		t.Errorf("the folded why reads %q", why)
	}

	// Items one to a line, the last with no line break: none parts them.
	flow := "phases: { 1: null }\nitems:\n- { id: a, title: A }\n- { id: b, title: B }"
	got, _ = edited(t, flow, func(d *Doc) error {
		return d.Append([]any{"items"}, NewMap("id", "c", "title", "12", "depends_on", []any{}))
	})
	if want := flow + "\n- id: c\n  title: \"12\"\n  depends_on: []\n"; got != want {
		t.Errorf("a list of flow items:\n got %q\nwant %q", got, want)
	}

	d, _ = OpenDoc("items: [a]\n")
	if err := d.Append([]any{"items"}, NewMap("id", "x")); err == nil {
		t.Error("a flow list is not appended to")
	}
}

func TestDocSetList(t *testing.T) {
	text := "items:\n  - { id: a, depends_on: [x, 'y'] }\n  - id: b\n    depends_on:\n      - x # first\n      - y\n      - z\n" +
		"  - id: c\n    title: C\n  - id: d\n    depends_on: ~\n"
	got, d := edited(t, text,
		func(d *Doc) error { return d.SetList([]any{"items", 0, "depends_on"}, []string{"p", "q, r"}) },
		func(d *Doc) error { return d.SetList([]any{"items", 1, "depends_on"}, []string{"p", "q"}) },
		func(d *Doc) error { return d.SetList([]any{"items", 2, "refs"}, []string{"docs/x.md"}) },
		func(d *Doc) error { return d.SetList([]any{"items", 3, "depends_on"}, []string{}) },
		func(d *Doc) error { return d.SetList([]any{"items", 0, "refs"}, []string{"r"}) },
	)
	want := "items:\n  - { id: a, depends_on: [p, \"q, r\"], refs: [r] }\n  - id: b\n    depends_on:\n      - p # first\n      - q\n" +
		"  - id: c\n    title: C\n    refs: [docs/x.md]\n  - id: d\n    depends_on: []\n"
	if got != want {
		t.Errorf("lists:\n got %q\nwant %q", got, want)
	}
	if deps := Prop(Prop(d.Want, "items").([]any)[1], "depends_on"); JSON(deps) != `["p","q"]` {
		t.Errorf("b's depends_on reads %s", JSON(deps))
	}
	got, _ = edited(t, "items:\n  - id: b\n    depends_on:\n      - x\n",
		func(d *Doc) error { return d.SetList([]any{"items", 0, "depends_on"}, []string{"x", "y"}) })
	if want := "items:\n  - id: b\n    depends_on:\n      - x\n      - y\n"; got != want {
		t.Errorf("a longer block list:\n got %q\nwant %q", got, want)
	}
}

func TestDocNote(t *testing.T) {
	text := "items:\n  - id: a\n    why: >\n      A gap\n      left behind.\n\n  - id: b\n    why: One line. # said\n" +
		"  - { id: c, why: one line }\n  - { id: d }\n  - id: e\n    why: |-\n      kept\n      as is\n  - id: f\n    why: >-\n      stripped\n"
	note := func(i int) func(d *Doc) error {
		return func(d *Doc) error { return d.Note([]any{"items", i, "why"}, "  Seen   again. ") }
	}
	got, d := edited(t, text, note(0), note(1), note(2), note(3), note(4), note(5))
	want := "items:\n  - id: a\n    why: >\n      A gap\n      left behind.\n\n      Seen again.\n\n" +
		"  - id: b\n    why: > # said\n      One line.\n\n      Seen again.\n" +
		"  - { id: c, why: \"one line\\nSeen again.\" }\n  - { id: d, why: \"Seen again.\" }\n" +
		"  - id: e\n    why: |-\n      kept\n      as is\n\n      Seen again.\n  - id: f\n    why: >-\n      stripped\n\n      Seen again.\n"
	if got != want {
		t.Errorf("notes:\n got %q\nwant %q", got, want)
	}
	items := Prop(d.Want, "items").([]any)
	for i, why := range []string{"A gap left behind.\nSeen again.\n", "One line.\nSeen again.\n", "one line\nSeen again.",
		"Seen again.", "kept\nas is\n\nSeen again.", "stripped\nSeen again."} {
		if got := Prop(items[i], "why"); got != why {
			t.Errorf("item %d's why reads %q, not %q", i, got, why)
		}
	}
	got, _ = edited(t, "items:\n  - id: a\n    deps:\n      - b\n  - id: c\n", func(d *Doc) error { return d.Note([]any{"items", 0, "why"}, "New.") })
	if want := "items:\n  - id: a\n    deps:\n      - b\n    why: >\n      New.\n  - id: c\n"; got != want {
		t.Errorf("a new why:\n got %q\nwant %q", got, want)
	}
	d, _ = OpenDoc("items:\n  - id: a\n    why: |+\n      kept\n\n")
	if err := d.Note([]any{"items", 0, "why"}, "x"); err == nil {
		t.Error("a block keeping its trailing lines is not noted")
	}
}

// A ledger file: a list at the top, a task appended with its checks as a
// block list of mappings below their key (slice 55).
func TestDocAppendTop(t *testing.T) {
	task := NewMap("id", "T-3", "type", "chore", "title", "Tidy", "why", "It drifted.",
		"done_when", []any{NewMap("run", "true"), NewMap("run", "go test ./...", "timeout", float64(900))})
	text := "# the ledger\n- id: T-1\n  title: One\n\n- { id: T-2, title: Two }\n\n# a closing note\n"
	got, d := edited(t, text, func(d *Doc) error { return d.Append(nil, task, "why") })
	added := "- id: T-3\n  type: chore\n  title: Tidy\n  why: >\n    It drifted.\n  done_when:\n" +
		"    - run: \"true\"\n    - run: go test ./...\n      timeout: 900\n"
	if want := "# the ledger\n- id: T-1\n  title: One\n\n- { id: T-2, title: Two }\n\n" + added + "\n# a closing note\n"; got != want {
		t.Errorf("a list at the top:\n got %q\nwant %q", got, want)
	}
	if list := d.Want.([]any); len(list) != 3 || JSON(Prop(list[2], "done_when")) != `[{"run":"true"},{"run":"go test ./...","timeout":900}]` {
		t.Errorf("the list reads %s", JSON(d.Want))
	}
	one, err := BlockItem(task, "why")
	if err != nil || one != added {
		t.Errorf("one item:\n got %q, %v\nwant %q", one, err, added)
	}
}
