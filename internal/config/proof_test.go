package config

import (
	"slices"
	"strings"
	"testing"
)

// proof.code is held to its paths and its {base} before any work done reads
// it (slice 100); a config without proof has none.
func TestProofCode(t *testing.T) {
	c, err := load(t, "version: 1\n")
	if err != nil || c.Proof != nil {
		t.Fatalf("got %+v, %v; want no proof", c.Proof, err)
	}
	c, err = load(t, "version: 1\nproof:\n  code:\n    paths: [\"src/**\"]\n    check: \"cc --since {base}\"\n")
	if err != nil || c.Proof == nil || c.Proof.Code == nil {
		t.Fatalf("got %+v, %v", c, err)
	}
	if code := c.Proof.Code; !slices.Equal(code.Paths, []string{"src/**"}) || code.Check != "cc --since {base}" {
		t.Fatalf("got %+v", code)
	}
	cases := map[string]string{
		"paths: [\"src/**\"]\n    check: \"cc --json\"":        "proof.code.check does not take {base}",
		"paths: []\n    check: \"cc --since {base}\"":          "proof.code.paths names no path",
		"paths: [\"?src\"]\n    check: \"cc --since {base}\"":  "proof.code.paths[0] is no glob",
		"check: \"cc --since {base}\"":                         "proof.code.paths is missing",
		"paths: [\"src/**\"]":                                  "proof.code.check is missing",
		"paths: [\"src/**\"]\n    check: \"{base}\"\n    x: 1": "unknown key proof.code.x",
	}
	for code, want := range cases {
		_, err := load(t, "version: 1\nproof:\n  code:\n    "+code+"\n")
		found := messages(t, err)
		if !slices.ContainsFunc(found, func(m string) bool { return strings.Contains(m, want) }) {
			t.Errorf("%q: got %q, want one saying %q", code, found, want)
		}
	}
}
