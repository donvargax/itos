package hooks

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
)

// Folder is the hooks folder git runs hook files from, where a git that
// runs no hook its config declares finds itos's (ADR-0058), which the
// caller binds: HooksPath, the folder, absolute, as git rev-parse
// --git-path hooks names it, core.hooksPath's when that is set; HookFile, a
// file's text, its error fs.ErrNotExist when there is none; WriteHookFile,
// a file written executable, its folder made; MoveHookFile, a file renamed.
type Folder interface {
	HooksPath(ctx context.Context) (string, error)
	HookFile(ctx context.Context, path string) (string, error)
	WriteHookFile(ctx context.Context, path, text string) error
	MoveHookFile(ctx context.Context, from, to string) error
}

// marker starts the line that makes a hook file itos's own, so a second
// install rewrites it rather than keeping it as the folder's hook.
const marker = "# itos hook install wrote this file"

// keptSuffix names the hook the folder held, kept beside itos's file:
// commit-msg's is commit-msg.before-itos, a name git runs no hook by.
const keptSuffix = ".before-itos"

// hookScript is itos's hook file, {run} its hook's command: the kept hook
// runs first, when there is one git would run, with the same arguments and
// the same standard input (pre-push's refs), which is saved for both; its
// failure is the hook's exit, and itos's does not run.
const hookScript = `#!/bin/sh
` + marker + ` (a git older than ` + Since + ` runs no hook its config declares).
# A hook this folder held before is kept as {kept}, and runs first with the same input.
kept="$0` + keptSuffix + `"
if [ -x "$kept" ]; then
  input=$(mktemp) || exit 1
  cat > "$input"
  "$kept" "$@" < "$input"
  status=$?
  if [ "$status" -eq 0 ]; then
    {run} < "$input"
    status=$?
  fi
  rm -f "$input"
  exit "$status"
fi
exec {run}
`

// hookFile is itos's hook file for the event, running bin's hook with git's
// arguments, its lines ended by LF alone, which sh reads on every platform.
func hookFile(bin, event string) string {
	return strings.NewReplacer("{run}", runLine(bin, event), "{kept}", event+keptSuffix).Replace(hookScript)
}

// runLine is the line that runs bin's hook for the event with git's
// arguments, the one a project's own hook file adds.
func runLine(bin, event string) string { return bin + " hook " + event + ` "$@"` }

// callsHook is whether a hook file's text runs itos's hook for the event.
func callsHook(text, event string) bool {
	return regexp.MustCompile(`\bitos hook ` + regexp.QuoteMeta(event) + `\b`).MatchString(text)
}

