// Command plugin-version is the plugin's version rule (T-074): a range that
// changes the Claude Code plugin raises the plugin's version.
//
// Claude Code offers an installed plugin an update only when the version in
// its plugin.json changes, and since T-069 that version is the plugin's own
// (integrations/claude-code/.claude-plugin/plugin.json; the marketplace entry
// gives none). A change to the plugin that leaves the version as it was
// reaches no one who installed it. So a range, <rev>..HEAD, whose trees differ
// under the plugin's folder must leave plugin.json's version higher, by
// semver, than it was at <rev>: a new behaviour a minor, a fix a patch, a
// breaking change a major. The trees are compared, not the commits, so a
// change undone within the range is no change. A range that leaves the folder
// alone passes and says so; a plugin.json new in the range passes with any
// version; one removed, or whose version is not semver at HEAD, fails.
//
// It refuses to pass on what it cannot read: no range start (CI's FROM empty:
// no green run to start from, or a range provider that failed), a start that
// is not a commit of this clone, a shallow clone, or a plugin.json at the
// start it cannot read, each stop it with exit 2.
//
// It imports nothing but the standard library, as tools/bin/schema-contract
// does (T-070), and reads git alone, with no network.
//
//	go run ./tools/bin/plugin-version -range-from <rev> [-plugin <folder>]
//
// -plugin is the plugin's folder, relative to the repository's top
// (integrations/claude-code; . if the plugin has a repository of its own),
// its manifest <folder>/.claude-plugin/plugin.json. Exit status: 0 passed, 1
// the plugin changed and its version was not raised, 2 the check could not
// run.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run())
}

func run() int {
	rangeFrom := flag.String("range-from", "", "the range's start: the plugin's changes and version are read over <rev>..HEAD")
	plugin := flag.String("plugin", "integrations/claude-code", "the plugin's folder, relative to the repository's top")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "plugin-version: unexpected argument %q\n", flag.Arg(0))
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, "plugin-version: "+format+"\n", a...)
		return 2
	}

	folder := strings.Trim(path.Clean(*plugin), "/")
	manifest := path.Join(folder, ".claude-plugin", "plugin.json")
	if *rangeFrom == "" {
		return fail("no range start (-range-from is empty), so what the range changed under %s/ cannot be told:\n"+
			"  in CI, FROM is empty when there is no green run on main to start from or the range provider failed;\n"+
			"  re-run once it answers, or run it by hand with the revision the range starts at", folder)
	}
	if shallow, err := git("rev-parse", "--is-shallow-repository"); err != nil {
		return fail("%v", err)
	} else if strings.TrimSpace(shallow) == "true" {
		return fail("this is a shallow clone, which may not have the range's start: fetch the whole history (actions/checkout's fetch-depth: 0)")
	}
	from, err := git("rev-parse", "--verify", "--quiet", *rangeFrom+"^{commit}")
	if err != nil {
		return fail("the range's start, %s, is not a commit of this clone: fetch it, or name one that is", *rangeFrom)
	}
	from = strings.TrimSpace(from)
	short := from[:min(7, len(from))]

	pathspec := folder + "/"
	if folder == "." {
		pathspec = "."
	}
	out, err := git("diff", "--name-only", "--no-renames", from, "HEAD", "--", pathspec)
	if err != nil {
		return fail("cannot read what changed since %s: %v", short, err)
	}
	var files []string
	for _, f := range strings.Split(out, "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		fmt.Printf("plugin-version: nothing under %s/ changed since %s: the plugin's version need not move\n", folder, short)
		return 0
	}

	oldText, hadManifest, err := show(from, manifest)
	if err != nil {
		return fail("cannot read %s at %s: %v", manifest, short, err)
	}
	newText, hasManifest, err := show("HEAD", manifest)
	if err != nil {
		return fail("cannot read %s at HEAD: %v", manifest, err)
	}
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, "plugin-version: %d file(s) under %s/ changed since %s:\n", len(files), folder, short)
		for _, f := range files {
			fmt.Fprintf(os.Stderr, "  %s\n", f)
		}
		fmt.Fprintf(os.Stderr, "\nplugin-version: "+format+"\n", a...)
		return 1
	}
	if !hasManifest {
		return refuse("%s is gone at HEAD, so Claude Code has no version to offer an update by: put it back", manifest)
	}
	newVersion, err := versionOf(newText)
	if err != nil {
		return refuse("%s at HEAD: %v: give it a version MAJOR.MINOR.PATCH", manifest, err)
	}
	if !hadManifest {
		fmt.Printf("plugin-version: %s is new since %s, at version %s: nothing to raise it above\n", manifest, short, newVersion.text)
		return 0
	}
	oldVersion, err := versionOf(oldText)
	if err != nil {
		return fail("%s at %s: %v", manifest, short, err)
	}
	if compare(newVersion, oldVersion) > 0 {
		fmt.Printf("plugin-version: %d file(s) under %s/ changed since %s, and the plugin's version went from %s to %s\n",
			len(files), folder, short, oldVersion.text, newVersion.text)
		return 0
	}
	how := "is still"
	if compare(newVersion, oldVersion) < 0 {
		how = "went down to"
	}
	return refuse("the plugin changed, but %s's version %s %s (it was %s at %s).\n"+
		"  Claude Code offers an installed plugin an update only when that version changes, so this change would reach no one.\n"+
		"  Raise \"version\" in %s above %s, as semver for the plugin itself:\n"+
		"  %s for a fix, %s for a new behaviour, %s for a breaking change (a chore commit, after the plugin's own).",
		manifest, how, newVersion.text, oldVersion.text, short,
		manifest, oldVersion.text, oldVersion.bump(2), oldVersion.bump(1), oldVersion.bump(0))
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

