// Command release-notes writes a release's notes from its commits (T-069):
// they are generated, never committed, and published as the release's
// description by the release workflow once the archives and checksums.txt
// exist.
//
//	go run ./tools/bin/release-notes -version <X.Y.Z> -checksums <checksums.txt>
//	                                 [-from <tag>] [-to <rev>] [-itos <bin>]
//
// The range is <from>..<to>: <from> the last release's tag (default: the
// newest vX.Y.Z tag reachable from <to> other than v<version>, which the
// release job has already made locally), <to> HEAD by default. The notes are
// Markdown on stdout:
//
//   - a title and a line saying what the range holds, and why the version is
//     what it is (a breaking change makes a major, a feat a minor, a fix a
//     patch, as tools/bin/release-version computes it);
//   - "What changed": every commit of the range, grouped by type, from
//     git-cliff (tools/bin/release-notes/cliff.toml, run by tools/bin/pinned);
//   - "Upgrading", last, from which a consumer's session updates alone: every
//     breaking change's BREAKING-CHANGE: footer (or its ! header); every
//     Upgrading: footer quoted, as `itos commit footers Upgrading` lists them
//     (those saying none left out); every Changes: entry, the old scenarios
//     and corpus cases a fix changes on purpose (T-071, `itos commit footers
//     Changes`); what changed in the config since <from>, from
//     tools/bin/schema-contract -json (T-070: a key added, removed, a type, an
//     enum value, a default); then the pin to change, filled from
//     checksums.txt: the install script with each platform's hash, the pin's
//     two lines with checksums.txt's own SHA-256, go install and the schema
//     line, and how to check.
//
// tools/selftest/release-notes.ts proves notes it writes against the range,
// and tools/selftest/release-cut.ts runs both on a snapshot. It imports
// nothing but the standard library (T-067). Exit status: 0 written, 2 a part
// could not be read (a tool that fails, a checksums.txt without an archive of
// a platform).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/donvargax/itos/v3/internal/release"
)

const (
	self       = "release-notes"
	repository = "donvargax/itos"
)

// The platforms a release publishes, in the install script's order
// (docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md).
var platforms = []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"}

