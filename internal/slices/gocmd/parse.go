package gocmd

import (
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the go command's line: whether it prints --json's one object.
type CLI struct {
	JSON bool
}

// Parse reads the arguments internal/cli's flag spec hands the go command,
// with the global --json: it takes none, and any is a usage error
// (out.Usage), given before anything is read.
func Parse(args []string, json bool) (CLI, error) {
	if len(args) > 0 {
		return CLI{}, out.Usage("go takes no arguments: %s", strings.Join(args, " "))
	}
	return CLI{JSON: json}, nil
}
