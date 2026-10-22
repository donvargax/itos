package cli

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	initslice "github.com/donvargax/itos/v8/internal/slices/initcmd"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runInit is `init [--stealth] [--policy <file>] [--plugin [<scope>]]
// [--git-shim [--git-shim-dir <folder>] | --no-git-shim] [--agent-rules |
// --no-agent-rules]`, the init slice (internal/slices/initcmd) over its
// ports bound to the real code; --git-shim-dir and --policy are read from
// where itos was started.
func runInit(args []string, o Out) (int, error) {
	ctx, p := context.Background(), initPorts{o: o}
	c, err := initslice.Parse(ctx, args, o.Origin, o.JSON, o.Tests, p)
	if err != nil {
		return 0, err
	}
	return c.Run(ctx, o.Stdout, o.Stderr, initslice.NewHandler(p))
}

// initPorts are the init slice's ports over the real code: git, the trees
// over it, the working tree, the flags of itos task that name no group,
// pin's release, hook install, the terminal, the programs on the PATH and
// git-shim install's links on the OS o names, with o.
type initPorts struct {
	vcs.Git
	o Out
}

// Trees is the tree adapter over git.
func (initPorts) Trees(context.Context) port.Trees { return trees.Trees{VCS: vcs.Git{}} }

// Worktree is the working tree's files.
func (initPorts) Worktree(context.Context) port.Source { return trees.Worktree }

// TaskFlags is ungroupedTaskFlags.
func (initPorts) TaskFlags(context.Context) []string { return ungroupedTaskFlags() }

// Release is pinned.
func (initPorts) Release(_ context.Context, want string) (string, []byte, error) { return pinned(want) }

// InstallHooks is hookInstall, writing to stdout, its object under json.
func (p initPorts) InstallHooks(_ context.Context, stdout io.Writer, json bool) (int, error) {
	o := p.o
	o.JSON, o.Stdout = json, stdout
	return hookInstall(false, false, o)
}

// Abs is filepath.Abs.
func (initPorts) Abs(_ context.Context, path string) (string, error) { return filepath.Abs(path) }

// WriteFile is os.WriteFile.
func (initPorts) WriteFile(_ context.Context, name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// MkdirAll is os.MkdirAll.
func (initPorts) MkdirAll(_ context.Context, path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteText is agentrules.WriteText.
func (initPorts) WriteText(_ context.Context, path, text string, crlf bool) error {
	return agentrules.WriteText(path, text, crlf)
}

// Terminal is onTerminal.
func (initPorts) Terminal(context.Context) bool { return onTerminal() }

// Stdin is os.Stdin.
func (initPorts) Stdin(context.Context) io.Reader { return os.Stdin }

// LookPath is exec.LookPath.
func (initPorts) LookPath(_ context.Context, file string) (string, error) { return exec.LookPath(file) }

// Command runs name with args, its stdout and stderr together.
func (initPorts) Command(_ context.Context, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var output strings.Builder
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}

// ShimPlace is shimPlace.
func (p initPorts) ShimPlace(_ context.Context, dir string) (self, link string, err error) {
	return shimPlace(dir, p.o.OS)
}

// LinksItos is linksItos.
func (initPorts) LinksItos(_ context.Context, link string) (self, other bool) { return linksItos(link) }

// MakeLink is makeLink.
func (p initPorts) MakeLink(_ context.Context, self, link string, replace bool) error {
	return makeLink(self, link, replace, p.o.OS)
}

// PathStanding is pathStanding.
func (p initPorts) PathStanding(_ context.Context, dir string) (at int, realGit string, realAt int) {
	return pathStanding(dir, p.o.OS)
}

// StandingLine is standingLine.
func (initPorts) StandingLine(_ context.Context, dir string, at int, realGit string, realAt int) string {
	return standingLine(dir, at, realGit, realAt)
}

// onTerminal is whether a person is at the other end: stdin and stdout both a
// terminal, a character device other than the null device.
func onTerminal() bool { return terminal(os.Stdin) && terminal(os.Stdout) }

func terminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(info, null)
}
