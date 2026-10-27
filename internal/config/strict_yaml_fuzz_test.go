package config

import (
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// FuzzStrictYAMLConfig holds fromText, the reader of the config, to
// value.Parse, itos's strict reading (T-329): a config Parse refuses (a key
// written twice at any depth, a second document, an alias that never ends)
// is refused, and one it accepts is read as Parse reads it, the file's tree
// holding the keys and values Parse reads, its path lists' $names expanded.
// The config reads YAML through Parse alone and types the tree from it, so
// it accepts nothing more than Parse does; it is stricter by schema (an
// unknown key, a value of another shape, a version that is not 1, a rule the
// cross-checks hold) and refuses those.
func FuzzStrictYAMLConfig(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		parsed, parseErr := value.Parse(text)
		loaded, readErr := fromText("itos.yaml", text, func(string) bool { return false })
		if parseErr != nil {
			if readErr == nil {
				t.Fatalf("value.Parse refuses %q (%v), fromText reads it", text, parseErr)
			}
			return
		}
		if readErr != nil {
			return
		}
		want, ok := parsed.(*value.Map)
		if !ok {
			t.Fatalf("value.Parse reads %q as %s, fromText reads it", text, value.JSON(parsed))
		}
		expandSets(want)
		if value.JSON(loaded.file) != value.JSON(want) {
			t.Fatalf("value.Parse reads %q as %s, fromText reads %s", text, value.JSON(want), value.JSON(loaded.file))
		}
	})
}