var (
	typed  = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?(!)?: `)
	semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

func main() {
	os.Exit(run())
}

func run() int {
	ver := flag.String("version", "", "the version released, X.Y.Z")
	sums := flag.String("checksums", "", "the release's checksums.txt")
	from := flag.String("from", "", "the last release's tag (default: the newest vX.Y.Z reachable from -to but v<version>)")
	to := flag.String("to", "HEAD", "the range's end")
	itos := flag.String("itos", "tools/bin/itos", "the itos whose commit footers reads the range's footers")
	flag.Parse()
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, self+": "+format+"\n", a...)
		return 2
	}
	if flag.NArg() > 0 || !semver.MatchString(*ver) || *sums == "" {
		return fail("usage: go run ./tools/bin/release-notes -version <X.Y.Z> -checksums <checksums.txt> [-from <tag>] [-to <rev>] [-itos <bin>]")
	}
	n := notes{Version: *ver, Repository: repository, Module: modulePath(*ver), From: *from}
	var err error
	if n.From == "" {
		if n.From, err = lastRelease(*to, "v"+*ver); err != nil {
			return fail("%v", err)
		}
	}
	rng := *to
	if n.From != "" {
		rng = n.From + ".." + *to
	}
	n.Range = rng
	if n.Hashes, n.Pin, err = readChecksums(*sums, *ver); err != nil {
		return fail("%v", err)
	}
	if n.Commits, err = commits(rng); err != nil {
		return fail("%v", err)
	}
	if n.Changed, err = cliff(rng); err != nil {
		return fail("%v", err)
	}
	if n.Upgrading, err = footers(*itos, "Upgrading", n.From, *to); err != nil {
		return fail("%v", err)
	}
	if n.Changes, err = footers(*itos, "Changes", n.From, *to); err != nil {
		return fail("%v", err)
	}
	whole(n.Upgrading, "Upgrading", n.Commits)
	if n.From != "" {
		if n.Config, err = configChanges(n.From); err != nil {
			return fail("%v", err)
		}
	}
	text, err := n.render()
	if err != nil {
		return fail("%v", err)
	}
	fmt.Print(text)
	return 0
}

// notes is everything the template reads.
type notes struct {
	Version, Repository, Module, From, Range, Changed, Pin string
	Hashes                                                 map[string]string // platform → its archive's SHA-256
	Commits                                                []commit
	Upgrading, Changes                                     []footer
	Config                                                 []finding
}

// modulePath is the Go module a version is go-installed from: the repository's
// path, ending in /v<major> from v2 as Go requires of a tag (the release cut,
// tools/bin/release-version, refuses a version go.mod's path does not match).
func modulePath(version string) string {
	major, _, _ := strings.Cut(version, ".")
	if major == "0" || major == "1" {
		return "github.com/" + repository
	}
	return "github.com/" + repository + "/v" + major
}

type commit struct {
	SHA, Header, Type, Breaking string // Breaking: what its footer asks, or its header for a !
	message                     string
}

type footer struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

type finding struct {
	Path     string `json:"path"`
	Change   string `json:"change"`
	Breaking bool   `json:"breaking"`
}

func command(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "RUST_LOG=warn")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %v\n%s", name, strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

// lastRelease is the newest release tag reachable from rev but own, or "",
// picked by release.Newest, the release cut's rule (T-090).
func lastRelease(rev, own string) (string, error) {
	out, err := command("git", "tag", "--merged", rev, "--list", "v*")
	if err != nil {
		return "", err
	}
	var tags []string
	for _, t := range strings.Split(out, "\n") {
		if t != own {
			tags = append(tags, t)
		}
	}
	return release.Newest(tags), nil
}

// readChecksums reads each platform's archive hash from checksums.txt, and
// the file's own SHA-256, which a pin names.
func readChecksums(path, version string) (map[string]string, string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	listed := map[string]string{}
	for _, line := range strings.Split(string(text), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 {
			listed[strings.TrimPrefix(fields[1], "*")] = fields[0]
		}
	}
	hashes := map[string]string{}
	for _, p := range platforms {
		name := archive(version, p)
		if listed[name] == "" {
			return nil, "", fmt.Errorf("%s lists no %s", path, name)
		}
		hashes[p] = listed[name]
	}
	sum := sha256.Sum256(text)
	return hashes, hex.EncodeToString(sum[:]), nil
}

func archive(version, platform string) string {
	if strings.HasPrefix(platform, "windows") {
		return fmt.Sprintf("itos-%s-%s.zip", version, platform)
	}
	return fmt.Sprintf("itos-%s-%s.tar.gz", version, platform)
}

// commits lists the range's commits, each with its type and what it breaks.
func commits(rng string) ([]commit, error) {
	out, err := command("git", "log", "--reverse", "--format=%H%x1f%B%x1e", rng)
	if err != nil {
		return nil, err
	}
	var list []commit
	for _, record := range strings.Split(out, "\x1e") {
		sha, message, ok := strings.Cut(strings.TrimLeft(record, "\n"), "\x1f")
		if !ok {
			continue
		}
		message = strings.TrimSpace(message)
		header, _, _ := strings.Cut(message, "\n")
		c := commit{SHA: sha, Header: header, Breaking: breaking(message), message: message}
		if t := typed.FindStringSubmatch(header); t != nil {
			c.Type = t[1]
		}
		list = append(list, c)
	}
	return list, nil
}

// footerKey is a footer line's start in a message's last paragraph: a capitalised token, as
// every footer here is, so a wrapped line that starts "pin: {" continues the one above.
var footerKey = regexp.MustCompile(`^([A-Z][A-Za-z-]*|BREAKING CHANGE): `)

// footerText is the text of the footer of a message's last paragraph that
// starts with key and, when starts is not "", that text: its first line and
// every line after it up to the next footer, as commits here wrap a long one;
// "" when there is none.
func footerText(message, key, starts string) string {
	_, rest, _ := strings.Cut(message, "\n")
	if strings.TrimSpace(rest) == "" {
		return ""
	}
	paragraphs := strings.Split(strings.TrimSpace(rest), "\n\n")
	var text []string
	in := false
	for _, line := range strings.Split(paragraphs[len(paragraphs)-1], "\n") {
		m := footerKey.FindStringSubmatch(line)
		switch {
		case m != nil && len(text) > 0:
			return strings.Join(text, "\n")
		case m != nil && m[1] == key && strings.HasPrefix(strings.TrimSpace(line[len(m[0]):]), starts):
			in, text = true, []string{strings.TrimSpace(line[len(m[0]):])}
		case m != nil:
			in = false
		case in:
			text = append(text, strings.TrimSpace(line))
		}
	}
	return strings.Join(text, "\n")
}

// breaking is what a commit marked as breaking asks, read as
// tools/bin/schema-contract reads the mark: its BREAKING-CHANGE: (or BREAKING
// CHANGE:) footer's text in the message's last paragraph, or its header when
// only a ! marks it; "" when it is not.
func breaking(message string) string {
	for _, key := range []string{"BREAKING-CHANGE", "BREAKING CHANGE"} {
		if text := footerText(message, key, ""); text != "" {
			return text
		}
	}
	header, _, _ := strings.Cut(message, "\n")
	if t := typed.FindStringSubmatch(header); t != nil && t[3] == "!" {
		return header
	}
	return ""
}

// whole gives each footer itos commit footers listed its whole text: itos
// reads a footer's first line alone (p3-text-footer-lines), and a commit here
// wraps a long one onto the lines below it.
func whole(listed []footer, key string, commits []commit) {
	for i, f := range listed {
		for _, c := range commits {
			if strings.HasPrefix(c.SHA, f.SHA) {
				if text := footerText(c.message, key, f.Text); text != "" {
					listed[i].Text = text
				}
			}
		}
	}
}

// cliff is the range's commits grouped by type, from git-cliff.
func cliff(rng string) (string, error) {
	pinned := filepath.Join("tools", "bin", "pinned")
	out, err := command(pinned, "git-cliff", "--offline", "--config", "tools/bin/release-notes/cliff.toml", "--strip", "all", rng)
	return strings.TrimSpace(out), err
}

// footers lists the range's footers by name, from itos commit footers.
func footers(itos, name, from, to string) ([]footer, error) {
	out, err := command(itos, "commit", "footers", name, from, to, "--json")
	if err != nil {
		return nil, err
	}
	var listed struct {
		Footers []footer `json:"footers"`
	}
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		return nil, fmt.Errorf("itos commit footers %s --json: %v", name, err)
	}
	return listed.Footers, nil
}

// configChanges is what changed in the config's schema since the release,
// from the schema contract (T-070).
func configChanges(from string) ([]finding, error) {
	out, err := command("go", "run", "./tools/bin/schema-contract", "-json", "-release", from)
	if err != nil {
		return nil, err
	}
	var found struct {
		Findings []finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		return nil, fmt.Errorf("schema-contract -json: %v", err)
	}
	return found.Findings, nil
}

// count says how many commits of the range are of a type, or breaking.
func (n notes) count(kind string) int {
	c := 0
	for _, commit := range n.Commits {
		if (kind == "breaking" && commit.Breaking != "") || (kind != "breaking" && commit.Type == kind) {
			c++
		}
	}
	return c
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (n notes) render() (string, error) {
	funcs := template.FuncMap{
		"count":  n.count,
		"plural": plural,
		"short":  func(sha string) string { return sha[:min(7, len(sha))] },
		// A footer's text as written, its lines kept: a blockquote whose later lines continue it
		// lazily, so the text reads back word for word.
		"quote": func(text string) string {
			return "> " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n     ")
		},
		"oneline": func(text string) string { return strings.Join(strings.Fields(text), " ") },
		"since": func() string {
			if n.From == "" {
				return "the first commit"
			}
			return n.From
		},
	}
	t, err := template.New("notes").Funcs(funcs).Parse(notesTemplate)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, n); err != nil {
		return "", err
	}
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(b.String(), "\n\n"), nil
}

const notesTemplate = `# itos {{.Version}}

Cut by CI from the {{plural (len .Commits) "commit" "commits"}} since {{since}}: {{plural (count "feat") "feat" "feats"}}, {{plural (count "fix") "fix" "fixes"}} and {{plural (count "breaking") "breaking change" "breaking changes"}}, where a breaking change makes a major release, a feat a minor one and a fix a patch.{{if .From}} Every change: https://github.com/{{.Repository}}/compare/{{.From}}...v{{.Version}}{{end}}

## What changed

{{.Changed}}

## Upgrading

Commit these changes as your project's rules ask (the install script and the pin are ` + "`build`" + `
commits, or whatever type your rules give ` + "`itos.yaml`" + ` and ` + "`tools/bin/`" + `).

