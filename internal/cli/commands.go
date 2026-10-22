package cli

import (
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// A command takes the arguments after its name and gives its exit code, or
// an error that failure reports.
type command func(args []string, o Out) (int, error)

// commands is the command table, main.ts's COMMANDS.
var commands = map[string]command{
	"task":     runTask,
	"work":     runWork,
	"commit":   runCommit,
	"verify":   runVerify,
	"tests":    runTests,
	"ci":       runCi,
	"hook":     runHook,
	"guard":    runGuard,
	"config":   runConfig,
	"version":  runVersion,
	"push":     runPush,
	"git-shim": runGitShim,
	"pin":      runPin,
	"upgrade":  runUpgrade,
	"init":     runInit,
	"followup": runFollowup,
	"decision": runDecision,
	"draft":    runDraft,
	"go":       runGo,
	"guide":    runGuide,
	"status":   runStatus,
}

// flagValue is the value after a flag, and whether there is one.
func flagValue(args []string, name string) (string, bool) {
	i := slices.Index(args, name)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}

// positional is the arguments that are neither flags nor the value of one of
// the valued flags.
func positional(args []string, valued ...string) []string {
	var found []string
	for i, a := range args {
		if strings.HasPrefix(a, "-") || (i > 0 && slices.Contains(valued, args[i-1])) {
			continue
		}
		found = append(found, a)
	}
	return found
}

// first is a list's first element, and whether it has one.
func first(list []string) (string, bool) {
	if len(list) == 0 {
		return "", false
	}
	return list[0], true
}

// split is a command's subcommand and the arguments after it.
func split(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}

// configPath is the config's path, as the working tree has it: every
// command but the commit-msg hook, which reads the index, reads its data
// from the working tree (ADR-0053).
func configPath() string { return config.Path(vcs.Git{}, trees.Worktree) }

// loadConfigFile is the config at file, it and its data read from the
// working tree.
func loadConfigFile(file string) (*config.Loaded, error) {
	return config.Load(vcs.Git{}, trees.Trees{VCS: vcs.Git{}}, trees.Worktree, file)
}

// loadConfig is the config at its path, read from the working tree.
func loadConfig() (*config.Loaded, error) { return loadConfigFile(configPath()) }
