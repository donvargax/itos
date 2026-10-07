// Command release-version computes the version the next release carries from
// the commits since the last one (T-069): the tag is the version, and nothing
// in the tree holds one.
//
// The last release is the newest vX.Y.Z tag reachable from HEAD, by its
// numbers, picked by internal/release (Newest, bug 20), the rule itos status
// reads the newest release with; the commits are <tag>..HEAD, so nothing before the tag counts. Of those:
//
//   - any commit marked as breaking (a ! before its header's colon, of any
//     type, or a BREAKING-CHANGE: or BREAKING CHANGE: footer in its message's
//     last paragraph) makes a major;
//   - else a feat makes a minor;
//   - else a fix makes a patch;
//   - else nothing is released: no feat, no fix and no breaking change is
//     nothing a consumer can feel.
//
// With no such tag the last version is 0.0.0, every commit counting. A shallow
// clone, which may hide the tag or the commits, stops it with exit 2, never a
// guess.
//
// A version is refused (exit 1, nothing on stdout) when its major does not
// match the module path of HEAD's go.mod (T-089): from v2 Go's module proxy
// takes a vN.x.y tag only from a module whose path ends in /vN, and a v0 or v1
// tag only from one with no such suffix, so a release cut from a mismatched
// go.mod could never be go-installed (v3.0.0 to v3.3.0 were cut from a path
// ending in /v2). The path moves first, in its own commits, and the release
// follows. A HEAD without a go.mod has no path to contradict.
//
// # Pre-releases
//
// While HEAD holds tools/bin/release-version/prerelease saying rc (T-118,
// T-119, decision 42), the commits that would cut the next major cut a release
// candidate of it instead, <major>.0.0-rc.<n>: n is one more than the highest
// rc tag of that version HEAD reaches (v7.0.0-rc.2 makes the next one rc.3), 1
// with none. After an rc, the commits since it decide whether another is cut:
// a feat, a fix or a breaking change since v7.0.0-rc.2 cuts rc.3, and with
// none of them nothing is released, though the commits since the last release
// still hold the breaking change. The last release stays the newest vX.Y.Z,
// which an rc tag never is, so an rc's notes and upgrading.json run from the
// last stable release, as the final one's will; and removing the file cuts
// that final <major>.0.0 from the same commits. A minor or a patch is cut as
// it always is, the mode or not; with no such file the mode is off, and a
// file saying anything but rc (blank lines and surrounding space aside) stops
// it with exit 2. The marker is a file of this tool's own, not a key of
// itos.yaml, whose schema holds only what itos reads (T-119); itos.yaml is not
// read. The module path rule above holds for an rc as for any version of its
// major.
//
// It prints key=value lines, which the release workflow appends to
// $GITHUB_OUTPUT as they are:
//
//	last=v2.3.0      the last release's tag, empty with none
//	next=2.4.0       the version to release, empty when nothing is releasable
//	bump=minor       major, minor, patch or none
//	range=v2.3.0..HEAD
//	prerelease=false true when next is a release candidate, which the
//	                 workflow publishes as a pre-release, never latest
//
// and on stderr one line saying why. A commit is read by internal/release
// (Type and Breaking, T-088), the copy itos status reads it with, and the last
// release is picked there too (Newest, bug 20); beside it, release-version
// imports only the standard library. Its unit tests run it
// over scratch histories.
//
//	go run ./tools/bin/release-version
//
// Exit status: 0 computed (next may be empty), 1 the version is refused, 2 it
// could not read the history, go.mod or the pre-release marker.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v7/internal/release"
)

const self = "release-version"

// marker is the file, from the repository's root, whose rc switches release
// candidates on.
const marker = "tools/bin/release-version/prerelease"

var suffix = regexp.MustCompile(`/v(\d+)$`)

func main() {
	os.Exit(run(os.Args[1:], "", os.Stdout, os.Stderr))
}

