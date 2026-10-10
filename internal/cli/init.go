package cli

// itos init (slice 48, features/init.feature): a repository made ready for
// itos in one command, and run again, a report of what is missing.
//
// It runs at the repository's top, after git init where the folder is no
// repository yet. Where no config is there (itos.yaml at the top, or the
// stealth one), it writes a starter (starter.go): the config, a ledger and a
// work registry under tasks/ and, when features/ holds feature files, a smoke
// set; then pins the newest release as itos pin does, where the release
// server answers; then declares the hooks in the git config as hook install
// does. With --stealth all of it goes beside the stealth config in the git
// folder, so nothing tracked changes; where the project has its own config,
// --stealth is a usage error and nothing is written. Where a config is
// there, it writes nothing: it reports what config check finds wrong and
// each hook of itos's the git config does not declare, or a git that runs no
// hook its config declares, naming what fixes it, exit 1 when anything is
// missing and 0 when nothing is. With --policy <file> it makes the project
// of a template's policy instead, fresh data and all, and refuses where a
// config or data is there (initpolicy.go).
//
// It is the launcher's own command, as pin is (internal/launch): where there
// is no config there is no pin to hand the run to, and the newest release
// must not run in its place.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/kind"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/release"
	"github.com/donvargax/itos/v7/internal/tests"
	"github.com/donvargax/itos/v7/internal/value"
	"github.com/donvargax/itos/v7/internal/version"
)

// initCommand is `init [--stealth] [--policy <file>] [--plugin [<scope>]]
// [--git-shim [--git-shim-dir <folder>] | --no-git-shim] [--agent-rules |
// --no-agent-rules]`; with --policy, policyInit (initpolicy.go).
func initCommand(args []string, o Out) (int, error) {
	stealth, policy := false, ""
	var plugin pluginFlag
	var shim shimFlag
	var rules rulesFlag
	for i := 0; i < len(args); i++ {
		n, err := parsePluginFlag(args, i, &plugin)
		if err == nil && n == 0 {
			n, err = parseShimFlag(args, i, &shim)
		}
		if err == nil && n == 0 {
			n, err = parseRulesFlag(args, i, &rules)
		}
		switch {
		case err != nil:
			return 0, err
		case n > 0:
			i += n - 1
		case args[i] == "--stealth":
			stealth = true
		case args[i] == "--policy" && i+1 < len(args):
			policy = args[i+1]
			i++
		default:
			return 0, usage("init takes only --stealth, --policy <file>, --plugin [<scope>], --git-shim, "+
				"--git-shim-dir <folder>, --no-git-shim, --agent-rules and --no-agent-rules (%s)", args[i])
		}
	}
	if shim.given && !shim.install && shim.dir != "" {
		return 0, usage("--git-shim-dir names where --git-shim links itos as git, not --no-git-shim")
	}
	if stealth && plugin.scope == "project" {
		return 0, pluginRefused()
	}
	log := o.Stdout
	if o.JSON {
		log = o.Stderr
	}
	if policy != "" {
		ask, answers := !o.JSON && onTerminal(), bufio.NewReader(os.Stdin)
		return policyInit(policy, stealth, pluginOffer{flag: plugin, stealth: stealth, ask: ask, answers: answers},
			shimOffer{flag: shim, ask: ask, answers: answers}, rulesOffer{flag: rules, stealth: stealth, ask: ask, answers: answers},
			log, o)
	}
	initialized, err := atTop(log)
	if err != nil {
		return 0, err
	}
	if file := config.Path(); exists(file) {
		// A stealth config is for a clone that cannot change the project:
		// where the project has its own, --stealth would only copy what it
		// commits into files that go stale unreported (bug 43).
		if stealth && !config.IsStealth(file) {
			return 0, usage("--stealth writes a config in the git folder for a clone that cannot change the project, "+
				"but this one has its own %s; run init without --stealth", file)
		}
		stealth = stealth || config.IsStealth(file)
		if stealth && plugin.scope == "project" {
			return 0, pluginRefused()
		}
		return initReport(file, pluginOffer{flag: plugin, stealth: stealth}, shimOffer{flag: shim},
			rulesOffer{flag: rules, stealth: stealth, rerun: true, file: file}, o)
	}
	if named := os.Getenv("ITOS_CONFIG"); named != "" {
		return 0, usage("init writes itos.yaml at the repository's top, or with --stealth in the git folder, "+
			"not the missing %s that --config or ITOS_CONFIG names", named)
	}
	// One reader for both questions, so the first cannot take the second's
	// answer into its buffer.
	ask, answers := !o.JSON && onTerminal(), bufio.NewReader(os.Stdin)
	return initWrite(stealth, initialized, pluginOffer{flag: plugin, stealth: stealth, ask: ask, answers: answers},
		shimOffer{flag: shim, ask: ask, answers: answers}, rulesOffer{flag: rules, stealth: stealth, ask: ask, answers: answers},
		log, o)
}

