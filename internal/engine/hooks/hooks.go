package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Since is the first git that runs the hooks its config declares:
// git 2.54.0's release notes (Documentation/RelNotes/2.54.0.adoc) say "Hook
// commands are now allowed to be defined (possibly centrally) in the
// configuration files, and run multiple of them for the same hook event."
const Since = "2.54.0"

// exitPolicy is Declare's refusal, internal/cli's ExitPolicy.
const exitPolicy = 1

// Events are the two hooks itos declares, in the order it writes them.
var Events = []string{"commit-msg", "pre-push"}

// callsItos is whether a hook's command runs itos's hook.
var callsItos = regexp.MustCompile(`\bitos hook (commit-msg|pre-push)\b`)

// Git is the git whose config declares the hooks, which the VCS port leaves
// to the adapter (internal/vcs/adapter's Git is one): the repository, its
// config, whether this git runs the hooks its config declares, and its
// version, as git version prints it after those words.
type Git interface {
	Locate(ctx context.Context, dir string) (vcsport.Location, error)
	Config(ctx context.Context, key string, local bool) []string
	SetConfig(ctx context.Context, key, value string) error
	ConfigHooksRun(ctx context.Context) bool
	Version(ctx context.Context) string
}

// configHook is one hook entry of the git config and what becomes of it.
type configHook struct {
	Name    string `json:"name"`
	Event   string `json:"event"`
	Command string `json:"command"`
	Action  string `json:"action"`
}

// Entry is the name itos's entry for a hook event has in the git config.
func Entry(event string) string { return "itos-" + event }

// section is an entry as the git config file holds it.
func (h configHook) section() string {
	return fmt.Sprintf("[hook %q]\n\tevent = %s\n\tcommand = %s", h.Name, h.Event, h.Command)
}

// Declared is whether the git config, at any of its levels, declares itos's
// hook for the event: its entry's event is the event and its command runs
// itos's hook.
func Declared(ctx context.Context, g Git, event string) bool {
	name := Entry(event)
	commands := g.Config(ctx, "hook."+name+".command", false)
	return len(commands) > 0 && callsItos.MatchString(commands[len(commands)-1]) &&
		slices.Contains(g.Config(ctx, "hook."+name+".event", false), event)
}

// Ports are the git whose config declares the hooks and the hooks folder a
// git that runs none of them runs hook files from.
type Ports interface {
	Git
	Folder
}

// Ready is nil when git will run itos's hook for each event, else the
// missing environment (exit 3), naming what to do: each event's entry
// declared in the git config, or, on a git that runs no hook its config
// declares, each event's hook file calling itos (filesReady). Outside a git
// repository it is nil: git's own refusal says why there.
func Ready(ctx context.Context, p Ports, events ...string) error {
	if _, err := p.Locate(ctx, ""); err != nil {
		return nil
	}
	if !p.ConfigHooksRun(ctx) {
		return filesReady(ctx, p, events)
	}
	var missing []string
	for _, event := range events {
		if !Declared(ctx, p, event) {
			missing = append(missing, event)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return kind.Wrap(kind.Missing, fmt.Errorf("itos's %s %s not declared in this clone's git config, so git "+
		"would run none of its checks: run itos hook install once in the clone (its worktrees share it), "+
		"then run this again", and(missing), plural(missing, "hook is", "hooks are")))
}

// plural is one for a single name, else many.
func plural(names []string, one, many string) string {
	if len(names) > 1 {
		return many
	}
	return one
}

// and joins names as a sentence lists them: "a", "a and b", "a, b and c"
// (internal/cli's and).
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// declareHook is what becomes of one entry: printed, left as it is,
// refused, or written.
func declareHook(ctx context.Context, g Git, h configHook, print, force bool) (string, error) {
	commands := g.Config(ctx, "hook."+h.Name+".command", true)
	current := ""
	if len(commands) > 0 {
		current = commands[len(commands)-1]
	}
	switch {
	case print:
		return "printed", nil
	case current == h.Command && slices.Equal(g.Config(ctx, "hook."+h.Name+".event", true), []string{h.Event}):
		return "unchanged", nil
	}
	foreign := current != "" && !callsItos.MatchString(current)
	if foreign && !force {
		return "refused", nil
	}
	if err := g.SetConfig(ctx, "hook."+h.Name+".command", h.Command); err != nil {
		return "", err
	}
	if err := g.SetConfig(ctx, "hook."+h.Name+".event", h.Event); err != nil {
		return "", err
	}
	if foreign {
		return "replaced", nil
	}
	return "wrote", nil
}

// Out is where Declare writes: the entries, or under JSON the object, to
// Stdout; a refusal to Stderr.
type Out struct {
	Stdout, Stderr io.Writer
	JSON           bool
}

// Declare declares both hooks, each running bin's `hook <event>`: 0
// when every entry is in place (or printed), 1 when one under itos's name
// that does not call itos stood in the way. On a git that runs no hook its
// config declares it writes hook files instead (writeFiles). say takes each
// entry's line but a printed one.
func Declare(ctx context.Context, p Ports, bin string, print, force bool, o Out, say func(string)) (int, error) {
	if !p.ConfigHooksRun(ctx) {
		return writeFiles(ctx, p, bin, print, o, say)
	}
	g := Git(p)
	var hooks []configHook
	for _, event := range Events {
		h := configHook{Name: Entry(event), Event: event, Command: bin + " hook " + event}
		action, err := declareHook(ctx, g, h, print, force)
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
			code = exitPolicy
		case h.Action != "printed":
			say(h.Action + " hook." + h.Name)
		case !o.JSON:
			fmt.Fprintln(o.Stdout, h.section())
		}
	}
	return code, nil
}

// PathSet is whether git's command line sets core.hooksPath for every
// git this itos runs (slice 128): git -c core.hooksPath=<folder>, which the
// git shim hands on in GIT_CONFIG_KEY_<n>, or a git that started itos in
// GIT_CONFIG_PARAMETERS. The key is read as git reads it, in any case.
func PathSet() bool {
	n, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	for i := 0; i < n; i++ {
		if strings.EqualFold(os.Getenv("GIT_CONFIG_KEY_"+strconv.Itoa(i)), "core.hooksPath") {
			return true
		}
	}
	return hooksPathParameter.MatchString(os.Getenv("GIT_CONFIG_PARAMETERS"))
}

// hooksPathParameter finds core.hooksPath among GIT_CONFIG_PARAMETERS's
// entries, each 'key'='value' or 'key=value', shell-quoted and separated by
// spaces, so a quote inside a value is never one an entry starts with.
var hooksPathParameter = regexp.MustCompile(`(?i)(^|\s)'core\.hookspath['=]`)
