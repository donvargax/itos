// Command schema-contract is the schema contract (T-070): a config the last
// release accepts is accepted by this tree, unless a commit since that release
// says the change is breaking.
//
// T-069 cuts a release for every feat or fix that lands green, its version read
// from the commits alone, so an incompatible config change no commit marks as
// breaking would ship as a minor. This holds the config's JSON Schema that
// tools/bin/config-schema generates from this tree to the itos.schema.json the
// last release published, over the subset that generator writes (type, enum,
// properties, required, additionalProperties, items, anyOf, default):
//
//   - breaking, each failing the check unless a commit since the release's tag
//     carries a BREAKING-CHANGE: (or BREAKING CHANGE:) footer or a ! in its
//     header: a key removed, a type narrowed, an enum value removed, a key
//     newly required, an object closed (a key it does not list now refused),
//     a default changed (added, removed or another value: for a config, a
//     default is behaviour);
//   - compatible, each passing and printed for the release notes: a key added,
//     a type widened, an enum value added, a key no longer required, an object
//     opened.
//
// The last release is the newest vX.Y.Z tag reachable from HEAD, and its
// schema the asset of its GitHub release, downloaded:
// <release-url>/<tag>/itos.schema.json. With no such tag there is no release
// to hold the tree to, which the check says and passes; a shallow clone, which
// may hide the tag, a failed download or a release without the asset stops it
// with exit 2, never a pass. A schema keyword the comparison does not read
// stops it too, so a generator that starts writing one is never compared in
// part.
//
// It imports nothing but the standard library, as tools/bin/deps-check does
// (T-067), and generates this tree's schema with go run ./tools/bin/config-schema.
//
//	go run ./tools/bin/schema-contract [-range-from <rev>] [-old <file>] [-new <file>]
//	                                   [-release-url <url>] [-repository <owner/name>]
//	                                   [-release <tag>] [-json]
//
// -range-from <rev> checks nothing unless a commit in <rev>..HEAD is a feat or a
// fix, which is how CI runs it over a push's range; an empty <rev> checks.
// -old and -new read the two schemas from files instead (the self-test's way,
// tools/selftest/schema-contract.ts). -release <tag> holds the tree to that
// release instead of the newest tag reachable from HEAD (the release job's,
// whose own tag is already made locally when it writes the notes). -json prints the findings as one JSON
// object on stdout instead of lines, for the release notes T-069 generates
// (tools/bin/release-notes):
//
//	{"release": "<tag, empty with none>", "findings": [{"path", "change", "breaking"}]}
//
// the exit status unchanged. Exit status: 0 passed, 1 a breaking change no
// commit marks, 2 the check could not run.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/donvargax/itos/v7/internal/release"
)

const asset = "itos.schema.json"

// finding is one difference between the release's schema and this tree's, at
// a config path (ledger.files, ci.steps[], tests.<key>.root).
type finding struct {
	Path     string `json:"path"`
	Change   string `json:"change"`
	Breaking bool   `json:"breaking"`
}

// asJSON is -json: the findings as one object on stdout.
var asJSON bool

// commit is one commit since the release's tag.
type commit struct {
	SHA, Header string
	Breaking    bool
}

func main() {
	os.Exit(run())
}

