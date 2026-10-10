package tests

import (
	"regexp"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/nextid"
)

// NextTag is `tests next-id <kind> <stem>` (slice 60): the next free tag of
// the kind with the stem, its tag prefix, the stem, a dash and a number one
// past the highest that any tag <tag_prefix><stem>-<n> of the kind's files at
// the working tree holds, live or @wip, a tag line of a file's header
// included, and any of others (the work registry's ids, since an item such
// as slice-<n> is named before its scenarios are written). A kind behind a
// command adapter has no tags to read: its listing's IDs count instead. A
// stem nothing uses starts at 1, padded to two digits where the kind's ID
// pattern is an ID- one and the tag is one of its IDs (ID-NEW-01, as the
// README's are), so a slice stays slice-1. It reads, writes nothing and
// judges nothing: a number a removed test held is not seen.
func NextTag(cfg *config.Loaded, name, stem string, others []string) (string, error) {
	k, err := KindOf(cfg, name)
	if err != nil {
		return "", err
	}
	ids := others
	if k.Adapter.Command != "" || k.Adapter.Name != "gherkin" {
		list, err := ListTests(cfg, name, "worktree")
		if err != nil {
			return "", err
		}
		for _, t := range list.Tests {
			ids = append(ids, t.ID)
		}
	} else {
		o, err := gherkinOptions(cfg, name, k)
		if err != nil {
			return "", err
		}
		byPath, err := featureTexts("worktree", o.Root)
		if err != nil {
			return "", err
		}
		for _, text := range byPath {
			ids = append(ids, tags(text, o.TagPrefix)...)
		}
	}
	width := 1
	if k.ID != nil && strings.HasPrefix(strings.TrimPrefix(*k.ID, "^"), "ID-") {
		if id, err := regexp.Compile("^(?:" + *k.ID + ")$"); err == nil && id.MatchString(stem+"-01") {
			width = 2
		}
	}
	return k.TagPrefix + nextid.Next(stem+"-", ids, width, nil), nil
}

// Area is whether stem is an area of the kind's ID pattern, ID-<AREA>: the
// pattern is an ID- one and takes <stem>-01. itos tests next-id claims an
// area's numbers through the id counter (slice 105); any other stem it only
// reads, slice and bug among them, whose numbers the commands that create
// the registry's items mint.
func Area(k config.Kind, stem string) bool {
	if k.ID == nil || !strings.HasPrefix(strings.TrimPrefix(*k.ID, "^"), "ID-") {
		return false
	}
	id, err := regexp.Compile("^(?:" + *k.ID + ")$")
	return err == nil && id.MatchString(stem+"-01")
}

// tags are the tags of a feature file's tag lines, without the tag prefix;
// a word without it is no tag of the kind's.
func tags(text, prefix string) []string {
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if !startsWithTag(line) {
			continue
		}
		for _, word := range strings.Fields(line) {
			if tag, ok := strings.CutPrefix(word, prefix); ok {
				found = append(found, tag)
			}
		}
	}
	return found
}
