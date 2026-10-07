package value

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

// Plain scalars resolve by the YAML 1.2 core schema, as the yaml package
// reads them, not by YAML 1.1's forms yaml/v3 also takes.
func TestResolve(t *testing.T) {
	for text, want := range map[string]any{
		"017": 17.0, "0o17": 15.0, "0x1f": 31.0, "-2.5e3": -2500.0, ".5": 0.5,
		"1_000": "1_000", "0b101": "0b101", "yes": "yes", "on": "on",
		"~": nil, "Null": nil, "TRUE": true, "False": false, "2001-12-14": "2001-12-14",
	} {
		if got := Resolve(text); got != want {
			t.Errorf("%s: got %#v, want %#v", text, got, want)
		}
	}
	if f, _ := Resolve(".nan").(float64); !math.IsNaN(f) {
		t.Error(".nan")
	}
}

func TestParse(t *testing.T) {
	v, err := Parse("b: 1\n2: x\na: [\"017\", 017, !!str 5]\n1: y\n")
	if err != nil {
		t.Fatal(err)
	}
	m := v.(*Map)
	if got := m.Keys(); !slices.Equal(got, []string{"1", "2", "b", "a"}) {
		t.Errorf("keys in JavaScript's order: %q", got)
	}
	if got := JSON(m.At("a")); got != `["017",17,"5"]` {
		t.Errorf("a: %s", got)
	}
	if m.At("c") != Undefined {
		t.Error("a missing key is undefined")
	}
	for _, text := range []string{"a: 1\na: 2\n", "a: 1\n---\nb: 2\n", "a: b: c\n"} {
		if _, err := Parse(text); err == nil {
			t.Errorf("%q parsed", text)
		}
	}
	if v, err := Parse("# nothing\n"); v != nil || err != nil {
		t.Errorf("empty: %v, %v", v, err)
	}
}

// An alias inside its own anchor is refused, naming it, where it once
// recursed until the stack overflowed (bug 39); an alias reused outside its
// anchor is still read, each place its own copy.
func TestParseAliasCycle(t *testing.T) {
	for _, text := range []string{"loop: &x [*x]\n", "a: &x\n  b: &y [1, *x]\n", "a: &x {b: [*x]}\n"} {
		_, err := Parse(text)
		if err == nil || !strings.Contains(err.Error(), "alias *x") || !strings.Contains(err.Error(), "inside its own anchor") {
			t.Errorf("%q: %v", text, err)
		}
	}
	v, err := Parse("a: &x [1, 2]\nb: *x\nc: [*x, *x]\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := JSON(v); got != `{"a":[1,2],"b":[1,2],"c":[[1,2],[1,2]]}` {
		t.Errorf("aliases reused: %s", got)
	}
}

// Nested aliases that would expand exponentially (a billion laughs: nine
// anchors, each aliasing the one before ten times) are refused once they
// pass maxAliased values, naming the alias that expanded, and quickly
// (bug 39).
func TestParseAliasExpansion(t *testing.T) {
	var b strings.Builder
	b.WriteString("l0: &l0 [lol]\n")
	for i := 1; i <= 9; i++ {
		fmt.Fprintf(&b, "l%d: &l%d [", i, i)
		for j := 0; j < 10; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "*l%d", i-1)
		}
		b.WriteString("]\n")
	}
	start := time.Now()
	_, err := Parse(b.String())
	if err == nil || !strings.Contains(err.Error(), "the alias *l") || !strings.Contains(err.Error(), "expands past 100000 values") {
		t.Errorf("a billion laughs: %v", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("a billion laughs took %s to refuse", took)
	}
	// Four levels, some twenty thousand values in all, are read.
	if _, err := Parse(strings.Join(strings.SplitAfter(b.String(), "\n")[:5], "")); err != nil {
		t.Errorf("four levels: %v", err)
	}
}

func TestRenderings(t *testing.T) {
	for f, want := range map[float64]string{1: "1", 1.5: "1.5", 1e21: "1e+21", 1e-7: "1e-7", 1e-6: "0.000001", -0.25: "-0.25", math.Inf(-1): "-Infinity"} {
		if got := Number(f); got != want {
			t.Errorf("Number(%v) = %q, want %q", f, got, want)
		}
	}
	if got := String([]any{"a", nil, 2.0}); got != "a,,2" {
		t.Errorf("String of a list: %q", got)
	}
	if got := JSON(NewMap("run", "a", "x", Undefined, "q", "\"<&>\n")); got != `{"run":"a","q":"\"<&>\n"}` {
		t.Errorf("JSON: %s", got)
	}
}

func TestYAML(t *testing.T) {
	got := YAML(NewMap(
		"list", []any{"sh", "-c"},
		"m", NewMap("a", "[^/]+", "b", "Commit rejected:", "c", "pass --as <handle>", "d", "true", "e", `"hi" there`, "h", "it's: x", "i", "- a", "f", 600.0, "g", false),
		"<kind>", NewMap(),
	))
	want := "list:\n  - sh\n  - -c\nm:\n  a: \"[^/]+\"\n  b: \"Commit rejected:\"\n  c: pass --as <handle>\n  d: \"true\"\n  e: '\"hi\" there'\n  h: \"it's: x\"\n  i: \"- a\"\n  f: 600\n  g: false\n<kind>: {}\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSpace(t *testing.T) {
	if got := Trim("\xef\xbb\xbf a \u00a0"); got != "a" {
		t.Errorf("Trim: %q", got)
	}
	if got := Collapse("a \t\u2003 b"); got != "a b" {
		t.Errorf("Collapse: %q", got)
	}
}

// Number() as JavaScript reads a string: what a group flag's value is
// compared by when the ledger's groups are numeric.
func TestToNumber(t *testing.T) {
	for text, want := range map[string]float64{
		"1": 1, " 2 ": 2, "": 0, "01": 1, "0x10": 16, "0b11": 3, "0o7": 7, "1e2": 100,
		".5": 0.5, "5.": 5, "-3": -3, "+4": 4,
	} {
		if got := ToNumber(text); got != want {
			t.Errorf("%q: got %v, want %v", text, got, want)
		}
	}
	for _, text := range []string{"abc", "1_000", "inf", "NaN", "0x", "-0x10", "1e", "0x1p-2"} {
		if got := ToNumber(text); !math.IsNaN(got) {
			t.Errorf("%q: got %v, want NaN", text, got)
		}
	}
	if !math.IsInf(ToNumber("-Infinity"), -1) {
		t.Error("-Infinity")
	}
}

func TestPadEnd(t *testing.T) {
	if got := PadEnd("ab", 4); got != "ab  " {
		t.Errorf("got %q", got)
	}
	if got := PadEnd("𝄞", 3); got != "𝄞 " {
		t.Errorf("got %q", got)
	}
	if got := PadEnd("failing", 3); got != "failing" {
		t.Errorf("got %q", got)
	}
}