func run() int {
	rangeFrom := flag.String("range-from", "", "check nothing unless a commit in this revision..HEAD is a feat or a fix")
	oldFile := flag.String("old", "", "the last release's schema from this file, instead of its GitHub release")
	newFile := flag.String("new", "", "this tree's schema from this file, instead of go run ./tools/bin/config-schema")
	releaseURL := flag.String("release-url", "https://github.com/{repository}/releases/download", "where a release's assets are, under <tag>/")
	repository := flag.String("repository", "", "owner/name on GitHub; default $GITHUB_REPOSITORY, else the remote origin's")
	flag.BoolVar(&asJSON, "json", false, "print the findings as one JSON object on stdout")
	release := flag.String("release", "", "the release's tag to hold the tree to, instead of the newest vX.Y.Z tag reachable from HEAD")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "schema-contract: unexpected argument %q\n", flag.Arg(0))
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, "schema-contract: "+format+"\n", a...)
		return 2
	}

	if *rangeFrom != "" {
		headers, err := git("log", "--format=%s", *rangeFrom+"..HEAD")
		if err != nil {
			return fail("cannot read the commits since %s: %v", *rangeFrom, err)
		}
		if !anyReleasable(headers) {
			say("schema-contract: no feat or fix since %s: nothing to check\n", *rangeFrom)
			return emit("", nil, 0)
		}
	}

	if shallow, err := git("rev-parse", "--is-shallow-repository"); err != nil {
		return fail("%v", err)
	} else if strings.TrimSpace(shallow) == "true" {
		return fail("this is a shallow clone, which may not have the last release's tag: fetch the whole history (actions/checkout's fetch-depth: 0)")
	}
	tag, err := lastRelease()
	if *release != "" {
		tag = *release
	}
	if err != nil {
		return fail("%v", err)
	}
	if tag == "" {
		say("schema-contract: no vX.Y.Z tag is reachable from HEAD, so there is no release whose config to hold this tree to: nothing to check\n")
		return emit("", nil, 0)
	}
	commits, err := commitsSince(tag)
	if err != nil {
		return fail("cannot read the commits since %s: %v", tag, err)
	}

	var oldText []byte
	if *oldFile != "" {
		oldText, err = os.ReadFile(*oldFile)
	} else {
		oldText, err = download(*releaseURL, *repository, tag)
	}
	if err != nil {
		return fail("%s's schema: %v", tag, err)
	}
	var newText []byte
	if *newFile != "" {
		newText, err = os.ReadFile(*newFile)
	} else {
		newText, err = generate()
	}
	if err != nil {
		return fail("this tree's schema: %v", err)
	}
	oldSchema, err := parse(oldText)
	if err != nil {
		return fail("%s's schema: %v", tag, err)
	}
	newSchema, err := parse(newText)
	if err != nil {
		return fail("this tree's schema: %v", err)
	}

	var findings []finding
	compare(oldSchema, newSchema, "", &findings)
	return emit(tag, findings, report(tag, findings, commits))
}

// say prints a line for a person: on stdout, or on stderr under -json, whose
// stdout is the object alone.
func say(format string, a ...any) {
	if asJSON {
		fmt.Fprintf(os.Stderr, format, a...)
	} else {
		fmt.Printf(format, a...)
	}
}

// emit prints the object under -json, and gives the exit status back.
func emit(tag string, findings []finding, status int) int {
	if !asJSON {
		return status
	}
	if findings == nil {
		findings = []finding{}
	}
	text, err := json.Marshal(struct {
		Release  string    `json:"release"`
		Findings []finding `json:"findings"`
	}{tag, findings})
	if err != nil {
		fmt.Fprintf(os.Stderr, "schema-contract: %v\n", err)
		return 2
	}
	fmt.Println(string(text))
	return status
}

// report prints the findings and gives the exit status: compatible ones on
// stdout, breaking ones on stdout when a commit marks the range as breaking,
// on stderr with the remedy when none does.
func report(tag string, findings []finding, commits []commit) int {
	if len(findings) == 0 {
		say("schema-contract: this tree's config schema is %s's: no change to a key, a type, a value or a default\n", tag)
		return 0
	}
	var marker *commit
	for i := range commits {
		if commits[i].Breaking {
			marker = &commits[i]
			break
		}
	}
	var unmarked []finding
	for _, f := range findings {
		switch {
		case !f.Breaking:
			say("schema-contract: compatible since %s: %s: %s\n", tag, display(f.Path), f.Change)
		case marker != nil:
			say("schema-contract: breaking since %s, marked by %.7s %q: %s: %s\n",
				tag, marker.SHA, marker.Header, display(f.Path), f.Change)
		default:
			unmarked = append(unmarked, f)
		}
	}
	if len(unmarked) == 0 {
		return 0
	}
	for _, f := range unmarked {
		fmt.Fprintf(os.Stderr, "schema-contract: breaking since %s: %s: %s\n", tag, display(f.Path), f.Change)
	}
	fmt.Fprintf(os.Stderr, "\nschema-contract: %d breaking change(s) to the config since %s, which no commit since it marks as breaking:\n"+
		"  a config %s accepts may be refused, or mean something else, and the release would ship as a minor.\n"+
		"  Mark the commit that makes the change with a BREAKING-CHANGE: footer saying what a consumer must change\n"+
		"  (itos commit --breaking '<what to change>'), or a ! in its header (feat!: …), so the release is a major;\n"+
		"  or keep the config compatible.\n", len(unmarked), tag, tag)
	return 1
}