// run computes the next release of the repository in dir ("" for the one it
// runs in), writing the key=value lines to stdout and why to stderr.
func run(args []string, dir string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(self, flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected argument %q\n", self, flags.Arg(0))
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, self+": "+format+"\n", a...)
		return 2
	}
	git := func(args ...string) (string, error) { return gitIn(dir, args...) }
	if shallow, err := git("rev-parse", "--is-shallow-repository"); err != nil {
		return fail("%v", err)
	} else if strings.TrimSpace(shallow) == "true" {
		return fail("this is a shallow clone, which may not have the last release's tag: fetch the whole history (actions/checkout's fetch-depth: 0)")
	}
	listed, err := git("tag", "--merged", "HEAD")
	if err != nil {
		return fail("%v", err)
	}
	tags := strings.Split(listed, "\n")
	tag := release.Newest(tags)
	rng := "HEAD"
	if tag != "" {
		rng = tag + "..HEAD"
	}
	b, err := bumpSince(git, rng)
	if err != nil {
		return fail("%v", err)
	}
	from := tag
	if from == "" {
		from = "no release (0.0.0)"
	}
	next, pre := "", false
	if b.kind != "none" {
		next = bumped(tag, b.kind)
		if b.kind == "major" {
			mode, err := prereleaseMode(git)
			if err != nil {
				return fail("%v", err)
			}
			if mode != "" {
				rc, n := lastCandidate(tags, next)
				if rc != "" {
					since, err := bumpSince(git, rc+"..HEAD")
					if err != nil {
						return fail("%v", err)
					}
					if since.kind == "none" {
						fmt.Fprintf(stdout, "last=%s\nnext=\nbump=none\nrange=%s\nprerelease=false\n", tag, rng)
						fmt.Fprintf(stderr, "%s: %d commit(s) since %s, %s, but %s says %s and none of the %d since %s is a feat, a fix or a breaking change: nothing to release\n",
							self, b.commits, from, b.why, marker, mode, since.commits, rc)
						return 0
					}
					b.why += fmt.Sprintf(", and %s says %s: %s since %s, the next candidate", marker, mode, since.why, rc)
				} else {
					b.why += fmt.Sprintf(", and %s says %s: the first candidate", marker, mode)
				}
				next, pre = fmt.Sprintf("%s-%s.%d", next, mode, n+1), true
			}
		}
		module, err := headModule(git)
		if err != nil {
			return fail("%v", err)
		}
		if problem := mismatch(next, module); problem != "" {
			fmt.Fprintf(stderr, "%s: %s\n", self, problem)
			return 1
		}
	}
	fmt.Fprintf(stdout, "last=%s\nnext=%s\nbump=%s\nrange=%s\nprerelease=%t\n", tag, next, b.kind, rng, pre)
	if next == "" {
		fmt.Fprintf(stderr, "%s: %d commit(s) since %s, none a feat, a fix or a breaking change: nothing to release\n", self, b.commits, from)
	} else {
		fmt.Fprintf(stderr, "%s: %d commit(s) since %s, %s: %s\n", self, b.commits, from, b.why, next)
	}
	return 0
}

// gitIn runs git in dir ("" for the folder it runs in) and gives its stdout.
func gitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

// bumpSince is what the commits of the range ask for.
func bumpSince(git func(...string) (string, error), rng string) (bump, error) {
	out, err := git("log", "--format=%H%x1f%B%x1e", rng)
	if err != nil {
		return bump{}, fmt.Errorf("cannot read the commits of %s: %v", rng, err)
	}
	return bumpOf(messages(out)), nil
}

// messages splits git log's records into each commit's message, trimmed.
func messages(log string) []string {
	var out []string
	for _, record := range strings.Split(log, "\x1e") {
		_, message, ok := strings.Cut(strings.TrimLeft(record, "\n"), "\x1f")
		if ok {
			out = append(out, strings.TrimSpace(message))
		}
	}
	return out
}

// bump is what the commits ask for: the kind, how many commits there were,
// and why, for the line on stderr.
type bump struct {
	kind, why string
	commits   int
}

