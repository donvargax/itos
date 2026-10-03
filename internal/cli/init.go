package cli

// itos init (slice 48, features/init.feature): a repository made ready for
// itos in one command, and run again, a report of what is missing.
//
// It runs at the repository's top, after git init where the folder is no
// repository yet. Where no config is there (itos.yaml at the top, or the
// stealth one), it writes a starter (starter.go): the config, a ledger and a
// work registry under tasks/ and, when features/ holds feature files, a smoke
// set; then pins the newest release as itos pin does, where the release
// server answers; then installs the hooks as hooks install does. With
// --stealth all of it goes beside the stealth config in the git folder and
// the hooks into the git config, so nothing tracked changes. Where a config
// is there, it writes nothing: it reports what config check finds wrong and
// each hook that does not call itos, naming the command that fixes it, exit 1
// when anything is missing and 0 when nothing is.
//
// It is the launcher's own command, as pin is (internal/launch): where there
// is no config there is no pin to hand the run to, and the newest release
// must not run in its place.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/release"
	"github.com/donvargax/itos/v2/internal/tests"
	"github.com/donvargax/itos/v2/internal/value"
)

// initCommand is `init [--stealth] [--plugin [<scope>]]`.
func initCommand(args []string, o Out) (int, error) {
	stealth := false
	var plugin pluginFlag
	for i := 0; i < len(args); i++ {
		n, err := parsePluginFlag(args, i, &plugin)
		switch {
		case err != nil:
			return 0, err
		case n > 0:
			i += n - 1
		case args[i] == "--stealth":
			stealth = true
		default:
			// The words of slice 48, which the last release's corpus holds
			// (T-071): naming --plugin here would fail it as a breaking change.
			return 0, usage("init takes only --stealth (%s)", args[i])
		}
	}
	if stealth && plugin.scope == "project" {
		return 0, pluginRefused()
	}
	log := o.Stdout
	if o.JSON {
		log = o.Stderr
	}
	initialized, err := atTop(log)
	if err != nil {
		return 0, err
	}
	if file := config.Path(); exists(file) {
		stealth = stealth || config.IsStealth(file)
		if stealth && plugin.scope == "project" {
			return 0, pluginRefused()
		}
		return initReport(file, pluginOffer{flag: plugin, stealth: stealth}, o)
	}
	if named := os.Getenv("ITOS_CONFIG"); named != "" {
		return 0, usage("init writes itos.yaml at the repository's top, or with --stealth in the git folder, "+
			"not the missing %s that --config or ITOS_CONFIG names", named)
	}
	offer := pluginOffer{flag: plugin, stealth: stealth, ask: !o.JSON && onTerminal(), answers: os.Stdin}
	return initWrite(stealth, initialized, offer, log, o)
}

// atTop moves to the top of the repository the folder is in, after git init
// when it is in none, and says whether it ran git init.
func atTop(log io.Writer) (bool, error) {
	if _, err := git.Output("rev-parse", "--git-dir"); err != nil {
		cmd := exec.Command(git.Bin(), "init", "-q")
		if text, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("git init: %s", value.Trim(string(text)))
		}
		here, _ := os.Getwd()
		fmt.Fprintf(log, "Ran git init: %s is a git repository now.\n", here)
		return true, nil
	}
	top, err := git.Output("rev-parse", "--show-toplevel")
	if top = value.Trim(top); err != nil || top == "" {
		return false, errors.New("init runs in a repository's working tree, not in its git folder")
	}
	return false, os.Chdir(top)
}

