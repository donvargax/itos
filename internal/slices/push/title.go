package push

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/work"
)

// titleLimit is how many characters of a pull request's title GitHub keeps.
const titleLimit = 256

// closing is the parenthesised list of issues that ends an item's title:
// "(#<n>)" or "(#<n>, #<m>, ...)", the title's runs of space being one space
// already.
var closing = regexp.MustCompile(`\((#\d+(?:, #\d+)*)\)$`)

// describe is the title and the description of the item's pull request, from
// the title the registry gives the item at the pushed head (work.TitleAt): the
// title is "<item>: <name>" cut to what GitHub keeps, the description the
// name whole and a line "Closes #<n>" for each issue the list that ends the
// name gives, each once. An item with no name is titled by its id alone, in
// both.
func (r pushRun) describe(item string) (title, body string) {
	name := work.TitleAt(r.cfg.Trees, r.cfg.Work.Registry, r.sha, item)
	if name == "" {
		return item, item
	}
	var lines []string
	if m := closing.FindStringSubmatch(name); m != nil {
		for _, issue := range strings.Split(m[1], ", ") {
			if !slices.Contains(lines, "Closes "+issue) {
				lines = append(lines, "Closes "+issue)
			}
		}
	}
	body = name
	if len(lines) > 0 {
		body += "\n\n" + strings.Join(lines, "\n")
	}
	return cut(item + ": " + name), body
}

// cut is the title as GitHub keeps it: its first 255 code points and an
// ellipsis (U+2026) when it has more than titleLimit.
func cut(title string) string {
	runes := []rune(title)
	if len(runes) <= titleLimit {
		return title
	}
	return string(runes[:titleLimit-1]) + "…"
}

// retitle gives the open pull request the item's title when it is still the
// one gh pr create --fill gave it, the subject of the first commit the push
// adds to the item's branch (first): a title a person edited is left as it is,
// and so is the description. failed, with the outcome reported, when gh pr
// edit does.
func (r pushRun) retitle(ctx context.Context, repository, current, first string) (a Answer, failed bool) {
	entry, err := r.ports.Show(ctx, first)
	if err != nil || entry.Subject != current {
		return Answer{}, false
	}
	title, _ := r.describe(r.branch)
	if title == current {
		return Answer{}, false
	}
	_, a, failed = r.gh(ctx, "pr", "edit", r.branch, "--repo", repository, "--title="+title)
	return a, failed
}