// show is a file at a commit, and whether the commit has it.
func show(rev, file string) ([]byte, bool, error) {
	if _, err := git("cat-file", "-e", rev+":"+file); err != nil {
		return nil, false, nil
	}
	out, err := git("show", rev+":"+file)
	if err != nil {
		return nil, true, err
	}
	return []byte(out), true, nil
}

// semver is a version as semver 2.0.0 orders it: the build metadata is kept
// in text alone, as it does not count.
type semver struct {
	text       string
	core       [3]uint64
	prerelease []string
}

var semverRE = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

func versionOf(text []byte) (semver, error) {
	var doc map[string]any
	if err := json.Unmarshal(text, &doc); err != nil {
		return semver{}, fmt.Errorf("not JSON: %v", err)
	}
	raw, ok := doc["version"]
	if !ok {
		return semver{}, fmt.Errorf("no \"version\"")
	}
	s, ok := raw.(string)
	if !ok {
		return semver{}, fmt.Errorf("\"version\" is not a string")
	}
	m := semverRE.FindStringSubmatch(s)
	if m == nil {
		return semver{}, fmt.Errorf("\"version\" is %q, which is not semver", s)
	}
	v := semver{text: s}
	for i := range 3 {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return semver{}, fmt.Errorf("\"version\" is %q: %v", s, err)
		}
		v.core[i] = n
	}
	if m[4] != "" {
		v.prerelease = strings.Split(m[4], ".")
	}
	return v, nil
}

// bump is the next version at a place: 0 the major, 1 the minor, 2 the patch.
func (v semver) bump(at int) string {
	c := v.core
	c[at]++
	for i := at + 1; i < 3; i++ {
		c[i] = 0
	}
	// A pre-release's patch is its own release: 2.4.0-rc.1 is raised to 2.4.0.
	if v.prerelease != nil && at == 2 {
		c = v.core
	}
	return fmt.Sprintf("%d.%d.%d", c[0], c[1], c[2])
}

// compare orders two versions by semver's precedence: below 0 a < b, 0 equal,
// above 0 a > b.
func compare(a, b semver) int {
	for i := range 3 {
		if a.core[i] != b.core[i] {
			if a.core[i] < b.core[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.prerelease == nil && b.prerelease == nil:
		return 0
	case a.prerelease == nil:
		return 1
	case b.prerelease == nil:
		return -1
	}
	for i := 0; i < len(a.prerelease) && i < len(b.prerelease); i++ {
		if c := compareIdentifier(a.prerelease[i], b.prerelease[i]); c != 0 {
			return c
		}
	}
	return len(a.prerelease) - len(b.prerelease)
}

func compareIdentifier(a, b string) int {
	an, aErr := strconv.ParseUint(a, 10, 64)
	bn, bErr := strconv.ParseUint(b, 10, 64)
	switch {
	case aErr == nil && bErr == nil:
		if an < bn {
			return -1
		} else if an > bn {
			return 1
		}
		return 0
	case aErr == nil:
		return -1 // numeric identifiers sort below alphanumeric ones
	case bErr == nil:
		return 1
	}
	return strings.Compare(a, b)
}
