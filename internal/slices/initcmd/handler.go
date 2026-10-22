package initcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what init asks: Stealth, the config in the git folder; Policy,
// the file whose policy the project takes, "" for none; the answers to the
// three offers, Plugin, Shim and Rules; and Origin, the folder itos was
// started in, which the policy is read from.
type Query struct {
	Stealth bool
	Policy  string
	Plugin  PluginFlag
	Shim    ShimFlag
	Rules   RulesFlag
	Origin  string
}

// Ports are what init reaches the world through, which internal/cli binds:
// the repository (the VCS port), which it is run in or git init makes, and
// its git config (hooks.Git), which declares itos's hooks; Trees, the trees
// itos reads its data from, and Worktree, the working tree, where the
// config and its data are read; TaskFlags, the flags of itos task that
// name no group, which internal/cli's flag spec holds; Release, the release
// to pin, the version wanted or with "" the newest, and its checksums.txt
// (pinengine.Pinned); InstallHooks, hook install run as itos hook install
// runs, writing to stdout, its object under json, and its exit code;
// Offers, the offers of the plugin, the git shim and the rules for agents,
// each saying what it does on log; and Abs and WriteFile, filepath.Abs and
// os.WriteFile, ports so a test makes them fail, as no failure of theirs
// comes alike from every OS.
type Ports interface {
	vcsport.VCS
	hooks.Git
	Trees(ctx context.Context) treeport.Trees
	Worktree(ctx context.Context) treeport.Source
	TaskFlags(ctx context.Context) []string
	Release(ctx context.Context, want string) (string, []byte, error)
	InstallHooks(ctx context.Context, stdout io.Writer, json bool) (int, error)
	Offers(ctx context.Context, o Offer, log io.Writer) Offered
	Abs(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, name string, data []byte, perm os.FileMode) error
}

// Offer is what the offers are told: the command line's answers, whether
// the config is the stealth one, whether this is a rerun where a config was
// there, and whether a terminal may be asked (a first run, without
// --json), where one is.
type Offer struct {
	Plugin  PluginFlag
	Shim    ShimFlag
	Rules   RulesFlag
	Stealth bool
	Rerun   bool
	Ask     bool
}

// Offered is what came of the offers: the plugin's, the git shim's and the
// rules', as --json prints them; Code, the highest of their exit codes;
// whether the plugin was installed at the project's scope, ProjectPlugin;
// and the files of the rules written or updated, RulesWritten.
type Offered struct {
	Plugin, Shim, Rules any
	Code                int
	ProjectPlugin       bool
	RulesWritten        []string
}

// Handler is init's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is where init writes as it works, and the global flags that shape it:
// --json, and Tests, where the run's test listings warn.
type Out struct {
	Stdout, Stderr io.Writer
	JSON           bool
	Tests          *tests.Warnings
}

// log is where init says what it does: stdout, or stderr under --json.
func (o Out) log() io.Writer {
	if o.JSON {
		return o.Stderr
	}
	return o.Stdout
}

// Answer is what Handle came to: its exit Code.
type Answer struct {
	Code int
}

// Handle readies the repository for itos, or with a config there reports
// what is missing; with --policy, policyInit (policy.go).
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	code, err := h.handle(ctx, q, o)
	return Answer{Code: code}, err
}

func (h Handler) handle(ctx context.Context, q Query, o Out) (int, error) {
	stealth, log := q.Stealth, o.log()
	if q.Policy != "" {
		return h.policyInit(ctx, q, log, o)
	}
	initialized, err := h.atTop(ctx, log)
	if err != nil {
		return 0, err
	}
	if file := h.configPath(ctx); exists(file) {
		// A stealth config is for a clone that cannot change the project:
		// where the project has its own, --stealth would only copy what it
		// commits into files that go stale unreported (bug 43).
		if stealth && !config.IsStealth(h.ports, file) {
			return 0, out.Usage("--stealth writes a config in the git folder for a clone that cannot change the project, "+
				"but this one has its own %s; run init without --stealth", file)
		}
		stealth = stealth || config.IsStealth(h.ports, file)
		if stealth && q.Plugin.Scope == "project" {
			return 0, pluginRefused()
		}
		return h.report(ctx, file, Offer{Plugin: q.Plugin, Shim: q.Shim, Rules: q.Rules, Stealth: stealth, Rerun: true}, o)
	}
	if named := os.Getenv("ITOS_CONFIG"); named != "" {
		return 0, out.Usage("init writes itos.yaml at the repository's top, or with --stealth in the git folder, "+
			"not the missing %s that --config or ITOS_CONFIG names", named)
	}
	return h.write(ctx, stealth, initialized, h.offer(q, o), log, o)
}

// offer is the offers' context on a first run: the command line's answers,
// and a terminal asked where there is one, but under --json.
func (h Handler) offer(q Query, o Out) Offer {
	return Offer{Plugin: q.Plugin, Shim: q.Shim, Rules: q.Rules, Stealth: q.Stealth, Ask: !o.JSON}
}

// configPath is the config's path, as the working tree has it.
func (h Handler) configPath(ctx context.Context) string {
	return config.Path(h.ports, h.ports.Worktree(ctx))
}

// loadConfigFile is the config at file, it and its data read from the
// working tree.
func (h Handler) loadConfigFile(ctx context.Context, file string) (*config.Loaded, error) {
	return config.Load(h.ports, h.ports.Trees(ctx), h.ports.Worktree(ctx), file)
}

// fromText is the config text would be at file, its data read from the
// working tree.
func (h Handler) fromText(ctx context.Context, file, text string, stealth bool) (*config.Loaded, error) {
	return config.FromText(h.ports, h.ports.Trees(ctx), h.ports.Worktree(ctx), file, text, stealth)
}

// atTop moves to the top of the repository the folder is in, after git init
// when it is in none, and says whether it ran git init.
func (h Handler) atTop(ctx context.Context, log io.Writer) (bool, error) {
	inRepo, err := h.toTop(ctx)
	if err != nil || inRepo {
		return false, err
	}
	return true, h.gitInit(ctx, log)
}

// toTop moves to the top of the repository the folder is in, and says
// whether it is in one; in a git folder it is a usage error.
func (h Handler) toTop(ctx context.Context) (bool, error) {
	l, err := h.ports.Locate(ctx, "")
	if err != nil {
		return false, nil
	}
	if l.Top == "" {
		return false, kind.Wrap(kind.Usage, errors.New("init runs in a repository's working tree, not in its git folder"))
	}
	return true, os.Chdir(l.Top)
}

// gitInit makes the folder a git repository, and says so; the git adapter
// locates it afresh from then on.
func (h Handler) gitInit(ctx context.Context, log io.Writer) error {
	if err := h.ports.Init(ctx); err != nil {
		return err
	}
	here, _ := os.Getwd()
	fmt.Fprintf(log, "Ran git init: %s is a git repository now.\n", here)
	return nil
}

// writtenFile is a file init wrote, or found there and kept.
type writtenFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// exists is whether there is a file or a folder at p.
func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
