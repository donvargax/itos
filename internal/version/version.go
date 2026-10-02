// Package version is itos's version, and whether it satisfies a config's
// requires: a list of comparators, each of >=, >, <=, <, = (or none) and a
// version, all of which must hold. It is tools/itos/version.ts, ported.
//
// The version is package.json's, written nowhere else, so a release is still
// one build commit to it and a tag: tools/bin/build-go.ts stamps it into the
// binary with -ldflags "-X github.com/donvargax/itos/internal/version.stamp=<v>".
// A binary built without the stamp says the module version Go records, which
// `go install github.com/donvargax/itos/cmd/itos@v<x>` sets.
package version

import (
	"regexp"
	"runtime/debug"
	"strings"
	"unicode"
)

// stamp is package.json's version, set at link time by tools/bin/build-go.ts.
var stamp string

// Unstamped is what a binary says it is when it was built with neither the
// stamp nor a module version (go build or go run in a checkout without VCS
// information): Go's own word for it.
const Unstamped = "(devel)"

// Version is the version this binary says it is: the stamp, else the module
// version Go recorded, without its leading v.
func Version() string {
	if stamp != "" {
		return stamp
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return fromModule(info.Main.Version)
	}
	return Unstamped
}

// fromModule reads a module version as Go records it ("v0.6.0", or "(devel)"
// when it knows none).
func fromModule(v string) string {
	if v == "" || v == Unstamped {
		return Unstamped
	}
	return strings.TrimPrefix(v, "v")
}

// leadingInt reads a number as JavaScript's parseInt does, its leading digits
// after an optional sign, and 0 where it would give NaN.
func leadingInt(s string) int {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	sign := 1
	if s != "" && (s[0] == '-' || s[0] == '+') {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return sign * n
}

func parts(v string) [3]int {
	var p [3]int
	for i, s := range strings.SplitN(v, ".", 4) {
		if i < 3 {
			p[i] = leadingInt(s)
		}
	}
	return p
}

// compare is negative, zero or positive, as a is older than, the same as or
// newer than b, on their first three parts, a missing part being 0.
func compare(a, b string) int {
	x, y := parts(a), parts(b)
	for i := range 3 {
		if x[i] != y[i] {
			return x[i] - y[i]
		}
	}
	return 0
}

var holds = map[string]func(int) bool{
	">=": func(c int) bool { return c >= 0 },
	">":  func(c int) bool { return c > 0 },
	"<=": func(c int) bool { return c <= 0 },
	"<":  func(c int) bool { return c < 0 },
	"=":  func(c int) bool { return c == 0 },
	"":   func(c int) bool { return c == 0 },
}

var comparator = regexp.MustCompile(`^(>=|>|<=|<|=)?v?(\d+(?:\.\d+){0,2})$`)

// Satisfies says whether version holds every comparator of the range. A
// comparator it cannot read never holds; an empty range always does.
func Satisfies(version, rng string) bool {
	for _, c := range strings.Fields(rng) {
		m := comparator.FindStringSubmatch(c)
		if m == nil || !holds[m[1]](compare(version, m[2])) {
			return false
		}
	}
	return true
}
