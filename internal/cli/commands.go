package cli

import (
	"github.com/donvargax/itos/v8/internal/config"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// A command takes the arguments after its name and gives its exit code, or
// an error that failure reports.
type command func(args []string, o Out) (int, error)

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
