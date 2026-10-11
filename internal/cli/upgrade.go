package cli

// itos upgrade (slice 75, features/upgrade-command.feature): moves a project
// to a newer itos, <version> or the newest (none, or latest), as itos pin
// moves the pin, and prints what every release after the version it leaves
// asks, oldest first, read from each release's upgrading.json (T-091) back
// through each one's previous, so no API lists the releases. A release with
// no upgrading.json, cut before T-091, is named by its notes, and the walk
// stops there.
//
// It edits only what a release fixes exactly: the pin (pin.go's code), the
// config's first line when it is the yaml-language-server schema line naming
// a release's itos.schema.json, the install script, tools/bin/install-itos
// (v2.0.0's Upgrading), its version= line and each platform's sum= hash from
// the new release's checksums.txt, and the config's keys each crossed
// release's published itos.schema.json declares renamed (a property's
// x-itos-renamed-from) or removed (its top-level x-itos-removed), oldest
// release first, so a key renamed twice ends under its newest name (slice
// 113). Never anything read from a commit's text: the breaking changes and the
// Upgrading footers are what is left by hand, which it prints last.
// Everything is fetched and every edit made in memory before a file is
// written, so a failure, or a key in a layout the config editor cannot change
// byte for byte (value.Doc's Rename and Remove), leaves every file as it was,
// and nothing is committed. Like pin it is the launcher's own command
// (internal/launch): the version pinned may predate it.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/version"
)

// installScriptPath is where a project keeps the install script the release
// notes write, from the repository's top.
const installScriptPath = "tools/bin/install-itos"

var (
	// scriptVersionLine is the install script's version= line.
	scriptVersionLine = regexp.MustCompile(`(?m)^version=(\S*)`)
	// scriptSum is a platform's line of the install script: its platform,
	// os-arch, and the SHA-256 of that platform's archive.
	scriptSum = regexp.MustCompile(`platform=([0-9a-z]+-[0-9a-z]+)[ \t]+sum=([0-9A-Fa-f]{64})\b`)
	// schemaLine is the line the release notes ask a config to start with,
	// the version in the schema's address its second group.
	schemaLine = regexp.MustCompile(`^(#[ \t]*yaml-language-server:[ \t]*\$schema=\S*/download/v)` +
		`(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)(/itos\.schema\.json[ \t]*\r?)$`)
)

// upgradingJSON is a release's upgrading.json, as tools/bin/release-notes
// -json writes it.
type upgradingJSON struct {
	Schema    int              `json:"schema"`
	Version   string           `json:"version"`
	Previous  string           `json:"previous"`
	Breaking  []upgradingEntry `json:"breaking"`
	Upgrading []upgradingEntry `json:"upgrading"`
	Changes   []upgradingEntry `json:"changes"`
	Config    []upgradingKey   `json:"config"`
}

// upgradingEntry is one thing a commit asks: the commit, its header, the
// text.
type upgradingEntry struct {
	Commit string `json:"commit"`
	Header string `json:"header"`
	Text   string `json:"text"`
}

// upgradingKey is a config key the release adds, removes or changes.
type upgradingKey struct {
	Key    string `json:"key"`
	Change string `json:"change"`
}

// releaseAsks is what one release asks, as --json prints it, and the config
// edits its schema declares.
type releaseAsks struct {
	Version   string           `json:"version"`
	Breaking  []upgradingEntry `json:"breaking"`
	Upgrading []upgradingEntry `json:"upgrading"`
	Changes   []upgradingEntry `json:"changes"`
	Config    []upgradingKey   `json:"config"`
	steps     []configStep
}

// configStep is a config edit a release's itos.schema.json declares: the key
// renamed from the key from, or removed.
type configStep struct {
	kind, key, from string
}

// appliedStep is a config edit upgrade made: the release that declared it,
// the step, and the value a removed key held.
type appliedStep struct {
	version string
	configStep
	value any
}

// json is the edit as --json prints it.
func (a appliedStep) json() *value.Map {
	m := value.NewMap("version", a.version, "kind", a.kind, "key", a.key)
	if a.kind == "renamed" {
		m.Set("from", a.from)
	} else {
		m.Set("value", a.value)
	}
	return m
}

