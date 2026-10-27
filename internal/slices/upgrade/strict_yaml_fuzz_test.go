package upgrade

import (
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// FuzzStrictYAMLWorkflow holds stepUses, the reader of a workflow, to
// value.Parse, itos's strict reading (T-329, K150b: it once read a key
// written twice and every document after the first silently): a workflow
// Parse refuses (a key written twice at any depth, a second document, an
// alias that never ends) is refused, and one it accepts gives the uses of
// the itos action Parse's value holds, each with its ref and with.version,
// in any order (the reader's is by line, which Parse does not give).
//
// The reader accepts more in one way, by reading the YAML nodes: a number, a
// boolean or null written as with.version is told as written (9.10, where
// Parse reads the number 9.1), so a with.version Parse reads as such a
// scalar matches any text, and one that is a list or a mapping is no
// version.
func FuzzStrictYAMLWorkflow(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		parsed, parseErr := value.Parse(text)
		read, readErr := stepUses("w.yml", []byte(text))
		if parseErr != nil {
			if readErr == nil {
				t.Fatalf("value.Parse refuses %q (%v), stepUses reads %+v", text, parseErr, read)
			}
			return
		}
		if readErr != nil {
			t.Fatalf("value.Parse reads %q, stepUses refuses it: %v", text, readErr)
		}
		if !usesMatch(parsed, read) {
			t.Fatalf("value.Parse reads %q as %s, stepUses reads %+v", text, value.JSON(parsed), read)
		}
	})
}

// wanted is a use Parse's value holds: its ref; whether its with.version is
// a scalar (versioned) and, when Parse reads it as a string (exact), which.
type wanted struct {
	ref       string
	versioned bool
	exact     bool
	version   string
}

// usesMatch is whether the uses read are those Parse's value holds.
func usesMatch(parsed any, read []actionUse) bool {
	var want []wanted
	jobs, _ := value.Prop(parsed, "jobs").(*value.Map)
	for _, name := range jobs.Keys() {
		steps, _ := value.Prop(jobs.At(name), "steps").([]any)
		for _, step := range steps {
			uses, _ := value.Prop(step, "uses").(string)
			ref, ok := strings.CutPrefix(uses, actionPrefix)
			if !ok || ref == "" {
				continue
			}
			w := wanted{ref: ref}
			switch v := value.Prop(value.Prop(step, "with"), "version").(type) {
			case string:
				w.versioned, w.exact, w.version = true, true, v
			case nil, float64, bool:
				w.versioned = true
			}
			want = append(want, w)
		}
	}
	if len(want) != len(read) {
		return false
	}
	left := slices.Clone(read)
	take := func(w wanted, exact bool) bool {
		at := slices.IndexFunc(left, func(u actionUse) bool {
			return u.Ref == w.ref && (u.VersionInput != nil) == w.versioned && (!exact || *u.VersionInput == w.version)
		})
		if at < 0 {
			return false
		}
		left = slices.Delete(left, at, at+1)
		return true
	}
	for _, exact := range []bool{true, false} {
		for _, w := range want {
			if w.exact == exact && !take(w, exact) {
				return false
			}
		}
	}
	return true
}
