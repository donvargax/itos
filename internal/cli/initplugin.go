package cli

// itos init's offer of the itos plugin for Claude Code (slice 49,
// features/init.feature;
// docs/decisions/0031-itos-init-offers-the-plugin-and-the-git-shim-opt-in-everywhere.md),
// made through Claude Code's own command line, the claude on the PATH: claude plugin list --json
// says whether itos@itos is installed (any scope, enabled where init runs),
// and claude plugin marketplace add donvargax/itos then claude plugin install
// itos@itos, both with --scope, install it.
//
// The offer is opt-in everywhere (the user's call, 2026-10-03). --plugin
// <scope> answers it: project (the committed .claude/settings.json), user
// (every repository of the person's), local (.claude/settings.local.json,
// this repository and this person alone) or no; a bare --plugin takes the
// mode's default, local under --stealth and project otherwise, and --stealth
// refuses project, which would change a file the project tracks. On a
// terminal with no --plugin, the first run asks, its default that same scope;
// anywhere else (an agent, CI) it installs nothing and says how to. Run again
// where a config is, it never asks: a plugin not installed is reported with
// itos init --plugin, never counted as missing, since it is an offer, and a
// --plugin given there installs it all the same, the flag being the ask.
// With no claude on the PATH there is nothing to offer: init says so only when
// --plugin asked for it.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v2/internal/git"
)

const (
	// pluginID is the plugin as Claude Code names it, <plugin>@<marketplace>.
	pluginID = "itos@itos"
	// pluginMarketplace is the marketplace that publishes it, this repository.
	pluginMarketplace = "donvargax/itos"
	// localSettings is the file Claude Code writes for the local scope.
	localSettings = ".claude/settings.local.json"
	// projectSettings is the one it writes for the project scope, committed.
	projectSettings = ".claude/settings.json"
)

// pluginScopes are --plugin's answers.
var pluginScopes = []string{"project", "user", "local", "no"}

// pluginFlag is how init's command line answered the offer: given is whether
// it has --plugin, scope its value, "" for a bare one.
type pluginFlag struct {
	given bool
	scope string
}

// parsePluginFlag reads --plugin out of init's arguments at i: --plugin
// <scope>, --plugin=<scope>, or a bare --plugin, the next argument being a
// flag or none. It gives how many arguments it read, 0 when args[i] is not
// --plugin.
func parsePluginFlag(args []string, i int, f *pluginFlag) (int, error) {
	a := args[i]
	if value, ok := strings.CutPrefix(a, "--plugin="); ok {
		if !slices.Contains(pluginScopes, value) {
			return 0, usage("--plugin takes project, user, local or no (%s)", value)
		}
		*f = pluginFlag{true, value}
		return 1, nil
	}
	if a != "--plugin" {
		return 0, nil
	}
	*f = pluginFlag{given: true}
	if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
		return 1, nil
	}
	if !slices.Contains(pluginScopes, args[i+1]) {
		return 0, usage("--plugin takes project, user, local or no (%s)", args[i+1])
	}
	f.scope = args[i+1]
	return 2, nil
}

// pluginDefault is a bare --plugin's scope, and the answer a terminal's
// question defaults to.
func pluginDefault(stealth bool) string {
	if stealth {
		return "local"
	}
	return "project"
}

// pluginRefused is the usage error of --plugin project under --stealth.
func pluginRefused() error {
	return usage("under --stealth, --plugin project would write %s, which the project commits; "+
		"--plugin local installs it for you alone in this repository, --plugin user for every one of yours", projectSettings)
}

// pluginOutcome is what came of the offer, init's --json "plugin": action
// one of installed, already, offered (not installed, and nobody asked),
// declined, no_claude, unknown (claude plugin list failed, nobody asked) or
// failed (exit 1); scope the one it is installed at, or was asked for, null
// for none; excluded whether init listed .claude/settings.local.json in
// .git/info/exclude; problem claude's output when it failed.
type pluginOutcome struct {
	Action   string `json:"action"`
	Scope    any    `json:"scope"`
	Excluded bool   `json:"excluded"`
	Problem  string `json:"problem,omitempty"`
}

// pluginEntry is one plugin of claude plugin list --json, the fields init
// reads.
type pluginEntry struct {
	ID      string `json:"id"`
	Scope   string `json:"scope"`
	Enabled bool   `json:"enabled"`
}

// installedPlugin is itos's entry in claude plugin list --json's output,
// when it is installed and enabled where init runs.
func installedPlugin(list []byte) (pluginEntry, bool, error) {
	var entries []pluginEntry
	if err := json.Unmarshal(list, &entries); err != nil {
		return pluginEntry{}, false, fmt.Errorf("its output is no JSON list of plugins: %v", err)
	}
	for _, e := range entries {
		if e.ID == pluginID && e.Enabled {
			return e, true, nil
		}
	}
	return pluginEntry{}, false, nil
}

// pluginOffer is the offer's context: the flag, the mode, whether a terminal
// may be asked (the first run, on a terminal, without --json), where it says
// what it did, and where an answer is read from.
type pluginOffer struct {
	flag    pluginFlag
	stealth bool
	ask     bool
	log     io.Writer
	answers io.Reader
}

