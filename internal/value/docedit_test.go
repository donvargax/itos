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
	d, _ = OpenDoc("phases: { 1: null, items: [] }\n")
	if err := d.Append([]any{"phases", "items"}, NewMap("id", "x")); err == nil {
		t.Error("an empty list in a flow mapping is not appended to")
	}
}

// An empty flow list, as itos init writes the registry's items (bug 12): the
// brackets go, and the item is a block list below its key, two columns in;
// a comment after them stays, and a list on a line of its own is replaced.
func TestDocAppendEmpty(t *testing.T) {
	item := NewMap("id", "p1-new", "title", "New", "depends_on", []any{})
	for _, c := range []struct{ name, text, want string }{
		{"after its key", "# the registry\nphases:\n  1: null\nitems: []\n",
			"# the registry\nphases:\n  1: null\nitems:\n  - id: p1-new\n    title: New\n    depends_on: []\n"},
		{"with a comment", "items:   [] # none yet\nnext: 1",
			"items: # none yet\n  - id: p1-new\n    title: New\n    depends_on: []\nnext: 1"},
		{"below its key", "top:\n  items:\n    []\n  next: 1\n",
			"top:\n  items:\n    - id: p1-new\n      title: New\n      depends_on: []\n  next: 1\n"},
	} {
		path := []any{"items"}
		if strings.HasPrefix(c.text, "top:") {
			path = []any{"top", "items"}
		}
		got, d := edited(t, c.text, func(d *Doc) error { return d.Append(path, item) })
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if list, ok := Prop(d.Want, "items").([]any); path[0] == "items" && (!ok || len(list) != 1) {
			t.Errorf("%s: the items read %s", c.name, JSON(d.Want))
		}
	}
	// A ledger file of comments and [], as init --stealth writes it.
	got, d := edited(t, "# the ledger\n[]\n", func(d *Doc) error { return d.Append(nil, NewMap("id", "T-1")) })
	if want := "# the ledger\n- id: T-1\n"; got != want {
		t.Errorf("a list at the top:\n got %q\nwant %q", got, want)
	}
	if JSON(d.Want) != `[{"id":"T-1"}]` {
		t.Errorf("the list reads %s", JSON(d.Want))
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
	want := "items:\n  - { id: a, depends_on: [p, \"q, r\"], refs: [r] }\n  - id: b\n    depends_on: [p, q]\n" +
		"  - id: c\n    title: C\n    refs: [docs/x.md]\n  - id: d\n    depends_on: []\n"
	if got != want {
		t.Errorf("lists:\n got %q\nwant %q", got, want)
	}
	if deps := Prop(Prop(d.Want, "items").([]any)[1], "depends_on"); JSON(deps) != `["p","q"]` {
		t.Errorf("b's depends_on reads %s", JSON(deps))
	}
	got, _ = edited(t, "items:\n  - id: b\n    depends_on:\n      - x\n",
		func(d *Doc) error { return d.SetList([]any{"items", 0, "depends_on"}, []string{"x", "y"}) })
	if want := "items:\n  - id: b\n    depends_on: [x, y]\n"; got != want {
		t.Errorf("a longer block list:\n got %q\nwant %q", got, want)
	}
}

// Bug 14: a list of any shape is written whole as one flow list on its key's
// line, the old one's text all taken out; what follows it is kept.
func TestDocSetListAnyShape(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"a flow list over several lines",
			"items:\n  - id: a\n    refs:\n      [\n        x.md, # one\n        \"y], z.md\",\n      ] # after\n    kind: idea\n",
			"items:\n  - id: a\n    refs: [p.md] # after\n    kind: idea\n"},
		{"a flow list opened on its key's line, closed below",
			"items:\n  - id: a\n    refs: [x.md,\n      y.md]\n    kind: idea\n",
			"items:\n  - id: a\n    refs: [p.md]\n    kind: idea\n"},
		{"a block list, a comment in it and one after it",
			"items:\n  - id: a\n    refs: # the refs\n      - x.md\n      # between\n      - >\n        y.md\n\n    # kept\n    kind: idea\n",
			"items:\n  - id: a\n    refs: [p.md]\n\n    # kept\n    kind: idea\n"},
		{"a block list at its key's column, last in the file",
			"items:\n  - id: a\n    refs:\n    - x.md\n    - y.md",
			"items:\n  - id: a\n    refs: [p.md]"},
	}
	for _, c := range cases {
		got, d := edited(t, c.text, func(d *Doc) error { return d.SetList([]any{"items", 0, "refs"}, []string{"p.md"}) })
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if refs := Prop(Prop(d.Want, "items").([]any)[0], "refs"); JSON(refs) != `["p.md"]` {
			t.Errorf("%s: the refs read %s", c.name, JSON(refs))
		}
	}
	got, _ := edited(t, "items:\n  - id: a\n    refs:\n      - a.md\n      - b.md\n    kind: idea\n",
		func(d *Doc) error { return d.SetList([]any{"items", 0, "refs"}, []string{}) })
	if want := "items:\n  - id: a\n    refs: []\n    kind: idea\n"; got != want {
		t.Errorf("a block list emptied:\n got %q\nwant %q", got, want)
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

// The queue's shape (slice 66): a block list whatever it was, added before
// items above the comments that lead into it, emptied to [].
func TestDocSetBlockList(t *testing.T) {
	set := func(list ...string) func(d *Doc) error {
		return func(d *Doc) error { return d.SetBlockList([]any{"queue"}, list, "items") }
	}
	for _, c := range []struct{ name, text, want string }{
		{"added before items, after a blank line",
			"phases:\n  1: a\n\n# the items\nitems: []\n",
			"phases:\n  1: a\n\nqueue:\n  - b\n  - \"c: d\"\n\n# the items\nitems: []\n"},
		{"added before items, no blank line",
			"phases: { 1: null }\nitems: []\n",
			"phases: { 1: null }\nqueue:\n  - b\n  - \"c: d\"\nitems: []\n"},
		{"added last with no items", "phases: { 1: null }\n", "phases: { 1: null }\nqueue:\n  - b\n  - \"c: d\"\n"},
		{"a flow list replaced", "queue: [x, y] # q\nitems: []\n", "queue:\n  - b\n  - \"c: d\" # q\nitems: []\n"},
		{"a block list replaced in its column", "queue:\n    - x\n    - y\n\nitems: []\n", "queue:\n    - b\n    - \"c: d\"\n\nitems: []\n"},
		{"nothing replaced", "queue:\nitems: []\n", "queue:\n  - b\n  - \"c: d\"\nitems: []\n"},
	} {
		if got, _ := edited(t, c.text, set("b", "c: d")); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
	if got, _ := edited(t, "queue:\n  - x\n  - y\nitems: []\n", set()); got != "queue: []\nitems: []\n" {
		t.Errorf("emptied: %q", got)
	}
}

// A key dropped with its value (slice 76): a folded block's lines, a blank
// line after it kept; a pair of a flow mapping with its comma, first, last
// or between; a key the mapping lacks is no edit.
func TestDocDrop(t *testing.T) {
	drop := func(i int, key string) func(d *Doc) error {
		return func(d *Doc) error { return d.Drop([]any{"items", i, key}) }
	}
	text := "items:\n  - id: a\n    why: >\n      A gap\n\n      left behind.\n    refs: [a.md]\n\n" +
		"  - id: b\n    status: done\n    why: One line. # said\n\n" +
		"  - { id: c, why: \"one, line\", status: done }\n  - { why: [x, y], id: d }\n  - { id: e, why: z }\n  - id: f\n"
	got, d := edited(t, text, drop(0, "why"), drop(1, "why"), drop(2, "why"), drop(3, "why"), drop(4, "why"), drop(5, "why"))
	want := "items:\n  - id: a\n    refs: [a.md]\n\n  - id: b\n    status: done\n\n" +
		"  - { id: c, status: done }\n  - { id: d }\n  - { id: e }\n  - id: f\n"
	if got != want {
		t.Errorf("dropped:\n got %q\nwant %q", got, want)
	}
	for i, item := range Prop(d.Want, "items").([]any) {
		if item.(*Map).Has("why") {
			t.Errorf("item %d still reads a why", i)
		}
	}
	d, _ = OpenDoc("items:\n  - why: first\n    id: a\n")
	if err := d.Drop([]any{"items", 0, "why"}); err == nil {
		t.Error("a key on its item's dash line is not dropped")
	}
}

func TestDocAdd(t *testing.T) {
	add := func(path []any, s string) func(d *Doc) error { return func(d *Doc) error { return d.Add(path, s) } }
	// A block mapping whose keys all hold collections: after the last one's
	// lines, at its keys' column, before what follows it.
	block := "commits:\n  footers:\n    Task: { source: ledger }\n  scopes:\n    docs:\n      only: [a]\n\nhooks: { bin: itos }\n"
	got, d := edited(t, block, add([]any{"commits", "since"}, "abc123"))
	want := "commits:\n  footers:\n    Task: { source: ledger }\n  scopes:\n    docs:\n      only: [a]\n  since: abc123\n\nhooks: { bin: itos }\n"
	if got != want || Prop(Prop(d.Want, "commits"), "since") != "abc123" {
		t.Errorf("a block mapping:\n got %q\nwant %q", got, want)
	}
	// A flow mapping, its line breaks CRLF: before the closing brace.
	got, _ = edited(t, "hooks: { bin: itos }\r\ncommits: { types: [a] }\r\n", add([]any{"commits", "since"}, "123"))
	if want := "hooks: { bin: itos }\r\ncommits: { types: [a], since: \"123\" }\r\n"; got != want {
		t.Errorf("a flow mapping:\n got %q\nwant %q", got, want)
	}
	// A key there already, an empty mapping and a path that is no mapping's
	// key are refused.
	for _, c := range []struct {
		text string
		path []any
	}{
		{"commits:\n  since: x\n", []any{"commits", "since"}},
		{"commits: {}\n", []any{"commits", "since"}},
		{"commits: [a]\n", []any{"commits", "since"}},
	} {
		d, err := OpenDoc(c.text)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Add(c.path, "y"); err == nil {
			t.Errorf("%q: Add was not refused", c.text)
		}
	}
}
