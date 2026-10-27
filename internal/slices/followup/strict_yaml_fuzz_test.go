package followup

import (
	"testing"

	"github.com/donvargax/itos/v8/internal/slices/followup/domain"
	"github.com/donvargax/itos/v8/internal/value"
)

// FuzzStrictYAMLFollow holds read, the reader of the threads' file, to
// value.Parse, itos's strict reading (T-329): a file Parse refuses (a key
// written twice at any depth, a second document, an alias that never ends)
// is refused, and one it accepts is read as Parse reads it, the same threads
// with the same notes and strings. It is the slice's reader that is held, not
// domain.Parse alone: the domain imports nothing of ours outside the slices
// (ADR-0053), so the slice asks value.CheckWritten before it parses.
//
// A null key and a null list item, at any depth, are refused too, since no
// file itos writes has one and yaml/v3 drops them (bug 152).
//
// The reader is stricter by schema: an unknown key, an id no thread may
// have, a status that is neither open nor closed are refused, and so is a
// value of another shape than its field's. It accepts more in one way, by
// yaml/v3's typed decoding: a number or a boolean written where a string
// goes is read as the text written (title: 1.50 is "1.50", where Parse
// reads the number 1.5), so a field Parse reads as such a scalar matches any
// text. An input with a merge key (<<) is not compared: yaml/v3 merges its
// mapping into the one holding it, where Parse reads a key named <<.
func FuzzStrictYAMLFollow(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		parsed, parseErr := value.Parse(text)
		got, readErr := read(domain.FileName, []byte(text))
		if parseErr != nil {
			if readErr == nil {
				t.Fatalf("value.Parse refuses %q (%v), read reads %+v", text, parseErr, got)
			}
			return
		}
		if readErr != nil {
			return
		}
		if hasNull(parsed) {
			t.Fatalf("value.Parse reads %q with a null key or list item, read reads %+v", text, got)
		}
		if hasMerge(parsed) {
			return
		}
		if !threadsMatch(parsed, got) {
			t.Fatalf("value.Parse reads %q as %s, read reads %+v", text, value.JSON(parsed), got)
		}
	})
}

// hasNull is whether a value has a null key (a key "") or a null list item,
// which no file itos writes has and the reader refuses (bug 152).
func hasNull(v any) bool {
	switch t := v.(type) {
	case *value.Map:
		for _, k := range t.Keys() {
			if k == "" || hasNull(t.At(k)) {
				return true
			}
		}
	case []any:
		for _, e := range t {
			if e == nil || hasNull(e) {
				return true
			}
		}
	}
	return false
}

// hasMerge is whether a value has a mapping with a merge key.
func hasMerge(v any) bool {
	switch t := v.(type) {
	case *value.Map:
		for _, k := range t.Keys() {
			if k == "<<" || hasMerge(t.At(k)) {
				return true
			}
		}
	case []any:
		for _, e := range t {
			if hasMerge(e) {
				return true
			}
		}
	}
	return false
}

// threadsMatch is whether the threads read are those Parse's value holds.
func threadsMatch(parsed any, got domain.File) bool {
	items, ok := list(parsed, "threads")
	if !ok || len(items) != len(got.Threads) {
		return false
	}
	for i, item := range items {
		th := got.Threads[i]
		notes, ok := list(item, "notes")
		if (item != nil && !value.IsMapping(item)) || !ok || len(notes) != len(th.Notes) {
			return false
		}
		if !text(value.Prop(item, "id"), th.ID) || !text(value.Prop(item, "with"), th.With) ||
			!text(value.Prop(item, "title"), th.Title) || !text(value.Prop(item, "status"), th.Status) ||
			!text(value.Prop(item, "closed"), th.Closed) || !texts(value.Prop(item, "docs"), th.Docs) {
			return false
		}
		for j, note := range notes {
			if (note != nil && !value.IsMapping(note)) ||
				!text(value.Prop(note, "at"), th.Notes[j].At) || !text(value.Prop(note, "text"), th.Notes[j].Text) {
				return false
			}
		}
	}
	return true
}

// list is the items a value's key holds: none for a value that has no such
// key or holds null; false for a value that is no mapping or null, or whose
// key holds anything but a list or null.
func list(v any, key string) ([]any, bool) {
	if v != nil && !value.IsMapping(v) {
		return nil, false
	}
	switch items := value.Prop(v, key).(type) {
	case nil:
		return nil, true
	case []any:
		return items, true
	}
	return nil, value.Prop(v, key) == value.Undefined
}

// text is whether a string field read as got is what Parse's value v gives.
func text(v any, got string) bool {
	switch t := v.(type) {
	case string:
		return got == t
	case nil:
		return got == ""
	case float64, bool:
		return true
	}
	return v == value.Undefined && got == ""
}

// texts is whether a list of strings read as got is what Parse's value v
// gives.
func texts(v any, got []string) bool {
	switch t := v.(type) {
	case nil:
		return len(got) == 0
	case []any:
		if len(t) != len(got) {
			return false
		}
		for i, e := range t {
			if !text(e, got[i]) {
				return false
			}
		}
		return true
	}
	return v == value.Undefined && len(got) == 0
}
