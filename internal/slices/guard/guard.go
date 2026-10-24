package guard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/slices/guard/domain"
)

// refused is the policy failure's exit code (itos --help), which Claude Code
// does not take as a block.
const refused = 1

// CLI is `guard <harness>`'s command line: the harness, claude-code alone so
// far, and the folder itos was started in, absolute, which a relative input
// folder is read from and which is judged when the input names none.
type CLI struct {
	Harness string
	Started string
}

// Parse reads the arguments internal/cli's flag spec hands the guard
// command, started being the folder itos was started in; a harness it does
// not know is an error the command line reports as a usage error.
func Parse(args []string, started string) (CLI, error) {
	harness := ""
	if len(args) > 0 {
		harness = args[0]
	}
	if harness != "claude-code" {
		return CLI{}, errors.New(strings.TrimSpace("unknown command: guard " + harness))
	}
	return CLI{Harness: harness, Started: started}, nil
}

// Query is what the guard asks of one tool call: Claude Code's input, and
// the absolute folder a relative input folder and -C are read from.
type Query struct {
	Input domain.Input
	Here  string
}

// Ports are what the guard reaches the world through, which internal/cli
// binds: Managed, whether itos manages the repository the folder dir is in.
type Ports interface {
	Managed(ctx context.Context, dir string) bool
}

// Handler is the guard's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Handle is the deny's reason for q, and whether there is one, the domain
// asking the ports, under ctx, whether each call's folder is managed.
func (h Handler) Handle(ctx context.Context, q Query) (string, bool) {
	return domain.Reason(q.Input, q.Here, func(dir string) bool { return h.ports.Managed(ctx, dir) })
}

// Run is `guard claude-code`, Claude Code's PreToolUse hook (the domain):
// Claude Code's input on stdin, a deny on stdout for a git commit or git
// push in a repository itos manages, nothing for anything else, exit 0
// either way. Claude Code blocks the tool on exit 2 alone and goes on after
// any other failure, so an input it cannot read is exit 1, its reason on
// stderr, and never an error the command line would report as a usage
// error's 2.
func (c CLI) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, h Handler) int {
	in, err := domain.Read(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "itos: guard claude-code cannot read Claude Code's %s input on stdin: %s\n", domain.Event, err)
		return refused
	}
	reason, deny := h.Handle(ctx, Query{Input: in, Here: c.Started})
	if !deny {
		return 0
	}
	if err := domain.Deny(stdout, reason); err != nil {
		fmt.Fprintf(stderr, "itos: guard claude-code cannot write its %s answer: %s\n", domain.Event, err)
		return refused
	}
	return 0
}
