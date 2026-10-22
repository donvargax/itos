package initcmd

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
)

// CLI is init's command line: the Query it asks; whether it prints --json's
// one object; and Tests, where the run's test listings warn.
type CLI struct {
	Query
	JSON  bool
	Tests *tests.Warnings
}

// projectSettings is the file Claude Code writes for the project scope,
// committed.
const projectSettings = ".claude/settings.json"

// pluginScopes are --plugin's answers.
var pluginScopes = []string{"project", "user", "local", "no"}

// PluginFlag is how init's command line answered the plugin's offer: Given
// is whether it has --plugin, Scope its value, "" for a bare one.
type PluginFlag struct {
	Given bool
	Scope string
}

// ShimFlag is how init's command line answered the git shim's offer: Given
// is whether it has --git-shim or --no-git-shim, Install which, Dir
// --git-shim-dir's folder made absolute where it was typed, "" for the
// folder holding itos.
type ShimFlag struct {
	Given   bool
	Install bool
	Dir     string
}

// RulesFlag is how init's command line answered the offer of the rules for
// agents: Given is whether it has --agent-rules or --no-agent-rules, Write
// which.
type RulesFlag struct {
	Given bool
	Write bool
}

// Parse reads the arguments after init, with the global --json, origin
// (the folder itos was started in, which --git-shim-dir and --policy are
// read from) and the run's test warnings: `init [--stealth] [--policy
// <file>] [--plugin [<scope>]] [--git-shim [--git-shim-dir <folder>] |
// --no-git-shim] [--agent-rules | --no-agent-rules]`. Every error is a
// usage error (out.Usage), but a --git-shim-dir that cannot be made
// absolute.
func Parse(args []string, origin string, json bool, warnings *tests.Warnings) (CLI, error) {
	c := CLI{Query: Query{Origin: origin}, JSON: json, Tests: warnings}
	q := &c.Query
	for i := 0; i < len(args); i++ {
		n, err := parsePluginFlag(args, i, &q.Plugin)
		if err == nil && n == 0 {
			n, err = parseShimFlag(args, i, &q.Shim, origin)
		}
		if err == nil && n == 0 {
			n, err = parseRulesFlag(args, i, &q.Rules)
		}
		switch {
		case err != nil:
			return c, err
		case n > 0:
			i += n - 1
		case args[i] == "--stealth":
			q.Stealth = true
		case args[i] == "--policy" && i+1 < len(args):
			q.Policy = args[i+1]
			i++
		default:
			return c, out.Usage("init takes only --stealth, --policy <file>, --plugin [<scope>], --git-shim, "+
				"--git-shim-dir <folder>, --no-git-shim, --agent-rules and --no-agent-rules (%s)", args[i])
		}
	}
	if q.Shim.Given && !q.Shim.Install && q.Shim.Dir != "" {
		return c, out.Usage("--git-shim-dir names where --git-shim links itos as git, not --no-git-shim")
	}
	if q.Stealth && q.Plugin.Scope == "project" {
		return c, pluginRefused()
	}
	return c, nil
}

// parsePluginFlag reads --plugin out of init's arguments at i: --plugin
// <scope>, --plugin=<scope>, or a bare --plugin, the next argument being a
// flag or none. It gives how many arguments it read, 0 when args[i] is not
// --plugin.
func parsePluginFlag(args []string, i int, f *PluginFlag) (int, error) {
	a := args[i]
	if value, ok := strings.CutPrefix(a, "--plugin="); ok {
		if !slices.Contains(pluginScopes, value) {
			return 0, out.Usage("--plugin takes project, user, local or no (%s)", value)
		}
		*f = PluginFlag{true, value}
		return 1, nil
	}
	if a != "--plugin" {
		return 0, nil
	}
	*f = PluginFlag{Given: true}
	if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
		return 1, nil
	}
	if !slices.Contains(pluginScopes, args[i+1]) {
		return 0, out.Usage("--plugin takes project, user, local or no (%s)", args[i+1])
	}
	f.Scope = args[i+1]
	return 2, nil
}

// pluginRefused is the usage error of --plugin project under --stealth.
func pluginRefused() error {
	return out.Usage("under --stealth, --plugin project would write %s, which the project commits; "+
		"--plugin local installs it for you alone in this repository, --plugin user for every one of yours", projectSettings)
}

// parseShimFlag reads --git-shim, --no-git-shim or --git-shim-dir <folder>
// (--git-shim-dir=<folder>) out of init's arguments at i, and gives how many
// arguments it read, 0 when args[i] is none of them.
func parseShimFlag(args []string, i int, f *ShimFlag, origin string) (int, error) {
	a := args[i]
	switch a {
	case "--git-shim", "--no-git-shim":
		install := a == "--git-shim"
		if f.Given && f.Install != install {
			return 0, out.Usage("init takes --git-shim or --no-git-shim, not both")
		}
		f.Given, f.Install = true, install
		return 1, nil
	}
	folder, ok := strings.CutPrefix(a, "--git-shim-dir=")
	n := 1
	if !ok {
		if a != "--git-shim-dir" {
			return 0, nil
		}
		if i+1 >= len(args) {
			return 0, out.Usage("--git-shim-dir needs <folder>")
		}
		folder, n = args[i+1], 2
	}
	if folder == "" {
		return 0, out.Usage("--git-shim-dir needs <folder>")
	}
	dir, err := filepath.Abs(typed(origin, folder))
	if err != nil {
		return 0, err
	}
	f.Dir = dir
	return n, nil
}

// parseRulesFlag reads --agent-rules or --no-agent-rules out of init's
// arguments at i, and gives how many arguments it read, 0 when args[i] is
// neither.
func parseRulesFlag(args []string, i int, f *RulesFlag) (int, error) {
	a := args[i]
	if a != "--agent-rules" && a != "--no-agent-rules" {
		return 0, nil
	}
	write := a == "--agent-rules"
	if f.Given && f.Write != write {
		return 0, out.Usage("init takes --agent-rules or --no-agent-rules, not both")
	}
	f.Given, f.Write = true, write
	return 1, nil
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
