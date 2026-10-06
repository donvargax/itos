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
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/kind"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/release"
	"github.com/donvargax/itos/v5/internal/value"
)

// pinTimeout bounds the one question pin asks, for a checksums.txt.
const pinTimeout = 30 * time.Second

// pinCommand is `pin [<version>]`.
func pinCommand(args []string, o Out) (int, error) {
	want, err := versionArg("pin", args)
	if err != nil {
		return 0, err
	}
	c, err := readConfigPin()
	if err != nil {
		return 0, err
	}
	file, was, wasSums := c.file, c.version, c.checksums
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
		if c.replaced(v, sum) {
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
	edited, err := c.moved(c.text, v, sum)
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

// configPin is the config itos finds, its text, and the pin it holds: its
// version and checksums as written, "" for none.
type configPin struct{ file, text, version, checksums string }

// readConfigPin reads the config itos finds, the stealth one included, and
// its pin.
func readConfigPin() (configPin, error) {
	file := config.Path()
	text, err := os.ReadFile(file)
	if err != nil {
		return configPin{}, &config.Error{File: file, Problems: []out.Problem{{
			Rule:    "config-unreadable",
			Message: "cannot be read: " + err.Error(),
			Fix:     "create " + file + ", or correct its YAML",
		}}}
	}
	tree, err := value.Parse(string(text))
	if err != nil {
		return configPin{}, config.Invalid(file, "cannot be read: "+err.Error())
	}
	pin := value.Prop(tree, "pin")
	was, _ := value.Prop(pin, "version").(string)
	wasSums, _ := value.Prop(pin, "checksums").(string)
	return configPin{file: file, text: string(text), version: was, checksums: wasSums}, nil
}

// replaced is whether the config pins the version v with other checksums
// than sum, those of its checksums.txt now: the release changed after it was
// pinned
// (docs/decisions/0025-itos-pin-refuses-a-release-changed-after-it-was-pinned.md).
func (c configPin) replaced(v, sum string) bool {
	return c.version == v && config.PinChecksums.MatchString(c.checksums) && c.checksums != sum
}

// moved is the text, the config's, with pin.version set to v and
// pin.checksums to sum, every other byte as it was.
func (c configPin) moved(text, v, sum string) (string, error) {
	edited, err := value.SetScalars(text, "pin", [][2]string{{"version", v}, {"checksums", sum}})
	if err != nil {
		return "", kind.Wrap(kind.Usage, fmt.Errorf("cannot edit the pin in %s (%s): set pin.version to %s and pin.checksums to %s",
			c.file, err, v, sum))
	}
	return edited, nil
}

// pinned is the release to pin, the version asked for or, with none, the
// newest, and its checksums.txt. Its errors have their kind: a server that
// cannot be reached or fails is kind.Temporary (release.Get), a release it
// does not have or cannot be pinned kind.Missing.
func pinned(want string) (string, []byte, error) {
	if want == "" {
		url := release.LatestURL("checksums.txt")
		sums, err := release.Get(url, pinTimeout)
		if err != nil {
			return "", nil, fmt.Errorf("cannot ask for the newest itos: %w", err)
		}
		v := release.VersionOf(sums)
		if v == "" {
			return "", nil, kind.Wrap(kind.Missing, fmt.Errorf("the newest itos's checksums.txt (%s) names no archive of itos", url))
		}
		return v, sums, nil
	}
	url := release.URL(want, "checksums.txt")
	sums, err := release.Get(url, pinTimeout)
	if err != nil {
		return "", nil, fmt.Errorf("cannot fetch itos %s: %w", want, err)
	}
	if v := release.VersionOf(sums); v != "" && v != want {
		return "", nil, kind.Wrap(kind.Missing, fmt.Errorf("the checksums.txt of itos %s (%s) lists the archives of itos %s", want, url, v))
	}
	return want, sums, nil
}

// writeKeepingMode writes the file's new text, keeping its permissions.
func writeKeepingMode(file, text string) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(file, []byte(text), mode)
}
