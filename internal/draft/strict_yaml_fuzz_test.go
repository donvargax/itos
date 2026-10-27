package draft

import (
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// FuzzStrictYAMLDraft holds parse, the reader of the drafts' list, to
// value.Parse, itos's strict reading (T-329): a list Parse refuses (a key
// written twice at any depth, a second document, an alias that never ends)
// is refused, and one it accepts is read as Parse reads it, the same drafts
// with the same strings.
//
// A null key and a null list item, at any depth, are refused too, since no
// file itos writes has one and yaml/v3 drops them (bug 152).
//
// The reader is stricter by schema: an unknown key, an id no draft may have,
// a draft that is neither a change nor a command are refused, and so is a
// value of another shape than its field's. It accepts more in one way, by
// yaml/v3's typed decoding: a number or a boolean written where a string
// goes is read as the text written (message: 1.50 is "1.50", where Parse
// reads the number 1.5), so a field Parse reads as such a scalar matches any
// text. An input with a merge key (<<) is not compared: yaml/v3 merges its
// mapping into the one holding it, where Parse reads a key named <<.
func FuzzStrictYAMLDraft(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		parsed, parseErr := value.Parse(text)
		read, readErr := parse("drafts.yaml", []byte(text))
		if parseErr != nil {
			if readErr == nil {
				t.Fatalf("value.Parse refuses %q (%v), parse reads %+v", text, parseErr, read)
			}
			return
		}
		if readErr != nil {
			return
		}
		if hasNull(parsed) {
			t.Fatalf("value.Parse reads %q with a null key or list item, parse reads %+v", text, read)
		}
		if hasMerge(parsed) {
			return
		}
		if !listMatches(parsed, read) {
			t.Fatalf("value.Parse reads %q as %s, parse reads %+v", text, value.JSON(parsed), read)
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

// listMatches is whether the drafts read are those Parse's value holds.
func listMatches(parsed any, read File) bool {
	var items []any
	switch top := parsed.(type) {
	case nil:
	case *value.Map:
		switch list := top.At("drafts").(type) {
		case nil:
		case []any:
			items = list
		default:
			if list != value.Undefined {
				return false
			}
		}
	default:
		return false
	}
	if len(items) != len(read.Drafts) {
		return false
	}
	for i, item := range items {
		if item != nil && !value.IsMapping(item) {
			return false
		}
		d := read.Drafts[i]
		if !text(value.Prop(item, "id"), d.ID) || !text(value.Prop(item, "message"), d.Message) ||
			!text(value.Prop(item, "base"), d.Base) || !text(value.Prop(item, "minted"), d.Minted) ||
			!texts(value.Prop(item, "paths"), d.Paths) || !texts(value.Prop(item, "command"), d.Command) {
			return false
		}
	}
	return true
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
