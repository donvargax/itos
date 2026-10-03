// The extensions' steps (extensions.feature): each extension is a shell script
// itos-<name> in a folder of the scenario's that is put first on the PATH of
// every command the scenario runs. The script records what it was given (its
// arguments, one a line, the folder it ran in and the ITOS_* variables it
// saw) in a folder of its own, then exits with the code the scenario chose, or
// runs the command the scenario gave it and records that command's output.
// The steps' environment strips the caller's ITOS_* (env), so every ITOS_*
// an extension sees is itos's, or the launcher's that the steps set.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func initializeExtensionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the extension "([^"]*)" on the PATH exits with code (\d+)$`, func(name string, code int) error {
		return w.extension(name, "exit "+strconv.Itoa(code))
	})
	sc.Step(`^the extension "([^"]*)" on the PATH runs "([^"]*)"$`, func(name, command string) error {
		return w.extension(name, command+` > "$rec/output"`)
	})

	sc.Step(`^the extension "([^"]*)" ran with the arguments "([^"]*)"$`, w.extensionRanWithArguments)
	sc.Step(`^the extension "([^"]*)" did not run$`, w.extensionDidNotRun)
	sc.Step(`^the extension "([^"]*)" ran in "([^"]*)"$`, w.extensionRanIn)
	sc.Step(`^the extension "([^"]*)" ran with (ITOS_[A-Z_]+) "([^"]*)"$`, func(name, variable, value string) error {
		return w.extensionVariable(name, variable, value, false)
	})
	sc.Step(`^the extension "([^"]*)" ran with (ITOS_[A-Z_]+) ending in "([^"]*)"$`, func(name, variable, value string) error {
		return w.extensionVariable(name, variable, value, true)
	})
	sc.Step(`^the extension "([^"]*)" was told the version its call back printed$`, w.extensionToldCallBackVersion)
}

// The folder on the PATH holding the scenario's extensions.
func (w *world) extensionsDir() string { return filepath.Join(w.support, "extensions") }

// The folder an extension's script records its last run in.
func (w *world) extensionRecord(name string) string {
	return filepath.Join(w.support, "extension-runs", name)
}

// An extension's script: what it records, then body.
func (w *world) extension(name, body string) error {
	if err := os.MkdirAll(w.extensionsDir(), 0o755); err != nil {
		return err
	}
	// Git for Windows' sh says its folder as /c/…; -W says it as windows names
	// it, C:/….
	pwd := "pwd -P"
	if runtime.GOOS == "windows" {
		pwd = "pwd -W"
	}
	script := "#!/bin/sh\n" +
		"rec=" + quote(w.extensionRecord(name)) + "\n" +
		`rm -rf "$rec" && mkdir -p "$rec" || exit 99` + "\n" +
		pwd + ` > "$rec/dir"` + "\n" +
		`for a in "$@"; do printf '%s\n' "$a"; done > "$rec/args"` + "\n" +
		`env | grep '^ITOS_' > "$rec/env"` + "\n" +
		body + "\n"
	return w.writeProgram(filepath.Join(w.extensionsDir(), name), script)
}

// What the extension's last run recorded in the file, or why it cannot be
// read.
func (w *world) extensionRecorded(name, file string) (string, error) {
	text, err := os.ReadFile(filepath.Join(w.extensionRecord(name), file))
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("the extension %s did not run\n%s", name, w.report())
	}
	return string(text), err
}

func (w *world) extensionRanWithArguments(name, args string) error {
	text, err := w.extensionRecorded(name, "args")
	if err != nil {
		return err
	}
	got := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if text == "" {
		got = nil
	}
	if want := strings.Fields(args); !slices.Equal(got, want) {
		return fmt.Errorf("the extension %s ran with the arguments %q, not %q\n%s", name, got, want, w.report())
	}
	return nil
}

func (w *world) extensionDidNotRun(name string) error {
	if _, err := os.Stat(w.extensionRecord(name)); err == nil {
		return fmt.Errorf("the extension %s ran\n%s", name, w.report())
	}
	return nil
}

func (w *world) extensionRanIn(name, folder string) error {
	text, err := w.extensionRecorded(name, "dir")
	if err != nil {
		return err
	}
	want, err := filepath.EvalSymlinks(filepath.Join(w.dir, folder))
	if err != nil {
		return err
	}
	// Its links followed as the folder's are, which on windows also gives a
	// short name (RUNNER~1) its long one.
	got := strings.TrimSpace(text)
	if real, err := filepath.EvalSymlinks(got); err == nil {
		got = real
	}
	if got != want {
		return fmt.Errorf("the extension %s ran in %s, not %s\n%s", name, got, want, w.report())
	}
	return nil
}

// The ITOS_* variable the extension saw, "" when it saw none.
func (w *world) extensionSaw(name, variable string) (string, error) {
	text, err := w.extensionRecorded(name, "env")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(text, "\n") {
		if value, ok := strings.CutPrefix(line, variable+"="); ok {
			return value, nil
		}
	}
	return "", nil
}

func (w *world) extensionVariable(name, variable, want string, suffix bool) error {
	got, err := w.extensionSaw(name, variable)
	if err != nil {
		return err
	}
	// A path's ending is read with slashes, whatever the platform's separator.
	if suffix && !strings.HasSuffix(filepath.ToSlash(got), want) || !suffix && got != want {
		how := "is not"
		if suffix {
			how = "does not end in"
		}
		return fmt.Errorf("the extension %s saw %s=%q, which %s %q\n%s", name, variable, got, how, want, w.report())
	}
	return nil
}

// The extension saw an ITOS_VERSION, and the itos it called back printed
// that version.
func (w *world) extensionToldCallBackVersion(name string) error {
	told, err := w.extensionSaw(name, "ITOS_VERSION")
	if err != nil {
		return err
	}
	printed, err := w.extensionRecorded(name, "output")
	if err != nil {
		return err
	}
	if told == "" || strings.TrimSpace(printed) != "itos "+told {
		return fmt.Errorf("the extension %s was told ITOS_VERSION=%q, and its call back printed %q\n%s",
			name, told, printed, w.report())
	}
	return nil
}
