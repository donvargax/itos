package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/slices/commit"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runCommit is `commit`, the commit slice (internal/slices/commit) over its
// ports bound to the real code.
func runCommit(args []string, o Out) (int, error) {
	c, err := commit.Parse(args, o.JSON, o.Quiet, o.Origin, o.Tests)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, commit.NewHandler(commitPorts{Git: vcs.Git{Dir: o.Dir}}))
}

// gitCommit is a commit itself, every argument git commit's but itos's own
// flags, whatever the first is: the git shim's git commit.
func gitCommit(args []string, o Out) (int, error) {
	c := commit.Git(args, o.JSON, o.Quiet, o.Origin, o.Tests)
	return c.Run(context.Background(), o.Stdout, o.Stderr, commit.NewHandler(commitPorts{Git: vcs.Git{Dir: o.Dir}}))
}

// commitPorts are the commit slice's ports over the real code: git, the
// hooks folder's files, the stealth check of the config's path, the config's two loads and the
// working tree's files.
type commitPorts struct {
	vcs.Git
	hookFiles
}

// Stealth is config.IsStealth of configPath().
func (p commitPorts) Stealth(context.Context) bool { return config.IsStealth(p.Git, configPath(p.Dir)) }

// LoadConfig is loadConfig.
func (p commitPorts) LoadConfig(context.Context) (*config.Loaded, error) { return loadConfig(p.Dir) }

// CommitConfig is commitConfig.
func (p commitPorts) CommitConfig(context.Context) (*config.Loaded, error, error) {
	return commitConfig(p.Dir)
}

// Read is the working tree's file.
func (p commitPorts) Read(ctx context.Context, path string) (string, error) {
	return trees.WorktreeIn(p.Dir).Read(ctx, path)
}

// commitConfig is the config itos commit reads its flags by: none, with the
// error loading it gave, when there is no config file, so that itos commit
// still runs git where itos is not set up; an error when there is one that
// does not load; for a command run in the folder dir.
func commitConfig(dir string) (cfg *config.Loaded, missing, err error) {
	file := configPath(dir)
	cfg, err = loadConfigFile(dir, file)
	if err == nil {
		return cfg, nil, nil
	}
	if _, statErr := os.Stat(within(dir, file)); statErr == nil {
		return nil, nil, err
	}
	return nil, err, nil
}

// AmendEnv is message.AmendEnv, whether the commit amends HEAD.
const AmendEnv = message.AmendEnv

// withoutFooters is message.WithoutFooters.
func withoutFooters(env []string) []string { return message.WithoutFooters(env) }
