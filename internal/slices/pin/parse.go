package pin

import pinengine "github.com/donvargax/itos/v8/internal/engine/pin"

// CLI is pin's command line: the Query it asks, and whether it prints the
// one object --json asks for.
type CLI struct {
	Query
	JSON bool
}

// Parse reads the arguments after pin, with the global --json: one
// <version> at most, a leading v dropped, none or latest the newest. Every
// error is a usage error (out.Usage).
func Parse(args []string, json bool) (CLI, error) {
	want, err := pinengine.VersionArg("pin", args)
	return CLI{Query: Query{Want: want}, JSON: json}, err
}
