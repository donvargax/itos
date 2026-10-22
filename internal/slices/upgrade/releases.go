package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	pinengine "github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/version"
)

var (
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

// releaseNotes is a release with no upgrading.json, named by its notes.
type releaseNotes struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
}

// walkReleases is what each release after from up to to asks, oldest first:
// to's upgrading.json, then its previous's, back to from, each with the
// config edits its itos.schema.json declares. A release with no
// upgrading.json, or one this itos cannot read, is named by its notes and
// ends the walk, as does a previous that is not older than its release.
func (h Handler) walkReleases(ctx context.Context, from, to string) ([]any, error) {
	found := []any{}
	for v := to; version.Compare(v, from) > 0; {
		body, err := h.ports.Fetch(ctx, release.URL(v, "upgrading.json"))
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
		steps, err := h.schemaSteps(ctx, v)
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
func (h Handler) schemaSteps(ctx context.Context, v string) ([]configStep, error) {
	url := release.URL(v, "itos.schema.json")
	body, err := h.ports.Fetch(ctx, url)
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

// moveInstallScript is the install script installing v: its version= line
// set to v and each platform's sum= to the hash the release's checksums.txt,
// sums, gives that platform's archive, every other byte as it was.
func moveInstallScript(script, v string, sums []byte) (string, error) {
	if _, err := pinengine.ScriptVersion(script); err != nil {
		return "", err
	}
	type edit struct {
		start, end int
		text       string
	}
	at := pinengine.ScriptVersionLine.FindStringSubmatchIndex(script)
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
