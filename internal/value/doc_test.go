package value

import "testing"

func TestDocEdits(t *testing.T) {
	block := "# the registry\nphases: { 1: a }\nitems:\n" +
		"  - id: p1-thing # an idea\n    title: A thing\n    owner: null\n    status: todo\n    kind: idea\n" +
		"    depends_on: []\n    why: >\n      A gap\n      left behind.\n" +
		"  - id: slice-3\n    title: 'Three'\n    status: \"todo\"\n    depends_on:\n      - p1-thing # first\n      - other\n"
	d, err := OpenDoc(block)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []error{
		d.Set([]any{"items", 0, "id"}, "slice-7"),
		d.Set([]any{"items", 0, "kind"}, "slice"),
		d.Set([]any{"items", 0, "owner"}, "someone"),
		d.Lead([]any{"items", 0, "why"}, "Was p1-thing."),
		d.Set([]any{"items", 1, "depends_on", 0}, "slice-7"),
		d.Set([]any{"items", 1, "status"}, "doing"),
		d.Set([]any{"items", 1, "owner"}, "someone"),
		d.Lead([]any{"items", 1, "why"}, "Was p1-other."),
	} {
		if step != nil {
			t.Fatal(step)
		}
	}
	got, err := d.Text()
	want := "# the registry\nphases: { 1: a }\nitems:\n" +
		"  - id: slice-7 # an idea\n    title: A thing\n    owner: someone\n    status: todo\n    kind: slice\n" +
		"    depends_on: []\n    why: >\n      Was p1-thing.\n      A gap\n      left behind.\n" +
		"  - id: slice-3\n    title: 'Three'\n    status: \"doing\"\n    owner: someone\n    why: Was p1-other.\n    depends_on:\n      - slice-7 # first\n      - other\n"
	if err != nil || got != want {
		t.Errorf("a block registry:\n got %q, %v\nwant %q", got, err, want)
	}
	if why := Prop(Prop(d.Want, "items").([]any)[0], "why"); why != "Was p1-thing. A gap left behind.\n" {
		t.Errorf("the folded why reads %q", why)
	}

	flow := "phases: { 1: null }\nitems:\n  - { id: p1-thing, title: p1-thing, phase: 1, owner: null, status: todo, kind: idea, depends_on: [] }\n" +
		"  - { id: slice-3, title: slice-3, phase: 1, owner: null, status: todo, depends_on: [p1-thing, x] }\n"
	d, err = OpenDoc(flow)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []error{
		d.Set([]any{"items", 0, "id"}, "slice-7"),
		d.Set([]any{"items", 0, "kind"}, "slice"),
		d.Lead([]any{"items", 0, "why"}, "Was p1-thing."),
		d.Set([]any{"items", 1, "depends_on", 0}, "slice-7"),
		d.Set([]any{"items", 1, "owner"}, "a, b"),
	} {
		if step != nil {
			t.Fatal(step)
		}
	}
	got, err = d.Text()
	want = "phases: { 1: null }\nitems:\n  - { id: slice-7, title: p1-thing, phase: 1, owner: null, status: todo, kind: slice, why: \"Was p1-thing.\", depends_on: [] }\n" +
		"  - { id: slice-3, title: slice-3, phase: 1, owner: \"a, b\", status: todo, depends_on: [slice-7, x] }\n"
	if err != nil || got != want {
		t.Errorf("a flow registry:\n got %q, %v\nwant %q", got, err, want)
	}

	d, err = OpenDoc("items:\n  - id: a\n    why: |\n      One.\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Lead([]any{"items", 0, "why"}, "Was b."); err != nil {
		t.Fatal(err)
	}
	if got, err := d.Text(); err != nil || got != "items:\n  - id: a\n    why: |\n      Was b.\n      One.\n" {
		t.Errorf("a literal why: %q, %v", got, err)
	}

	for name, path := range map[string][]any{
		"no such item": {"items", 5, "id"},
		"no such key":  {"items", 0, "nothing", "id"},
		"an anchor":    {"items", 1, "id"},
	} {
		d, err := OpenDoc("items:\n  - id: a\n  - &x { id: b }\n")
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Set(path, "z"); err == nil {
			t.Errorf("%s: set", name)
		}
	}
}
