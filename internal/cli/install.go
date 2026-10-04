package cli

// `hooks install [--manager <m>] [--print] [--force]` (hooks.ts's
// hooksInstall): the one-line shims calling the two hooks, for the hook
// manager in use. The manager is the one --manager names, else hooks.manager,
// else the one found by its markers, in this order: Vite+ (a `.vite-hooks/`
// folder, or core.hooksPath set to `.vite-hooks/_` by `vp config`), husky
// (`.husky/`), lefthook (`lefthook.yml`), pre-commit or prek
// (`.pre-commit-config.yaml`), else plain git. Vite+, husky and git keep
// hooks as files, so their shims are written; lefthook and pre-commit keep
// them in their config, so their snippet is printed to add there. A hook file
// that is not a shim is never replaced without --force: the pre-commit hook,
// say, is the project's own. Under a stealth config the manager is the git
// config (gitconfig.go), unless --manager or hooks.manager names another.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/git"
	"github.com/donvargax/itos/v3/internal/out"
	"github.com/donvargax/itos/v3/internal/value"
)

// managerNames are how the output names each manager.
var managerNames = map[string]string{
	"vp":         "Vite+",
	"git":        "plain git",
	"husky":      "husky",
	"lefthook":   "lefthook",
	"pre-commit": "pre-commit",
	"prek":       "prek",
	"git-config": "the git config",
}

// lefthookFiles are the files lefthook reads its config from.
var lefthookFiles = []string{"lefthook.yml", "lefthook.yaml", ".lefthook.yml", ".lefthook.yaml"}

// hooksPath is the repository's own core.hooksPath, "" when it has none.
func hooksPath(root string) string {
	v, err := git.Output("-C", root, "config", "--local", "core.hooksPath")
	if err != nil {
		return ""
	}
	return value.Trim(v)
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// absolute is a path resolved against root, as Node's resolve gives it.
func absolute(root, p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

var trailingSlashes = regexp.MustCompile(`/+$`)

// pointsAt is whether core.hooksPath names a manager's folder.
func pointsAt(hooks, root, dir string) bool {
	return hooks != "" && (absolute(root, hooks) == absolute(root, dir) ||
		strings.HasSuffix(trailingSlashes.ReplaceAllString(hooks, ""), dir))
}

// foundManager is a hook manager and what says it is the one: its marker,
// or the flag or key that named it.
type foundManager struct {
	manager, marker string
	named           bool
}

// detectManager is the hook manager in use under root, by its markers.
func detectManager(root string) foundManager {
	hooks := hooksPath(root)
	switch {
	case isDir(filepath.Join(root, ".vite-hooks")):
		return foundManager{"vp", ".vite-hooks/", false}
	case pointsAt(hooks, root, ".vite-hooks/_"):
		return foundManager{"vp", "core.hooksPath " + hooks, false}
	case isDir(filepath.Join(root, ".husky")):
		return foundManager{"husky", ".husky/", false}
	case pointsAt(hooks, root, ".husky/_"):
		return foundManager{"husky", "core.hooksPath " + hooks, false}
	}
	for _, f := range lefthookFiles {
		if exists(filepath.Join(root, f)) {
			return foundManager{"lefthook", f, false}
		}
	}
	if exists(filepath.Join(root, ".pre-commit-config.yaml")) {
		return foundManager{"pre-commit", ".pre-commit-config.yaml", false}
	}
	return foundManager{"git", "no hook manager's marker", false}
}

// chosenManager is the manager --manager names, else the one hooks.manager
// names, else the git config for a stealth config, whose hooks must not touch
// the project's, else the one its markers show.
func chosenManager(flag string, cfg *config.Loaded, file, root string) foundManager {
	if flag != "" {
		return foundManager{flag, "--manager " + flag, true}
	}
	if m := cfg.Hooks.Manager; m != nil && *m != "" {
		return foundManager{*m, "hooks.manager " + *m, true}
	}
	if cfg.Stealth {
		return foundManager{"git-config", "stealth config " + file, true}
	}
	return detectManager(root)
}

// shimNames are the two hooks, in the order they are written.
var shimNames = []string{"commit-msg", "pre-push"}

// shimLine is a hook's one-line body.
func shimLine(name, bin string) string {
	if name == "commit-msg" {
		return "exec " + bin + ` hook commit-msg "$1"`
	}
	return "exec " + bin + ` hook pre-push "$@"`
}

var callsItos = regexp.MustCompile(`\bitos hook (commit-msg|pre-push)\b`)

// isShim is whether a hook file only calls itos: one line, besides comments
// and a shebang.
func isShim(text string) bool {
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if t := value.Trim(l); t != "" && !strings.HasPrefix(t, "#") {
			lines = append(lines, l)
		}
	}
	return len(lines) == 1 && callsItos.MatchString(lines[0])
}

// hookDir is where a file manager keeps its hooks, relative to root.
func hookDir(manager, root string) (string, error) {
	switch manager {
	case "vp":
		return ".vite-hooks", nil
	case "husky":
		return ".husky", nil
	}
	dir, err := git.Read("-C", root, "rev-parse", "--git-path", "hooks")
	return value.Trim(dir), err
}

// snippet is what a config-file manager takes.
func snippet(manager, bin string) string {
	if manager == "lefthook" {
		return `commit-msg:
  commands:
    itos:
      run: ` + bin + ` hook commit-msg {1}
pre-push:
  commands:
    itos:
      run: ` + bin + ` hook pre-push {1} {2}
      use_stdin: true`
	}
	return `default_install_hook_types: [pre-commit, commit-msg, pre-push]
repos:
  - repo: local
    hooks:
      - id: itos-commit-msg
        name: itos hook commit-msg
        entry: ` + bin + ` hook commit-msg
        language: system
        stages: [commit-msg]
      - id: itos-pre-push
        name: itos hook pre-push
        entry: ` + bin + ` hook pre-push
        language: system
        stages: [pre-push]
        pass_filenames: false
        always_run: true`
}

// readIf is a file's text, and whether it is there.
func readIf(p string) (string, bool) {
	text, err := os.ReadFile(p)
	return string(text), err == nil
}

// printSnippet is lefthook's, pre-commit's or prek's snippet to add to their
// config.
func printSnippet(found foundManager, root, bin string, o Out, say func(string)) (int, error) {
	file := ".pre-commit-config.yaml"
	if found.manager == "lefthook" {
		file = lefthookFiles[0]
		for _, f := range lefthookFiles {
			if exists(filepath.Join(root, f)) {
				file = f
				break
			}
		}
	}
	text := snippet(found.manager, bin)
	current, _ := readIf(filepath.Join(root, file))
	installed := strings.Contains(current, bin+" hook commit-msg")
	if o.JSON {
		return 0, out.Emit(o.Stdout,
			out.Field{Key: "manager", Value: found.manager},
			out.Field{Key: "marker", Value: found.marker},
			out.Field{Key: "file", Value: file},
			out.Field{Key: "snippet", Value: text},
			out.Field{Key: "installed", Value: installed})
	}
	if installed {
		say(file + " already calls itos; its snippet:")
	} else {
		say("Add this to " + file + ":")
	}
	fmt.Fprintln(o.Stdout, text)
	return 0, nil
}

// shimFile is one hook file and what becomes of it.
type shimFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Action  string `json:"action"`
}

