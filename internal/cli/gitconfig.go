package cli

// `hooks install --manager git-config` (slice 33, features/stealth.feature):
// itos's hooks declared in the repository's own git config, the local one
// that is never committed, as hook.<name>.event and hook.<name>.command. Git
// (2.5x) runs a hook declared there as well as the one in core.hooksPath or
// the hooks folder, so itos's hooks run beside a project's own without
// touching its hook files or its settings, and a hook manager that resets
// core.hooksPath cannot remove them. It is the manager a stealth config
// picks. Each hook is one entry, itos-commit-msg and itos-pre-push (git
// refuses a hook named after its event), whose command is hooks.bin's
// `hook <event>`, git appending the hook's arguments; the pre-push one only
// when hooks.pre_push gives it commands to run, since without them every
// push would fail, and a pre-push entry of itos's is removed when it gives
// none. An entry already as itos would write it is left alone, so running it
// twice changes nothing; one under itos's name that does not call itos is
// replaced only with --force, as a hook file is.

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/donvargax/itos/v3/internal/git"
	"github.com/donvargax/itos/v3/internal/out"
	"github.com/donvargax/itos/v3/internal/value"
)

// configHook is one hook entry of the git config and what becomes of it.
type configHook struct {
	Name    string `json:"name"`
	Event   string `json:"event"`
	Command string `json:"command"`
	Action  string `json:"action"`
}

// hookEntry is the name itos's entry for a hook event has in the git config.
func hookEntry(event string) string { return "itos-" + event }

// section is an entry as the git config file holds it.
func (h configHook) section() string {
	return fmt.Sprintf("[hook %q]\n\tevent = %s\n\tcommand = %s", h.Name, h.Event, h.Command)
}

// configHooksRun is whether this git runs the hooks its config declares: git
// hook list names a hook declared for the one command. A git without them
// has no git hook list, or lists only the hooks folder's.
func configHooksRun(root string) bool {
	const probe = "itos-probe"
	listed, err := git.Output("-C", root, "-c", "hook."+probe+".event=commit-msg",
		"-c", "hook."+probe+".command=true", "hook", "list", "commit-msg")
	return err == nil && slices.Contains(strings.Fields(listed), probe)
}

// localValues are the values of a key in the repository's own config.
func localValues(root, key string) []string {
	got, err := git.Output("-C", root, "config", "--local", "--get-all", key)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(got, "\n"), "\n")
}

// gitConfig runs git config on the repository's own config.
func gitConfig(root string, args ...string) error {
	cmd := exec.Command(git.Bin(), append([]string{"-C", root, "config", "--local"}, args...)...)
	if text, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config %s: %s", strings.Join(args, " "), value.Trim(string(text)))
	}
	return nil
}

// declare is what becomes of one entry: printed, left as it is, refused, or
// written.
func declare(root string, h configHook, print, force bool) (string, error) {
	commands := localValues(root, "hook."+h.Name+".command")
	current := ""
	if len(commands) > 0 {
		current = commands[len(commands)-1]
	}
	switch {
	case print:
		return "printed", nil
	case current == h.Command && slices.Equal(localValues(root, "hook."+h.Name+".event"), []string{h.Event}):
		return "unchanged", nil
	}
	foreign := current != "" && !callsItos.MatchString(current)
	if foreign && !force {
		return "refused", nil
	}
	if err := gitConfig(root, "--replace-all", "hook."+h.Name+".command", h.Command); err != nil {
		return "", err
	}
	if err := gitConfig(root, "--replace-all", "hook."+h.Name+".event", h.Event); err != nil {
		return "", err
	}
	if foreign {
		return "replaced", nil
	}
	return "wrote", nil
}

// undeclare removes itos's entry for a hook it no longer installs, when
// there is one that calls itos.
func undeclare(root string, h configHook, print bool) (string, error) {
	commands := localValues(root, "hook."+h.Name+".command")
	if print || len(commands) == 0 || !callsItos.MatchString(commands[len(commands)-1]) {
		return "", nil
	}
	if err := gitConfig(root, "--remove-section", "hook."+h.Name); err != nil {
		return "", err
	}
	return "removed", nil
}

// declareHooks is `hooks install` for the git config: 0 when every entry is
// in place (or printed), 1 when one under itos's name that does not call
// itos stood in the way, 3 when this git runs no hook its config declares.
func declareHooks(found foundManager, root, bin string, prePush, print, force bool, o Out, say func(string)) (int, error) {
	if !print && !configHooksRun(root) {
		version, _ := git.Output("--version")
		fmt.Fprintf(o.Stderr, "itos: hooks install --manager git-config needs a git that runs the hooks its config declares (git hook list shows them), which %s does not\n",
			strings.TrimPrefix(value.Trim(version), "git version "))
		return ExitMissing, nil
	}
	var hooks []configHook
	for _, event := range shimNames {
		h := configHook{Name: hookEntry(event), Event: event, Command: bin + " hook " + event}
		var action string
		var err error
		if event == "pre-push" && !prePush {
			action, err = undeclare(root, h, print)
		} else {
			action, err = declare(root, h, print, force)
		}
		if err != nil {
			return 0, err
		}
		if action != "" {
			h.Action = action
			hooks = append(hooks, h)
		}
	}
	if o.JSON {
		if err := out.Emit(o.Stdout,
			out.Field{Key: "manager", Value: found.manager},
			out.Field{Key: "marker", Value: found.marker},
			out.Field{Key: "hooks", Value: hooks}); err != nil {
			return 0, err
		}
	}
	code := 0
	for _, h := range hooks {
		switch {
		case h.Action == "refused":
			fmt.Fprintf(o.Stderr, "hook.%s does not call itos; pass --force to replace it\n", h.Name)
			code = ExitPolicy
		case h.Action != "printed":
			say(h.Action + " hook." + h.Name)
		case !o.JSON:
			fmt.Fprintln(o.Stdout, h.section())
		}
	}
	return code, nil
}
