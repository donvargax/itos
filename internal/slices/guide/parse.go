package guide

import (
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	guidetext "github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the guide command's line: the Query it asks, and whether it prints
// --json's one object.
type CLI struct {
	Query
	JSON bool
}

// Parse reads the arguments internal/cli's flag spec hands the guide
// command, with the global --json: one guide's or procedure's name. Every
// error is a usage error (out.Usage) naming the guides, then the
// procedures, given before anything is read.
func Parse(args []string, json bool) (CLI, error) {
	known := "the guides are " + strings.Join(guidetext.Names(), ", ") +
		"; the procedures are " + strings.Join(guidetext.Procedures(), ", ")
	name, rest := argsengine.Split(args)
	_, ok := guidetext.Text(name)
	switch {
	case name == "":
		return CLI{}, out.Usage("guide needs a name: %s", known)
	case !ok:
		return CLI{}, out.Usage("no guide or procedure %s: %s", name, known)
	case len(rest) > 0 && guidetext.IsProcedure(name):
		return CLI{}, out.Usage("guide %s takes no argument: %s", name, strings.Join(rest, " "))
	case len(rest) > 0:
		return CLI{}, out.Usage("guide takes one name: %s", strings.Join(args, " "))
	}
	return CLI{Query: Query{Name: name}, JSON: json}, nil
}