func bumpOf(messages []string) bump {
	var breaking, feats, fixes int
	for _, m := range messages {
		switch {
		case release.Breaking(m):
			breaking++
		case release.Type(m) == "feat":
			feats++
		case release.Type(m) == "fix":
			fixes++
		}
	}
	b := bump{commits: len(messages), kind: "none"}
	switch {
	case breaking > 0:
		b.kind, b.why = "major", fmt.Sprintf("%d breaking change(s)", breaking)
	case feats > 0:
		b.kind, b.why = "minor", fmt.Sprintf("%d feat(s), no breaking change", feats)
	case fixes > 0:
		b.kind, b.why = "patch", fmt.Sprintf("%d fix(es), no feat and no breaking change", fixes)
	}
	return b
}

// bumped is the version after tag (0.0.0 for none) bumped by kind.
func bumped(tag, kind string) string {
	parts := [3]int{}
	if release.IsTag(tag) {
		for i, n := range strings.Split(tag[1:], ".") {
			parts[i], _ = strconv.Atoi(n)
		}
	}
	switch kind {
	case "major":
		parts = [3]int{parts[0] + 1, 0, 0}
	case "minor":
		parts = [3]int{parts[0], parts[1] + 1, 0}
	case "patch":
		parts[2]++
	}
	return fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2])
}

// prereleaseMode is the pre-release mode HEAD's marker file switches on, rc,
// the one there is; "" when HEAD has no such file.
func prereleaseMode(git func(...string) (string, error)) (string, error) {
	listed, err := git("ls-tree", "--name-only", "HEAD", "--", marker)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(listed) == "" {
		return "", nil
	}
	text, err := git("show", "HEAD:"+marker)
	if err != nil {
		return "", err
	}
	if mode := strings.TrimSpace(text); mode != "rc" {
		return "", fmt.Errorf("HEAD's %s says %q, and the one pre-release mode is rc: write rc in it, or remove it to release as usual", marker, mode)
	}
	return "rc", nil
}

// lastCandidate is the highest release candidate of version (X.Y.Z) among
// the tags, v<version>-rc.<n>, and its n; "" and 0 with none.
func lastCandidate(tags []string, version string) (string, int) {
	prefix := "v" + version + "-rc."
	best, highest := "", 0
	for _, t := range tags {
		t = strings.TrimSpace(t)
		digits, ok := strings.CutPrefix(t, prefix)
		if !ok || digits == "" || strings.Trim(digits, "0123456789") != "" {
			continue
		}
		if n, err := strconv.Atoi(digits); err == nil && (best == "" || n > highest) {
			best, highest = t, n
		}
	}
	return best, highest
}

// headModule is the module path HEAD's go.mod declares, or "" when HEAD has no
// go.mod.
func headModule(git func(...string) (string, error)) (string, error) {
	listed, err := git("ls-tree", "--name-only", "HEAD", "--", "go.mod")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(listed) == "" {
		return "", nil
	}
	mod, err := git("show", "HEAD:go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(mod, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", fmt.Errorf("HEAD's go.mod has no module line")
}

// mismatch says why next cannot be released from module, or "" when it can:
// from v2 the path must end in /v<major>, and below it in no such suffix. A
// module of "" (no go.mod) never mismatches.
func mismatch(next, module string) string {
	if module == "" {
		return ""
	}
	major, _ := strconv.Atoi(next[:strings.Index(next, ".")])
	has := 0
	if m := suffix.FindStringSubmatch(module); m != nil {
		has, _ = strconv.Atoi(m[1])
	}
	want := 0
	if major >= 2 {
		want = major
	}
	if has == want {
		return ""
	}
	base := suffix.ReplaceAllString(module, "")
	needs := base
	if want != 0 {
		needs = fmt.Sprintf("%s/v%d", base, want)
	}
	return fmt.Sprintf("refusing v%s: its major is %d, but go.mod's module path is %s, and Go's module proxy takes a v%d tag only from %s: move the module path first (go.mod's module line, every import, the -X ldflags that stamp the version, the go install lines), then release", next, major, module, major, needs)
}
