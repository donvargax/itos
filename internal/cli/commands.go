package cli

import (
	"github.com/donvargax/itos/v8/internal/config"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// A command takes the arguments after its name and gives its exit code, or
// an error that failure reports.
type command func(args []string, o Out) (int, error)

// configPath is the config's path for a command run in the folder dir, as
// the working tree has it: every command but the commit-msg hook, which
// reads the index, reads its data from the working tree (ADR-0053).
func configPath(dir string) string { return config.Path(vcs.Git{Dir: dir}, trees.WorktreeIn(dir)) }

// loadConfigFile is the config at file, it and its data read from the
// working tree, for a command run in the folder dir.
func loadConfigFile(dir, file string) (*config.Loaded, error) {
	v := vcs.Git{Dir: dir}
	return config.Load(v, trees.Trees{VCS: v}, trees.WorktreeIn(dir), file)
}

// loadConfig is the config at its path, read from the working tree, for a
// command run in the folder dir.
func loadConfig(dir string) (*config.Loaded, error) { return loadConfigFile(dir, configPath(dir)) }
