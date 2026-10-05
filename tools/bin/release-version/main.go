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
// It prints key=value lines, which the release workflow appends to
// $GITHUB_OUTPUT as they are:
//
//	last=v2.3.0      the last release's tag, empty with none
//	next=2.4.0       the version to release, empty when nothing is releasable
//	bump=minor       major, minor, patch or none
//	range=v2.3.0..HEAD
//
// and on stderr one line saying why. A commit is read by internal/release
// (Type and Breaking, T-088), the copy itos status reads it with, and the last
// release is picked there too (Newest, bug 20); beside it, release-version
// imports only the standard library.
//
//	go run ./tools/bin/release-version
//
// Exit status: 0 computed (next may be empty), 1 the version is refused, 2 it
// could not read the history or go.mod.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v4/internal/release"
)

const self = "release-version"

var suffix = regexp.MustCompile(`/v(\d+)$`)

func main() {
	os.Exit(run())
}

func run() int {
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "%s: unexpected argument %q\n", self, flag.Arg(0))
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, self+": "+format+"\n", a...)
		return 2
	}
	if shallow, err := git("rev-parse", "--is-shallow-repository"); err != nil {
		return fail("%v", err)
	} else if strings.TrimSpace(shallow) == "true" {
		return fail("this is a shallow clone, which may not have the last release's tag: fetch the whole history (actions/checkout's fetch-depth: 0)")
	}
	tag, err := lastRelease()
	if err != nil {
		return fail("%v", err)
	}
	rng := "HEAD"
	if tag != "" {
		rng = tag + "..HEAD"
	}
	out, err := git("log", "--format=%H%x1f%B%x1e", rng)
	if err != nil {
		return fail("cannot read the commits of %s: %v", rng, err)
	}
	b := bumpOf(messages(out))
	next := ""
	if b.kind != "none" {
		next = bumped(tag, b.kind)
		module, err := headModule()
		if err != nil {
			return fail("%v", err)
		}
		if problem := mismatch(next, module); problem != "" {
			fmt.Fprintf(os.Stderr, "%s: %s\n", self, problem)
			return 1
		}
	}
	fmt.Printf("last=%s\nnext=%s\nbump=%s\nrange=%s\n", tag, next, b.kind, rng)
	from := tag
	if from == "" {
		from = "no release (0.0.0)"
	}
	if next == "" {
		fmt.Fprintf(os.Stderr, "%s: %d commit(s) since %s, none a feat, a fix or a breaking change: nothing to release\n", self, b.commits, from)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %d commit(s) since %s, %s: %s\n", self, b.commits, from, b.why, next)
	}
	return 0
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

// lastRelease is the newest release tag reachable from HEAD, by
// release.Newest's rule, or "" for none.
func lastRelease() (string, error) {
	out, err := git("tag", "--merged", "HEAD")
	if err != nil {
		return "", err
	}
	return release.Newest(strings.Split(out, "\n")), nil
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

// headModule is the module path HEAD's go.mod declares, or "" when HEAD has no
// go.mod.
func headModule() (string, error) {
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