1. **What the commits ask.**
{{- if eq (count "breaking") 0}} No commit since {{since}} is a breaking change.{{end}}
{{- range .Commits}}{{if .Breaking}}
   - Breaking, {{short .SHA}} ` + "`{{.Header}}`" + `:
     {{quote .Breaking}}
{{- end}}{{end}}
{{- if .Upgrading}}
   Each ` + "`Upgrading:`" + ` footer since {{since}} (` + "`itos commit footers Upgrading {{.From}} v{{.Version}}`" + `):
{{- range .Upgrading}}
   - {{short .SHA}} ` + "`{{.Subject}}`" + `:
     {{quote .Text}}
{{- end}}
{{- else}} Every ` + "`feat`" + ` and ` + "`fix`" + ` since {{since}} says ` + "`Upgrading: none`" + `.{{end}}

2. **What it changes on purpose.**
{{- if .Changes}} The old scenarios and conformance cases the range's ` + "`Changes:`" + ` footers name,
   which a fix changes because they held the bug (T-071):
{{- range .Changes}}
   - {{short .SHA}} ` + "`{{.Subject}}`" + `: {{oneline .Text}}
{{- end}}
{{- else}} No fix since {{since}} names an old scenario or case it changes (` + "`Changes:`" + `).{{end}}

