package next

import (
	"testing"
)

// RevisionOf is the revision Handle gives of the item, over the same facts,
// so a mutating command handed it back (--expect, slice 208) judges it as
// next computed it; an id no item has is refused as next refuses it, with
// no revision; a registry that is not there likewise, and a questions file
// that cannot be read is the error.
func TestRevisionOf(t *testing.T) {
	p := over(t, fixture{
		files: map[string]string{"work-items.yaml": registryOf(
			item("id: slice-9, title: Nine, phase: 1, owner: ana, status: todo, depends_on: []"),
		), "people.yaml": "- ana\n"},
		head: map[string]string{"features/a.feature": feature("@ID-A-01 @wip @slice-9")},
	})
	a, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"})
	if err != nil || a.Next == nil {
		t.Fatalf("Handle(slice-9) = %+v, %v", a, err)
	}
	revision, refused, err := RevisionOf(t.Context(), p.cfg, p.tracker, "slice-9")
	if err != nil || refused != nil || revision != a.Next.Revision || !digest.MatchString(revision) {
		t.Errorf("RevisionOf(slice-9) = %q, %v, %v, want Handle's %q", revision, refused, err, a.Next.Revision)
	}
	revision, refused, err = RevisionOf(t.Context(), p.cfg, p.tracker, "nope")
	if err != nil || refused == nil || refused.Rule != "work-unknown-item" || revision != "" {
		t.Errorf("RevisionOf(nope) = %q, %v, %v, want the unknown item refused", revision, refused, err)
	}

	p = over(t, fixture{files: map[string]string{"people.yaml": "- ana\n"}})
	if revision, refused, err = RevisionOf(t.Context(), p.cfg, p.tracker, "slice-9"); err != nil || refused == nil || revision != "" {
		t.Errorf("RevisionOf with no registry = %q, %v, %v, want refused", revision, refused, err)
	}

	p = over(t, fixture{
		files:     map[string]string{"work-items.yaml": registryOf(item("id: slice-9, title: Nine, phase: 1, owner: ana, status: todo, depends_on: []")), "people.yaml": "- ana\n"},
		questions: "questions: [",
	})
	if revision, refused, err = RevisionOf(t.Context(), p.cfg, p.tracker, "slice-9"); err == nil || refused != nil || revision != "" {
		t.Errorf("RevisionOf over questions that cannot be read = %q, %v, %v, want the error", revision, refused, err)
	}
}
