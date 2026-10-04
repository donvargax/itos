package release

// What the next release would carry (slice 70, itos status): the newest
// release is the highest tag of the form v<semver>, a prerelease below its
// release as semver orders them, and a releasable commit is one the release
// cut counts, as tools/bin/release-version reads it (T-069): a feat, a fix,
// or a commit of any type marked as breaking. release-version imports the
// standard library alone, so it keeps its own copy of these rules; the two
// read a commit alike.

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// typed is a Conventional Commits header's type, scope and !, as
	// release-version reads it.
	typed = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?(!)?: `)
	// semverTag is a tag v<semver>: its three numbers, its prerelease and
	// its build metadata.
	semverTag = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*)?$`)
)

// Releasable says whether a commit's message makes a release: a feat or a
// fix, or one of any type marked as breaking.
func Releasable(message string) bool {
	header, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
	if t := typed.FindStringSubmatch(header); t != nil && (t[1] == "feat" || t[1] == "fix") {
		return true
	}
	return Breaking(message)
}

// Breaking says whether a commit's message is marked as breaking: a !
// before its header's colon, or a BREAKING-CHANGE: or BREAKING CHANGE:
// footer in its last paragraph.
func Breaking(message string) bool {
	header, rest, _ := strings.Cut(strings.TrimSpace(message), "\n")
	if t := typed.FindStringSubmatch(header); t != nil && t[3] == "!" {
		return true
	}
	if strings.TrimSpace(rest) == "" {
		return false
	}
	paragraphs := strings.Split(strings.TrimSpace(rest), "\n\n")
	for _, line := range strings.Split(paragraphs[len(paragraphs)-1], "\n") {
		if strings.HasPrefix(line, "BREAKING-CHANGE:") || strings.HasPrefix(line, "BREAKING CHANGE:") {
			return true
		}
	}
	return false
}

// IsTag says whether the tag names a release: v<semver>.
func IsTag(tag string) bool { return semverTag.MatchString(tag) }

// Newest is the highest of the tags that name a release, as semver orders
// them, "" when none does.
func Newest(tags []string) string {
	newest := ""
	for _, t := range tags {
		if IsTag(t) && (newest == "" || compareTags(t, newest) > 0) {
			newest = t
		}
	}
	return newest
}

// compareTags orders two release tags as semver does: by their numbers,
// then a prerelease below its release, prereleases by their identifiers,
// numeric ones numerically and below alphanumeric ones; build metadata does
// not count.
func compareTags(a, b string) int {
	x, y := semverTag.FindStringSubmatch(a), semverTag.FindStringSubmatch(b)
	for i := 1; i <= 3; i++ {
		m, _ := strconv.ParseUint(x[i], 10, 64)
		n, _ := strconv.ParseUint(y[i], 10, 64)
		if m != n {
			if m < n {
				return -1
			}
			return 1
		}
	}
	switch {
	case x[4] == y[4]:
		return 0
	case x[4] == "":
		return 1
	case y[4] == "":
		return -1
	}
	p, q := strings.Split(x[4], "."), strings.Split(y[4], ".")
	for i := 0; i < len(p) && i < len(q); i++ {
		if c := compareIdentifier(p[i], q[i]); c != 0 {
			return c
		}
	}
	return len(p) - len(q)
}

func compareIdentifier(a, b string) int {
	m, aErr := strconv.ParseUint(a, 10, 64)
	n, bErr := strconv.ParseUint(b, 10, 64)
	switch {
	case aErr == nil && bErr == nil:
		if m == n {
			return 0
		}
		if m < n {
			return -1
		}
		return 1
	case aErr == nil:
		return -1
	case bErr == nil:
		return 1
	}
	return strings.Compare(a, b)
}
