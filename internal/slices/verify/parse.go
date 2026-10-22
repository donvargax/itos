package verify

import (
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
)

// CLI is verify's command line: the Query it asks, whether it prints
// --json's one object, and Tests, where the run's test listings warn.
type CLI struct {
	Query
	JSON  bool
	Tests *tests.Warnings
}

// Parse reads the arguments internal/cli's flag spec hands the verify
// command, with the global --json and the run's test warnings: <from> <to>,
// or none, which only a stealth config takes (Handle asks). One alone is a
// usage error.
func Parse(args []string, json bool, warnings *tests.Warnings) (CLI, error) {
	c := CLI{JSON: json, Tests: warnings}
	switch {
	case len(args) == 0:
		c.Bare = true
	case len(args) < 2:
		return CLI{}, needsRange()
	default:
		c.From, c.To = args[0], args[1]
	}
	return c, nil
}

// needsRange is the usage error of a range not given.
func needsRange() error {
	return out.Usage("verify needs <from> <to>")
}
