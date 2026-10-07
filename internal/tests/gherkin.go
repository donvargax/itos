package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/source"
	"github.com/donvargax/itos/v7/internal/value"
)

// Options are a Gherkin kind's: the folder of its feature files (a test's
// file is relative to it), a scenario ID's pattern without its tag prefix,
// the tag prefix and the wip tag.
type Options struct {
	Root, ID, TagPrefix, WipTag string
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
	idTag, err := regexp.Compile(regexp.QuoteMeta(o.TagPrefix) + "(" + o.ID + `)\b`)
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

// featureFiles are the feature files under a folder, depth first, each
// folder's entries by name; none when it cannot be read.
func featureFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		switch {
		case e.IsDir():
			files = append(files, featureFiles(path)...)
		case strings.HasSuffix(e.Name(), ".feature"):
			files = append(files, path)
		}
	}
	return files
}

// featureTexts are the feature files under a folder as path to text, paths
// as git gives them: from the working tree's file system, the index or a
// commit. A tree that cannot be read has none.
func featureTexts(tree, root string) (map[string]string, error) {
	if tree != "worktree" {
		return source.Texts(tree, root, func(p string) bool { return strings.HasSuffix(p, ".feature") }), nil
	}
	texts := map[string]string{}
	for _, path := range featureFiles(root) {
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		texts[path] = string(text)
	}
	return texts, nil
}

// gherkinList is the adapter protocol's list of a tree's scenarios.
func gherkinList(o Options, at string) (List, error) {
	byPath, err := featureTexts(at, o.Root)
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
		for _, id := range f.IDs {
			tests = append(tests, Test{ID: id, File: file, Live: !f.FileWip && !f.Blocks[id].Wip})
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
	has := regexp.MustCompile("(^|" + value.Space + ")" + regexp.QuoteMeta(tag) + "(" + value.Space + "|$)")
	tagLine := func(line string) bool { return startsWithTag(line) && has.MatchString(line) }
	var tagged []Test
	for _, name := range cfg.Tests.Keys {
		k := cfg.Tests.Values[name]
		if k.Adapter.Command != "" || k.Adapter.Name != "gherkin" {
			continue
		}
		o, err := gherkinOptions(cfg, name, k)
		if err != nil {
			return nil, err
		}
		byPath, err := featureTexts(at, o.Root)
		if err != nil {
			return nil, err
		}
		paths := make([]string, 0, len(byPath))
		for path := range byPath {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			f, err := ParseFeature(byPath[path], o)
			if err != nil {
				return nil, err
			}
			whole := false
			for _, line := range strings.Split(f.Header, "\n") {
				whole = whole || tagLine(line)
			}
			rel, err := filepath.Rel(o.Root, path)
			if err != nil {
				return nil, err
			}
			for _, id := range f.IDs {
				b := f.Blocks[id]
				first, _, _ := strings.Cut(b.Body, "\n")
				if whole || tagLine(first) {
					tagged = append(tagged, Test{ID: o.TagPrefix + id, File: filepath.ToSlash(rel), Live: !f.FileWip && !b.Wip})
				}
			}
		}
	}
	return tagged, nil
}
