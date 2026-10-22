package commit

import (
	"context"
	"io"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
	"github.com/donvargax/itos/v8/internal/engine/refs"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/scope"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what commit asks: Sub, "" for a commit itself, whose Args are
// every argument of the command line, or check-message, check-paths or
// footers; check-message's File (from the top, "-" for stdin) and At;
// check-paths' Type and Files (from the top, with forward slashes);
// footers' Footer, From and To. Origin is the folder the person stood in,
// relative to the repository's top, where a commit's -F file is read and
// git runs; "" when itos runs where it was started.
type Query struct {
	Sub              string
	Args             []string
	File, At         string
	Type             string
	Files            []string
	Footer, From, To string
	Origin           string
}

// Ports are what commit reaches the world through, which internal/cli
// binds: the repository (the VCS port), whose refs a subcommand's must
// name, its git config and hooks folder (hooks.Ports, gitrun.Config),
// which hold itos's hooks and the notes' rewriting; Stealth, whether the config is
// the stealth one; LoadConfig, the config, loaded from the working tree;
// CommitConfig, the one a commit reads its flags by, nil with the error
// loading it gave as missing where there is no config file; and Read, a
// file of the working tree.
type Ports interface {
	vcsport.VCS
	hooks.Ports
	gitrun.Config
	Stealth(ctx context.Context) bool
	LoadConfig(ctx context.Context) (*config.Loaded, error)
	CommitConfig(ctx context.Context) (cfg *config.Loaded, missing, err error)
	Read(ctx context.Context, path string) (string, error)
}

// Handler is commit's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is where what Handle runs writes as it works, check-message's
// report and git's own words, and the global flags that shape it: --json,
// -q, and Tests, where the run's test listings warn.
type Out struct {
	Stdout, Stderr io.Writer
	JSON, Quiet    bool
	Tests          *tests.Warnings
}

// span is a range as --json names it.
type span struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Answer is what Handle found or did: Code, the exit it came to
// (check-message's, git's, or a refusal's); Problems, check-paths'
// rejections, judged by Rules, or the footers a commit lacks, refused with
// the config's Reject message before git ran; Footers, the footers of a
// range; and for a commit, Said, a line itos says on stderr instead of
// going on, and Commit, HEAD under --json once git made one.
type Answer struct {
	Code     int
	Problems []out.Problem
	Rules    *scope.Rules
	Reject   string
	Footers  []message.Said
	Said     string
	Commit   string
}

// Handle answers q, writing where o says as it runs: check-message's report
// and a commit's git.
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	switch q.Sub {
	case "check-message":
		return h.checkMessage(ctx, q, o)
	case "check-paths":
		return h.checkPaths(ctx, q)
	case "footers":
		return h.footers(ctx, q)
	}
	return h.commit(ctx, q, o)
}

// checkMessage is `commit check-message <file|-> [--at <sha>]` (commit.ts's
// checkMessage): one message through the header lint's delegate and the
// footer rules, as the commit-msg hook reads it. 0 when it passes, 1 when
// not. --at, whose ref must name a commit, reads the footers' IDs at that
// commit, as ITOS_AT does without it. Under a stealth config the footers
// are the ones ITOS_FOOTERS hands over, as itos commit hands them to the
// hook, never the message's.
func (h Handler) checkMessage(ctx context.Context, q Query, o Out) (Answer, error) {
	if err := refs.Check(ctx, h.ports, "commit check-message", q.At); err != nil {
		return Answer{}, err
	}
	var text string
	if q.File == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return Answer{}, err
		}
		text = string(raw)
	} else {
		var err error
		if text, err = h.ports.Read(ctx, q.File); err != nil {
			return Answer{}, err
		}
	}
	at := q.At
	if at == "" {
		at = os.Getenv("ITOS_AT")
	}
	cfg, err := h.ports.LoadConfig(ctx)
	if err != nil {
		return Answer{}, err
	}
	code, err := message.Check(cfg, text, message.Reading{At: at, Note: os.Getenv(message.FootersEnv), Warn: o.Stderr, Tests: o.Tests},
		message.Streams{JSON: o.JSON, Stdout: o.Stdout, Stderr: o.Stderr})
	return Answer{Code: code}, err
}

// checkPaths is `commit check-paths --type <t> <path>…` (commit-scope.ts's
// checkPaths): the path rules alone, for planning a split, and what they
// reject.
func (h Handler) checkPaths(ctx context.Context, q Query) (Answer, error) {
	cfg, err := h.ports.LoadConfig(ctx)
	if err != nil {
		return Answer{}, err
	}
	rules, err := scope.Of(cfg)
	if err != nil {
		return Answer{}, err
	}
	found, err := rules.Issues(q.Type, q.Files)
	if err != nil {
		return Answer{}, err
	}
	return Answer{Rules: rules, Problems: found}, nil
}

// footers is `commit footers <name> <from> <to>`: the range's footers of
// commits.footers.<name>, a footer of free text, each with the commit it
// came from, leaving out those that say none, for a release's notes. The
// range's refs must name commits; a usage error when <name> is not a
// footer of free text.
func (h Handler) footers(ctx context.Context, q Query) (Answer, error) {
	if err := refs.Check(ctx, h.ports, "commit footers", q.From, q.To); err != nil {
		return Answer{}, err
	}
	cfg, err := h.ports.LoadConfig(ctx)
	if err != nil {
		return Answer{}, err
	}
	if f, ok := cfg.Commits.Footers.Get(q.Footer); !ok || !f.Text() {
		return Answer{}, out.Usage("commit footers needs a footer of free text: commits.footers.%s is not one (source: text)", q.Footer)
	}
	said, err := message.Gathered(cfg, q.From, q.To, q.Footer)
	return Answer{Footers: said}, err
}
