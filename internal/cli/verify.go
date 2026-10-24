package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/slices/verify"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runVerify is `verify`, the verify slice (internal/slices/verify) over its
// ports bound to the real code.
func runVerify(args []string, o Out) (int, error) {
	c, err := verify.Parse(args, o.JSON, o.Tests)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, verify.NewHandler(verifyPorts{vcs.Git{Dir: o.Dir}}))
}

// verifyPorts are the verify slice's ports over the real code: git, the
// stealth check of the config's path and the config's load.
type verifyPorts struct{ vcs.Git }

// Stealth is config.IsStealth of configPath().
func (p verifyPorts) Stealth(context.Context) bool { return config.IsStealth(p.Git, configPath(p.Dir)) }

// Config is loadConfig.
func (p verifyPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig(p.Dir) }

// span is a range as --json names it.
type span struct {
	From string `json:"from"`
	To   string `json:"to"`
}