// atTop moves to the top of the repository the folder is in, after git init
// when it is in none, and says whether it ran git init.
func atTop(log io.Writer) (bool, error) {
	inRepo, err := toTop()
	if err != nil || inRepo {
		return false, err
	}
	return true, gitInit(log)
}

// toTop moves to the top of the repository the folder is in, and says
// whether it is in one; in a git folder it is a usage error.
func toTop() (bool, error) {
	if _, err := git.Output("rev-parse", "--git-dir"); err != nil {
		return false, nil
	}
	top, err := git.Output("rev-parse", "--show-toplevel")
	if top = value.Trim(top); err != nil || top == "" {
		return false, kind.Wrap(kind.Usage, errors.New("init runs in a repository's working tree, not in its git folder"))
	}
	return true, os.Chdir(top)
}

// gitInit makes the folder a git repository, and says so.
func gitInit(log io.Writer) error {
	cmd := exec.Command(git.Bin(), "init", "-q")
	if text, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %s", value.Trim(string(text)))
	}
	here, _ := os.Getwd()
	fmt.Fprintf(log, "Ran git init: %s is a git repository now.\n", here)
	return nil
}

// writtenFile is a file init wrote, or found there and kept.
type writtenFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// initWrite writes the starter, pins the newest release, installs the hooks
// and makes the plugin's offer, the git shim's and that of the rules for
// agents; its exit code is hook install's, else 1 when claude failed at
// installing the plugin asked for, the git shim asked for could not be
// linked or the rules asked for could not be written.
func initWrite(stealth, initialized bool, offer pluginOffer, shimOffer shimOffer, rulesOffer rulesOffer, log io.Writer,
	o Out) (int, error) {
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
		// No scenario with an ID tag: the footer is required of no type, so
		// the config is written again saying so.
		if len(list.Tests) == 0 && files[0].Action == "wrote" {
			s.untagged = true
			if err := os.WriteFile(file, []byte(s.config()), 0o666); err != nil {
				return 0, err
			}
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
		switch {
		case s.untagged:
			say("No scenario under %s carries an ID tag, so the smoke set names none and no feat or fix needs a "+
				"Scenarios footer yet: tag a scenario with its ID, such as @ID-PAGE-01 on the line above it, then "+
				"set commits.footers.Scenarios.required_for to [feat, fix] in %s.", starterFeatures, filepath.ToSlash(file))
		case s.scenarios:
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
	offer.log, shimOffer.log, rulesOffer.log = log, log, log
	if o.JSON {
		offer.log, shimOffer.log, rulesOffer.log = io.Discard, io.Discard, io.Discard
	}
	plugin, pluginCode := offer.run()
	shim, shimCode := shimOffer.run()
	rulesOffer.file = file
	rules, rulesCode := rulesOffer.run()
	code := hooksCode
	if code == 0 {
		code = max(pluginCode, shimCode, rulesCode)
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
		fields = append(fields, out.Field{Key: "hooks", Value: hooks}, out.Field{Key: "plugin", Value: plugin},
			out.Field{Key: "git_shim", Value: shim}, out.Field{Key: "agent_rules", Value: rules})
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
		for _, f := range rules.Files {
			if f.Action != "kept" {
				paths = append(paths, f.Path)
			}
		}
		fmt.Fprintf(log, "Commit what init wrote with the task that adopts itos: git add %s, then itos commit --task %s -m 'chore: adopt itos'\n",
			strings.Join(paths, " "), task)
	}
	return code, nil
}

// initHooks runs hook install on the config just written; under --json its
// object, without its schema, for init's own.
func initHooks(o Out) (int, any, error) {
	if !o.JSON {
		code, err := hookInstall(false, false, o)
		return code, nil, err
	}
	var buf bytes.Buffer
	code, err := hookInstall(false, false, Out{JSON: true, Quiet: o.Quiet, Stdout: &buf, Stderr: o.Stderr})
	if err != nil || buf.Len() == 0 {
		return code, nil, err
	}
	// Its keys in the order it wrote them, its schema left out (out.Emit
	// writes it first).
	hooks := json.RawMessage(bytes.Replace(buf.Bytes(), []byte(`"schema": 1,`), nil, 1))
	if !json.Valid(hooks) {
		return 0, nil, fmt.Errorf("hook install wrote no JSON object: %s", buf.Bytes())
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
// what is missing, exit 1 when anything is; then what it notes, never
// counted as missing (slice 50): a people file the config names that the
// repository lacks or cannot be read (config check's warning), and a pin
// behind the newest release, where the release server answers; then makes
// the plugin's offer and the git shim's, which never ask, and install only
// for --plugin and --git-shim; then the offer of the rules for agents, which
// reports a block the config no longer matches and writes one only for
// --agent-rules. None of them not in place is counted as missing.
func initReport(file string, offer pluginOffer, shimOffer shimOffer, rulesOffer rulesOffer, o Out) (int, error) {
	_, found, notes, _, err := configFindings("")
	if err != nil {
		return 0, err
	}
	if cfg, err := config.Load(file); err == nil {
		found = append(found, tagged("hooks", hookProblems())...)
		if behind := pinBehind(cfg); behind != nil {
			notes = append(notes, Found{*behind, "pin"})
		}
	}
	code := 0
	if len(found) > 0 {
		code = ExitPolicy
	}
	if o.JSON {
		if found == nil {
			found = []Found{}
		}
		if notes == nil {
			notes = []Found{}
		}
		offer.log, shimOffer.log, rulesOffer.log = io.Discard, io.Discard, io.Discard
		plugin, pluginCode := offer.run()
		shim, shimCode := shimOffer.run()
		rules, rulesCode := rulesOffer.run()
		return max(code, pluginCode, shimCode, rulesCode), out.Emit(o.Stdout, out.Field{Key: "config", Value: file},
			out.Field{Key: "action", Value: "checked"}, out.Field{Key: "missing", Value: found},
			out.Field{Key: "plugin", Value: plugin}, out.Field{Key: "notes", Value: notes},
			out.Field{Key: "git_shim", Value: shim}, out.Field{Key: "agent_rules", Value: rules})
	}
	list := func(found []Found) {
		for _, f := range found {
			line := f.Message
			if f.Fix != "" {
				line += "; " + f.Fix
			}
			fmt.Fprintf(o.Stdout, "  %s\n", line)
		}
	}
	fmt.Fprintf(o.Stdout, "%s is there already, so init wrote nothing.\n", file)
	if len(found) == 0 {
		fmt.Fprintln(o.Stdout, "Nothing is missing: the config, its ledger, registry and smoke sets are sound, and the hooks call itos.")
	} else {
		fmt.Fprintln(o.Stdout, "Missing:")
		list(found)
	}
	if len(notes) > 0 {
		fmt.Fprintln(o.Stdout, "Noted, not counted as missing:")
		list(notes)
	}
	offer.log, shimOffer.log, rulesOffer.log = o.Stdout, o.Stdout, o.Stdout
	_, pluginCode := offer.run()
	_, shimCode := shimOffer.run()
	_, rulesCode := rulesOffer.run()
	return max(code, pluginCode, shimCode, rulesCode), nil
}

// pinBehind is the note of a pin behind the newest release, nil when the
// config pins none, pins the newest or one newer, or the release server
// cannot say which is the newest.
func pinBehind(cfg *config.Loaded) *out.Problem {
	if cfg.Pin == nil || cfg.Pin.Version == "" {
		return nil
	}
	newest, _, err := pinned("")
	if err != nil || version.Compare(cfg.Pin.Version, newest) >= 0 {
		return nil
	}
	return &out.Problem{
		Rule:    "pin-behind",
		Message: fmt.Sprintf("the pin, itos %s, is behind the newest release, itos %s", cfg.Pin.Version, newest),
		Fix:     "itos pin moves it there; read the notes of the releases between them before you commit it",
	}
}

// hookProblems are itos's hooks git would not run, as hook install declares
// them in the git config (slice 91): a git that runs no hook its config
// declares, which no entry can mend, else each event whose entry is missing
// or does not run itos.
func hookProblems() []out.Problem {
	const root = "."
	var problems []out.Problem
	missing := func(event, message, fix string) {
		problems = append(problems, out.Problem{Rule: "hook-missing", Message: "the " + event + " hook: " + message, Fix: fix})
	}
	if !configHooksRun(root) {
		missing("commit-msg", "git "+gitVersion()+" runs no hook its config declares",
			"upgrade git to "+configHooksSince+" or later, then run itos hook install")
		return problems
	}
	for _, event := range hookEvents {
		if !declared(root, event) {
			missing(event, "hook."+hookEntry(event)+" in the git config does not run itos", "run itos hook install")
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
