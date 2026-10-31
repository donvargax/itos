package guide

import (
	_ "embed"
	"maps"
	"slices"
	"strings"
)

//go:embed coordinate.md
var coordinate string

//go:embed work.md
var work string

//go:embed brief.md
var brief string

//go:embed land.md
var land string

//go:embed repair.md
var repair string

//go:embed decide.md
var decide string

//go:embed commit.md
var commit string

//go:embed push.md
var push string

//go:embed drafter.md
var drafter string

//go:embed reviewer.md
var reviewer string

// Coordinate is the name of the coordinator's guide, the one itos go prints.
const Coordinate = "coordinate"

// Commit is the name of the procedure read before committing, the one the
// config's commit policy is appended to.
const Commit = "commit"

var guides = map[string]string{
	Coordinate: coordinate,
	"work":     work,
}

// procedures are read before a step, each named in its guide's map: the
// coordinator's brief, land, repair and decide, the implementer's commit
// and push.
var procedures = map[string]string{
	"brief":  brief,
	"land":   land,
	"repair": repair,
	"decide": decide,
	Commit:   commit,
	"push":   push,
}

// Text is the guide or the procedure by its name, as written, and whether
// there is one. Its lines end in LF wherever the binary was built: go:embed
// takes a file's bytes as the checkout has them, and a checkout that writes
// text with CRLF (git's core.autocrlf, as on a Windows runner) would embed it
// so.
func Text(name string) (string, bool) {
	text, ok := guides[name]
	if !ok {
		text, ok = procedures[name]
	}
	return lf(text), ok
}

// lf is a text with its lines ending in LF, whatever the checkout wrote.
func lf(text string) string { return strings.ReplaceAll(text, "\r\n", "\n") }

// The roles itos brief fills a template for (slice-204): the drafter, who
// specifies an item, the implementer, who builds it, and the reviewer, who
// judges the work.
const (
	Drafter     = "drafter"
	Implementer = "implementer"
	Reviewer    = "reviewer"
)

// Roles are the roles a template is shipped for, in the order a brief
// names them.
var Roles = []string{Drafter, Implementer, Reviewer}

// templates are the drafter's and the reviewer's, shipped beside the
// procedures and no guide names: itos guide prints neither.
var templates = map[string]string{
	Drafter:  drafter,
	Reviewer: reviewer,
}

// Template is the role's brief template, the paragraphs itos brief fills
// from an item, as written and with its lines ending in LF (Text), and
// whether the role has one. The implementer's is the brief procedure's own,
// the quoted block of its "## The brief" section less the quote marks, so
// the one itos guide brief prints to a person and the one itos brief fills
// are the same text.
func Template(role string) (string, bool) {
	if role == Implementer {
		return quoted(lf(brief)), true
	}
	text, ok := templates[role]
	return lf(text), ok
}

// quoted is the quoted block of the text's "## The brief" section, its
// lines less their quote marks.
func quoted(text string) string {
	_, section, _ := strings.Cut(text, "\n## The brief\n")
	var lines []string
	for _, line := range strings.Split(section, "\n") {
		if rest, ok := strings.CutPrefix(line, ">"); ok {
			lines = append(lines, strings.TrimPrefix(rest, " "))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// IsProcedure is whether the name is a procedure's, not a guide's.
func IsProcedure(name string) bool {
	_, ok := procedures[name]
	return ok
}

// Names are the guides' names, sorted.
func Names() []string { return slices.Sorted(maps.Keys(guides)) }

// Procedures are the procedures' names, sorted.
func Procedures() []string { return slices.Sorted(maps.Keys(procedures)) }