// manualStep is what a release leaves the person to do: a breaking change's
// or an Upgrading footer's text, or the notes of a release with no
// upgrading.json.
type manualStep struct {
	Version string `json:"version"`
	Text    string `json:"text"`
}

// releaseNotes is a release with no upgrading.json, named by its notes.
type releaseNotes struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
}

// upgradeCommand is `upgrade [<version>]`.
func upgradeCommand(args []string, o Out) (int, error) {
	want, err := versionArg("upgrade", args)
	if err != nil {
		return 0, err
	}
	c, err := readConfigPin()
	if err != nil {
		return 0, err
	}
	script, hasScript, err := readInstallScript()
	if err != nil {
		return 0, err
	}
	u := upgrade{o: o, to: want, releases: []any{}, edited: []string{}, applied: []appliedStep{}}
	hasPin := config.PinVersion.MatchString(c.version)
	switch {
	case hasPin:
		u.from = c.version
	case hasScript:
		if u.from, err = scriptVersion(script); err != nil {
			return 0, fmt.Errorf("cannot read %s: %w; every file is left as it was", installScriptPath, err)
		}
	default:
		return u.refuse("%s pins no itos, and there is no %s whose version to move: nothing to upgrade; "+
			"itos pin pins a release", c.file, installScriptPath)
	}
	if want != "" && version.Compare(want, u.from) < 0 {
		return u.refuse("itos %s is older than %s, the version this project is on: itos upgrade only moves "+
			"forward; itos pin %s moves it back", want, u.from, want)
	}
	v, sums, err := pinned(want)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s; every file is left as it was\n", err)
		return ExitCode(err), nil
	}
	u.to = v
	sum := release.SHA256(sums)
	if version.Compare(v, u.from) < 0 {
		return u.refuse("the newest itos, %s, is older than %s, the version this project is on: nothing to "+
			"upgrade; itos pin %s moves it back", v, u.from, v)
	}
	if v == u.from {
		if hasPin && c.replaced(v, sum) {
			return u.refuse("%s pins itos %s with the checksums %s, but its checksums.txt is %s now: the release "+
				"changed after it was pinned; every file is left as it was", c.file, v, c.checksums, sum)
		}
		if o.JSON {
			return 0, u.emit("already")
		}
		fmt.Fprintf(o.Stdout, "This project is already on itos %s: nothing to upgrade.\n", v)
		return 0, nil
	}
	if u.releases, err = walkReleases(u.from, v); err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s; every file is left as it was\n", err)
		return ExitCode(err), nil
	}

	text, applied, err := applySteps(c.file, c.text, u.releases)
	if err != nil {
		return 0, err
	}
	u.applied = applied
	if hasPin {
		if text, err = c.moved(text, v, sum); err != nil {
			return 0, err
		}
	}
	text = moveSchemaLine(text, v)
	newScript := script
	if hasScript {
		if newScript, err = moveInstallScript(script, v, sums); err != nil {
			return 0, fmt.Errorf("cannot edit %s: %w; every file is left as it was", installScriptPath, err)
		}
	}
	if text != c.text {
		if err := writeKeepingMode(c.file, text); err != nil {
			return 0, err
		}
		u.edited = append(u.edited, c.file)
	}
	if newScript != script {
		if err := writeKeepingMode(installScriptPath, newScript); err != nil {
			return 0, err
		}
		u.edited = append(u.edited, installScriptPath)
	}
	if o.JSON {
		return 0, u.emit("upgraded")
	}
	u.print()
	return 0, nil
}

// upgrade is one run's report: the version left, the one moved to, what
// each release between asks and the files edited.
type upgrade struct {
	o        Out
	from, to string
	releases []any
	edited   []string
	applied  []appliedStep
}

func (u upgrade) emit(action string) error {
	applied := make([]any, len(u.applied))
	for i, a := range u.applied {
		applied[i] = a.json()
	}
	return out.Emit(u.o.Stdout, out.Field{Key: "action", Value: action}, out.Field{Key: "from", Value: u.from},
		out.Field{Key: "to", Value: u.to}, out.Field{Key: "releases", Value: u.releases},
		out.Field{Key: "edited", Value: u.edited}, out.Field{Key: "applied", Value: applied},
		out.Field{Key: "manual", Value: u.manual()})
}

