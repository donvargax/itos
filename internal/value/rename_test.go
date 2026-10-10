package value

import (
	"strings"
	"testing"
)

func TestDocRename(t *testing.T) {
	rename := func(from, to string) func(d *Doc) error {
		return func(d *Doc) error {
			had, err := d.Rename(strings.Split(from, "."), strings.Split(to, "."))
			if err == nil && !had {
				t.Errorf("%s was not there", from)
			}
			return err
		}
	}
	cases := []struct{ name, from, to, text, want string }{
		{"within its mapping, the name alone changes",
			"ci.watch.timeout", "ci.watch.wait",
			"version: 1\n# Wait ten minutes.\nci:\n  watch:\n    timeout: \"600\" # ten\n    provider: none\n",
			"version: 1\n# Wait ten minutes.\nci:\n  watch:\n    wait: \"600\" # ten\n    provider: none\n"},
		{"a quoted key keeps its quotes",
			"a.b", "a.c",
			"a:\n  'b': 1\n  d: 2\n", "a:\n  'c': 1\n  d: 2\n"},
		{"into another mapping, after its last key, the emptied mapping gone",
			"ci.watch.timeout", "work.wait",
			"version: 1\nci:\n  steps: []\n  watch:\n    timeout: 600 # ten\nwork:\n  registry: r.yaml\n",
			"version: 1\nci:\n  steps: []\nwork:\n  registry: r.yaml\n  wait: 600 # ten\n"},
		{"a block value moves indented to its new place",
			"a.b.c", "d.c",
			"a:\n  b:\n    c:\n      - x\n\n      - y\n    e: 1\nd:\n  f: 2\n",
			"a:\n  b:\n    e: 1\nd:\n  f: 2\n  c:\n    - x\n\n    - y\n"},
		{"to the mapping the emptied one was the last key of",
			"r.a.b", "r.c",
			"r:\n  z: 0\n  a:\n    b: 1 # one\nq: 2\n",
			"r:\n  z: 0\n  c: 1 # one\nq: 2\n"},
		{"to a mapping written just before the emptied one",
			"a.b", "d.b",
			"d:\n  e: 2\na:\n  b: 1\n",
			"d:\n  e: 2\n  b: 1\n"},
		{"the mappings the new path lacks are added",
			"a.b", "c.d.e",
			"a:\n  b: |\n    one\n    two\n  k: 1\n",
			"a:\n  k: 1\nc:\n  d:\n    e: |\n      one\n      two\n"},
	}
	for _, c := range cases {
		if got, _ := edited(t, c.text, rename(c.from, c.to)); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}

	d, _ := OpenDoc("a:\n  b: 1\n")
	if had, err := d.Rename([]string{"a", "x"}, []string{"a", "y"}); had || err != nil {
		t.Errorf("a key that is not there: %t, %v", had, err)
	}
}

func TestDocRenameRefuses(t *testing.T) {
	cases := []struct{ name, text, from, to, says string }{
		{"a flow mapping", "ci:\n  watch: { timeout: 600 }\n", "ci.watch.timeout", "ci.watch.wait", "ci.watch is a flow mapping"},
		{"an anchor", "a: &x\n  b: 1\nc: *x\n", "a.b", "a.d", "a has an anchor"},
		{"an alias in the value", "x: &x 1\na:\n  b: [*x]\n", "a.b", "a.d", "a.b has an anchor"},
		{"a key set already", "a:\n  b: 1\n  d: 2\n", "a.b", "a.d", "a.d is set already"},
		{"one path inside the other", "a:\n  b: 1\n", "a", "a.b", "one inside the other"},
		{"a new parent that is no mapping", "a:\n  b: 1\nc: 2\n", "a.b", "c.d", "c is not a mapping"},
		{"a new parent that is empty", "a:\n  b: 1\nc: {}\n", "a.b", "c.d", "c is a flow mapping"},
		{"a top that is a list", "- a\n", "a", "b", ""},
	}
	for _, c := range cases {
		d, err := OpenDoc(c.text)
		if err != nil {
			t.Fatal(err)
		}
		_, err = d.Rename(strings.Split(c.from, "."), strings.Split(c.to, "."))
		if c.says == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v, not one saying %q", c.name, err, c.says)
		}
	}
	if _, err := (&Doc{}).Rename(nil, []string{"a"}); err == nil {
		t.Error("no path was renamed")
	}
}

func TestDocRemove(t *testing.T) {
	cases := []struct{ name, path, text, want string }{
		{"a key and its lines, the comment above kept", "ci.watch.timeout",
			"ci:\n  watch:\n    # ten\n    timeout:\n      - 1\n    provider: none\n",
			"ci:\n  watch:\n    # ten\n    provider: none\n"},
		{"the mappings it leaves empty, up to the top", "a.b.c",
			"version: 1\na:\n  b:\n    c: 1\nd: 2\n", "version: 1\nd: 2\n"},
	}
	for _, c := range cases {
		var held any
		got, _ := edited(t, c.text, func(d *Doc) error {
			v, had, err := d.Remove(strings.Split(c.path, "."))
			if !had {
				t.Errorf("%s: not there", c.name)
			}
			held = v
			return err
		})
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if held == nil {
			t.Errorf("%s: no value held", c.name)
		}
	}

	d, _ := OpenDoc("a: { b: 1 }\n")
	if _, _, err := d.Remove([]string{"a", "b"}); err == nil || !strings.Contains(err.Error(), "a is a flow mapping") {
		t.Errorf("a flow mapping: %v", err)
	}
	if v, had, err := d.Remove([]string{"a", "c"}); v != nil || had || err != nil {
		t.Errorf("a key that is not there: %v, %t, %v", v, had, err)
	}
	if _, _, err := d.Remove(nil); err == nil {
		t.Error("no path was removed")
	}
}

