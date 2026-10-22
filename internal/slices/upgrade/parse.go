package upgrade

import (
	"slices"
	"strings"

	pinengine "github.com/donvargax/itos/v8/internal/engine/pin"
)

// CLI is upgrade's command line: the Query it asks, and whether it prints
// the one object --json asks for.
type CLI struct {
	Query
	JSON bool
}

// Parse reads the arguments after upgrade, with the global --json: one
// <version> at most, a leading v dropped, none or latest the newest. Every
// error is a usage error (out.Usage).
func Parse(args []string, json bool) (CLI, error) {
	want, err := pinengine.VersionArg("upgrade", args)
	return CLI{Query: Query{Want: want}, JSON: json}, err
}

// NamingVersion is an upgrade's command line, its global flags anywhere,
// with v in place of its version, or after the command word when it names
// none: what the launcher hands the version it moves to (UpgradeTarget in
// internal/cli, slice 126).
func NamingVersion(args []string, v string) []string {
	named := slices.Clone(args)
	command := -1
	for i := 0; i < len(named); i++ {
		switch a := named[i]; {
		case a == "--config" || a == "--root":
			i++
		case strings.HasPrefix(a, "-"):
		case command < 0:
			command = i
		default:
			named[i] = v
			return named
		}
	}
	return slices.Insert(named, command+1, v)
}
