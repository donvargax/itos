package value

import (
	"testing"
)

// pinPairs is what the fuzz target of SetScalars sets: the pin itos pin
// writes.
var pinPairs = [][2]string{{"version", "9.1.0"}, {"checksums", "0123abc"}}

// FuzzStrictYAMLSetScalars holds SetScalars, which reads a document's nodes
// to edit it in place, to Parse, itos's strict reading (T-329): a text Parse
// refuses (a key written twice at any depth, a second document, an alias
// that never ends) is refused, and one it accepts is edited into a text
// Parse reads as the original with pin's version and checksums set and
// nothing else changed. The editor is stricter by layout: a top that is no
// block mapping, a pin it cannot edit (an alias, a tag, a flow top) it
// refuses.
func FuzzStrictYAMLSetScalars(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		parsed, parseErr := Parse(text)
		edited, editErr := SetScalars(text, "pin", pinPairs)
		if parseErr != nil {
			if editErr == nil {
				t.Fatalf("Parse refuses %q (%v), SetScalars writes %q", text, parseErr, edited)
			}
			return
		}
		if editErr != nil {
			return
		}
		top, ok := Copy(parsed).(*Map)
		if !ok {
			t.Fatalf("Parse reads %q as %s, SetScalars edits it into %q", text, JSON(parsed), edited)
		}
		pin, _ := top.At("pin").(*Map)
		if pin == nil {
			pin = NewMap()
		}
		for _, p := range pinPairs {
			pin.Set(p[0], p[1])
		}
		top.Set("pin", pin)
		got, err := Parse(edited)
		if err != nil {
			t.Fatalf("SetScalars edits %q into %q, which Parse refuses: %v", text, edited, err)
		}
		if !same(got, top) {
			t.Fatalf("Parse reads %q with the pin set as %s, SetScalars edits it into %q, read as %s", text, JSON(top), edited, JSON(got))
		}
	})
}

// FuzzStrictYAMLOpenDoc holds OpenDoc, which reads a document's nodes to
// edit it by paths, to Parse, itos's strict reading (T-329): a text Parse
// refuses (a key written twice at any depth, a second document, an alias
// that never ends) is refused, and one it accepts is opened as the value
// Parse reads, which Text, with no edit made, gives back as it was. The
// editor is stricter by layout: a top that is neither a block mapping, a
// block list nor an empty list it refuses.
func FuzzStrictYAMLOpenDoc(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		parsed, parseErr := Parse(text)
		doc, openErr := OpenDoc(text)
		if parseErr != nil {
			if openErr == nil {
				t.Fatalf("Parse refuses %q (%v), OpenDoc opens it as %s", text, parseErr, JSON(doc.Want))
			}
			return
		}
		if openErr != nil {
			return
		}
		if !same(doc.Want, parsed) {
			t.Fatalf("Parse reads %q as %s, OpenDoc as %s", text, JSON(parsed), JSON(doc.Want))
		}
		if back, err := doc.Text(); err != nil || back != text {
			t.Fatalf("OpenDoc of %q gives back %q (%v)", text, back, err)
		}
	})
}