// filesReady is Ready on a git that runs no hook its config declares: nil
// when each event's hook file in the hooks folder git uses calls itos's
// hook, whoever wrote it, else the missing environment, naming hook install.
func filesReady(ctx context.Context, p Ports, events []string) error {
	folder, top, err := hooksFolder(ctx, p)
	if err != nil {
		return kind.Wrap(kind.Missing, err)
	}
	named, _ := fromTop(top, folder)
	var missing []string
	for _, event := range events {
		if text, err := p.HookFile(ctx, filepath.Join(folder, event)); err != nil || !callsHook(text, event) {
			missing = append(missing, event)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return kind.Wrap(kind.Missing, fmt.Errorf("git %s runs no hook its config declares, and no hook file in %s "+
		"calls itos's %s %s, so git would run none of its checks: run itos hook install once in the clone, "+
		"then run this again", p.Version(ctx), named, and(missing), plural(missing, "hook", "hooks")))
}

// hooksFolder is the folder git runs hook files from and the work tree's
// top, "" where there is none (a bare repository).
func hooksFolder(ctx context.Context, p Ports) (folder, top string, err error) {
	if folder, err = p.HooksPath(ctx); err != nil {
		return "", "", err
	}
	l, _ := p.Locate(ctx, "")
	return folder, l.Top, nil
}

// fromTop is path as itos names it, from the top with forward slashes when
// it is under top, else as it is, and whether it is in the work tree: under
// top and not in the git folder there.
func fromTop(top, path string) (string, bool) {
	rel, err := filepath.Rel(top, path)
	if top == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(path), false
	}
	rel = filepath.ToSlash(rel)
	first, _, _ := strings.Cut(rel, "/")
	return rel, first != ".git"
}

// fileHook is one hook file and what becomes of it, with the refusal
// that stops the install when it is refused: its Name is its path from the
// top, with forward slashes, as git names paths on every platform, and
// inTree whether it is in the work tree.
type fileHook struct {
	configHook
	path, text, refusal string
	inTree              bool
}

// planFile is what becomes of the hook file at path, itos's want: printed;
// unchanged, as itos would write it or already calling itos's hook;
// refused, in the work tree and not calling itos (the project's, whether
// git tracks it or not: a file itos wrote there would show in every git
// status, for some commit to carry), or a hook that does not call itos
// where one is kept already; wrote, itos's own rewritten; written, a new
// file; replaced, the hook it held kept beside it, to run first.
func planFile(ctx context.Context, f Folder, h fileHook, bin string, print bool) (string, string, error) {
	if print {
		return "printed", "", nil
	}
	text, err := f.HookFile(ctx, h.path)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", "", err
	}
	switch {
	case exists && text == h.text:
		return "unchanged", "", nil
	case h.inTree:
		if exists && callsHook(text, h.Event) {
			return "unchanged", "", nil
		}
		return "refused", fmt.Sprintf("%s is in the work tree, the project's, so itos writes no file there: add the "+
			"line %s to it, a sh script made if there is none, then run itos hook install again", h.Name,
			runLine(bin, h.Event)), nil
	case !exists:
		return "written", "", nil
	case strings.Contains(text, marker):
		return "wrote", "", nil
	case callsHook(text, h.Event):
		return "unchanged", "", nil
	}
	if _, err := f.HookFile(ctx, h.path+keptSuffix); !errors.Is(err, fs.ErrNotExist) {
		if err != nil {
			return "", "", err
		}
		return "refused", fmt.Sprintf("%s does not call itos, and %s holds the hook kept before it: move one "+
			"of them away, then run itos hook install again", h.Name, h.Name+keptSuffix), nil
	}
	return "replaced", "", nil
}

// writeFiles is Declare on a git that runs no hook its config declares:
// each hook a file in the hooks folder git uses (Folder's HooksPath), as
// planFile decides, a hook the folder held kept beside it and run first.
// When one is refused, exit 1, no file is written and each that is not in
// place already is refused too, so the clone is never half installed.
// --print prints the files and writes nothing; --json is Declare's object,
// each hook named by its file's path.
func writeFiles(ctx context.Context, p Ports, bin string, print bool, o Out, say func(string)) (int, error) {
	folder, top, err := hooksFolder(ctx, p)
	if err != nil {
		return 0, err
	}
	var files []fileHook
	code := 0
	for _, event := range Events {
		path := filepath.Join(folder, event)
		named, inTree := fromTop(top, path)
		h := fileHook{configHook: configHook{Name: named, Event: event, Command: bin + " hook " + event},
			path: path, text: hookFile(bin, event), inTree: inTree}
		if h.Action, h.refusal, err = planFile(ctx, p, h, bin, print); err != nil {
			return 0, err
		}
		if h.Action == "refused" {
			code = exitPolicy
		}
		files = append(files, h)
	}
	for i, h := range files {
		switch {
		case code != 0 && h.Action != "unchanged":
			files[i].Action = "refused"
		case h.Action == "replaced":
			if err := p.MoveHookFile(ctx, h.path, h.path+keptSuffix); err != nil {
				return 0, err
			}
			fallthrough
		case h.Action == "wrote" || h.Action == "written":
			if err := p.WriteHookFile(ctx, h.path, h.text); err != nil {
				return 0, err
			}
		}
	}
	hooks := make([]configHook, len(files))
	for i, h := range files {
		hooks[i] = h.configHook
	}
	if o.JSON {
		if err := out.Emit(o.Stdout, out.Field{Key: "hooks", Value: hooks}); err != nil {
			return 0, err
		}
	}
	for _, h := range files {
		switch {
		case h.refusal != "":
			fmt.Fprintf(o.Stderr, "itos: hook install: %s\n", h.refusal)
		case h.Action == "refused":
		case h.Action == "replaced":
			say("replaced " + h.Name + ", keeping the hook it held as " + h.Name + keptSuffix + ", which runs first")
		case h.Action != "printed":
			say(h.Action + " " + h.Name)
		case !o.JSON:
			fmt.Fprintf(o.Stdout, "==> %s <==\n%s", h.Name, h.text)
		}
	}
	return code, nil
}
