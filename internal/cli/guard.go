package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/donvargax/itos/v7/internal/guard"
)

// GuardSince is the first itos with guard claude-code (v6.0.0, decision 36;
// guard claude-code before it): the launcher answers nothing for the guard
// where it would hand it to an older itos, which has no such command, and
// whose usage error's exit 2 Claude Code would take as a block (slice 44,
// internal/launch).
const GuardSince = "6.0.0"

// guardCommand is `guard <harness>`, a harness's guard of an agent's git
// commit and git push: guard claude-code alone so far.
func guardCommand(args []string, o Out) (int, error) {
	sub, _ := split(args)
	if sub == "claude-code" {
		return guardClaudeCode(o)
	}
	return 0, usage("unknown command: guard %s", sub)
}

// guardClaudeCode is `guard claude-code`, Claude Code's PreToolUse hook
// (internal/guard): Claude Code's input on stdin, a deny on stdout for a git
// commit or git push in a repository itos manages, nothing for anything else,
// exit 0 either way. Claude Code blocks the tool on exit 2 alone and goes on
// after any other failure, so an input it cannot read is exit 1, its reason
// on stderr, and never an error the command line would report as a usage
// error's 2. The folder judged when the input names none, and the one a
// relative one is read from, is where itos was started, before a run from a
// subfolder moved to the top.
func guardClaudeCode(o Out) (int, error) {
	in, err := guard.Read(os.Stdin)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: guard claude-code cannot read Claude Code's %s input on stdin: %s\n", guard.Event, err)
		return ExitPolicy, nil
	}
	here, err := filepath.Abs(typed("."))
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: guard claude-code cannot tell the folder it runs in: %s\n", err)
		return ExitPolicy, nil
	}
	reason, deny := guard.Reason(in, here)
	if !deny {
		return 0, nil
	}
	if err := guard.Deny(o.Stdout, reason); err != nil {
		fmt.Fprintf(o.Stderr, "itos: guard claude-code cannot write its %s answer: %s\n", guard.Event, err)
		return ExitPolicy, nil
	}
	return 0, nil
}
