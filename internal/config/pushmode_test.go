package config

import (
	"slices"
	"strings"
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

// push.pr (slice 179) is off by default: draft false and review_by empty; a
// file's values are read, a draft that is not a bool and a review_by that is
// not a list of strings are refused, and both keys are in the schema.
func TestPushPR(t *testing.T) {
	c, err := load(t, "version: 1\n")
	if err != nil || c.Push.PR.Draft || len(c.Push.PR.ReviewBy) != 0 {
		t.Errorf("by default: %+v, %v", c.Push.PR, err)
	}
	c, err = load(t, "version: 1\npush: { mode: pr, pr: { draft: true, review_by: [copilot-pull-request-reviewer, bo] } }\n")
	if err != nil || !c.Push.PR.Draft || !slices.Equal(c.Push.PR.ReviewBy, []string{"copilot-pull-request-reviewer", "bo"}) {
		t.Errorf("set: %+v, %v", c.Push.PR, err)
	}
	for text, want := range map[string]string{
		"version: 1\npush: { pr: { draft: yes please } }\n":  "push.pr.draft should be a boolean",
		"version: 1\npush: { pr: { review_by: copilot } }\n": "push.pr.review_by should be a list",
		"version: 1\npush: { pr: { review_by: [1] } }\n":     "push.pr.review_by should be a list of strings",
		"version: 1\npush: { pr: { squash: true } }\n":       "unknown key push.pr.squash",
	} {
		_, err := load(t, text)
		if got := messages(t, err); len(got) != 1 || !strings.HasPrefix(got[0], want) {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
	for _, key := range []string{"push.pr", "push.pr.draft", "push.pr.review_by"} {
		if ok, _ := KnownKey(key); !ok {
			t.Errorf("%s is no key of the schema", key)
		}
	}
}
