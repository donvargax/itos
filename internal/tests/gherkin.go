package tests

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	gherkin "github.com/cucumber/gherkin/go/v42"
	messages "github.com/cucumber/messages/go/v34"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Options are a Gherkin kind's: the folder of its feature files (a test's
// file is relative to it), a scenario ID's pattern without its tag prefix,
// the tag prefix and the wip tag.
type Options struct {
	Root, ID, TagPrefix, WipTag string
}

// idTag finds <tag_prefix><id> as a whole word, the ID its first group.
func (o Options) idTag() (*regexp.Regexp, error) {
	return regexp.Compile(regexp.QuoteMeta(o.TagPrefix) + "(" + o.ID + `)\b`)
}

// Block is one scenario as written: its tag line, its Scenario line and its
// steps, and whether its tag line holds the wip tag.
type Block struct {
	Wip  bool
	Body string
}

// Feature is a feature file read: its header (everything before the first
// scenario, the Background included), whether a tag line of the header holds
// the wip tag, and its scenarios by ID, in the order written.
type Feature struct {
	Header  string
	FileWip bool
	IDs     []string
	Blocks  map[string]Block
}

// startsWithTag is whether a line, trimmed, begins with a tag.
func startsWithTag(line string) bool { return strings.HasPrefix(value.Trim(line), "@") }

// ParseFeature reads a feature file's header and its scenario blocks. A
// scenario is the block starting at a line that begins with a tag and holds
// <tag_prefix><id> as a whole word, up to the next such line. An error when
// the kind's ID pattern does not compile.
func ParseFeature(text string, o Options) (Feature, error) {
	idTag, err := o.idTag()
	if err != nil {
		return Feature{}, err
	}
	wipTag := regexp.MustCompile("(^|" + value.Space + ")" + regexp.QuoteMeta(o.WipTag) + "(" + value.Space + "|$)")
	f := Feature{Blocks: map[string]Block{}}
	var header []string
	var id string
	var lines []string
	flush := func() {
		if lines == nil {
			return
		}
		if _, ok := f.Blocks[id]; !ok {
			f.IDs = append(f.IDs, id)
		}
		f.Blocks[id] = Block{
			Wip:  wipTag.MatchString(lines[0]),
			Body: strings.TrimRightFunc(strings.Join(lines, "\n"), isTrimmed),
		}
	}
	for _, line := range strings.Split(text, "\n") {
		m := idTag.FindStringSubmatch(line)
		switch {
		case m != nil && m[1] != "" && startsWithTag(line):
			flush()
			id, lines = m[1], []string{line}
		case lines != nil:
			lines = append(lines, line)
		default:
			header = append(header, line)
		}
	}
	flush()
	f.Header = strings.TrimRightFunc(strings.Join(header, "\n"), isTrimmed)
	for _, line := range header {
		if startsWithTag(line) && wipTag.MatchString(line) {
			f.FileWip = true
		}
	}
	return f, nil
}

// isTrimmed is what trimEnd removes.
func isTrimmed(r rune) bool { return value.Trim(string(r)) == "" }

// featureFiles are the feature files under a folder, read from the folder
// base, depth first, each folder's entries by name; none when it cannot be
// read.
func featureFiles(base, dir string) []string {
	entries, err := os.ReadDir(vcsport.In(base, dir))
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		switch {
		case e.IsDir():
			files = append(files, featureFiles(base, path)...)
		case strings.HasSuffix(e.Name(), ".feature"):
			files = append(files, path)
		}
	}
	return files
}

// featureTexts are the feature files under a folder as path to text, paths
// as git gives them: from the working tree's file system, from the folder
// itos runs in (the config's Folder), the index or a commit. A tree that
// cannot be read has none.
func featureTexts(cfg *config.Loaded, tree, root string) (map[string]string, error) {
	if tree != "worktree" {
		return cfg.Trees.Texts(context.Background(), tree, root, func(p string) bool { return strings.HasSuffix(p, ".feature") }), nil
	}
	texts := map[string]string{}
	for _, path := range featureFiles(cfg.Folder(), root) {
		text, err := os.ReadFile(vcsport.In(cfg.Folder(), path))
		if err != nil {
			return nil, err
		}
		texts[path] = string(text)
	}
	return texts, nil
}

// gherkinList is the adapter protocol's list of a tree's scenarios.
func gherkinList(cfg *config.Loaded, o Options, at string) (List, error) {
	byPath, err := featureTexts(cfg, at, o.Root)
	if err != nil {
		return List{}, err
	}
	texts := map[string]string{}
	files := []string{}
	for path, text := range byPath {
		rel, err := filepath.Rel(o.Root, path)
		if err != nil {
			return List{}, err
		}
		rel = filepath.ToSlash(rel)
		texts[rel] = text
		files = append(files, rel)
	}
	sort.Strings(files)
	tests := []Test{}
	for _, file := range files {
		f, err := ParseFeature(texts[file], o)
		if err != nil {
			return List{}, err
		}
		prints := Fingerprints(texts[file], o)
		for _, id := range f.IDs {
			tests = append(tests, Test{ID: id, File: file, Live: !f.FileWip && !f.Blocks[id].Wip, Fingerprint: prints[id]})
		}
	}
	return List{Protocol: 1, Tests: tests, Files: files}, nil
}

