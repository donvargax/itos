package cli

import (
	"context"
	"errors"
	"time"

	"github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/slices/followup"
	"github.com/donvargax/itos/v8/internal/slices/followup/domain"
	"github.com/donvargax/itos/v8/internal/store/private/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runFollowup is `followup`, the followup slice (internal/slices/followup)
// over its ports bound to the real code; a doc path is read from where itos
// was started, before a run from a subfolder moved to the top. A threads'
// file the domain refuses is a data file itos refuses, exit 2.
func runFollowup(args []string, o Out) (int, error) {
	c, err := followup.Parse(args, o.Origin, o.JSON, o.Quiet)
	if err != nil {
		return 0, usage("%s", err)
	}
	code, err := c.Run(context.Background(), o.Stdout, o.Stderr, followup.NewHandler(followupPorts{adapter.Store{OS: o.OS, VCS: vcs.Git{}}}))
	var invalid *domain.Invalid
	if errors.As(err, &invalid) {
		err = kind.Wrap(kind.Usage, err)
	}
	return code, err
}

// followupPorts are the followup slice's ports over the real code: the
// private store's adapter and the clock.
type followupPorts struct{ adapter.Store }

// Now is time.Now.
func (followupPorts) Now(context.Context) time.Time { return time.Now() }

// subArgs is args.Sub.
func subArgs(command, sub string, list []string, valued, switches []string) ([]string, map[string]string, map[string]bool, error) {
	return args.Sub(command, sub, list, valued, switches)
}