// run makes the offer at the repository's top, and gives its outcome and its
// exit code: 1 when claude failed at what --plugin or the person asked for,
// else 0.
func (p pluginOffer) run() (pluginOutcome, int) {
	say := func(format string, a ...any) { fmt.Fprintf(p.log, format+"\n", a...) }
	scope := p.flag.scope
	if p.flag.given && scope == "" {
		scope = pluginDefault(p.stealth)
	}
	if scope == "no" {
		return pluginOutcome{Action: "declined"}, 0
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		if !p.flag.given {
			return pluginOutcome{Action: "no_claude"}, 0
		}
		say("Claude Code was not found (no claude on the PATH), so the itos plugin for it is not installed; "+
			"with Claude Code installed, itos init --plugin %s installs it.", scope)
		return pluginOutcome{Action: "no_claude", Scope: scope}, 0
	}
	runClaude := func(args ...string) (string, error) {
		cmd := exec.Command(claude, args...)
		var output strings.Builder
		cmd.Stdout, cmd.Stderr = &output, &output
		err := cmd.Run()
		return output.String(), err
	}
	failed := func(scope any, command, output string, err error) (pluginOutcome, int) {
		problem := fmt.Sprintf("claude %s failed (%s)", command, failureOf(err))
		if text := strings.TrimSpace(output); text != "" {
			problem += ":\n" + text
		}
		say("%s", problem)
		return pluginOutcome{Action: "failed", Scope: scope, Problem: problem}, ExitPolicy
	}

	list, err := runClaude("plugin", "list", "--json")
	var entry pluginEntry
	installed := false
	if err == nil {
		entry, installed, err = installedPlugin([]byte(list))
	}
	if err != nil {
		if p.flag.given {
			return failed(scope, "plugin list --json", list, err)
		}
		say("claude plugin list --json failed (%s), so init cannot say whether the itos plugin for Claude Code is installed; "+
			"itos init --plugin installs it.", failureOf(err))
		return pluginOutcome{Action: "unknown", Problem: strings.TrimSpace(list)}, 0
	}
	if installed {
		say("The itos plugin for Claude Code, %s, is installed already (its %s scope).", pluginID, entry.Scope)
		return pluginOutcome{Action: "already", Scope: entry.Scope}, 0
	}
	if !p.flag.given {
		if p.ask {
			scope = p.question()
		}
		switch scope {
		case "":
			say("%s", p.howTo())
			return pluginOutcome{Action: "offered"}, 0
		case "no":
			return pluginOutcome{Action: "declined"}, 0
		}
	}

	// The marketplace may be known already, at this scope or another, so its
	// add failing is not the install failing: the install says.
	added, addErr := runClaude("plugin", "marketplace", "add", pluginMarketplace, "--scope", scope)
	if output, err := runClaude("plugin", "install", pluginID, "--scope", scope); err != nil {
		if addErr != nil {
			output = "claude plugin marketplace add " + pluginMarketplace + " --scope " + scope +
				" failed (" + failureOf(addErr) + "): " + strings.TrimSpace(added) + "\n" + output
		}
		return failed(scope, "plugin install "+pluginID+" --scope "+scope, output, err)
	}
	outcome := pluginOutcome{Action: "installed", Scope: scope}
	say("Installed the itos plugin for Claude Code, %s, at its %s scope.", pluginID, scope)
	if p.stealth && scope == "local" {
		excluded, err := excludeIfShown(localSettings)
		if err != nil {
			say("Could not list %s in the git folder's info/exclude: %v", localSettings, err)
		} else if excluded {
			outcome.Excluded = true
			say("Listed %s in the git folder's info/exclude, so git status stays clean.", localSettings)
		}
	}
	return outcome, 0
}

// howTo is what the offer says where nobody was asked.
func (p pluginOffer) howTo() string {
	if p.stealth {
		return "The itos plugin for Claude Code is not installed: itos init --plugin installs it for you alone in this repository " +
			"(" + localSettings + ", kept out of git status), --plugin user for every repository of yours."
	}
	return "The itos plugin for Claude Code is not installed: itos init --plugin installs it for this repository " +
		"(" + projectSettings + ", to commit), --plugin user for every repository of yours, --plugin local for you alone in this one."
}

// question asks the terminal which scope to install the plugin at, until it
// answers one: "" when its input ends first.
func (p pluginOffer) question() string {
	choices := "project (this repository, in " + projectSettings + ", to commit), user (every repository of yours), " +
		"local (you alone, in this one) or no"
	if p.stealth {
		choices = "user (every repository of yours), local (you alone, in this one) or no"
	}
	def := pluginDefault(p.stealth)
	lines := bufio.NewReader(p.answers)
	for {
		fmt.Fprintf(p.log, "Install the itos plugin for Claude Code? %s [%s]: ", choices, def)
		line, err := lines.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		switch {
		case answer == "" && err != nil:
			fmt.Fprintln(p.log)
			return ""
		case answer == "":
			return def
		case answer == "project" && p.stealth:
			fmt.Fprintf(p.log, "Under --stealth the project's %s is left alone.\n", projectSettings)
		case slices.Contains(pluginScopes, answer):
			return answer
		default:
			fmt.Fprintln(p.log, "Answer project, user, local or no.")
		}
		if err != nil {
			return ""
		}
	}
}

// failureOf is how a command's failure is said: its exit code, or why it did
// not run.
func failureOf(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Sprintf("exit %d", exit.ExitCode())
	}
	return err.Error()
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

// excludeIfShown lists a path of the working tree in the git folder's
// info/exclude when git status shows it untracked, so it stays out of git
// status, and says whether it did. Whether git shows it is git's to say: the
// project's own ignore files, or the person's, may already leave it out.
func excludeIfShown(p string) (bool, error) {
	status, err := git.Read("status", "--porcelain", "--untracked-files=all", "--", p)
	if err != nil || !strings.HasPrefix(status, "?? ") {
		return false, err
	}
	file, err := git.Read("rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return false, err
	}
	file = strings.TrimSpace(file)
	text, _ := readIf(file)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
		return false, err
	}
	return true, os.WriteFile(file, []byte(text+"/"+p+"\n"), 0o666)
}
