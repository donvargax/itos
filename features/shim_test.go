// The git shim's steps (shim.feature): the itos binary under test linked as
// git (git.exe on windows) in a folder of the scenario's support folder, put first on the PATH of
// every command the scenario runs, and git run through that link. The binary
// is the one the itos under test runs, not tools/bin/itos, a script that finds
// its checkout from its own path: an itos run tells an extension its binary in
// ITOS_BIN, so a throwaway extension asks for it. The link and its folder go
// with the support folder after the scenario, and the PATH is only ever the
// scenario's commands' own, so no shim is left on the PATH of anything else.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

func initializeShimSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^itos is linked as git before the real git on the PATH$`, w.linkShim)
	sc.Step(`^another itos is linked as git before the real git on the PATH$`, w.linkOtherShim)
	sc.Step(`^a repository with no itos config, its change to "([^"]*)" staged$`, w.plainRepository)

	sc.Step(`^git runs "([^"]*)"$`, func(line string) error { return w.gitThroughShim(w.dir, line) })
	sc.Step(`^git runs "([^"]*)" from "([^"]*)"$`, func(line, folder string) error {
		return w.gitThroughShim(filepath.Join(w.dir, folder), line)
	})
	sc.Step(`^git runs "([^"]*)" in that repository$`, func(line string) error {
		return w.gitThroughShim(w.plain(), line)
	})

	sc.Step(`^git exits with code (\d+)$`, w.gitExitsWith)
	sc.Step(`^that repository's HEAD says "([^"]*)"$`, w.plainHeadSays)
	sc.Step(`^"([^"]*)" runs itos$`, w.runsItos)
}

// The folder holding the scenario's git link.
func (w *world) shimFolder() string { return filepath.Join(w.support, "shim") }

// The repository with no itos config.
func (w *world) plain() string { return filepath.Join(w.support, "plain") }

// The folders the scenario puts first on the PATH: its git link's, then its
// extensions', each when it has one.
func (w *world) pathFirst() []string {
	var first []string
	for _, dir := range []string{w.shimFolder(), w.extensionsDir()} {
		if _, err := os.Stat(dir); err == nil {
			first = append(first, dir)
		}
	}
	return first
}

var (
	itosBinaryOnce sync.Once
	itosBinary     string
	itosBinaryErr  error
)

// The binary the itos under test runs, as an extension is told it.
func (w *world) itosBinary() (string, error) {
	itosBinaryOnce.Do(func() {
		probe, err := os.MkdirTemp("", "itos-features-probe-")
		if err != nil {
			itosBinaryErr = err
			return
		}
		defer os.RemoveAll(probe)
		script := "#!/bin/sh\nprintf '%s\\n' \"$ITOS_BIN\"\n"
		if err := w.writeProgram(filepath.Join(probe, "itos-binary"), script); err != nil {
			itosBinaryErr = err
			return
		}
		cmd := exec.Command(w.bin, "binary")
		cmd.Dir = w.dir
		cmd.Env = append(w.env(), "PATH="+probe+string(os.PathListSeparator)+callerPath())
		out, err := cmd.Output()
		itosBinary = strings.TrimSpace(string(out))
		if err != nil || !filepath.IsAbs(itosBinary) {
			itosBinaryErr = fmt.Errorf("the itos under test does not say its binary (ITOS_BIN %q): %v", itosBinary, err)
		}
	})
	return itosBinary, itosBinaryErr
}

func (w *world) linkShim() error {
	bin, err := w.itosBinary()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(w.shimFolder(), 0o755); err != nil {
		return err
	}
	return os.Symlink(bin, programPath(filepath.Join(w.shimFolder(), "git")))
}

// A copy of the itos binary under test, another file as a pinned release in
// the launcher's cache or a repository's own build is beside the global itos
// the shim links (bug 45), linked as git in the scenario's git folder: a
// symbolic link to the copy, named itos as an install's target is, or on
// windows a hard link when symbolic links are not allowed.
func (w *world) linkOtherShim() error {
	bin, err := w.itosBinary()
	if err != nil {
		return err
	}
	text, err := os.ReadFile(bin)
	if err != nil {
		return err
	}
	other := programPath(filepath.Join(w.support, "other-itos", "itos"))
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(other, text, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(w.shimFolder(), 0o755); err != nil {
		return err
	}
	link := programPath(filepath.Join(w.shimFolder(), "git"))
	err = os.Symlink(other, link)
	if err != nil && runtime.GOOS == "windows" {
		err = os.Link(other, link)
	}
	return err
}

// A repository of its own in the support folder, with no itos config, and a
// new file staged in it.
func (w *world) plainRepository(path string) error {
	if err := os.MkdirAll(w.plain(), 0o755); err != nil {
		return err
	}
	if err := w.gitIn(w.plain(), "init", "-q", "-b", "main"); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(w.plain(), path), []byte("A note.\n"), 0o644); err != nil {
		return err
	}
	return w.gitIn(w.plain(), "add", "--", path)
}

// git run through the scenario's link, in the folder dir, with the arguments
// as a shell would split the line.
func (w *world) gitThroughShim(dir, line string) error {
	args, err := shellWords(line)
	if err != nil {
		return err
	}
	link := programPath(filepath.Join(w.shimFolder(), "git"))
	if _, err := os.Lstat(link); err != nil {
		return fmt.Errorf("itos is not linked as git: %w", err)
	}
	return w.run(dir, link, args...)
}

func (w *world) gitExitsWith(code int) error {
	if w.exit != code {
		return fmt.Errorf("git exited %d, not %d\n%s", w.exit, code, w.report())
	}
	return nil
}

func (w *world) plainHeadSays(text string) error {
	cmd := exec.Command("git", "log", "-1", "--format=%B")
	cmd.Dir = w.plain()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("that repository has no HEAD: %v\n%s", err, w.report())
	}
	if !strings.Contains(string(out), text) {
		return fmt.Errorf("that repository's HEAD does not say %q:\n%s", text, out)
	}
	return nil
}

// The program at path in the scratch repository (path.exe on windows) is the
// itos binary under test, its links followed.
func (w *world) runsItos(path string) error {
	bin, err := w.itosBinary()
	if err != nil {
		return err
	}
	want, err := os.Stat(bin)
	if err != nil {
		return err
	}
	got, err := os.Stat(programPath(filepath.Join(w.dir, path)))
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", path, err, w.report())
	}
	if !os.SameFile(got, want) {
		return fmt.Errorf("%s is not the itos binary %s\n%s", path, bin, w.report())
	}
	return nil
}
