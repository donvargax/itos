package version

import (
	"regexp"
	"runtime/debug"
	"strings"
	"unicode"
)

// stamp is the version, set at link time (-ldflags -X).
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

// split is a version's three numbers and its pre-release, the part after the
// first - that follows them ("" with none), build metadata after a + left
// out, as semver leaves it out of precedence.
func split(v string) ([3]int, string) {
	v, _, _ = strings.Cut(v, "+")
	if i := strings.IndexByte(v, '-'); i > 0 {
		return parts(v[:i]), v[i+1:]
	}
	return parts(v), ""
}

// Compare is negative, zero or positive, as a is older than, the same as or
// newer than b, by semver's precedence: their first three parts, a missing
// part being 0, then a pre-release below its release (9.2.0-rc.1 below
// 9.2.0, bug 50), and pre-releases of one release by their dot-separated
// identifiers, in turn: numbers as numbers (rc.2 below rc.10), a number below
// a word, words by their bytes, and fewer identifiers below more when the
// shared ones are the same.
func Compare(a, b string) int {
	x, xp := split(a)
	y, yp := split(b)
	if c := compareParts(x, y); c != 0 {
		return c
	}
	switch {
	case xp == yp:
		return 0
	case xp == "":
		return 1
	case yp == "":
		return -1
	}
	xs, ys := strings.Split(xp, "."), strings.Split(yp, ".")
	for i := 0; i < len(xs) && i < len(ys); i++ {
		if c := compareIdentifiers(xs[i], ys[i]); c != 0 {
			return c
		}
	}
	return len(xs) - len(ys)
}

// compareParts orders two versions' three numbers.
func compareParts(x, y [3]int) int {
	for i := range 3 {
		if x[i] != y[i] {
			return x[i] - y[i]
		}
	}
	return 0
}

// compareIdentifiers orders two identifiers of a pre-release: two numbers by
// their values however long, a number below a word, two words by their bytes.
func compareIdentifiers(a, b string) int {
	an, bn := numeric(a), numeric(b)
	switch {
	case an && bn:
		a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return strings.Compare(a, b)
	case an:
		return -1
	case bn:
		return 1
	}
	return strings.Compare(a, b)
}

// numeric is whether an identifier is digits alone.
func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
// comparator it cannot read never holds; an empty range always does. It
// reads the version's three numbers alone, its pre-release left out, as a
// range names X.Y.Z alone: a build of a checkout, X.Y.Z-dev.N.g<sha>, and a
// release candidate X.Y.Z-rc.N satisfy what X.Y.Z does, as they always have
// (bug 50 ordered pre-releases for Compare, not for a range).
func Satisfies(version, rng string) bool {
	core, _ := split(version)
	for _, c := range strings.Fields(rng) {
		m := comparator.FindStringSubmatch(c)
		if m == nil || !holds[m[1]](compareParts(core, parts(m[2]))) {
			return false
		}
	}
	return true
}
