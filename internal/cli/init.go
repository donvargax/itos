package cli

import (
	"bufio"
	"context"
	"io"
	"os"

	initslice "github.com/donvargax/itos/v8/internal/slices/initcmd"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runInit is `init [--stealth] [--policy <file>] [--plugin [<scope>]]
// [--git-shim [--git-shim-dir <folder>] | --no-git-shim] [--agent-rules |
// --no-agent-rules]`, the init slice (internal/slices/initcmd) over its
// ports bound to the real code; --git-shim-dir and --policy are read from
// where itos was started.
func runInit(args []string, o Out) (int, error) {
	c, err := initslice.Parse(args, o.Origin, o.JSON, o.Tests)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, initslice.NewHandler(initPorts{o: o}))
}

// initPorts are the init slice's ports over the real code: git, the trees
// over it, the working tree, the flags of itos task that name no group,
// pin's release, hook install and the offers, with o.
type initPorts struct {
	vcs.Git
	o Out
}

// Trees is the tree adapter over git.
func (initPorts) Trees(context.Context) port.Trees { return trees.Trees{VCS: vcs.Git{}} }

// Worktree is the working tree's files.
func (initPorts) Worktree(context.Context) port.Source { return trees.Worktree }

// TaskFlags is ungroupedTaskFlags.
func (initPorts) TaskFlags(context.Context) []string { return ungroupedTaskFlags() }

// Release is pinned.
func (initPorts) Release(_ context.Context, want string) (string, []byte, error) { return pinned(want) }

// InstallHooks is hookInstall, writing to stdout, its object under json.
func (p initPorts) InstallHooks(_ context.Context, stdout io.Writer, json bool) (int, error) {
	o := p.o
	o.JSON, o.Stdout = json, stdout
	return hookInstall(false, false, o)
}

// Offers makes the plugin's offer, the git shim's and that of the rules for
// agents, in that order (initplugin.go, initshim.go, initrules.go), a
// terminal asked where one is and the offer may ask, through one reader, so
// the first cannot take the second's answer into its buffer.
func (p initPorts) Offers(_ context.Context, o initslice.Offer, log io.Writer) initslice.Offered {
	ask, answers := o.Ask && onTerminal(), bufio.NewReader(os.Stdin)
	plugin, pluginCode := pluginOffer{flag: pluginFlag{o.Plugin.Given, o.Plugin.Scope}, stealth: o.Stealth, ask: ask,
		log: log, answers: answers}.run()
	shim, shimCode := shimOffer{flag: shimFlag{o.Shim.Given, o.Shim.Install, o.Shim.Dir}, ask: ask, log: log,
		answers: answers, goos: p.o.OS}.run()
	rules, rulesCode := rulesOffer{flag: rulesFlag{o.Rules.Given, o.Rules.Write}, stealth: o.Stealth, rerun: o.Rerun,
		ask: ask, log: log, answers: answers}.run()
	offered := initslice.Offered{Plugin: plugin, Shim: shim, Rules: rules, Code: max(pluginCode, shimCode, rulesCode),
		ProjectPlugin: plugin.Action == "installed" && plugin.Scope == "project"}
	for _, f := range rules.Files {
		if f.Action != "kept" {
			offered.RulesWritten = append(offered.RulesWritten, f.Path)
		}
	}
	return offered
}

// writtenFile is a file init wrote, or found there and kept.
type writtenFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}
