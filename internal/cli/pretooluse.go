package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/donvargax/itos/v5/internal/guard"
)

// GuardSince is the first itos with hook pre-tool-use: the launcher answers
// nothing for the hook where it would hand it to an older itos, whose usage
// error's exit 2 Claude Code would take as a block (slice 44,
// internal/launch).
const GuardSince = "2.3.0"

// hookPreToolUse is `hook pre-tool-use`, Claude Code's PreToolUse hook
// (internal/guard): Claude Code's input on stdin, a deny on stdout for a git
// commit or git push in a repository itos manages, nothing for anything else,
// exit 0 either way. Claude Code blocks the tool on exit 2 alone and goes on
// after any other failure, so an input it cannot read is exit 1, its reason
// on stderr, and never an error the command line would report as a usage
// error's 2. The folder judged when the input names none, and the one a
// relative one is read from, is where itos was started, before a run from a
// subfolder moved to the top.
func hookPreToolUse(o Out) (int, error) {
	in, err := guard.Read(os.Stdin)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: hook pre-tool-use cannot read Claude Code's %s input on stdin: %s\n", guard.Event, err)
		return ExitPolicy, nil
	}
	here, err := filepath.Abs(typed("."))
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: hook pre-tool-use cannot tell the folder it runs in: %s\n", err)
		return ExitPolicy, nil
	}
	reason, deny := guard.Reason(in, here)
	if !deny {
		return 0, nil
	}
	if err := guard.Deny(o.Stdout, reason); err != nil {
		fmt.Fprintf(o.Stderr, "itos: hook pre-tool-use cannot write its %s answer: %s\n", guard.Event, err)
		return ExitPolicy, nil
	}
	return 0, nil
}
