package cli

import (
	"context"
	"fmt"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
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
	return c.Run(context.Background(), o.Stdout, o.Stderr, verify.NewHandler(verifyPorts{}))
}

// verifyPorts are the verify slice's ports over the real code: git, the
// stealth check of the config's path and the config's load.
type verifyPorts struct{ vcs.Git }

// Stealth is config.IsStealth of configPath().
func (verifyPorts) Stealth(context.Context) bool { return config.IsStealth(vcs.Git{}, configPath()) }

// Config is loadConfig.
func (verifyPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig() }

// span is a range as --json names it.
type span struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// printSinceIssues says commits.since is no commit here, one FAIL line a
// problem.
func printSinceIssues(missing []out.Problem, o Out) {
	for _, p := range missing {
		fmt.Fprintf(o.Stderr, "FAIL %s\n", p.Message)
	}
}