// Tagged are the scenarios of the config's Gherkin kinds at a tree whose
// tag line, or a tag line of their file's header, holds the tag as a whole
// word (work done's "@slice-<n>", slice 53), in each kind's order, each ID
// with its kind's tag prefix, as the tag line writes it. A kind behind a
// command adapter is left out: the protocol's list carries no tags.
func Tagged(cfg *config.Loaded, tag, at string) ([]Test, error) {
	found, _, err := readTagged(cfg, tag, at, false)
	return found, err
}

// WipExample is an Examples block still tagged with its kind's wip tag, under
// an outline of Test: Name is the block's Examples name, "" when it has none,
// and Line the line of its Examples keyword in the file Test names.
type WipExample struct {
	Test Test
	Name string
	Line int
}

// TaggedExamples are Tagged and, in the order written, the Examples blocks
// tagged with the kind's wip tag, as Gherkin reads a block's tags, under the
// live scenarios among them (a scenario that is not live is given by Tagged
// as such, and its blocks are not listed again).
func TaggedExamples(cfg *config.Loaded, tag, at string) ([]Test, []WipExample, error) {
	return readTagged(cfg, tag, at, true)
}

// TaggedFile is a scenario Tagged gives with Path, its feature file from
// the repository's top (Test's File is from its kind's root), which itos
// brief reads and names (slice-204).
type TaggedFile struct {
	Test
	Path string
}

// TaggedFiles is Tagged with each scenario's feature file from the top.
func TaggedFiles(cfg *config.Loaded, tag, at string) ([]TaggedFile, error) {
	var found []TaggedFile
	err := eachTagged(cfg, tag, at, func(o Options, _ string, in []Test) {
		for _, t := range in {
			found = append(found, TaggedFile{Test: t, Path: path.Join(filepath.ToSlash(o.Root), t.File)})
		}
	})
	return found, err
}

func readTagged(cfg *config.Loaded, tag, at string, examples bool) ([]Test, []WipExample, error) {
	var found []Test
	var wip []WipExample
	err := eachTagged(cfg, tag, at, func(o Options, text string, in []Test) {
		found = append(found, in...)
		if examples {
			wip = append(wip, wipExamples(o, text, in)...)
		}
	})
	return found, wip, err
}

// wipExamples are the Examples blocks tagged with the wip tag under the live
// scenarios of a feature file's text.
func wipExamples(o Options, text string, in []Test) []WipExample {
	idTag, _ := o.idTag()
	var counter messages.Incrementing
	doc, err := gherkin.ParseGherkinDocument(strings.NewReader(text), counter.NewId)
	if err != nil || doc.Feature == nil {
		return nil
	}
	scenarios := map[string]*messages.Scenario{}
	walkScenarios(doc.Feature, func(_ []*messages.Tag, _ []*messages.Step, s *messages.Scenario) {
		if id := scenarioID(idTag, s); id != "" {
			scenarios[id] = s
		}
	})
	var wip []WipExample
	for _, t := range in {
		s := scenarios[strings.TrimPrefix(t.ID, o.TagPrefix)]
		if !t.Live || s == nil {
			continue
		}
		for _, e := range s.Examples {
			if slices.ContainsFunc(e.Tags, func(g *messages.Tag) bool { return g.Name == o.WipTag }) {
				wip = append(wip, WipExample{Test: t, Name: e.Name, Line: int(e.Location.Line)})
			}
		}
	}
	return wip
}

// eachTagged calls visit for each feature file of the config's Gherkin kinds
// at a tree that holds scenarios Tagged gives for the tag, with the kind's
// options, the file's text and those scenarios.
func eachTagged(cfg *config.Loaded, tag, at string, visit func(o Options, text string, found []Test)) error {
	has := regexp.MustCompile("(^|" + value.Space + ")" + regexp.QuoteMeta(tag) + "(" + value.Space + "|$)")
	tagLine := func(line string) bool { return startsWithTag(line) && has.MatchString(line) }
	for _, name := range cfg.Tests.Keys {
		k := cfg.Tests.Values[name]
		if k.Adapter.Command != "" || k.Adapter.Name != "gherkin" {
			continue
		}
		o, err := gherkinOptions(cfg, name, k)
		if err != nil {
			return err
		}
		byPath, err := featureTexts(cfg, at, o.Root)
		if err != nil {
			return err
		}
		paths := make([]string, 0, len(byPath))
		for path := range byPath {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			f, err := ParseFeature(byPath[path], o)
			if err != nil {
				return err
			}
			whole := false
			for _, line := range strings.Split(f.Header, "\n") {
				whole = whole || tagLine(line)
			}
			rel, _ := filepath.Rel(o.Root, path)
			var found []Test
			for _, id := range f.IDs {
				b := f.Blocks[id]
				first, _, _ := strings.Cut(b.Body, "\n")
				if whole || tagLine(first) {
					found = append(found, Test{ID: o.TagPrefix + id, File: filepath.ToSlash(rel), Live: !f.FileWip && !b.Wip})
				}
			}
			if found != nil {
				visit(o, byPath[path], found)
			}
		}
	}
	return nil
}
