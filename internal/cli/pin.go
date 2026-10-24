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
	return c.Run(context.Background(), o.Stdout, o.Stderr, pinslice.NewHandler(pinPorts{o.Dir}))
}

// pinPorts are the pin slice's ports over the real code, in the folder
// itos runs in.
type pinPorts struct{ dir string }

// Config is readConfigPin.
func (p pinPorts) Config(context.Context) (pin.Config, error) { return readConfigPin(p.dir) }

// Release is pinned.
func (pinPorts) Release(_ context.Context, want string) (string, []byte, error) { return pinned(want) }

// readConfigPin reads the config itos finds in the folder dir, the stealth
// one included, and its pin.
func readConfigPin(dir string) (pin.Config, error) {
	return pin.ReadConfig(dir, configPath(dir))
}

// pinned is the release to pin and its checksums.txt (pin.Pinned).
func pinned(want string) (string, []byte, error) {
	return pin.Pinned(want)
}

// writeKeepingMode writes the file's new text, read from the folder dir,
// keeping its permissions.
func writeKeepingMode(dir, file, text string) error {
	return pin.WriteKeepingMode(dir, file, text)
}
