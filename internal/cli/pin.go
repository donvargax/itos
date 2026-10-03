package cli

// itos pin (slice 47, features/pin.feature): moves the config's pin to a
// release, the newest by default, as the launcher asks for it, or the one
// named. It writes pin.version and pin.checksums, the SHA-256 of that
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

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/release"
	"github.com/donvargax/itos/v2/internal/value"
)

// pinTimeout bounds the one question pin asks, for a checksums.txt.
const pinTimeout = 30 * time.Second

// pinCommand is `pin [<version>]`.
func pinCommand(args []string, o Out) (int, error) {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return 0, usage("pin takes only a <version> (%s)", a)
		}
	}
	if len(args) > 1 {
		return 0, usage("pin takes one <version> at most: %s", strings.Join(args, " "))
	}
	want := ""
	if len(args) == 1 {
		want = strings.TrimPrefix(args[0], "v")
		if !config.PinVersion.MatchString(want) {
			return 0, usage("pin needs a release's version, such as 2.5.0, not %s", args[0])
		}
	}
	file := config.Path()
	text, err := os.ReadFile(file)
	if err != nil {
		return 0, &config.Error{File: file, Problems: []out.Problem{{
			Rule:    "config-unreadable",
			Message: "cannot be read: " + err.Error(),
			Fix:     "create " + file + ", or correct its YAML",
		}}}
	}
	tree, err := value.Parse(string(text))
	if err != nil {
		return 0, config.Invalid(file, "cannot be read: "+err.Error())
	}
	pin := value.Prop(tree, "pin")
	was, _ := value.Prop(pin, "version").(string)
	wasSums, _ := value.Prop(pin, "checksums").(string)

	v, sums, err := pinned(want)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s; %s is left as it was\n", err, file)
		return ExitMissing, nil
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
		if wasSums != sum {
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
	edited, err := value.SetScalars(string(text), "pin", [][2]string{{"version", v}, {"checksums", sum}})
	if err != nil {
		return 0, fmt.Errorf("cannot edit the pin in %s (%s): set pin.version to %s and pin.checksums to %s",
			file, err, v, sum)
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

// pinned is the release to pin, the version asked for or, with none, the
// newest, and its checksums.txt.
func pinned(want string) (string, []byte, error) {
	if want == "" {
		url := release.LatestURL("checksums.txt")
		sums, err := release.Get(url, pinTimeout)
		if err != nil {
			return "", nil, fmt.Errorf("cannot ask for the newest itos: %w", err)
		}
		v := release.VersionOf(sums)
		if v == "" {
			return "", nil, fmt.Errorf("the newest itos's checksums.txt (%s) names no archive of itos", url)
		}
		return v, sums, nil
	}
	url := release.URL(want, "checksums.txt")
	sums, err := release.Get(url, pinTimeout)
	if err != nil {
		return "", nil, fmt.Errorf("cannot fetch itos %s: %w", want, err)
	}
	if v := release.VersionOf(sums); v != "" && v != want {
		return "", nil, fmt.Errorf("the checksums.txt of itos %s (%s) lists the archives of itos %s", want, url, v)
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
