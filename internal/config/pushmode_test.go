package config

import (
	"slices"
	"testing"
)

// push.mode is trunk when the file leaves it out and when it says trunk, pr
// when it says pr; any other value is refused naming the key and both modes
// (slice 174).
func TestPushMode(t *testing.T) {
	for text, want := range map[string]string{
		"version: 1\n":                        "trunk",
		"version: 1\npush: { mode: trunk }\n": "trunk",
		"version: 1\npush: { mode: pr }\n":    "pr",
	} {
		c, err := load(t, text)
		if err != nil || c.Push.Mode != want {
			t.Errorf("%q: push.mode is %q, %v, want %q", text, c.Push.Mode, err, want)
		}
	}
	_, err := load(t, "version: 1\npush: { mode: squash }\n")
	if got, want := messages(t, err), []string{`push.mode should be one of trunk, pr, not "squash"`}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if ok, _ := KnownKey("push.mode"); !ok {
		t.Error("push.mode is no key of the schema")
	}
}
