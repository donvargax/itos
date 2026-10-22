package cli

// itos pin (slice 47, features/pin.feature): moves the config's pin to a
// release, the newest by default or with latest (slice 87), as the launcher
// asks for it, or the one named. It writes pin.version and pin.checksums, the SHA-256 of that
// release's checksums.txt, in the config itos finds, the stealth one
// included, changing those values' bytes alone (value.SetScalars), and
// commits nothing: the bump is the project's own commit. Like git-shim
// install it is the launcher's own command, run by the binary called whatever
// the pin says (internal/launch), since the version pinned may predate it.

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
)

// pinTimeout bounds the one question pin asks, for a checksums.txt.
const pinTimeout = pin.Timeout

// runPin is `pin [<version>]`.
func runPin(args []string, o Out) (int, error) {
	want, err := versionArg("pin", args)
	if err != nil {
		return 0, err
	}
	c, err := readConfigPin()
	if err != nil {
		return 0, err
	}
	file, was, wasSums := c.File, c.Version, c.Checksums
	v, sums, err := pinned(want)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s; %s is left as it was\n", err, file)
		return ExitCode(err), nil
	}
	sum := release.SHA256(sums)
	report := func(action string, code int) (int, error) {
		fields := []out.Field{{Key: "config", Value: file}, {Key: "action", Value: action},
			{Key: "version", Value: v}, {Key: "checksums", Value: sum}}
		if was != "" {
			fields = append(fields, out.Field{Key: "previous", Value: was})
		}
		if action == "pinned" {
			fields = append(fields, out.Field{Key: "notes", Value: release.NotesURL(v)})
		}
		return code, out.Emit(o.Stdout, fields...)
	}
	if was == v && config.PinChecksums.MatchString(wasSums) {
		if c.Replaced(v, sum) {
			if !o.JSON {
				fmt.Fprintf(o.Stderr, "itos: %s pins itos %s with the checksums %s, but its checksums.txt is %s now: "+
					"the release changed after it was pinned; the pin is left as it was\n", file, v, wasSums, sum)
				return ExitPolicy, nil
			}
			return report("refused", ExitPolicy)
		}
		if o.JSON {
			return report("already", 0)
		}
		fmt.Fprintf(o.Stdout, "%s already pins itos %s: nothing to change.\n", file, v)
		return 0, nil
	}
	edited, err := c.Moved(c.Text, v, sum)
	if err != nil {
		return 0, err
	}
	if err := writeKeepingMode(file, edited); err != nil {
		return 0, err
	}
	if o.JSON {
		return report("pinned", 0)
	}
	if was != "" {
		fmt.Fprintf(o.Stdout, "Pinned itos %s in %s, from %s.\n", v, file, was)
	} else {
		fmt.Fprintf(o.Stdout, "Pinned itos %s in %s, which pinned none.\n", v, file)
	}
	fmt.Fprintf(o.Stdout, "Read its notes, their Upgrading section above all, before you commit the change: %s\n",
		release.NotesURL(v))
	return 0, nil
}

// versionArg is the one <version> a command that moves the pin takes, a
// leading v dropped, "" when it names none or names latest, which means the
// newest as no version does (slice 87).
func versionArg(name string, args []string) (string, error) {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return "", usage("%s takes only a <version> (%s)", name, a)
		}
	}
	if len(args) > 1 {
		return "", usage("%s takes one <version> at most: %s", name, strings.Join(args, " "))
	}
	if len(args) == 0 || args[0] == "latest" {
		return "", nil
	}
	want := strings.TrimPrefix(args[0], "v")
	if !config.PinVersion.MatchString(want) {
		return "", usage("%s needs a release's version, such as 2.5.0, or latest, not %s", name, args[0])
	}
	return want, nil
}

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