// place is what becomes of one hook file: printed, left as it is, refused,
// or written (executable for plain git, as a new file).
func place(full, content string, print, force, executable bool) (string, error) {
	current, there := readIf(full)
	switch {
	case print:
		return "printed", nil
	case there && current == content:
		return "unchanged", nil
	}
	foreign := there && !isShim(current)
	if foreign && !force {
		return "refused", nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o777); err != nil {
		return "", err
	}
	mode := os.FileMode(0o666)
	if executable {
		mode = 0o755
	}
	if err := os.WriteFile(full, []byte(content), mode); err != nil {
		return "", err
	}
	if foreign {
		return "replaced", nil
	}
	return "wrote", nil
}

// hooksInstall is `hooks install`: 0 when every shim is in place (or
// printed), 1 when a hook that is not a shim stood in the way and --force was
// not given.
func hooksInstall(flag string, print, force bool, o Out) (int, error) {
	const root = "."
	file := config.Path()
	cfg, err := config.Load(file)
	if err != nil {
		return 0, err
	}
	bin := cfg.Hooks.Bin
	found := chosenManager(flag, cfg, file, root)
	log := o.Stdout
	if o.JSON {
		log = o.Stderr
	}
	say := func(line string) { fmt.Fprintln(log, line) }
	verb := "Found"
	if found.named {
		verb = "Using"
	}
	say(fmt.Sprintf("%s %s (%s)", verb, managerNames[found.manager], found.marker))
	if found.manager == "git-config" {
		return declareHooks(found, root, bin, cfg.Hooks.PrePush != nil, print, force, o, say)
	}
	if found.manager != "vp" && found.manager != "husky" && found.manager != "git" {
		return printSnippet(found, root, bin, o, say)
	}
	dir, err := hookDir(found.manager, root)
	if err != nil {
		return 0, err
	}
	plain := found.manager == "git"
	files := make([]shimFile, len(shimNames))
	for i, name := range shimNames {
		p := path.Join(dir, name)
		content := shimLine(name, bin) + "\n"
		if plain {
			content = "#!/bin/sh\n" + content
		}
		action, err := place(filepath.Join(root, p), content, print, force, plain)
		if err != nil {
			return 0, err
		}
		files[i] = shimFile{p, content, action}
	}
	if o.JSON {
		if err := out.Emit(o.Stdout,
			out.Field{Key: "manager", Value: found.manager},
			out.Field{Key: "marker", Value: found.marker},
			out.Field{Key: "files", Value: files}); err != nil {
			return 0, err
		}
	}
	report(files, o, say)
	for _, f := range files {
		if f.Action == "refused" {
			return ExitPolicy, nil
		}
	}
	return 0, nil
}

// report prints one line per file, as its action says.
func report(files []shimFile, o Out, say func(string)) {
	for _, f := range files {
		switch {
		case f.Action == "refused":
			fmt.Fprintf(o.Stderr, "%s is not an itos shim; pass --force to replace it\n", f.Path)
		case f.Action != "printed":
			say(f.Action + " " + f.Path)
		case !o.JSON:
			fmt.Fprintf(o.Stdout, "==> %s\n%s\n", f.Path, trimEnd(f.Content))
		}
	}
}

// trimEnd is a text without the white space at its end, as JavaScript's
// trimEnd.
func trimEnd(s string) string {
	return strings.TrimRightFunc(s, func(r rune) bool { return value.Trim(string(r)) == "" })
}
