package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"

	gherkin "github.com/cucumber/gherkin/go/v42"
	messages "github.com/cucumber/messages/go/v34"
)

// fingerprintVersion is the canonical text's first line: a change of what the text holds is a
// new version, never a silent shift of every fingerprint.
const fingerprintVersion = "itos scenario fingerprint v1"

// Fingerprints are the scenarios of a feature file's text by bare ID, each the SHA-256 in
// lowercase hex of its canonical text (Canonical). A scenario's ID is the first of its tags to
// hold <tag_prefix><id>, as ParseFeature reads a tag line; one written twice is the last. A text
// Gherkin does not parse has none: its tests are listed without one; so does a kind whose ID
// pattern does not compile, which ParseFeature refuses.
func Fingerprints(text string, o Options) map[string]string {
	prints := map[string]string{}
	idTag, err := o.idTag()
	if err != nil {
		return prints
	}
	var counter messages.Incrementing
	doc, err := gherkin.ParseGherkinDocument(strings.NewReader(text), counter.NewId)
	if err != nil || doc.Feature == nil {
		return prints
	}
	walkScenarios(doc.Feature, func(tags []*messages.Tag, steps []*messages.Step, s *messages.Scenario) {
		if id := scenarioID(idTag, s); id != "" {
			sum := sha256.Sum256([]byte(Canonical(slices.Concat(tags, s.Tags), steps, s)))
			prints[id] = hex.EncodeToString(sum[:])
		}
	})
	return prints
}

// scenarioID is the bare ID of a scenario: the first of its tags to hold <tag_prefix><id>, ""
// when none does.
func scenarioID(idTag *regexp.Regexp, s *messages.Scenario) string {
	for _, t := range s.Tags {
		if m := idTag.FindStringSubmatch(t.Name); m != nil && m[1] != "" {
			return m[1]
		}
	}
	return ""
}

// walkScenarios calls visit for each scenario of the feature, outlines included, in the order
// written, with the tags it inherits from the feature and its Rule and the steps of the
// Backgrounds before it.
func walkScenarios(feature *messages.Feature, visit func(tags []*messages.Tag, steps []*messages.Step, s *messages.Scenario)) {
	var background []*messages.Step
	for _, child := range feature.Children {
		switch {
		case child.Background != nil:
			background = append(background, child.Background.Steps...)
		case child.Scenario != nil:
			visit(feature.Tags, background, child.Scenario)
		case child.Rule != nil:
			rule := child.Rule
			steps := slices.Clone(background)
			for _, c := range rule.Children {
				if c.Background != nil {
					steps = append(steps, c.Background.Steps...)
				} else if c.Scenario != nil {
					visit(slices.Concat(feature.Tags, rule.Tags), steps, c.Scenario)
				}
			}
		}
	}
}

// Canonical is the text a scenario's fingerprint is the hash of (the package's doc gives its
// lines): its effective tags, the steps of the Backgrounds before it and then its own, and its
// Examples.
func Canonical(tags []*messages.Tag, background []*messages.Step, s *messages.Scenario) string {
	var b strings.Builder
	b.WriteString(fingerprintVersion + "\n")
	writeTags(&b, tags)
	for _, step := range slices.Concat(background, s.Steps) {
		b.WriteString("step " + escape(step.Text) + "\n")
		if d := step.DocString; d != nil {
			b.WriteString("doc " + escape(d.MediaType) + "\t" + escape(d.Content) + "\n")
		}
		if t := step.DataTable; t != nil {
			for _, row := range t.Rows {
				writeRow(&b, "row", row)
			}
		}
	}
	for _, e := range s.Examples {
		b.WriteString("examples\n")
		writeTags(&b, e.Tags)
		if e.TableHeader != nil {
			writeRow(&b, "header", e.TableHeader)
		}
		for _, row := range e.TableBody {
			writeRow(&b, "row", row)
		}
	}
	return b.String()
}

// writeTags writes a tag line for each name of the tags, sorted, each once.
func writeTags(b *strings.Builder, tags []*messages.Tag) {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		b.WriteString("tag " + escape(name) + "\n")
	}
}

// writeRow writes a table row's line: the kind, a space and its cells, each escaped, joined by
// a tab.
func writeRow(b *strings.Builder, kind string, row *messages.TableRow) {
	cells := make([]string, 0, len(row.Cells))
	for _, c := range row.Cells {
		cells = append(cells, escape(c.Value))
	}
	b.WriteString(kind + " " + strings.Join(cells, "\t") + "\n")
}

// escaper writes a field with no line's or cell's separator in it.
var escaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`)

func escape(s string) string { return escaper.Replace(s) }