// manual is what every release crossed leaves the person to do, oldest
// first: its breaking changes and its Upgrading footers, or for a release
// with no upgrading.json the notes to read.
func (u upgrade) manual() []manualStep {
	steps := []manualStep{}
	for _, r := range u.releases {
		switch r := r.(type) {
		case releaseNotes:
			steps = append(steps, manualStep{r.Version, fmt.Sprintf("itos %s publishes no upgrading.json: read the "+
				"Upgrading section of its notes, and of each release before it since %s: %s", r.Version, u.from, r.Notes)})
		case releaseAsks:
			for _, e := range append(slices.Clone(r.Breaking), r.Upgrading...) {
				steps = append(steps, manualStep{r.Version, e.Text})
			}
		}
	}
	return steps
}

// refuse says why nothing is upgraded, exit 1.
func (u upgrade) refuse(format string, a ...any) (int, error) {
	if u.o.JSON {
		return ExitPolicy, u.emit("refused")
	}
	fmt.Fprintf(u.o.Stderr, "itos: "+format+"\n", a...)
	return ExitPolicy, nil
}

// print is the report for a person: the move, then each release, oldest
// first, with the config edits made for it and what it changes, then By
// hand: what is left to do, and the checks the notes end with.
func (u upgrade) print() {
	w := u.o.Stdout
	edited := "nothing needed editing"
	if len(u.edited) > 0 {
		edited = "edited " + strings.Join(u.edited, " and ")
	}
	fmt.Fprintf(w, "Upgraded itos from %s to %s: %s, and committed nothing.\n", u.from, u.to, edited)
	fmt.Fprintf(w, "Each release since %s, oldest first:\n", u.from)
	for _, r := range u.releases {
		if r, ok := r.(releaseAsks); ok {
			u.printRelease(w, r)
		}
	}
	manual := u.manual()
	if len(manual) == 0 {
		fmt.Fprintln(w, "\nBy hand: nothing.")
		fmt.Fprintf(w, "\nCheck: itos version prints itos %s, itos version --check exits 0 and itos config check "+
			"passes. Commit the move as the project's own commit.\n", u.to)
		return
	}
	fmt.Fprintln(w, "\nBy hand:")
	for _, r := range u.releases {
		switch r := r.(type) {
		case releaseNotes:
			fmt.Fprintf(w, "\nitos %s publishes no upgrading.json: read the Upgrading section of its notes, "+
				"and of each release before it since %s, by hand: %s\n", r.Version, u.from, r.Notes)
		case releaseAsks:
			if len(r.Breaking)+len(r.Upgrading) > 0 {
				fmt.Fprintf(w, "\nitos %s (%s):\n", r.Version, release.NotesURL(r.Version))
				entries(w, "Breaking", r.Breaking)
				entries(w, "Upgrading", r.Upgrading)
			}
		}
	}
	fmt.Fprintf(w, "\nDo what they ask, then check: itos version prints itos %s, itos version --check exits 0 "+
		"and itos config check passes. Commit the move as the project's own commit.\n", u.to)
}

// printRelease is a release's config edits, as made, and what it changes
// that asks nothing of the person: the cases it changes on purpose and its
// config keys.
func (u upgrade) printRelease(w io.Writer, r releaseAsks) {
	fmt.Fprintf(w, "\nitos %s (%s):\n", r.Version, release.NotesURL(r.Version))
	edits := 0
	for _, a := range u.applied {
		if a.version != r.Version {
			continue
		}
		edits++
		if a.kind == "renamed" {
			fmt.Fprintf(w, "  Renamed %s to %s.\n", a.from, a.key)
		} else {
			fmt.Fprintf(w, "  Removed %s, which held %s.\n", a.key, value.JSON(a.value))
		}
	}
	if edits+len(r.Changes)+len(r.Config) == 0 {
		fmt.Fprintln(w, "  nothing to edit.")
		return
	}
	entries(w, "Changes on purpose", r.Changes)
	for _, k := range r.Config {
		fmt.Fprintf(w, "  Config key %s: %s\n", k.Key, k.Change)
	}
}

