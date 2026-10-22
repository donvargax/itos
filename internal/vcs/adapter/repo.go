package adapter

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// Init is git init -q, after which Locate asks git again from every
// folder: one it found in no repository may be in this one now.
func (g Git) Init(ctx context.Context) error {
	text, err := g.git().Command(ctx, "init", "-q").CombinedOutput()
	g.Forget()
	if err != nil {
		return fmt.Errorf("git init: %s", strings.TrimSpace(string(text)))
	}
	return nil
}

// NotIgnored is whether git check-ignore exits 1 for path: in the work
// tree and not ignored. Outside the work tree or in no repository it exits
// 128, and for an ignored path 0.
func (g Git) NotIgnored(ctx context.Context, path string) bool {
	err := g.git().Command(ctx, "check-ignore", "-q", "--", path).Run()
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

// The git config, which the port leaves to the adapter: itos's hooks are
// declared there (git 2.54.0 on runs them), and notes.rewriteRef carries the
// itos notes through an amend and a rebase.

// Config are a key's values in the git config, the repository's own when
// local is set, else every level's; none when it has none.
func (g Git) Config(_ context.Context, key string, local bool) []string {
	args := []string{"-C", ".", "config"}
	if local {
		args = append(args, "--local")
	}
	got, err := g.git().Output(append(args, "--get-all", key)...)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(got, "\n"), "\n")
}

// SetConfig sets a key of the repository's own config to the one value;
// its error says git's words.
func (g Git) SetConfig(_ context.Context, key, value string) error {
	text, err := g.git().Command(context.Background(), "-C", ".", "config", "--local", "--replace-all", key, value).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git config --replace-all %s %s: %s", key, value, strings.TrimSpace(string(text)))
	}
	return nil
}

// AddConfig adds a value to a key of the repository's own config.
func (g Git) AddConfig(ctx context.Context, key, value string) error {
	return g.git().Command(ctx, "config", "--local", "--add", key, value).Run()
}

// ConfigHooksRun is whether this git runs the hooks its config declares: git
// hook list names a hook declared for the one command. A git without them
// has no git hook list, or lists only the hooks folder's.
func (g Git) ConfigHooksRun(context.Context) bool {
	const probe = "itos-probe"
	listed, err := g.git().Output("-C", ".", "-c", "hook."+probe+".event=commit-msg",
		"-c", "hook."+probe+".command=true", "hook", "list", "commit-msg")
	return err == nil && slices.Contains(strings.Fields(listed), probe)
}

// Version is the version of the git itos runs, as git version prints it
// after those words.
func (g Git) Version(context.Context) string {
	version, _ := g.git().Output("--version")
	return strings.TrimPrefix(strings.TrimSpace(version), "git version ")
}

// Forget has Locate ask git again from every folder: a repository was made
// or removed since (Init, a test's).
func (Git) Forget() {
	locateMu.Lock()
	defer locateMu.Unlock()
	clear(located)
}
