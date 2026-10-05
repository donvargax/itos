package release

// What the next release would carry (slice 70, itos status): the newest
// release is the highest tag vX.Y.Z, three numbers and nothing else, as the
// release cut counts from it (bug 20), and a releasable commit is one the release
// cut counts (T-069): a feat, a fix, or a commit of any type marked as
// breaking. tools/bin/release-version imports Type, Breaking and Newest
// from here (T-088, bug 20), so status and the release cut read a commit and
// pick the last release from one copy.

import (
	"regexp"
	"strings"
)

var (
	// typed is a Conventional Commits header's type, scope and !.
	typed = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?(!)?: `)
	// releaseTag is a release's tag, vX.Y.Z, and its three numbers.
	releaseTag = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)
)

// Releasable says whether a commit's message makes a release: a feat or a
// fix, or one of any type marked as breaking.
func Releasable(message string) bool {
	if t := Type(message); t == "feat" || t == "fix" {
		return true
	}
	return Breaking(message)
}

// Type is the Conventional Commits type of a commit's header, "" when the
// header has none.
func Type(message string) string {
	header, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
	if t := typed.FindStringSubmatch(header); t != nil {
		return t[1]
	}
	return ""
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

// IsTag says whether the tag names a release: v followed by three
// dot-separated numbers and nothing else. itos never cuts a prerelease, so a
// tag with a prerelease or build metadata names none (bug 20); a number with
// leading zeros counts, as the release cut has always taken it.
func IsTag(tag string) bool { return releaseTag.MatchString(tag) }

// Newest is the highest of the tags that name a release, by their numbers,
// "" when none does. The release cut (tools/bin/release-version) and itos
// status both pick the last release with it. Two spellings of one version
// (v1.0.0, v01.0.0) go to the shorter, then the later by name, so the pick
// does not hang on the order the tags are listed in.
func Newest(tags []string) string {
	newest := ""
	for _, t := range tags {
		if IsTag(t) && (newest == "" || compareTags(t, newest) > 0) {
			newest = t
		}
	}
	return newest
}

// compareTags orders two release tags by their numbers, each compared as a
// number however long, then a tie as Newest breaks it.
func compareTags(a, b string) int {
	x, y := releaseTag.FindStringSubmatch(a), releaseTag.FindStringSubmatch(b)
	for i := 1; i <= 3; i++ {
		if c := compareNumbers(x[i], y[i]); c != 0 {
			return c
		}
	}
	if len(a) != len(b) {
		return len(b) - len(a)
	}
	return strings.Compare(a, b)
}

// compareNumbers orders two runs of digits by the numbers they write.
func compareNumbers(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}
