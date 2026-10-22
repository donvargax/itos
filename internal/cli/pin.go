package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/engine/pin"
	pinslice "github.com/donvargax/itos/v8/internal/slices/pin"
)

// pinTimeout bounds the one question pin asks, for a checksums.txt.
const pinTimeout = pin.Timeout

// runPin is `pin [<version>]`, the pin slice (internal/slices/pin) over its
// ports bound to the real code.
func runPin(args []string, o Out) (int, error) {
	c, err := pinslice.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, pinslice.NewHandler(pinPorts{}))
}

// pinPorts are the pin slice's ports over the real code.
type pinPorts struct{}

// Config is readConfigPin.
func (pinPorts) Config(context.Context) (pin.Config, error) { return readConfigPin() }

// Release is pinned.
func (pinPorts) Release(_ context.Context, want string) (string, []byte, error) { return pinned(want) }

// readConfigPin reads the config itos finds, the stealth one included, and
// its pin.
func readConfigPin() (pin.Config, error) {
	return pin.ReadConfig(configPath())
}

// pinned is the release to pin and its checksums.txt (pin.Pinned).
func pinned(want string) (string, []byte, error) {
	return pin.Pinned(want)
}

// writeKeepingMode writes the file's new text, keeping its permissions.
func writeKeepingMode(file, text string) error {
	return pin.WriteKeepingMode(file, text)
}
