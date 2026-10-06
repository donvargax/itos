package cli

// The names v6.0.0 renamed (slice 89, decision 36; docs/CLI.md rules 3 to 6,
// 17, 18 and 40). An old name keeps no command working: it exits 2 with one
// line naming the new one, and an old command's name never reaches an
// itos-<name> extension on the PATH. Nothing else keeps an old interface
// working (p1-drop-compat-code).

import (
	"errors"
	"strings"

	"github.com/donvargax/itos/v5/internal/kind"
)

// renamedCommands are the command paths v6.0.0 renamed, each with the line
// that names what replaced it.
var renamedCommands = []struct{ old, line string }{
	{"hook pre-tool-use", "itos hook pre-tool-use is itos guard claude-code since v6.0.0"},
	{"hooks", "itos hooks install is itos hook install since v6.0.0"},
	{"ask", "itos ask is itos question since v6.0.0, the questions for the person the work is for; " +
		"to ask someone else, itos followup"},
	{"follow", "itos follow is itos followup since v6.0.0"},
}

// renamedFlags are the flags v6.0.0 renamed, by the command's path and the
// old flag: the new flag.
var renamedFlags = map[string]string{
	"work promote --as": "--id",
	"work queue --drop": "--remove",
}

// renamedCommand is the refusal of the command path words names when
// v6.0.0 renamed it, nil when it did not: a usage error, exit 2.
func renamedCommand(words []string) error {
	path := strings.Join(words, " ")
	for _, r := range renamedCommands {
		if path == r.old || strings.HasPrefix(path, r.old+" ") {
			return kind.Wrap(kind.Usage, errors.New(r.line))
		}
	}
	return nil
}

// retiredName is whether name was a command before v6.0.0 renamed it, so
// it never names an extension.
func retiredName(name string) bool {
	for _, r := range renamedCommands {
		if r.old == name {
			return true
		}
	}
	return false
}

// renamedFlag is the refusal of the command's flag when v6.0.0 renamed it,
// nil when it did not.
func renamedFlag(command, flag string) error {
	to, ok := renamedFlags[command+" "+flag]
	if !ok {
		return nil
	}
	return kind.Wrap(kind.Usage, errors.New(command+" "+flag+" is "+to+" since v6.0.0"))
}