// entries is what commits ask, each under its kind, commit and header.
func entries(w io.Writer, kind string, list []upgradingEntry) {
	for _, e := range list {
		fmt.Fprintf(w, "  %s (%s %s):\n", kind, shortSHA(e.Commit), e.Header)
		for _, line := range strings.Split(strings.TrimRight(e.Text, "\n"), "\n") {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// walkReleases is what each release after from up to to asks, oldest first:
// to's upgrading.json, then its previous's, back to from, each with the
// config edits its itos.schema.json declares. A release with no
// upgrading.json, or one this itos cannot read, is named by its notes and
// ends the walk, as does a previous that is not older than its release.
func walkReleases(from, to string) ([]any, error) {
	found := []any{}
	for v := to; version.Compare(v, from) > 0; {
		body, err := release.Get(release.URL(v, "upgrading.json"), pinTimeout)
		if release.NotFound(err) {
			found = append(found, releaseNotes{Version: v, Notes: release.NotesURL(v)})
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cannot fetch what itos %s asks: %w", v, err)
		}
		var a upgradingJSON
		if json.Unmarshal(body, &a) != nil || a.Schema != 1 {
			found = append(found, releaseNotes{Version: v, Notes: release.NotesURL(v)})
			break
		}
		steps, err := schemaSteps(v)
		if err != nil {
			return nil, err
		}
		found = append(found, releaseAsks{Version: v, Breaking: orEmpty(a.Breaking), Upgrading: orEmpty(a.Upgrading),
			Changes: orEmpty(a.Changes), Config: orEmpty(a.Config), steps: steps})
		if !config.PinVersion.MatchString(a.Previous) || version.Compare(a.Previous, v) >= 0 {
			break
		}
		v = a.Previous
	}
	slices.Reverse(found)
	return found, nil
}

// dottedPath is a config key's path as a schema declares it: keys parted by
// dots.
var dottedPath = regexp.MustCompile(`^[^.\s]+(\.[^.\s]+)*$`)

// schemaSteps is the config edits the itos.schema.json of the release v
// declares, renames first, in the order its properties are written, then
// removals: none when it publishes no schema. One it publishes that declares
// them in a shape this itos cannot read is kind.Missing, since applying
// some of a release's edits and not others could leave a value nowhere.
func schemaSteps(v string) ([]configStep, error) {
	url := release.URL(v, "itos.schema.json")
	body, err := release.Get(url, pinTimeout)
	if release.NotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot fetch the schema of itos %s: %w", v, err)
	}
	unreadable := func(what string) error {
		return kind.Wrap(kind.Missing, fmt.Errorf("the itos.schema.json of itos %s (%s) %s", v, url, what))
	}
	doc, err := value.ParseJSON(string(body))
	if err != nil || !value.IsMapping(doc) {
		return nil, unreadable("is not a JSON object")
	}
	var steps []configStep
	var walk func(schema any, path string) error
	walk = func(schema any, path string) error {
		properties, ok := value.Prop(schema, "properties").(*value.Map)
		if !ok {
			return nil
		}
		for _, k := range properties.Keys() {
			property, at := properties.At(k), strings.TrimPrefix(path+"."+k, ".")
			if from := value.Prop(property, "x-itos-renamed-from"); from != value.Undefined {
				old, ok := from.(string)
				if !ok || !dottedPath.MatchString(old) {
					return unreadable("declares " + at + " renamed from " + value.JSON(from) + ", not a key's dotted path")
				}
				steps = append(steps, configStep{kind: "renamed", key: at, from: old})
			}
			if err := walk(property, at); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc, ""); err != nil {
		return nil, err
	}
	if removed := value.Prop(doc, "x-itos-removed"); removed != value.Undefined {
		list, ok := removed.([]any)
		if !ok {
			return nil, unreadable("lists x-itos-removed as " + value.JSON(removed) + ", not a list")
		}
		for _, key := range list {
			path, ok := key.(string)
			if !ok || !dottedPath.MatchString(path) {
				return nil, unreadable("lists " + value.JSON(key) + " in x-itos-removed, not a key's dotted path")
			}
			steps = append(steps, configStep{kind: "removed", key: path})
		}
	}
	return steps, nil
}

// applySteps is the config's text with the edits each release's schema
// declares made, release by release, oldest first, each on the text the one
// before left, and the edits made: a key that is not there is no edit. One
// the config editor cannot make byte for byte (a flow mapping, an anchor or
// an alias, a new key set already) is a usage error naming the key, exit 2,
// before any file is written.
func applySteps(file, text string, releases []any) (string, []appliedStep, error) {
	applied := []appliedStep{}
	for _, r := range releases {
		asks, ok := r.(releaseAsks)
		if !ok {
			continue
		}
		for _, step := range asks.steps {
			made := appliedStep{version: asks.Version, configStep: step}
			edited, had, err := applyStep(text, &made)
			if err != nil {
				what := "remove " + step.key
				if step.kind == "renamed" {
					what = "rename " + step.from + " to " + step.key
				}
				return "", nil, kind.Wrap(kind.Usage, fmt.Errorf("cannot %s in %s, as itos %s's schema declares (%s): "+
					"edit it by hand, then run itos upgrade again; every file is left as it was", what, file, asks.Version, err))
			}
			if had {
				text = edited
				applied = append(applied, made)
			}
		}
	}
	return text, applied, nil
}

// applyStep is the text with the step made, whether its key was there, and
// for a removal the value it held.
func applyStep(text string, made *appliedStep) (string, bool, error) {
	d, err := value.OpenDoc(text)
	if err != nil {
		return "", false, err
	}
	var had bool
	if made.kind == "renamed" {
		had, err = d.Rename(strings.Split(made.from, "."), strings.Split(made.key, "."))
	} else {
		made.value, had, err = d.Remove(strings.Split(made.key, "."))
	}
	if err != nil || !had {
		return "", false, err
	}
	edited, err := d.Text()
	return edited, err == nil, err
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// readInstallScript is the install script's text and whether there is one.
func readInstallScript() (string, bool, error) {
	text, err := os.ReadFile(installScriptPath)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("cannot read %s: %w", installScriptPath, err)
	}
	return string(text), true, nil
}

// scriptVersion is the version the install script installs, from its one
// version= line.
func scriptVersion(script string) (string, error) {
	lines := scriptVersionLine.FindAllStringSubmatch(script, -1)
	if len(lines) != 1 {
		return "", kind.Wrap(kind.Usage, fmt.Errorf("it has %d version= lines, not one", len(lines)))
	}
	if !config.PinVersion.MatchString(lines[0][1]) {
		return "", kind.Wrap(kind.Usage, fmt.Errorf("its version= line names no release's version: %s", lines[0][1]))
	}
	return lines[0][1], nil
}

// moveInstallScript is the install script installing v: its version= line
// set to v and each platform's sum= to the hash the release's checksums.txt,
// sums, gives that platform's archive, every other byte as it was.
func moveInstallScript(script, v string, sums []byte) (string, error) {
	if _, err := scriptVersion(script); err != nil {
		return "", err
	}
	type edit struct {
		start, end int
		text       string
	}
	at := scriptVersionLine.FindStringSubmatchIndex(script)
	edits := []edit{{at[2], at[3], v}}
	for _, m := range scriptSum.FindAllStringSubmatchIndex(script, -1) {
		platform := script[m[2]:m[3]]
		archive := "itos-" + v + "-" + platform + ".tar.gz"
		if strings.HasPrefix(platform, "windows-") {
			archive = "itos-" + v + "-" + platform + ".zip"
		}
		sum, ok := release.Listed(sums, archive)
		if !ok {
			return "", kind.Wrap(kind.Missing, fmt.Errorf("the checksums.txt of itos %s lists no %s, the archive of its %s line", v, archive, platform))
		}
		edits = append(edits, edit{m[4], m[5], sum})
	}
	slices.SortFunc(edits, func(a, b edit) int { return a.start - b.start })
	var b strings.Builder
	last := 0
	for _, e := range edits {
		b.WriteString(script[last:e.start])
		b.WriteString(e.text)
		last = e.end
	}
	b.WriteString(script[last:])
	return b.String(), nil
}

// moveSchemaLine is the config's text with its first line, when it is the
// schema line naming a release's itos.schema.json, naming v's instead.
func moveSchemaLine(text, v string) string {
	first, rest, found := strings.Cut(text, "\n")
	m := schemaLine.FindStringSubmatch(first)
	if m == nil {
		return text
	}
	moved := m[1] + v + m[3]
	if !found {
		return moved
	}
	return moved + "\n" + rest
}
