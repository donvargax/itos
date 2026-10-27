package idcounter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// FuzzStrictYAMLCounter holds readCounters to value.Parse, itos's strict
// reading (T-329): a counter file Parse refuses (a key written twice at any
// depth, a second document, an alias that never ends) is refused, and one it
// accepts is read as Parse reads it, the same categories with the same
// numbers. The reader may be stricter: a top that is no mapping, a counters
// that is no mapping or a number that is no integer it refuses.
func FuzzStrictYAMLCounter(f *testing.F) {
	file := filepath.Join(f.TempDir(), "ids.yaml")
	f.Fuzz(func(t *testing.T, text string) {
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		parsed, parseErr := value.Parse(text)
		read, readErr := readCounters(file)
		if parseErr != nil {
			if readErr == nil {
				t.Fatalf("value.Parse refuses %q (%v), readCounters reads %v", text, parseErr, read)
			}
			return
		}
		if readErr != nil {
			return
		}
		want, ok := counters(parsed)
		if !ok {
			t.Fatalf("value.Parse reads %q as %s, readCounters reads %v", text, value.JSON(parsed), read)
		}
		if len(read) != len(want) {
			t.Fatalf("value.Parse reads %q as %v, readCounters reads %v", text, want, read)
		}
		for k, n := range want {
			if got, has := read[k]; !has || got != n {
				t.Fatalf("value.Parse reads %q as %v, readCounters reads %v", text, want, read)
			}
		}
	})
}

// counters is the categories Parse's value holds under counters, with their
// numbers; false when the value is not a counter file.
func counters(parsed any) (map[string]int, bool) {
	want := map[string]int{}
	if parsed == nil {
		return want, true
	}
	top, ok := parsed.(*value.Map)
	if !ok {
		return nil, false
	}
	section := top.At("counters")
	if section == nil || section == value.Undefined {
		return want, true
	}
	m, ok := section.(*value.Map)
	if !ok {
		return nil, false
	}
	for _, k := range m.Keys() {
		n, ok := m.At(k).(float64)
		if !ok || n != float64(int(n)) {
			return nil, false
		}
		want[k] = int(n)
	}
	return want, true
}
