package initcmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
// runs, writing to stdout, its object under json, and its exit code; Abs,
// a path made absolute from the folder itos runs in (the VCS's Folder),
// and WriteFile, MkdirAll and WriteText, os.WriteFile, os.MkdirAll and
// agentrules.WriteText of a path read from there, ports so a test makes
// them fail, as no failure of theirs comes alike from every OS; and At, the
// ports of the same run in the folder dir, the top init moves to.
//
// The offers reach the rest: Terminal, whether a person is at the other
// end, stdin and stdout both a terminal, and Stdin, where a terminal's
// answers are read from; LookPath, exec.LookPath, which finds claude and
// the first git on the PATH, and Command, a program run with its stdout and
// stderr together, its error an *exec.ExitError when it exits other than 0;
// and git-shim install's ShimPlace (the itos running and the link to it in
// dir, "" for its own folder), LinksItos (whether a path links to this
// itos, or is another itos), MakeLink, PathStanding (where a
// folder and the real git stand on the PATH, -1 when not there) and
// StandingLine (what that means for the shim).
type Ports interface {
	vcsport.VCS
	hooks.Git
	Trees(ctx context.Context) treeport.Trees
	Worktree(ctx context.Context) treeport.Source
	TaskFlags(ctx context.Context) []string
	Release(ctx context.Context, want string) (string, []byte, error)
	InstallHooks(ctx context.Context, stdout io.Writer, json bool) (int, error)
	Abs(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, name string, data []byte, perm os.FileMode) error
	MkdirAll(ctx context.Context, path string, perm os.FileMode) error
	WriteText(ctx context.Context, path, text string, crlf bool) error
	Terminal(ctx context.Context) bool
	Stdin(ctx context.Context) io.Reader
	LookPath(ctx context.Context, file string) (string, error)
	Command(ctx context.Context, name string, args ...string) (string, error)
	ShimPlace(ctx context.Context, dir string) (self, link string, err error)
	IsItos(ctx context.Context, path string) bool
	LinksItos(ctx context.Context, link string) (self, other bool)
	MakeLink(ctx context.Context, self, link string, replace bool) error
	PathStanding(ctx context.Context, dir string) (at int, realGit string, realAt int)
	StandingLine(ctx context.Context, dir string, at int, realGit string, realAt int) string
	At(ctx context.Context, dir string) Ports
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
	Plugin        pluginOutcome
	Shim          shimOutcome
	Rules         rulesOutcome
	Code          int
	ProjectPlugin bool
	RulesWritten  []string
}

// makeOffers makes the plugin's offer, the git shim's and that of the rules
// for agents, in that order, a terminal asked where one is and the offer
// may ask, through one reader, so the first cannot take the second's answer
// into its buffer.
func (h Handler) makeOffers(ctx context.Context, o Offer, log io.Writer) Offered {
	ask, answers := o.Ask && h.ports.Terminal(ctx), bufio.NewReader(h.ports.Stdin(ctx))
	plugin, pluginCode := pluginOffer{flag: o.Plugin, stealth: o.Stealth, ask: ask,
		log: log, answers: answers, ports: h.ports}.run(ctx)
	shim, shimCode := shimOffer{flag: o.Shim, ask: ask, log: log,
		answers: answers, ports: h.ports}.run(ctx)
	rules, rulesCode := rulesOffer{flag: o.Rules, stealth: o.Stealth, rerun: o.Rerun,
		ask: ask, log: log, answers: answers, ports: h.ports}.run(ctx)
	offered := Offered{Plugin: plugin, Shim: shim, Rules: rules, Code: max(pluginCode, shimCode, rulesCode),
		ProjectPlugin: plugin.Action == "installed" && plugin.Scope == "project"}
	for _, f := range rules.Files {
		if f.Action != "kept" {
			offered.RulesWritten = append(offered.RulesWritten, f.Path)
		}
	}
	return offered
}

// Handler is init's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// at is the handler over the ports in the folder dir (Ports.At).
func (h Handler) at(ctx context.Context, dir string) Handler {
	return Handler{ports: h.ports.At(ctx, dir)}
}

// path is the path p read from the folder itos runs in.
func (h Handler) path(ctx context.Context, p string) string {
	return vcsport.Within(ctx, h.ports, p)
}

// named is err naming the file p as it was given, not as it was read from
// the folder itos runs in.
func named(err error, p string) error {
	var failed *fs.PathError
	if errors.As(err, &failed) {
		failed.Path = p
	}
	return err
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
	h, initialized, err := h.atTop(ctx, log)
	if err != nil {
		return 0, err
	}
	if file := h.configPath(ctx); exists(h.path(ctx, file)) {
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

// atTop is the handler at the top of the repository the folder is in, after
// git init when it is in none, and says whether it ran git init.
func (h Handler) atTop(ctx context.Context, log io.Writer) (Handler, bool, error) {
	h, inRepo, err := h.toTop(ctx)
	if err != nil || inRepo {
		return h, false, err
	}
	return h, true, h.gitInit(ctx, log)
}

// toTop is the handler at the top of the repository the folder is in, and
// whether it is in one; in a git folder it is a usage error.
func (h Handler) toTop(ctx context.Context) (Handler, bool, error) {
	l, err := h.ports.Locate(ctx, "")
	if err != nil {
		return h, false, nil
	}
	if l.Top == "" {
		return h, false, kind.Wrap(kind.Usage, errors.New("init runs in a repository's working tree, not in its git folder"))
	}
	return h.at(ctx, l.Top), true, nil
}

// gitInit makes the folder itos runs in a git repository, and says so; the
// git adapter locates it afresh from then on.
func (h Handler) gitInit(ctx context.Context, log io.Writer) error {
	if err := h.ports.Init(ctx); err != nil {
		return err
	}
	fmt.Fprintf(log, "Ran git init: %s is a git repository now.\n", h.ports.Folder(ctx))
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