// writtenFile is a file init wrote, or found there and kept.
type writtenFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// initWrite writes the starter, pins the newest release, installs the hooks
// and makes the plugin's offer; its exit code is hooks install's, else 1 when
// claude failed at installing the plugin asked for.
func initWrite(stealth, initialized bool, offer pluginOffer, log io.Writer, o Out) (int, error) {
	file := "itos.yaml"
	if stealth {
		common, err := git.Read("rev-parse", "--git-common-dir")
		if err != nil {
			return 0, err
		}
		file = filepath.Join(value.Trim(common), config.StealthFolder, "itos.yaml")
	}
	beside := func(p string) string {
		if stealth {
			return filepath.Join(filepath.Dir(file), p)
		}
		return p
	}
	since := ""
	if head, err := git.Output("rev-parse", "--verify", "--quiet", "HEAD^{commit}"); err == nil {
		since = value.Trim(head)
	}
	s := starter{stealth: stealth, since: since, scenarios: hasFeatureFiles(starterFeatures)}
	version, sums, pinErr := pinned("")
	if pinErr == nil {
		s.pin = &[2]string{version, release.SHA256(sums)}
	}

	var files []writtenFile
	write := func(p, text string) error {
		action := "kept"
		if !exists(p) {
			if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte(text), 0o666); err != nil {
				return err
			}
			action = "wrote"
		}
		files = append(files, writtenFile{filepath.ToSlash(p), action})
		return nil
	}
	if err := write(file, s.config()); err != nil {
		return 0, err
	}
	if err := write(beside(starterLedger), s.ledger()); err != nil {
		return 0, err
	}
	if err := write(beside(starterRegistry), starterRegistryText); err != nil {
		return 0, err
	}
	smokeIDs := 0
	if s.scenarios {
		cfg, err := config.Load(file)
		if err != nil {
			return 0, err
		}
		list, err := tests.ListTests(cfg, "scenario", "worktree")
		if err != nil {
			return 0, err
		}
		text, n := starterSmokeSet(list)
		smokeIDs = n
		if err := write(beside(starterSmoke), text); err != nil {
			return 0, err
		}
	}

	if !o.JSON {
		say := func(format string, a ...any) { fmt.Fprintf(log, format+"\n", a...) }
		for _, f := range files {
			say("%s %s", f.Action, f.Path)
		}
		switch {
		case since != "":
			say("commits.since is HEAD, %s: no commit before it is judged.", since[:7])
		default:
			say("The repository has no commit, so the config has no commits.since: every commit is judged.")
		}
		if s.scenarios {
			say("The smoke set names %s, one of each feature file with a live one; "+
				"a feat or a fix names the ones it turns green (itos commit --scenarios).", count(smokeIDs, "scenario"))
		}
		if pinErr == nil {
			say("Pinned itos %s, the newest release.", version)
		} else {
			say("Pinned no itos: %s. Run itos pin when the release server can be reached.", pinErr)
		}
	}

	hooksCode, hooks, err := initHooks(o)
	if err != nil {
		return 0, err
	}
	offer.log = log
	if o.JSON {
		offer.log = io.Discard
	}
	plugin, pluginCode := offer.run()
	code := hooksCode
	if code == 0 {
		code = pluginCode
	}
	if o.JSON {
		var at, pin any
		if since != "" {
			at = since
		}
		fields := []out.Field{{Key: "config", Value: filepath.ToSlash(file)}, {Key: "action", Value: "initialized"},
			{Key: "git_init", Value: initialized}, {Key: "since", Value: at}, {Key: "files", Value: files}}
		if s.pin != nil {
			pin = map[string]string{"version": s.pin[0], "checksums": s.pin[1]}
		}
		fields = append(fields, out.Field{Key: "pin", Value: pin})
		if pinErr != nil {
			fields = append(fields, out.Field{Key: "pin_problem", Value: pinErr.Error()})
		}
		fields = append(fields, out.Field{Key: "hooks", Value: hooks}, out.Field{Key: "plugin", Value: plugin})
		return code, out.Emit(o.Stdout, fields...)
	}
	if hooksCode != 0 {
		return hooksCode, nil
	}
	if stealth {
		fmt.Fprintln(log, "Nothing the project tracks changed: commit as ever, with itos commit, and push with itos push.")
	} else {
		var paths []string
		task := "<a task of the ledger>"
		for _, f := range files {
			if f.Action == "wrote" {
				paths = append(paths, f.Path)
				if f.Path == starterLedger {
					task = "T-1"
				}
			}
		}
		// The plugin installed for the project is declared in its settings,
		// which the commit carries.
		if plugin.Action == "installed" && plugin.Scope == "project" {
			if status, _ := git.Output("status", "--porcelain", "--", projectSettings); status != "" {
				paths = append(paths, projectSettings)
			}
		}
		fmt.Fprintf(log, "Commit what init wrote with the task that adopts itos: git add %s, then itos commit --task %s -m 'chore: adopt itos'\n",
			strings.Join(paths, " "), task)
	}
	return code, nil
}

// initHooks runs hooks install on the config just written; under --json its
// object, without its schema, for init's own.
func initHooks(o Out) (int, any, error) {
	if !o.JSON {
		code, err := hooksInstall("", false, false, o)
		return code, nil, err
	}
	var buf bytes.Buffer
	code, err := hooksInstall("", false, false, Out{JSON: true, Quiet: o.Quiet, Stdout: &buf, Stderr: o.Stderr})
	if err != nil || buf.Len() == 0 {
		return code, nil, err
	}
	// Its keys in the order it wrote them, its schema left out (out.Emit
	// writes it first).
	hooks := json.RawMessage(bytes.Replace(buf.Bytes(), []byte(`"schema": 1,`), nil, 1))
	if !json.Valid(hooks) {
		return 0, nil, fmt.Errorf("hooks install wrote no JSON object: %s", buf.Bytes())
	}
	return code, hooks, nil
}

