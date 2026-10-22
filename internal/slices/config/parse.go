package config

import (
	"path/filepath"
	"slices"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
)

// CLI is the config command's line: the Query it asks; whether it prints
// --json's one object, or under --quiet no line when check finds nothing;
// and Tests, where the run's test listings warn.
type CLI struct {
	Query
	JSON, Quiet bool
	Tests       *tests.Warnings
}

// Parse reads the arguments internal/cli's flag spec hands the config
// command, with the global flags, origin (the folder itos was started in,
// which check's --ledger is read from) and the run's test warnings: get and
// its one key, or check, its --print-defaults and --ledger. Every error is a
// usage error (out.Usage).
func Parse(args []string, origin string, json, quiet bool, warnings *tests.Warnings) (CLI, error) {
	c := CLI{JSON: json, Quiet: quiet, Tests: warnings}
	sub, rest := argsengine.Split(args)
	if sub == "get" {
		keys := argsengine.Positional(rest)
		if len(keys) != 1 || keys[0] == "" {
			return c, out.Usage("config get needs one <key>, a dotted path such as hooks.bin")
		}
		c.Query = Query{Sub: sub, Key: keys[0]}
		return c, nil
	}
	if sub != "check" {
		return c, out.Usage("unknown command: config %s", sub)
	}
	ledger, _ := argsengine.FlagValue(args, "--ledger")
	c.Query = Query{Sub: sub, Defaults: slices.Contains(args, "--print-defaults"), Ledger: typed(origin, ledger)}
	return c, nil
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from origin, the subfolder they stood in (Out.Origin): relative to it, as
// git reads a path typed there. An absolute path, "-" (stdin) and every path
// of a run that did not move stay as typed.
func typed(origin, p string) string {
	if origin == "" || p == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}
