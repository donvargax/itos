package hook

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/scope"
	"github.com/donvargax/itos/v8/internal/shell"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what hook asks: Sub, commit-msg, pre-push or install;
// commit-msg's File, the message git wrote (from the top); pre-push's Remote,
// the remote git pushes to, a name or an address; install's Print and Force.
type Query struct {
	Sub          string
	File         string
	Remote       string
	Print, Force bool
}

// Ports are what hook reaches the world through, which internal/cli binds:
// the repository (the VCS port), its git config and hooks folder
// (hooks.Ports), where hook install puts itos's hooks; Trees, the trees it holds (the tree
// port), the index the commit-msg hook reads its data from; LoadConfig, the
// config, loaded from the working tree; Read, a file of the working tree;
// TaskFlags, the flags ledger.group.label may not name, as config check
// judges them; and Shell, a command run through cfg's shell, a staged range
// command's and hooks.pre_push's.
type Ports interface {
	vcsport.VCS
	hooks.Ports
	Trees(ctx context.Context) treeport.Trees
	LoadConfig(ctx context.Context) (*config.Loaded, error)
	// RemoteHead is the branch the remote's HEAD names, asked of the remote
	// (git ls-remote --symref); "" when it names none.
	RemoteHead(ctx context.Context, remote string, talk vcsport.Talk) (string, error)
	Read(ctx context.Context, path string) (string, error)
	TaskFlags(ctx context.Context) []string
	Shell(ctx context.Context, cfg *config.Loaded, command string, o shell.Options) shell.Result
	CaptureIndex(ctx context.Context) (treeport.Source, error)
	TaskSnapshot(ctx context.Context, tree string) (Snapshot, error)
	// CheckEnv is the environment a check that runs in the working folder
	// (snapshot: false) runs in: the hook's own, less what belongs to this
	// commit alone.
	CheckEnv(ctx context.Context) []string
	// Evidence is the engine that looks up the checks that opt in to evidence
	// records by tree, the tree the hook captured, and runs a miss in a
	// checkout of its own. Its Hook is set.
	Evidence(ctx context.Context, cfg *config.Loaded, tree string) *evidence.Engine
}

// Snapshot is a private checkout and capture directory, with a Git-local
// environment removed and a release function for every exit from task checking.
type Snapshot struct {
	Dir, TempDir string
	Env          []string
	Remove       func() error
}

// Handler is hook's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is where what Handle runs writes as it works, the header lint's
// report, the tasks' checks, hooks.pre_push's commands and hook install's
// entries, and the global flags that shape it: --json, and Tests, where the
// run's test listings warn.
type Out struct {
	Stdout, Stderr io.Writer
	JSON           bool
	Tests          *tests.Warnings
}

// Answer is what Handle came to: Code, the hook's exit; and the rejection
// Run prints on stderr, if any: Problems, judged by Rules, as the path
// rules reject them; else Lines, each a problem under the Header line; else
// Said, written as it is.
type Answer struct {
	Code     int
	Rules    *scope.Rules
	Problems []out.Problem
	Header   string
	Lines    []string
	Said     string
}

// Handle answers q, writing where o says as it runs.
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	switch q.Sub {
	case "commit-msg":
		return h.commitMsg(ctx, q.File, o)
	case "pre-push":
		return h.prePush(ctx, q.Remote, o)
	}
	return h.install(ctx, q, o)
}

// install is `hook install`: 0 when both hooks are declared, or on a git
// that runs no hook its config declares written as hook files (or
// printed), 1 when an entry under itos's name that does not call itos stood
// in the way and --force was not given, or a hooks folder inside the work
// tree (hooks.Declare). Its lines go to stdout, to stderr under --json.
func (h Handler) install(ctx context.Context, q Query, o Out) (Answer, error) {
	cfg, err := h.ports.LoadConfig(ctx)
	if err != nil {
		return Answer{}, err
	}
	log := o.Stdout
	if o.JSON {
		log = o.Stderr
	}
	code, err := hooks.Declare(ctx, h.ports, cfg.Hooks.Bin, q.Print, q.Force,
		hooks.Out{Stdout: o.Stdout, Stderr: o.Stderr, JSON: o.JSON}, func(line string) { fmt.Fprintln(log, line) })
	return Answer{Code: code}, err
}