3. **The config.**
{{- if .Config}} Since {{since}} (` + "`tools/bin/schema-contract`" + `, T-070):
{{- range .Config}}
   - ` + "`{{.Path}}`" + `: {{.Change}}{{if .Breaking}} (breaking){{end}}
{{- end}}
{{- else}} No key, type, value or default changed since {{since}}: a config it accepts, this release
   accepts and reads the same (` + "`tools/bin/schema-contract`" + `, T-070).{{end}}

4. **Move the install script to {{.Version}}.** A project on the install script
   (` + "`tools/bin/install-itos`" + `, v2.0.0's Upgrading, step 1) changes its ` + "`version=`" + ` line and each
   platform's hash, from this release's ` + "`checksums.txt`" + `; the whole script, for a project that has
   none yet:

   ` + "```sh" + `
   #!/bin/sh
   # tools/bin/install-itos: the pinned itos, into .tools/bin/
   set -eu
   version={{.Version}}
   cd "$(git rev-parse --show-toplevel)"
   case "$(uname -s)-$(uname -m)" in
   Linux-x86_64) platform=linux-amd64 sum={{index .Hashes "linux-amd64"}} ;;
   Linux-aarch64 | Linux-arm64) platform=linux-arm64 sum={{index .Hashes "linux-arm64"}} ;;
   Darwin-x86_64) platform=darwin-amd64 sum={{index .Hashes "darwin-amd64"}} ;;
   Darwin-arm64) platform=darwin-arm64 sum={{index .Hashes "darwin-arm64"}} ;;
   MINGW*-x86_64 | MSYS*-x86_64) platform=windows-amd64 sum={{index .Hashes "windows-amd64"}} ;;
   *) echo "install-itos: no itos archive for $(uname -s)-$(uname -m)" >&2 && exit 1 ;;
   esac
   case "$platform" in
   windows-*) archive="itos-$version-$platform.zip" bin=.tools/bin/itos.exe ;;
   *) archive="itos-$version-$platform.tar.gz" bin=.tools/bin/itos ;;
   esac
   # What was installed, so a call with nothing to do runs nothing.
   pin=".tools/bin/itos.pin"
   if [ -x "$bin" ] && [ "$(cat "$pin" 2>/dev/null)" = "$version $sum" ]; then exit 0; fi
   tmp=$(mktemp -d)
   trap 'rm -rf "$tmp"' EXIT
   curl -fsSL -o "$tmp/$archive" "https://github.com/{{.Repository}}/releases/download/v$version/$archive"
   if command -v sha256sum >/dev/null; then check="sha256sum -c -"; else check="shasum -a 256 -c -"; fi
   (cd "$tmp" && echo "$sum  $archive" | $check)
   mkdir -p .tools/bin
   case "$archive" in
   *.zip) unzip -o -q "$tmp/$archive" itos.exe -d .tools/bin ;;
   *) tar -xzf "$tmp/$archive" -C .tools/bin itos ;;
   esac
   echo "$version $sum" > "$pin"
   echo "install-itos: itos $version in $bin"
   ` + "```" + `

5. **Or move the pin to {{.Version}}.** A repository that pins a release in ` + "`itos.yaml`" + `
   (v2.1.0's Upgrading, step 2) changes its two lines; ` + "`pin.checksums`" + ` is the SHA-256 of this
   release's ` + "`checksums.txt`" + ` itself, one hash for every platform:

   ` + "```yaml" + `
   pin:
     version: {{.Version}}
     checksums: {{.Pin}}
   ` + "```" + `

   A CI step that installs itos by ` + "`go install`" + ` moves to
   ` + "`go install {{.Module}}/cmd/itos@v{{.Version}}`" + `.

6. **Point the schema line at v{{.Version}}'s.** If ` + "`itos.yaml`" + `'s first line names a release's
   ` + "`itos.schema.json`" + `, make it
   ` + "`# yaml-language-server: $schema=https://github.com/{{.Repository}}/releases/download/v{{.Version}}/itos.schema.json`" + `.

7. **Check:** ` + "`tools/bin/itos version`" + ` (or ` + "`itos version`" + ` in the repository, with a pin)
   prints ` + "`itos {{.Version}}`" + `, ` + "`tools/bin/itos version --check`" + ` exits 0 (widen a cap in
   ` + "`requires`" + ` that {{.Version}} is past), and ` + "`tools/bin/itos config check`" + ` passes. To report a
   problem itos causes, open an issue on {{.Repository}} with the label ` + "`consumer-report`" + `
   (https://github.com/{{.Repository}}/issues/new?template=consumer-report.yml).

The archives are attested by the workflow that built them:
` + "`gh attestation verify <archive> -R {{.Repository}}`" + ` checks one, and
` + "`sha256sum --ignore-missing -c checksums.txt`" + ` checks the ones downloaded beside it.
`