func display(path string) string {
	if path == "" {
		return "the config"
	}
	return path
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

var (
	releasable = regexp.MustCompile(`^(feat|fix)(\([^)]*\))?!?: `)
	bang       = regexp.MustCompile(`^[a-zA-Z]+(\([^)]*\))?!: `)
)

func anyReleasable(headers string) bool {
	for _, h := range strings.Split(headers, "\n") {
		if releasable.MatchString(h) {
			return true
		}
	}
	return false
}

// lastRelease is the newest release tag reachable from HEAD, or "" for none,
// picked by release.Newest, the release cut's rule (T-090).
func lastRelease() (string, error) {
	out, err := git("tag", "--merged", "HEAD", "--list", "v*")
	if err != nil {
		return "", err
	}
	return release.Newest(strings.Split(out, "\n")), nil
}

// commitsSince lists the commits in <tag>..HEAD, each with whether it is
// marked as breaking: a ! before its header's colon, or a BREAKING-CHANGE: or
// BREAKING CHANGE: footer in the message's last paragraph.
func commitsSince(tag string) ([]commit, error) {
	out, err := git("log", "--format=%H%x1f%B%x1e", tag+"..HEAD")
	if err != nil {
		return nil, err
	}
	var commits []commit
	for _, record := range strings.Split(out, "\x1e") {
		sha, message, ok := strings.Cut(strings.TrimLeft(record, "\n"), "\x1f")
		if !ok {
			continue
		}
		message = strings.TrimSpace(message)
		header, _, _ := strings.Cut(message, "\n")
		commits = append(commits, commit{SHA: sha, Header: header, Breaking: markedBreaking(message)})
	}
	return commits, nil
}

func markedBreaking(message string) bool {
	header, rest, _ := strings.Cut(message, "\n")
	if bang.MatchString(header) {
		return true
	}
	paragraphs := strings.Split(strings.TrimSpace(rest), "\n\n")
	if strings.TrimSpace(rest) == "" {
		return false
	}
	for _, line := range strings.Split(paragraphs[len(paragraphs)-1], "\n") {
		if strings.HasPrefix(line, "BREAKING-CHANGE:") || strings.HasPrefix(line, "BREAKING CHANGE:") {
			return true
		}
	}
	return false
}

var client = &http.Client{Timeout: 60 * time.Second}

// download fetches the release's schema; any answer but 200 is an error, so a
// release without the asset, or one not published yet, never passes.
func download(base, repository, tag string) ([]byte, error) {
	if strings.Contains(base, "{repository}") {
		if repository == "" {
			repository = os.Getenv("GITHUB_REPOSITORY")
		}
		if repository == "" {
			var err error
			if repository, err = originRepository(); err != nil {
				return nil, err
			}
		}
		base = strings.ReplaceAll(base, "{repository}", repository)
	}
	url := strings.TrimSuffix(base, "/") + "/" + tag + "/" + asset
	say("schema-contract: %s's schema from %s\n", tag, url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot download it (it needs the network): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s: the release has no %s, or is not published yet", url, resp.Status, asset)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}

var githubRemote = regexp.MustCompile(`github\.com[:/]([^/]+/[^/]+?)(\.git)?/?$`)

func originRepository() (string, error) {
	out, err := git("remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("no -repository, no GITHUB_REPOSITORY and no remote origin: %v", err)
	}
	m := githubRemote.FindStringSubmatch(strings.TrimSpace(out))
	if m == nil {
		return "", fmt.Errorf("the remote origin, %s, is not on GitHub: say -repository", strings.TrimSpace(out))
	}
	return m[1], nil
}

// generate is this tree's schema, from tools/bin/config-schema.
func generate() ([]byte, error) {
	dir, err := os.MkdirTemp("", "schema-contract-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, asset)
	cmd := exec.Command("go", "run", "./tools/bin/config-schema", out)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stderr, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go run ./tools/bin/config-schema: %v\n%s", err, stderr.String())
	}
	return os.ReadFile(out)
}

// The keywords tools/bin/config-schema writes, the only ones compared.
var known = map[string]bool{
	"$schema": true, "title": true, "description": true, "type": true, "enum": true,
	"properties": true, "required": true, "additionalProperties": true, "items": true,
	"anyOf": true, "default": true,
}

func parse(text []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	root, ok := doc.(map[string]any)
	if !ok {
		return nil, errors.New("not a JSON object")
	}
	return root, validate(root, "")
}

// validate refuses a schema with a keyword the comparison does not read, or
// one of an unexpected shape.
func validate(n map[string]any, path string) error {
	for k, v := range n {
		if !known[k] {
			return fmt.Errorf("%s has %q, which the comparison does not read: teach tools/bin/schema-contract what changing it breaks", display(path), k)
		}
		switch k {
		case "type":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s has a type that is not a string", display(path))
			}
		case "properties":
			props, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("%s has properties that are not an object", display(path))
			}
			for key, p := range props {
				child, ok := p.(map[string]any)
				if !ok {
					return fmt.Errorf("%s is not a schema", at(path, key))
				}
				if err := validate(child, at(path, key)); err != nil {
					return err
				}
			}
		case "items", "additionalProperties":
			if _, ok := v.(bool); ok && k == "additionalProperties" {
				continue
			}
			child, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("%s has %s that is not a schema", display(path), k)
			}
			sub := path + "[]"
			if k == "additionalProperties" {
				sub = at(path, "<key>")
			}
			if err := validate(child, sub); err != nil {
				return err
			}
		case "anyOf":
			alts, ok := v.([]any)
			if !ok {
				return fmt.Errorf("%s has an anyOf that is not a list", display(path))
			}
			for _, a := range alts {
				child, ok := a.(map[string]any)
				if !ok {
					return fmt.Errorf("%s has an anyOf entry that is not a schema", display(path))
				}
				if err := validate(child, path); err != nil {
					return err
				}
			}
		case "enum", "required":
			if _, ok := v.([]any); !ok {
				return fmt.Errorf("%s has %s that is not a list", display(path), k)
			}
		}
	}
	return nil
}

