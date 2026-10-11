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
	return strings.ReplaceAll(text, "\r\n", "\n"), ok
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
