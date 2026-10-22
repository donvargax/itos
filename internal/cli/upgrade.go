package cli

import (
	"context"
	"slices"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	"github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/launch"
	"github.com/donvargax/itos/v8/internal/release"
	upgradeslice "github.com/donvargax/itos/v8/internal/slices/upgrade"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// UpgradeTarget reads the arguments, as the launcher gets them, for an itos
// upgrade it can hand on (slice 126): the version it moves to, <version> or
// the newest, asked for as upgrade asks, and the same command line with
// that version named in place of none or latest, every flag kept. ok is
// false for anything else, left to this binary: another command, an
// extension, --help, a line upgrade refuses as written, or one with a "--".
// An error is a version that cannot be had, its kind as upgrade gives it.
func UpgradeTarget(args []string) (h launch.Handoff, ok bool, err error) {
	g := Parse(args)
	if g.Extension != "" || g.Help || len(g.Rest) == 0 || g.Rest[0] != "upgrade" || slices.Contains(args, "--") {
		return h, false, nil
	}
	read, err := readLine(args)
	if err != nil {
		return h, false, nil
	}
	want, err := pin.VersionArg("upgrade", read.Rest[1:])
	if err != nil {
		return h, false, nil
	}
	if h.Version, h.Checksums, err = pinned(want); err != nil {
		return h, true, err
	}
	h.Args = upgradeslice.NamingVersion(args, h.Version)
	return h, true, nil
}

// runUpgrade is `upgrade [<version>]`, the upgrade slice
// (internal/slices/upgrade) over its ports bound to the real code.
func runUpgrade(args []string, o Out) (int, error) {
	c, err := upgradeslice.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, upgradeslice.NewHandler(upgradePorts{}))
}

// upgradePorts are the upgrade slice's ports over the real code: pin's,
// the release server and the real git.
type upgradePorts struct{}

// Config is readConfigPin.
func (upgradePorts) Config(context.Context) (pin.Config, error) { return readConfigPin() }

// Release is pinned.
func (upgradePorts) Release(_ context.Context, want string) (string, []byte, error) {
	return pinned(want)
}

// Fetch is release.Get, within pinTimeout.
func (upgradePorts) Fetch(_ context.Context, url string) ([]byte, error) {
	return release.Get(url, pinTimeout)
}

// Stealth is config.IsStealth over the real git.
func (upgradePorts) Stealth(_ context.Context, file string) bool {
	return config.IsStealth(vcs.Git{}, file)
}

// Rules is agentrules.Refreshed over the real git.
func (upgradePorts) Rules(ctx context.Context, stealth bool) []agentrules.Edit {
	return agentrules.Refreshed(ctx, vcs.Git{}, stealth)
}

// Write is writeKeepingMode.
func (upgradePorts) Write(_ context.Context, file, text string) error {
	return writeKeepingMode(file, text)
}