func at(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// compare appends what changed from old to new at path: its default, then the
// values each accepts, alternative by alternative.
func compare(old, new map[string]any, path string, out *[]finding) {
	add := func(breaking bool, format string, a ...any) {
		*out = append(*out, finding{Path: path, Change: fmt.Sprintf(format, a...), Breaking: breaking})
	}
	od, hadDefault := old["default"]
	nd, hasDefault := new["default"]
	switch {
	case hadDefault && !hasDefault:
		add(true, "default removed (was %s)", text(od))
	case !hadDefault && hasDefault:
		add(true, "default added (%s), where there was none", text(nd))
	case hadDefault && !reflect.DeepEqual(od, nd):
		add(true, "default changed from %s to %s", text(od), text(nd))
	}

	olds, news := alternatives(old), alternatives(new)
	oldAny, newAny := hasType(olds, ""), hasType(news, "")
	switch {
	case oldAny && newAny:
		return
	case newAny:
		add(false, "type widened: any value is now accepted (was %s)", typeNames(olds))
		return
	case oldAny:
		add(true, "type narrowed: any value was accepted, now only %s", typeNames(news))
		return
	}
	used := map[int]bool{}
	for _, o := range olds {
		t := typeOf(o)
		match := -1
		for i, n := range news {
			if !used[i] && typeOf(n) == t {
				match = i
				break
			}
		}
		if match < 0 {
			add(true, "type narrowed: %s is no longer accepted (now %s)", article(t), typeNames(news))
			continue
		}
		used[match] = true
		compareSame(o, news[match], t, path, out)
	}
	for i, n := range news {
		if !used[i] {
			add(false, "type widened: %s is now accepted", article(typeOf(n)))
		}
	}
}

// compareSame compares two alternatives of the same type: their enums, their
// items, their keys.
func compareSame(old, new map[string]any, t, path string, out *[]finding) {
	add := func(p string, breaking bool, format string, a ...any) {
		*out = append(*out, finding{Path: p, Change: fmt.Sprintf(format, a...), Breaking: breaking})
	}
	oe, oldEnum := old["enum"].([]any)
	ne, newEnum := new["enum"].([]any)
	switch {
	case oldEnum && !newEnum:
		add(path, false, "values widened: any %s is now accepted, not only %s", t, text(oe))
	case !oldEnum && newEnum:
		add(path, true, "values narrowed: only %s is now accepted", text(ne))
	case oldEnum:
		for _, v := range oe {
			if !contains(ne, v) {
				add(path, true, "enum value %s removed", text(v))
			}
		}
		for _, v := range ne {
			if !contains(oe, v) {
				add(path, false, "enum value %s added", text(v))
			}
		}
	}

	if t == "array" || old["items"] != nil || new["items"] != nil {
		compare(schemaAt(old, "items"), schemaAt(new, "items"), path+"[]", out)
	}

	if t != "object" && old["properties"] == nil && new["properties"] == nil &&
		old["additionalProperties"] == nil && new["additionalProperties"] == nil {
		return
	}
	op, _ := old["properties"].(map[string]any)
	np, _ := new["properties"].(map[string]any)
	oldClosed, newClosed := old["additionalProperties"] == false, new["additionalProperties"] == false
	keys := map[string]bool{}
	for k := range op {
		keys[k] = true
	}
	for k := range np {
		keys[k] = true
	}
	for _, k := range sorted(keys) {
		o, inOld := op[k].(map[string]any)
		n, inNew := np[k].(map[string]any)
		switch {
		case inOld && inNew:
			compare(o, n, at(path, k), out)
		case inOld && newClosed:
			add(at(path, k), true, "key removed")
		case inOld:
			// No longer listed, the key falls under the open object's schema.
			compare(o, schemaAt(new, "additionalProperties"), at(path, k), out)
		case oldClosed:
			add(at(path, k), false, "key added")
		default:
			// Listed now, the key was the open object's.
			compare(schemaAt(old, "additionalProperties"), n, at(path, k), out)
		}
	}
	oldRequired, newRequired := set(old["required"]), set(new["required"])
	for _, k := range sorted(newRequired) {
		if !oldRequired[k] {
			add(at(path, k), true, "key newly required")
		}
	}
	for _, k := range sorted(oldRequired) {
		if !newRequired[k] {
			add(at(path, k), false, "key no longer required")
		}
	}
	switch {
	case !oldClosed && newClosed:
		add(path, true, "object closed: a key it does not list is now refused")
	case oldClosed && !newClosed:
		add(path, false, "object opened: a key it does not list is now accepted")
	case !oldClosed:
		compare(schemaAt(old, "additionalProperties"), schemaAt(new, "additionalProperties"), at(path, "<key>"), out)
	}
}

// alternatives is a schema's anyOf entries, flattened, or the schema itself.
func alternatives(n map[string]any) []map[string]any {
	alts, ok := n["anyOf"].([]any)
	if !ok {
		return []map[string]any{n}
	}
	var out []map[string]any
	for _, a := range alts {
		out = append(out, alternatives(a.(map[string]any))...)
	}
	return out
}

func typeOf(n map[string]any) string {
	t, _ := n["type"].(string)
	return t
}

func hasType(alts []map[string]any, t string) bool {
	for _, a := range alts {
		if typeOf(a) == t {
			return true
		}
	}
	return false
}

func typeNames(alts []map[string]any) string {
	var names []string
	for _, a := range alts {
		names = append(names, article(typeOf(a)))
	}
	return strings.Join(names, " or ")
}

func article(t string) string {
	switch t {
	case "":
		return "any value"
	case "array", "object":
		return "an " + t
	default:
		return "a " + t
	}
}

// schemaAt is the schema under a keyword; an absent one, or true, accepts
// anything.
func schemaAt(n map[string]any, key string) map[string]any {
	if s, ok := n[key].(map[string]any); ok {
		return s
	}
	return map[string]any{}
}

func set(v any) map[string]bool {
	out := map[string]bool{}
	list, _ := v.([]any)
	for _, e := range list {
		if s, ok := e.(string); ok {
			out[s] = true
		}
	}
	return out
}

func sorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(list []any, v any) bool {
	for _, e := range list {
		if reflect.DeepEqual(e, v) {
			return true
		}
	}
	return false
}

func text(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