// hasFeatureFiles is whether a folder holds a feature file, at any depth.
func hasFeatureFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".feature") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// starterSmokeSet is a smoke set naming each feature file's first live test, and
// how many it names.
func starterSmokeSet(list tests.List) (string, int) {
	var b strings.Builder
	b.WriteString(starterSmokeHeader)
	seen := map[string]bool{}
	n := 0
	for _, t := range list.Tests {
		if !t.Live || seen[t.File] {
			continue
		}
		seen[t.File] = true
		n++
		fmt.Fprintf(&b, "- file: %s\n  scenarios:\n    - id: \"@%s\"\n      why: picked by itos init, the file's first live scenario\n",
			t.File, t.ID)
	}
	if n == 0 {
		b.WriteString("[]\n")
	}
	return b.String(), n
}

// initReport is init where a config is there: it writes nothing and lists
// what is missing, exit 1 when anything is, then makes the plugin's offer,
// which never asks, and installs it only for --plugin; a plugin not installed
// is reported, never counted as missing.
func initReport(file string, offer pluginOffer, o Out) (int, error) {
	_, found, _, _, err := configFindings("")
	if err != nil {
		return 0, err
	}
	if cfg, err := config.Load(file); err == nil {
		found = append(found, tagged("hooks", hookProblems(cfg, file))...)
	}
	code := 0
	if len(found) > 0 {
		code = ExitPolicy
	}
	if o.JSON {
		if found == nil {
			found = []Found{}
		}
		offer.log = io.Discard
		plugin, pluginCode := offer.run()
		return max(code, pluginCode), out.Emit(o.Stdout, out.Field{Key: "config", Value: file}, out.Field{Key: "action", Value: "checked"},
			out.Field{Key: "missing", Value: found}, out.Field{Key: "plugin", Value: plugin})
	}
	fmt.Fprintf(o.Stdout, "%s is there already, so init wrote nothing.\n", file)
	if len(found) == 0 {
		fmt.Fprintln(o.Stdout, "Nothing is missing: the config, its ledger, registry and smoke sets are sound, and the hooks call itos.")
	} else {
		fmt.Fprintln(o.Stdout, "Missing:")
		for _, f := range found {
			line := f.Message
			if f.Fix != "" {
				line += "; " + f.Fix
			}
			fmt.Fprintf(o.Stdout, "  %s\n", line)
		}
	}
	offer.log = o.Stdout
	_, pluginCode := offer.run()
	return max(code, pluginCode), nil
}

// hookProblems are the hooks hooks install would put in place that do not
// call itos, for the manager it would pick: a shim file missing, not
// executable for plain git (outside Windows, which has no executable bit and
// runs a hook whatever its mode) or not calling itos; a lefthook or pre-commit
// config without itos's snippet; an entry of the git config missing. The
// pre-push one under the git config only when hooks.pre_push gives it
// commands, as hooks install declares it.
func hookProblems(cfg *config.Loaded, file string) []out.Problem {
	const root = "."
	bin := cfg.Hooks.Bin
	found := chosenManager("", cfg, file, root)
	install := "run itos hooks install"
	var problems []out.Problem
	missing := func(event, message, fix string) {
		problems = append(problems, out.Problem{Rule: "hook-missing", Message: "the " + event + " hook: " + message, Fix: fix})
	}
	switch found.manager {
	case "git-config":
		if !configHooksRun(root) {
			missing("commit-msg", "this git runs no hook its config declares (git hook list shows none)",
				"use a git that runs them, then "+install)
			return problems
		}
		for _, event := range shimNames {
			if event == "pre-push" && cfg.Hooks.PrePush == nil {
				continue
			}
			name := hookEntry(event)
			commands := localValues(root, "hook."+name+".command")
			if len(commands) == 0 || !callsItos.MatchString(commands[len(commands)-1]) {
				missing(event, "hook."+name+" in the git config does not run itos", install)
			}
		}
	case "lefthook", "pre-commit", "prek":
		config := ".pre-commit-config.yaml"
		if found.manager == "lefthook" {
			config = lefthookFiles[0]
			for _, f := range lefthookFiles {
				if exists(filepath.Join(root, f)) {
					config = f
					break
				}
			}
		}
		text, _ := readIf(filepath.Join(root, config))
		for _, event := range shimNames {
			if !strings.Contains(text, bin+" hook "+event) {
				missing(event, config+" does not call itos", install+" and add the snippet it prints")
			}
		}
	default:
		dir, err := hookDir(found.manager, root)
		if err != nil {
			missing("commit-msg", "the hooks folder is unknown ("+err.Error()+")", install)
			return problems
		}
		for _, event := range shimNames {
			p := path.Join(dir, event)
			info, err := os.Stat(filepath.Join(root, p))
			text, _ := readIf(filepath.Join(root, p))
			switch {
			case err != nil:
				missing(event, p+" is not there", install)
			case !callsItos.MatchString(text):
				missing(event, p+" does not call itos", install+" --force to replace it, or call "+bin+" hook "+event+" from it")
			case found.manager == "git" && runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0:
				missing(event, p+" is not executable", install+" --force, or chmod +x "+p)
			}
		}
	}
	return problems
}

// count is a number of things, the noun in the plural unless there is one.
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
