package cli

// itos's hooks in the git config (slice 33, and since slice 91 the only
// place itos installs them, decision 37): the repository's own git config,
// the local one that is never committed and that every worktree of the clone
// shares, declares each hook as hook.<name>.event and hook.<name>.command.
// Git runs a hook declared there as well as the one in core.hooksPath or the
// hooks folder, whatever core.hooksPath says, so itos's hooks run beside a
// project's own without touching its hook files or its settings, and a hook
// manager that resets core.hooksPath, or a hooks folder a fresh worktree
// lacks (issue #16), cannot remove them. Git runs the hooks its config
// declares from 2.54.0 on (configHooksSince). Each hook is one entry,
// itos-commit-msg and itos-pre-push (git refuses a hook named after its
// event), whose command is hooks.bin's `hook <event>`, git appending the
// hook's arguments. An entry already as itos would write it is left alone,
// so running hook install twice changes nothing; one under itos's name that
// does not call itos is replaced only with --force.
//
// Every itos command that commits or pushes asks hooksReady first, and
// refuses with exit 3 where git would run none of itos's checks: a git that
// runs no hook its config declares, or no entry of itos's for the hook the
// command relies on (commit-msg for a commit, pre-push for a push).

import (
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/kind"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/value"
)

// configHooksSince is the first git that runs the hooks its config declares:
// git 2.54.0's release notes (Documentation/RelNotes/2.54.0.adoc) say "Hook
// commands are now allowed to be defined (possibly centrally) in the
// configuration files, and run multiple of them for the same hook event."
const configHooksSince = "2.54.0"

// hookEvents are the two hooks itos declares, in the order it writes them.
var hookEvents = []string{"commit-msg", "pre-push"}

// callsItos is whether a hook's command runs itos's hook.
var callsItos = regexp.MustCompile(`\bitos hook (commit-msg|pre-push)\b`)

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

// gitVersion is the version the git itos runs gives, as "git version"
// prints it after those words.
func gitVersion() string {
	version, _ := git.Output("--version")
	return strings.TrimPrefix(value.Trim(version), "git version ")
}

// declared is whether the git config, at any of its levels, declares itos's
// hook for the event: its entry's event is the event and its command runs
// itos's hook.
func declared(root, event string) bool {
	name := hookEntry(event)
	commands := configValues(root, "", "hook."+name+".command")
	return len(commands) > 0 && callsItos.MatchString(commands[len(commands)-1]) &&
		slices.Contains(configValues(root, "", "hook."+name+".event"), event)
}

// errOldGit is the refusal of a git that runs no hook its config declares.
func errOldGit() error {
	return kind.Wrap(kind.Missing, fmt.Errorf("git %s runs no hook its config declares, so it would run none of "+
		"itos's checks: upgrade git to %s or later", gitVersion(), configHooksSince))
}

// hooksReady is nil when git will run itos's hook for each event, else the
// missing environment (exit 3), naming what to do: a git that runs the hooks
// its config declares, then each event's entry declared in the git config.
// Outside a git repository it is nil: git's own refusal says why there.
func hooksReady(events ...string) error {
	const root = "."
	if !git.Succeeds("-C", root, "rev-parse", "--git-dir") {
		return nil
	}
	if !configHooksRun(root) {
		return errOldGit()
	}
	var missing []string
	for _, event := range events {
		if !declared(root, event) {
			missing = append(missing, event)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	hooks := "hook is"
	if len(missing) > 1 {
		hooks = "hooks are"
	}
	return kind.Wrap(kind.Missing, fmt.Errorf("itos's %s %s not declared in this clone's git config, so git "+
		"would run none of its checks: run itos hook install once in the clone (its worktrees share it), "+
		"then run this again", and(missing), hooks))
}

// configValues are the values of a key in the git config: the repository's
// own when scope is "--local", every level's when it is "".
func configValues(root, scope, key string) []string {
	args := []string{"-C", root, "config"}
	if scope != "" {
		args = append(args, scope)
	}
	got, err := git.Output(append(args, "--get-all", key)...)
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
	commands := configValues(root, "--local", "hook."+h.Name+".command")
	current := ""
	if len(commands) > 0 {
		current = commands[len(commands)-1]
	}
	switch {
	case print:
		return "printed", nil
	case current == h.Command && slices.Equal(configValues(root, "--local", "hook."+h.Name+".event"), []string{h.Event}):
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

// declareHooks declares both hooks, each running bin's `hook <event>`: 0
// when every entry is in place (or printed), 1 when one under itos's name
// that does not call itos stood in the way, 3 when this git runs no hook its
// config declares.
func declareHooks(root, bin string, print, force bool, o Out, say func(string)) (int, error) {
	if !print && !configHooksRun(root) {
		fmt.Fprintf(o.Stderr, "itos: hook install: %s\n", errOldGit())
		return ExitMissing, nil
	}
	var hooks []configHook
	for _, event := range hookEvents {
		h := configHook{Name: hookEntry(event), Event: event, Command: bin + " hook " + event}
		action, err := declare(root, h, print, force)
		if err != nil {
			return 0, err
		}
		h.Action = action
		hooks = append(hooks, h)
	}
	if o.JSON {
		if err := out.Emit(o.Stdout, out.Field{Key: "hooks", Value: hooks}); err != nil {
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
