package pin

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/value"
)

// Timeout bounds the one question pin asks, for a checksums.txt.
const Timeout = 30 * time.Second

// InstallScript is where a project keeps the install script the release
// notes write, from the repository's top.
const InstallScript = "tools/bin/install-itos"

// ScriptVersionLine is the install script's version= line.
var ScriptVersionLine = regexp.MustCompile(`(?m)^version=(\S*)`)

// Config is the config itos finds, its text, and the pin it holds: its
// version and checksums as written, "" for none.
type Config struct{ File, Text, Version, Checksums string }

// ReadConfig reads the config, file, and its pin.
func ReadConfig(file string) (Config, error) {
	text, err := os.ReadFile(file)
	if err != nil {
		return Config{}, &config.Error{File: file, Problems: []out.Problem{{
			Rule:    "config-unreadable",
			Message: "cannot be read: " + err.Error(),
			Fix:     "create " + file + ", or correct its YAML",
		}}}
	}
	tree, err := value.Parse(string(text))
	if err != nil {
		return Config{}, config.Invalid(file, "cannot be read: "+err.Error())
	}
	pin := value.Prop(tree, "pin")
	was, _ := value.Prop(pin, "version").(string)
	wasSums, _ := value.Prop(pin, "checksums").(string)
	return Config{File: file, Text: string(text), Version: was, Checksums: wasSums}, nil
}

// Replaced is whether the config pins the version v with other checksums
// than sum, those of its checksums.txt now: the release changed after it was
// pinned
// (docs/decisions/0025-itos-pin-refuses-a-release-changed-after-it-was-pinned.md).
func (c Config) Replaced(v, sum string) bool {
	return c.Version == v && config.PinChecksums.MatchString(c.Checksums) && c.Checksums != sum
}

// Moved is the text, the config's, with pin.version set to v and
// pin.checksums to sum, every other byte as it was.
func (c Config) Moved(text, v, sum string) (string, error) {
	edited, err := value.SetScalars(text, "pin", [][2]string{{"version", v}, {"checksums", sum}})
	if err != nil {
		return "", kind.Wrap(kind.Usage, fmt.Errorf("cannot edit the pin in %s (%s): set pin.version to %s and pin.checksums to %s",
			c.File, err, v, sum))
	}
	return edited, nil
}

// Pinned is the release to pin, the version asked for or, with none, the
// newest, and its checksums.txt. Its errors have their kind: a server that
// cannot be reached or fails is kind.Temporary (release.Get), a release it
// does not have or cannot be pinned kind.Missing.
func Pinned(want string) (string, []byte, error) {
	if want == "" {
		url := release.LatestURL("checksums.txt")
		sums, err := release.Get(url, Timeout)
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
	sums, err := release.Get(url, Timeout)
	if err != nil {
		return "", nil, fmt.Errorf("cannot fetch itos %s: %w", want, err)
	}
	if v := release.VersionOf(sums); v != "" && v != want {
		return "", nil, kind.Wrap(kind.Missing, fmt.Errorf("the checksums.txt of itos %s (%s) lists the archives of itos %s", want, url, v))
	}
	return want, sums, nil
}

// WriteKeepingMode writes the file's new text, keeping its permissions.
func WriteKeepingMode(file, text string) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(file, []byte(text), mode)
}

// ReadInstallScript is the install script's text and whether there is one.
func ReadInstallScript() (string, bool, error) {
	text, err := os.ReadFile(InstallScript)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("cannot read %s: %w", InstallScript, err)
	}
	return string(text), true, nil
}

// ScriptVersion is the version the install script installs, from its one
// version= line.
func ScriptVersion(script string) (string, error) {
	lines := ScriptVersionLine.FindAllStringSubmatch(script, -1)
	if len(lines) != 1 {
		return "", kind.Wrap(kind.Usage, fmt.Errorf("it has %d version= lines, not one", len(lines)))
	}
	if !config.PinVersion.MatchString(lines[0][1]) {
		return "", kind.Wrap(kind.Usage, fmt.Errorf("its version= line names no release's version: %s", lines[0][1]))
	}
	return lines[0][1], nil
}
