package pin

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	pinengine "github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/release"
)

// Query is what pin asks: the version to pin, Want, "" for the newest.
type Query struct {
	Want string
}

// Ports are what pin reaches the world through, which internal/cli binds:
// Config, the config itos finds (the stealth one included) and its pin;
// Release, the release to pin, the version wanted or with "" the newest,
// and its checksums.txt, its errors of their kind (pinengine.Pinned).
type Ports interface {
	Config(ctx context.Context) (pinengine.Config, error)
	Release(ctx context.Context, want string) (string, []byte, error)
}

// Handler is pin's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// The actions pin takes, as --json names them: the pin moved, the pin
// already on the release, and a pin on it with other checksums refused.
const (
	pinned  = "pinned"
	already = "already"
	refused = "refused"
)

// Answer is what Handle did: the config's File, the version it pinned,
// Was, and its checksums as written, WasSums ("" for none); the release's
// Version and the SHA-256 of its checksums.txt, Checksums; the Action
// taken; or Unfetched, the release that cannot be asked for, the config
// untouched.
type Answer struct {
	File, Was, WasSums string
	Version, Checksums string
	Action             string
	Unfetched          error
}

// Handle moves the config's pin to the release q wants. A pin already on
// that version with valid checksums is left as it is: with the release's,
// already; with others, refused, as the release changed after it was
// pinned (docs/decisions/0025). Otherwise pin.version and pin.checksums are
// set in the config's text, every other byte as it was, and written back
// keeping its mode.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	c, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	a := Answer{File: c.File, Was: c.Version, WasSums: c.Checksums}
	v, sums, err := h.ports.Release(ctx, q.Want)
	if err != nil {
		a.Unfetched = err
		return a, nil
	}
	a.Version, a.Checksums = v, release.SHA256(sums)
	if a.Was == v && config.PinChecksums.MatchString(a.WasSums) {
		a.Action = already
		if c.Replaced(v, a.Checksums) {
			a.Action = refused
		}
		return a, nil
	}
	edited, err := c.Moved(c.Text, v, a.Checksums)
	if err != nil {
		return Answer{}, err
	}
	if err := pinengine.WriteKeepingMode(a.File, edited); err != nil {
		return Answer{}, err
	}
	a.Action = pinned
	return a, nil
}