func TestShift(t *testing.T) {
	for _, c := range []struct {
		text  string
		delta int
		want  string
	}{
		{" a\n    b\n\n    c\n", -2, " a\n  b\n\n  c\n"},
		{" a\n b\n", 2, " a\n   b\n"},
		{" a\n b\n", 0, " a\n b\n"},
		// A line that is not indented by spaces as far is left as it is.
		{" a\n\tb\n", -2, " a\n\tb\n"},
	} {
		if got := shift(c.text, c.delta); got != c.want {
			t.Errorf("shift(%q, %d) = %q, want %q", c.text, c.delta, got, c.want)
		}
	}
}

// More refusals: a key the text does not write (the document intended has
// it, the text does not), a key with an anchor, a path through something
// that is no mapping on the way to the new key's place, and a value on the
// text's last line, with no line break, moved.
func TestDocRenameMore(t *testing.T) {
	d, _ := OpenDoc("a:\n  b: 1\n")
	d.Want = NewMap("a", NewMap("b", float64(1), "x", float64(2)))
	if _, err := d.Rename([]string{"a", "x"}, []string{"a", "y"}); err == nil || !strings.Contains(err.Error(), "a.x is not written") {
		t.Errorf("a key the text lacks: %v", err)
	}
	d, _ = OpenDoc("a:\n  &k b: 1\n")
	if _, err := d.Rename([]string{"a", "b"}, []string{"a", "c"}); err == nil || !strings.Contains(err.Error(), "a.b has an anchor") {
		t.Errorf("a key with an anchor: %v", err)
	}
	d, _ = OpenDoc("a:\n  b: 1\nc: 2\n")
	if _, err := d.Rename([]string{"a", "b"}, []string{"c", "d", "e"}); err == nil || !strings.Contains(err.Error(), "c is not a mapping") {
		t.Errorf("a scalar on the way: %v", err)
	}
	if got, _ := edited(t, "d:\n  e: 2\na:\n  b: 1", func(d *Doc) error {
		_, err := d.Rename([]string{"a", "b"}, []string{"d", "b"})
		return err
	}); got != "d:\n  e: 2\n  b: 1\n" {
		t.Errorf("the last line moved: %q", got)
	}
}

// What a Doc refuses when its text no longer holds the nodes it was read
// into: the edits' own checks, which a text read back would otherwise catch
// too late to name the key.
func TestDocEditsOfAChangedText(t *testing.T) {
	doc := func(text, changed string) *Doc {
		d, err := OpenDoc(text)
		if err != nil {
			t.Fatal(err)
		}
		d.e.text = changed
		return d
	}
	rename := func(d *Doc, from, to string) error {
		_, err := d.Rename(strings.Split(from, "."), strings.Split(to, "."))
		return err
	}
	two := "r:\n  a:\n    b: 1\n  z: 0\nd:\n  e: 2\n"
	cases := map[string]error{
		"a key past the end":                              rename(doc("a:\n  b: 1\n", ""), "a.b", "a.c"),
		"a quoted key whose quote does not close":         rename(doc("a:\n  'b': 1\n", "a:\n  'b  1\n"), "a.b", "a.c"),
		"a plain key not written there":                   rename(doc("a:\n  b: 1\n", "a:\n  x: 1\n"), "a.b", "a.c"),
		"a key moved past the end":                        rename(doc("a:\n  b: 1\nd:\n  e: 2\n", ""), "a.b", "d.b"),
		"a key moved that does not start its line":        rename(doc("a:\n  b:\n    c: 1\nd:\n  e: 2\n", "a:\n  b:\n   xc: 1\nd:\n  e: 2\n"), "a.b.c", "d.c"),
		"an emptied mapping that does not start its line": rename(doc(two, "r:\n xa:\n    b: 1\n  z: 0\nd:\n  e: 2\n"), "r.a.b", "d.b"),
		"a new place whose last key is past its line":     rename(doc("a:\n  b: 1\nd:\n  e: 2\n", "a:\n  b: 1\nd:\n \n"), "a.b", "d.b"),
	}
	_, _, cases["a key removed past the end"] = doc("a:\n  b: 1\n  c: 2\n", "").Remove([]string{"a", "b"})
	cases["a key dropped past the end"] = doc("a:\n  b: 1\n", "").Drop([]any{"a", "b"})
	for name, err := range cases {
		if err == nil {
			t.Errorf("%s: edited", name)
		}
	}
}

func TestDocDropRefuses(t *testing.T) {
	d, _ := OpenDoc("a:\n  b: &x 1\nc: 2\n")
	if err := d.Drop([]any{"a", "b"}); err == nil || !strings.Contains(err.Error(), "anchor") {
		t.Errorf("an anchor: %v", err)
	}
	if err := d.Drop([]any{"c", "d"}); err == nil {
		t.Error("a key of a scalar was dropped")
	}
}
