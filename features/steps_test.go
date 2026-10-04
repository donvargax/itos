// The steps. itos is a black box here: each scenario builds a scratch git
// repository in a temporary folder, runs the binary ITOS_BIN names
// (tools/bin/itos by default, the Go binary built from cmd/itos, relative to
// the repository's root) in it, and reads its exit code and output. Nothing here imports or reads itos's code,
// so the same steps judge any implementation of it.
package features

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

// One scenario's state.
type world struct {
	root          string // the itos checkout: where go.mod is
	bin           string // the itos binary under test
	dir           string // the scratch repository, or the clone of it itos runs in
	origin        string // the scratch repository a clone was made of, if one was
	support       string // files the scenario needs outside the repository
	config        scratchConfig
	commits       []string          // the scratch repository's commits, oldest first
	scenarioFiles map[string]string // each scenario ID written, to its feature file
	ledger        []ledgerTask      // the tasks of the ledger's one file (ledgerPath), in order
	dataDir       string            // where itos's config and data are written: the root, or the git folder (stealth)
	linked        string            // the linked worktree of the scratch repository, when the scenario adds one
	watchURL      string            // the run the watch command reports
	registryLines []string          // the registry's items, one line each, as the work steps wrote them
	noGh          bool              // the PATH has no gh
	messageFile   string            // the message file's text, as a step wrote it

	exit           int
	stdout, stderr string

	releases  *releaseServer // the release server, when the scenario starts one
	vars      []string       // variables the scenario sets in itos's environment, NAME=value
	ranMark   int            // the fake itos runs recorded before the last run of itos began
	askedMark int            // the requests the release server had before the last run of itos began
	// The config's text before the last run of itos began, nil when there was
	// none.
	configBefore []byte
	// Whether each run of itos records the folder first (init.feature's "no
	// file changed since the last run"), and what it recorded last.
	snapshotRuns bool
	filesBefore  map[string]string
}

// What a scenario sets in the scratch repository's itos.yaml.
type scratchConfig struct {
	headerLintCommand string       // the header lint delegated to this command
	headerLintBuiltin bool         // the header lint itos's own (use: builtin)
	since             string       // commits.since
	rangeCheck        bool         // a range check that records where its range starts
	moves             *moves       // the kind's range check is the built-in moves rule
	recordingShell    bool         // shell is the recording shell
	ciSteps           []string     // ci.steps
	ciTests           string       // a kind of named tests, with run and recognize templates, run by the last of ci.steps
	runSelect         string       // the scenario kind's run.select, when ciTests gives it none
	smokeRuns         []string     // commands the kind of ciTests recognizes as its smoke run
	stopAtFirst       *bool        // ci.stop_at_first_failure
	nightlyTasks      string       // ci.nightly.steps runs the done tasks' checks: "every" of them, or "static"
	costStatic        []string     // ci.cost.static
	covers            []cover      // ci.covers
	nightlyOnly       []string     // ci.nightly_only
	registry          string       // work.registry
	taskChecks        *bool        // hooks.commit_msg.task_checks
	checkTimeout      int          // hooks.commit_msg.check_timeout, when above 0
	statuses          []string     // work.statuses
	groupsKey         string       // work.groups_key
	noPeople          bool         // the config names no people file (no work.people)
	noWork            bool         // the config has no work section, only work's defaults
	smoke             bool         // tests.scenario has a smoke set, features/smoke.yaml
	scenarios         bool         // the config has the kind tests.scenario, reading features/
	smokeEveryFile    *bool        // tests.scenario.smoke.every_file
	noTagPrefix       bool         // the kind written without tag_prefix
	hooksManager      string       // hooks.manager
	hooksBin          string       // hooks.bin
	prePushRecord     bool         // hooks.pre_push's commands record that they ran
	watch             *watchConfig // ci.watch
	ledgerFooter      string       // the key of the footer whose source is the ledger; Task when empty
	textFooter        *textFooter  // a footer of free text
	featMustTouch     string       // commits.scopes.feat.must_touch, one glob, when set
	ledgerFiles       string       // ledger.files; tasks/phase-{group}.yaml when empty
	prosePaths        string       // ci.prose.paths, one glob
	proseSteps        string       // ci.prose.steps, one command
	pin               *[2]string   // pin.version and pin.checksums
	comments          []string     // comment lines written after the pin's line
	settings          []setting
}

// The built-in moves rule as the kind's range check: the types it leaves
// alone, and the renames it allows, each an ID and its new name.
type moves struct {
	exceptTypes []string
	renames     [][2]string
}

// A footer of free text (source: text): its key, the types it is required
// for, and the commit after which it is required (its since), when one.
type textFooter struct {
	key   string
	types []string
	since string
}

// One ci.covers rule: the step that has done what a check matching the
// pattern does.
type cover struct{ by, matches string }

// One task of the scratch ledger: its ID and its checks, each written as YAML.
type ledgerTask struct {
	id     string
	checks []string
}

// One key the scenario sets by its dotted path, to a string. A key under
// ledger or commits goes into that section; any other starts a section of its
// own, so it may not be one the config already writes.
type setting struct{ key, value string }

func initializeScenario(sc *godog.ScenarioContext) {
	w := &world{}
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return ctx, w.setUp()
	})
	sc.After(func(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
		w.stopReleaseServer()
		os.RemoveAll(w.dir)
		os.RemoveAll(w.support)
		if w.origin != "" {
			os.RemoveAll(w.origin)
		}
		return ctx, err
	})

	sc.Step(`^a repository made from a template, its first commit "([^"]*)"$`, w.templateRepository)
	sc.Step(`^the commit "([^"]*)" on top of it$`, w.commitOnTop)
	sc.Step(`^a repository whose ledger has the task "([^"]*)"$`, func(task string) error {
		return w.repositoryWithTask(task)
	})
	sc.Step(`^a repository whose ledger has the tasks "([^"]*)" and "([^"]*)"$`, func(a, b string) error {
		return w.repositoryWithTask(a, b)
	})
	sc.Step(`^a change to "([^"]*)" is staged$`, w.stageChange)
	sc.Step(`^the ledger folder is missing$`, w.ledgerFolderMissing)
	sc.Step(`^the header lint is commitlint's conventional config$`, w.conventionalHeaderLint)
	sc.Step(`^commits\.since names the first commit$`, w.sinceFirstCommit)
	sc.Step(`^commits\.since is "([^"]*)"$`, w.sinceIs)
	sc.Step(`^itos runs in a clone of the repository one commit deep$`, w.shallowClone)
	sc.Step(`^a range check that records where its range starts$`, w.recordingRangeCheck)
	sc.Step(`^the config's shell is the recording shell$`, w.recordingShell)
	sc.Step(`^the task "([^"]*)" has the check "([^"]*)"$`, w.taskHasCheck)
	sc.Step(`^the CI steps are "([^"]*)"$`, func(step string) error { return w.ciStepsAre(step) })
	sc.Step(`^the config has no work section$`, func() error {
		w.config.noWork = true
		return w.writeConfig()
	})
	sc.Step(`^the CI steps run the named tests of the kind "([^"]*)"$`, w.ciStepsRunTests)
	sc.Step(`^the smoke set is empty$`, w.emptySmokeSet)
	sc.Step(`^the kind "([^"]*)" recognizes "([^"]*)" as its smoke run$`, w.recognizesSmokeRun)
	sc.Step(`^the task "([^"]*)" has a check that runs the scenario "([^"]*)"$`, func(task, id string) error {
		return w.taskHasCheck(task, "run-scenarios "+id)
	})
	sc.Step(`^the commit "([^"]*)" naming the task "([^"]*)" on top of it$`, func(message, task string) error {
		return w.commitOnTop(message + "\n\nTask: " + task + "\n")
	})
	sc.Step(`^the commit "([^"]*)" naming the task "([^"]*)" in the footer "([^"]*)" on top of it$`, func(message, task, key string) error {
		return w.commitOnTop(message + "\n\n" + key + ": " + task + "\n")
	})
	sc.Step(`^the commit "([^"]*)" touching only "([^"]*)" on top of it$`, w.commitTouchingOnly)
	sc.Step(`^the commit "([^"]*)" naming the task "([^"]*)"$`, func(message, task string) error {
		return w.commitOnTop(message + "\n\nTask: " + task + "\n")
	})
	sc.Step(`^the commit "([^"]*)" naming the task "([^"]*)" with the footer "([^"]*)"$`, func(message, task, footer string) error {
		return w.commitOnTop(message + "\n\nTask: " + task + "\n" + footer + "\n")
	})
	sc.Step(`^the config requires an? "([^"]*)" footer of free text for "([^"]*)"$`, w.requiresTextFooter)
	sc.Step(`^the config's feat commits must touch "([^"]*)"$`, func(glob string) error {
		w.config.featMustTouch = glob
		return w.writeConfig()
	})
	sc.Step(`^the "([^"]*)" footer is required only after HEAD$`, w.textFooterSinceHead)
	sc.Step(`^the ledger footer is called "([^"]*)"$`, w.ledgerFooterIs)
	sc.Step(`^the ledger's files are "([^"]*)"$`, w.ledgerFilesAre)
	sc.Step(`^the prose paths are "([^"]*)" and the prose steps are "([^"]*)"$`, w.proseIs)
	sc.Step(`^the CI steps are "([^"]*)" then a step that records it ran$`, func(step string) error {
		return w.ciStepsAre(step, "printf 'ran\\n' > "+recordingStepFile)
	})
	sc.Step(`^the header lint is the command "([^"]*)"$`, w.headerLintIs)
	sc.Step(`^the header lint is itos's built-in one$`, w.builtinHeaderLint)
	sc.Step(`^ci\.stop_at_first_failure is (true|false)$`, w.stopAtFirstFailureIs)
	sc.Step(`^work\.registry is "([^"]*)"$`, w.registryIs)
	sc.Step(`^the work registry at "([^"]*)" has the item "([^"]*)" with the status "([^"]*)"$`, w.registryAt)
	sc.Step(`^the work registry at "([^"]*)" with the item "([^"]*)" with the status "([^"]*)" is staged$`, w.stagedRegistryAt)
	sc.Step(`^the working tree's "([^"]*)" sets the item "([^"]*)" to the status "([^"]*)"$`, w.workingRegistry)
	sc.Step(`^the commit of a registry at "([^"]*)" with the item "([^"]*)" with the status "([^"]*)"$`, w.committedRegistryAt)
	sc.Step(`^an itos\.yaml with the unknown key "([^"]*)" is staged$`, w.stagedConfigKey)
	sc.Step(`^a ledger whose task "([^"]*)" has the unknown key "([^"]*)" is staged$`, w.stagedLedgerKey)
	sc.Step(`^the task "([^"]*)" has the static check "([^"]*)"$`, func(task, check string) error {
		return w.stagedChecks(task, staticCheck(check))
	})
	sc.Step(`^the task "([^"]*)" has a static check that records it ran$`, func(task string) error {
		return w.stagedChecks(task, staticCheck(recordingCheck))
	})
	sc.Step(`^the task "([^"]*)" has the late check "([^"]*)"$`, func(task, check string) error {
		return w.stagedChecks(task, fmt.Sprintf("{ run: %q, cost: late }", check))
	})
	sc.Step(`^the task "([^"]*)" has the late check "([^"]*)" then a static check that records it ran$`, func(task, check string) error {
		return w.stagedChecks(task, fmt.Sprintf("{ run: %q, cost: late }", check), staticCheck(recordingCheck))
	})
	sc.Step(`^the tasks "([^"]*)" and "([^"]*)" each have the counting check$`, func(a, b string) error {
		if err := w.stagedChecks(a, countingCheck("run")); err != nil {
			return err
		}
		return w.stagedChecks(b, countingCheck("run"))
	})
	sc.Step(`^the task "([^"]*)" has the counting check as (run|fails)$`, func(task, mode string) error {
		return w.stagedChecks(task, countingCheck(mode))
	})
	sc.Step(`^hooks\.commit_msg\.task_checks is (true|false)$`, w.taskChecksAre)
	sc.Step(`^hooks\.commit_msg\.check_timeout is (\d+)$`, w.checkTimeoutIs)
	sc.Step(`^work\.statuses is "([^"]*)"$`, w.statusesAre)
	sc.Step(`^the work registry has the item "([^"]*)" with the status "([^"]*)"$`, func(item, status string) error {
		return w.workingRegistry(startingRegistry, item, status)
	})
	sc.Step(`^the work registry has the item "([^"]*)" with the status "([^"]*)" and the item "([^"]*)" with the status "([^"]*)"$`, func(a, aStatus, b, bStatus string) error {
		return w.workingRegistryItems(startingRegistry, [][2]string{{a, aStatus}, {b, bStatus}})
	})
	sc.Step(`^the nightly steps run the checks of the done tasks$`, func() error { return w.nightlyTasksAre("every") })
	sc.Step(`^the nightly steps run the static checks of the done tasks$`, func() error { return w.nightlyTasksAre("static") })
	sc.Step(`^work\.groups_key is "([^"]*)"$`, w.groupsKeyIs)
	sc.Step(`^the people file is missing$`, w.peopleFileMissing)
	sc.Step(`^no identity can be looked up$`, w.noIdentity)
	sc.Step(`^itos proposes "([^"]*)" to start$`, w.proposesToStart)
	sc.Step(`^the work registry gives the group "([^"]*)" to the owner "([^"]*)" under "([^"]*)"$`, w.registryGroupOwner)
	sc.Step(`^the work registry has the item "([^"]*)" in the group "([^"]*)", which it does not list$`, w.registryUnlistedGroup)
	sc.Step(`^ledger\.group\.label is "([^"]*)"$`, func(label string) error {
		return w.configSets("ledger.group.label", label)
	})
	sc.Step(`^a feature file "([^"]*)" with the live scenario "([^"]*)"$`, w.featureFile)
	sc.Step(`^the committed feature file "([^"]*)" with the live scenario "([^"]*)"$`, w.committedFeatureFile)
	sc.Step(`^the kind's range check is the built-in moves rule, except for "([^"]*)"$`, w.movesRule)
	sc.Step(`^the moves rule allows renaming "([^"]*)" to "([^"]*)"$`, w.movesAllowRename)
	sc.Step(`^a change to the steps of the scenario "([^"]*)" is staged$`, func(id string) error {
		return w.stageScenarioEdit(id, changeSteps)
	})
	sc.Step(`^the scenario "([^"]*)" is staged renamed to "([^"]*)"$`, func(id, name string) error {
		return w.stageScenarioEdit(id, rename(name))
	})
	sc.Step(`^the scenario "([^"]*)" is staged moved to "([^"]*)"$`, w.stageScenarioMove)
	sc.Step(`^the commit "([^"]*)" naming the task "([^"]*)" and changing the steps of the scenario "([^"]*)" on top of it$`, w.commitChangingSteps)
	sc.Step(`^the smoke set lists only "([^"]*)"$`, w.smokeSetLists)
	sc.Step(`^smoke\.every_file is (true|false)$`, w.smokeEveryFileIs)
	sc.Step(`^the kind leaves out tag_prefix$`, w.noTagPrefix)
	sc.Step(`^a "([^"]*)" folder$`, w.folder)
	sc.Step(`^hooks\.manager is "([^"]*)"$`, w.hooksManagerIs)
	sc.Step(`^hooks\.bin is "([^"]*)"$`, w.hooksBinIs)
	sc.Step(`^ci\.cost\.static is "([^"]*)"$`, w.costStaticIs)
	sc.Step(`^ci\.covers says the step "([^"]*)" covers "([^"]*)"$`, w.coversIs)
	sc.Step(`^ci\.nightly_only is "([^"]*)"$`, w.nightlyOnlyIs)
	sc.Step(`^"([^"]*)" is a script that records it ran$`, w.recordingScript)
	sc.Step(`^"([^"]*)" is a script that records its arguments$`, w.argumentsScript)
	sc.Step(`^the kind "([^"]*)" runs a selection as "([^"]*)"$`, w.kindRunsSelection)
	sc.Step(`^the config sets "([^"]*)" to "([^"]*)"$`, w.configSets)

	initializeReleaseSteps(sc, w)
	initializeExtensionSteps(sc, w)
	initializeStealthSteps(sc, w)
	initializeCommitSteps(sc, w)
	initializePushSteps(sc, w)
	initializeWatchSteps(sc, w)
	initializeShimSteps(sc, w)
	initializeGuardSteps(sc, w)
	initializeInitSteps(sc, w)
	initializeWorkSteps(sc, w)
	initializeShowSteps(sc, w)

	sc.Step(`^itos verifies every commit up to HEAD$`, func() error { return w.itos("verify", "", "HEAD") })
	sc.Step(`^itos checks the config$`, func() error { return w.itos("config", "check") })
	sc.Step(`^itos checks the config as JSON$`, func() error { return w.itos("config", "check", "--json") })
	sc.Step(`^itos prints the defaults of the config$`, func() error { return w.itos("config", "check", "--print-defaults") })
	sc.Step(`^itos checks the work registry$`, func() error { return w.itos("work", "check") })
	sc.Step(`^itos verifies the commits after the first$`, func() error {
		if len(w.commits) == 0 {
			return errors.New("the repository has no commit yet")
		}
		return w.itos("verify", w.commits[0], "HEAD")
	})
	sc.Step(`^itos lists the "([^"]*)" footers of the commits after the first$`, func(key string) error {
		if len(w.commits) == 0 {
			return errors.New("the repository has no commit yet")
		}
		return w.itos("commit", "footers", key, w.commits[0], "HEAD")
	})
	sc.Step(`^itos checks the staged moves of the kind "([^"]*)"$`, func(kind string) error {
		return w.itos("tests", "moves", kind)
	})
	sc.Step(`^itos checks the smoke set$`, func() error { return w.itos("tests", "smoke", "check", "scenario") })
	sc.Step(`^itos installs the hooks$`, func() error { return w.itos("hooks", "install") })
	sc.Step(`^itos runs the task "([^"]*)"$`, func(task string) error { return w.itos("task", task) })
	sc.Step(`^itos runs the task "([^"]*)" from "([^"]*)"$`, func(task, folder string) error {
		return w.itosIn(filepath.Join(w.dir, folder), "task", task)
	})
	sc.Step(`^itos runs the tasks "([^"]*)" and "([^"]*)"$`, func(a, b string) error { return w.itos("task", a, b) })
	sc.Step(`^itos runs the pending tasks$`, func() error { return w.itos("task", "--pending") })
	sc.Step(`^itos runs the tasks of the group "([^"]*)"$`, func(group string) error {
		return w.itos("task", "--group", group)
	})
	sc.Step(`^itos runs the tasks of the group "([^"]*)" by the flag "([^"]*)"$`, func(group, flag string) error {
		return w.itos("task", flag, group)
	})
	sc.Step(`^itos lists the tasks$`, func() error { return w.itos("task", "list") })
	sc.Step(`^itos runs the nightly$`, func() error { return w.itos("ci", "run", "--nightly") })
	sc.Step(`^itos runs CI over every commit up to HEAD$`, func() error { return w.itos("ci", "run", "", "HEAD") })
	sc.Step(`^itos runs CI over the commits after the first$`, func() error {
		if len(w.commits) == 0 {
			return errors.New("the repository has no commit yet")
		}
		return w.itos("ci", "run", w.commits[0], "HEAD")
	})
	sc.Step(`^itos plans CI over the commits after the first$`, func() error {
		if len(w.commits) == 0 {
			return errors.New("the repository has no commit yet")
		}
		return w.itos("ci", "plan", w.commits[0], "HEAD")
	})
	sc.Step(`^itos prints the help of "([^"]*)"$`, func(command string) error {
		return w.itos(append(strings.Fields(command), "--help")...)
	})
	sc.Step(`^the commit-msg hook checks the message "([^"]*)"$`, w.commitMsgHook)
	sc.Step(`^the commit-msg hook checks the message:$`, func(message *godog.DocString) error {
		return w.commitMsgHook(message.Content + "\n")
	})

	sc.Step(`^itos exits with code (\d+)$`, w.exitsWith)
	sc.Step(`^its output says "([^"]*)"$`, w.outputSays)
	sc.Step(`^its output does not say "([^"]*)"$`, w.outputDoesNotSay)
	sc.Step(`^its output names the rule "([^"]*)"$`, w.outputNamesRule)
	sc.Step(`^its output is a JSON report that is not valid$`, w.invalidJSONReport)
	sc.Step(`^its JSON lists the item "([^"]*)" with its title$`, w.jsonListsItem)
	sc.Step(`^its output lists "([^"]*)" as "([^"]*)"$`, w.outputLists)
	sc.Step(`^the counting check ran once$`, func() error { return w.countingCheckRan(1) })
	sc.Step(`^the counting check did not run$`, func() error { return w.countingCheckRan(0) })
	sc.Step(`^the range check started at the first commit$`, w.rangeCheckStartedAtFirst)
	sc.Step(`^the recording shell ran "([^"]*)"$`, w.recordingShellRan)
	sc.Step(`^the recording shell ran the range check$`, w.recordingShellRanRangeCheck)
	sc.Step(`^the recording step ran$`, func() error { return w.recordingStepRan(true) })
	sc.Step(`^the recording step did not run$`, func() error { return w.recordingStepRan(false) })
	sc.Step(`^the recording check ran$`, func() error { return w.recordingCheckRan(true) })
	sc.Step(`^the recording check did not run$`, func() error { return w.recordingCheckRan(false) })
	sc.Step(`^the file "([^"]*)" calls itos$`, w.fileCallsItos)
	sc.Step(`^"([^"]*)" was given "([^"]*)"$`, func(script, arg string) error {
		return w.scriptWasGiven(script, arg, true)
	})
	sc.Step(`^"([^"]*)" was not given "([^"]*)"$`, func(script, arg string) error {
		return w.scriptWasGiven(script, arg, false)
	})
}

func (w *world) setUp() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	w.root = root
	w.bin = os.Getenv("ITOS_BIN")
	if w.bin == "" {
		// The Go binary, built from this tree on demand: what the hooks and CI
		// run, and since the TypeScript left (T-062) the one implementation.
		w.bin = "tools/bin/itos"
	}
	if !filepath.IsAbs(w.bin) {
		w.bin = filepath.Join(root, w.bin)
	}
	if w.dir, err = os.MkdirTemp("", "itos-features-"); err != nil {
		return err
	}
	if w.support, err = os.MkdirTemp("", "itos-features-support-"); err != nil {
		return err
	}
	return w.git("init", "-q", "-b", "main")
}

// The folder holding go.mod, above the working directory go test gives.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}

// The environment every command runs in: the caller's, less what would make
// git or itos read anything but the scratch repository (a hook's GIT_DIR, CI's
// settings), with no global or system git config and a fixed identity, and
// what keeps the launcher off the network and any real cache (launcherEnv),
// and the scenario's git link and extensions first on the PATH (pathFirst),
// the caller's PATH without its claude (callerPath) after them.
func (w *world) env() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "ITOS_") ||
			strings.HasPrefix(name, "GITHUB_") || strings.HasPrefix(name, "GH_") || name == "CI" {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=itos features",
		"GIT_AUTHOR_EMAIL=features@localhost",
		"GIT_COMMITTER_NAME=itos features",
		"GIT_COMMITTER_EMAIL=features@localhost",
		"PATH="+w.basePath(),
	)
	env = append(env, w.launcherEnv()...)
	if first := w.pathFirst(); len(first) > 0 {
		env = append(env, "PATH="+strings.Join(append(first, w.basePath()), string(os.PathListSeparator)))
	}
	return env
}

// basePath is the PATH every command starts from: the caller's without its
// claude (callerPath), and without its gh too when the scenario has none.
func (w *world) basePath() string {
	if w.noGh {
		return pathHiding("claude", "gh")
	}
	return callerPath()
}

var (
	hiddenPathsMu sync.Mutex
	hiddenPaths   = map[string]string{}
	callerPathDir string
)

// callerPath is the caller's PATH with no claude on it, so no scenario
// reaches the Claude Code of the machine it runs on: each folder holding one
// is replaced by a folder of links to everything else in it, as the corpus's
// hide does, or left out where links cannot be made. A scenario's own claude
// goes first on the PATH (init_test.go).
func callerPath() string { return pathHiding("claude") }

// pathHiding is the caller's PATH with none of the programs named on it,
// made once a run for each set of names, as callerPath hides claude.
func pathHiding(names ...string) string {
	hiddenPathsMu.Lock()
	defer hiddenPathsMu.Unlock()
	key := strings.Join(names, " ")
	if text, ok := hiddenPaths[key]; ok {
		return text
	}
	hidden := func(e os.DirEntry) bool { return isOneOf(e, names) }
	var folders []string
	for i, folder := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(folder)
		if err != nil || !slices.ContainsFunc(entries, hidden) {
			folders = append(folders, folder)
			continue
		}
		if callerPathDir == "" {
			if callerPathDir, err = os.MkdirTemp("", "itos-features-path-"); err != nil {
				continue
			}
		}
		links := filepath.Join(callerPathDir, strconv.Itoa(len(hiddenPaths)), strconv.Itoa(i))
		if linkAllBut(folder, links, entries, hidden) == nil {
			folders = append(folders, links)
		}
	}
	text := strings.Join(folders, string(os.PathListSeparator))
	hiddenPaths[key] = text
	return text
}

// removeCallerPath removes the folders callerPath and pathHiding made.
func removeCallerPath() {
	if callerPathDir != "" {
		_ = os.RemoveAll(callerPathDir)
	}
}

// isOneOf is whether a folder's entry is a program of the names the PATH
// would find: the name, or on windows the name with any extension.
func isOneOf(e os.DirEntry, names []string) bool {
	name := e.Name()
	if runtime.GOOS == "windows" {
		name = strings.TrimSuffix(name, filepath.Ext(name))
		return slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(name, n) })
	}
	return slices.Contains(names, name)
}

// linkAllBut makes links a folder of links to every entry of folder but the
// hidden ones.
func linkAllBut(folder, links string, entries []os.DirEntry, hidden func(os.DirEntry) bool) error {
	if err := os.MkdirAll(links, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if hidden(e) {
			continue
		}
		if err := os.Symlink(filepath.Join(folder, e.Name()), filepath.Join(links, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) git(args ...string) error { return w.gitIn(w.dir, args...) }

// git run in the folder dir.
func (w *world) gitIn(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = w.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func (w *world) head() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = w.dir
	cmd.Env = w.env()
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func (w *world) write(path, text string) error {
	full := filepath.Join(w.dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(text), 0o644)
}

// Every file staged, then one commit with the message.
func (w *world) commit(message string) error {
	if err := w.git("add", "-A"); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", message); err != nil {
		return err
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, sha)
	return nil
}

// A word for sh: the text in single quotes.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// The scratch repository's itos.yaml: a ledger of tasks/phase-<n>.yaml (or
// where the scenario moved it), the Conventional Commit types, a ledger footer
// the non-feature types need (Task:, unless the scenario calls it otherwise),
// a scenario footer when there is a kind of named tests, docs commits held to
// prose, a CI, and a people list, plus what the scenario set.
func (w *world) writeConfig() error {
	var b strings.Builder
	b.WriteString("version: 1\n")
	if p := w.config.pin; p != nil {
		fmt.Fprintf(&b, "pin: { version: %q, checksums: %q }\n", p[0], p[1])
	}
	for _, c := range w.config.comments {
		b.WriteString(c + "\n")
	}
	if w.config.recordingShell {
		fmt.Fprintf(&b, "shell: [%q]\n", w.recordingShellPath())
	}
	fmt.Fprintf(&b, "ledger:\n  files: %q\n  id: \"T-\\\\d+\"\n", w.ledgerFilesPattern())
	b.WriteString(w.settingsUnder("ledger"))
	footer := w.config.ledgerFooter
	if footer == "" {
		footer = "Task"
	}
	fmt.Fprintf(&b, `commits:
  types: [feat, fix, refactor, perf, test, build, ci, chore, docs, style, revert]
  footers:
    %s:
      source: ledger
`, footer)
	b.WriteString(`      required_for: [refactor, perf, test, build, ci, chore, revert]
      validate_for: all
      read_at: commit
`)
	if kind := w.testsKind(); kind != "" {
		fmt.Fprintf(&b, `    Scenarios:
      source: { tests: %s }
      strip_prefix: "@"
      validate_for: [feat, fix]
      must_be_live: true
`, kind)
	}
	if t := w.config.textFooter; t != nil {
		fmt.Fprintf(&b, "    %s:\n      source: text\n      required_for: [%s]\n", t.key, strings.Join(t.types, ", "))
		if t.since != "" {
			fmt.Fprintf(&b, "      since: %q\n", t.since)
		}
	}
	b.WriteString(`  scopes:
    docs: { only: ["**/*.md", "docs/**", "tasks/**"] }
`)
	if w.config.featMustTouch != "" {
		fmt.Fprintf(&b, "    feat: { must_touch: [%q] }\n", w.config.featMustTouch)
	}
	if w.config.since != "" {
		fmt.Fprintf(&b, "  since: %q\n", w.config.since)
	}
	if w.config.headerLintCommand != "" {
		lint := w.config.headerLintCommand
		fmt.Fprintf(&b, "  header_lint:\n    hook: %q\n    stdin: %q\n", lint, lint)
	} else if w.config.headerLintBuiltin {
		b.WriteString("  header_lint:\n    use: builtin\n")
	}
	b.WriteString(w.settingsUnder("commits"))
	if kind := w.testsKind(); kind != "" {
		fmt.Fprintf(&b, `tests:
  %s:
    root: features
    id: "ID-[A-Z]+-\\d+"
`, kind)
		if !w.config.noTagPrefix {
			b.WriteString("    tag_prefix: \"@\"\n")
		}
	}
	if w.config.runSelect != "" && w.config.ciTests == "" {
		fmt.Fprintf(&b, "    run:\n      select: %q\n      ids_pattern: \"@(?:{ids})\\\\b\"\n", w.config.runSelect)
	}
	if w.config.ciTests != "" {
		// A runner that only says what it would run, as the conformance case's
		// does, and a check written as "run-scenarios <pattern>" read back as a
		// selection of the kind.
		b.WriteString(`    run:
      whole: "echo run every scenario"
      select: "printf 'run %s\\n' {pattern}"
      ids_pattern: "@(?:{ids})\\b"
    recognize:
      - { command: "run-scenarios {pattern}", as: pattern }
`)
		for _, command := range w.config.smokeRuns {
			fmt.Fprintf(&b, "      - { command: %q, as: smoke }\n", command)
		}
	}
	if w.config.smoke || w.config.ciTests != "" {
		b.WriteString("    smoke:\n      file: features/smoke.yaml\n")
		if w.config.smokeEveryFile != nil {
			fmt.Fprintf(&b, "      every_file: %t\n", *w.config.smokeEveryFile)
		}
	}
	if w.config.rangeCheck || w.config.moves != nil {
		b.WriteString("    range_checks:\n")
	}
	if w.config.rangeCheck {
		record := "printf '%s\\n' {from} > " + quote(filepath.Join(w.support, "range-from"))
		fmt.Fprintf(&b, "      - name: record\n        range: %q\n", record)
	}
	if m := w.config.moves; m != nil {
		fmt.Fprintf(&b, "      - name: moves\n        builtin: moves\n        except_types: [%s]\n",
			strings.Join(m.exceptTypes, ", "))
		if len(m.renames) > 0 {
			b.WriteString("        allowed_renames:\n")
			for _, r := range m.renames {
				fmt.Fprintf(&b, "          %q: %q\n", r[0], r[1])
			}
		}
	}
	b.WriteString(w.ciSection())
	if !w.config.noWork {
		w.writeWork(&b)
	}
	if w.config.hooksManager != "" || w.config.hooksBin != "" || w.config.taskChecks != nil || w.config.checkTimeout > 0 ||
		w.config.prePushRecord {
		b.WriteString("hooks:\n")
	}
	if w.config.hooksManager != "" {
		fmt.Fprintf(&b, "  manager: %q\n", w.config.hooksManager)
	}
	if w.config.hooksBin != "" {
		fmt.Fprintf(&b, "  bin: %q\n", w.config.hooksBin)
	}
	if w.config.prePushRecord {
		record := "printf '%s\\n' ran >> " + quote(w.prePushRecord())
		fmt.Fprintf(&b, "  pre_push: { per_base: %q, whole: %q }\n", record, record)
	}
	if w.config.taskChecks != nil || w.config.checkTimeout > 0 {
		b.WriteString("  commit_msg:\n")
		if w.config.taskChecks != nil {
			fmt.Fprintf(&b, "    task_checks: %t\n", *w.config.taskChecks)
		}
		if w.config.checkTimeout > 0 {
			fmt.Fprintf(&b, "    check_timeout: %d\n", w.config.checkTimeout)
		}
	}
	for _, s := range w.config.settings {
		if path := strings.Split(s.key, "."); path[0] != "ledger" && path[0] != "commits" {
			b.WriteString(nested(path, s.value, 0))
		}
	}
	return w.write(w.data("itos.yaml"), b.String())
}

// writeWork writes the config's work section: the registry, groups key and
// statuses the scenario set, and the people file unless it names none.
func (w *world) writeWork(b *strings.Builder) {
	b.WriteString("work: { ")
	if w.config.registry != "" {
		fmt.Fprintf(b, "registry: %q, ", w.config.registry)
	}
	if w.config.groupsKey != "" {
		fmt.Fprintf(b, "groups_key: %q, ", w.config.groupsKey)
	}
	if w.config.statuses != nil {
		fmt.Fprintf(b, "statuses: [%s], ", strings.Join(w.config.statuses, ", "))
	}
	if !w.config.noPeople {
		b.WriteString("people: { source: yaml, file: people.yaml } ")
	}
	b.WriteString("}\n")
}

// The scratch config's one kind of named tests, when the scenario needs one:
// scenario, or the kind its CI steps run. Its footer, Scenarios:, is required
// for no type, so a scenario that names none is judged as before it.
func (w *world) testsKind() string {
	switch {
	case w.config.ciTests != "":
		return w.config.ciTests
	case w.config.rangeCheck || w.config.smoke || w.config.moves != nil || w.config.scenarios:
		return "scenario"
	}
	return ""
}

// A path of itos's config and data as the scratch repository holds it: in
// the root, or in the folder of the git folder that the stealth mode reads.
func (w *world) data(path string) string { return filepath.Join(w.dataDir, path) }

// The scratch config's ci section: the CI steps the scenario gives, none when
// it gives none, since a scenario that runs CI for a task's checks or a
// footer's names needs no step of its own, and the CI settings it sets.
func (w *world) ciSection() string {
	var b strings.Builder
	b.WriteString("ci:\n  steps:")
	if len(w.config.ciSteps) == 0 && w.config.ciTests == "" {
		b.WriteString(" []")
	}
	b.WriteString("\n")
	for _, step := range w.config.ciSteps {
		fmt.Fprintf(&b, "    - %q\n", step)
	}
	if w.config.ciTests != "" {
		fmt.Fprintf(&b, "    - { tests: %s }\n", w.config.ciTests)
	}
	if w.config.stopAtFirst != nil {
		fmt.Fprintf(&b, "  stop_at_first_failure: %t\n", *w.config.stopAtFirst)
	}
	if len(w.config.costStatic) > 0 {
		b.WriteString("  cost:\n    static:\n")
		for _, pattern := range w.config.costStatic {
			fmt.Fprintf(&b, "      - %q\n", pattern)
		}
	}
	if len(w.config.covers) > 0 {
		b.WriteString("  covers:\n")
		for _, rule := range w.config.covers {
			fmt.Fprintf(&b, "    - { by: %q, matches: %q }\n", rule.by, rule.matches)
		}
	}
	if len(w.config.nightlyOnly) > 0 {
		b.WriteString("  nightly_only:\n")
		for _, command := range w.config.nightlyOnly {
			fmt.Fprintf(&b, "    - %q\n", command)
		}
	}
	if w.config.proseSteps != "" {
		fmt.Fprintf(&b, "  prose: { paths: [%q], steps: [%q] }\n", w.config.prosePaths, w.config.proseSteps)
	}
	switch w.config.nightlyTasks {
	case "every":
		b.WriteString("  nightly:\n    steps: [{ tasks: done }]\n")
	case "static":
		b.WriteString("  nightly:\n    steps: [{ tasks: done, cost: static }]\n")
	}
	b.WriteString(w.watchSection())
	return b.String()
}

// The scenario's settings under a section the config writes, as lines below it.
func (w *world) settingsUnder(section string) string {
	var b strings.Builder
	for _, s := range w.config.settings {
		if path := strings.Split(s.key, "."); path[0] == section && len(path) > 1 {
			b.WriteString(nested(path[1:], s.value, 1))
		}
	}
	return b.String()
}

// The YAML lines that set the key path to value, the first key at the given
// depth.
func nested(path []string, value string, depth int) string {
	var b strings.Builder
	for i, key := range path {
		b.WriteString(strings.Repeat("  ", depth+i) + key + ":")
		if i < len(path)-1 {
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, " %q\n", value)
	return b.String()
}

// Where a scratch repository's work registry starts: itos's default.
const startingRegistry = "tasks/work-items.yaml"

// The files every scratch repository starts with: its config, a ledger with
// the tasks, none with a check, an empty work registry and its people, where
// the scenario keeps itos's data, and a README.
func (w *world) startingFiles(tasks ...string) error {
	w.ledger = nil
	for _, id := range tasks {
		w.ledger = append(w.ledger, ledgerTask{id: id})
	}
	files := map[string]string{
		w.data(w.ledgerPath()):   w.ledgerText(),
		w.data(startingRegistry): "phases: {}\nitems: []\n",
		w.data("people.yaml"):    "- someone\n",
		"README.md":              "# Scratch\n",
	}
	for path, text := range files {
		if err := w.write(path, text); err != nil {
			return err
		}
	}
	return w.writeConfig()
}

// Each task's title, by its place in the ledger, so that no title is a word a
// status line could be read as.
var taskTitles = []string{"Tidy", "Sweep", "Dust"}

// The scratch ledger's files, as ledger.files gives them.
func (w *world) ledgerFilesPattern() string {
	if w.config.ledgerFiles == "" {
		return "tasks/phase-{group}.yaml"
	}
	return w.config.ledgerFiles
}

// The scratch ledger's one file, of phase 1.
func (w *world) ledgerPath() string {
	return strings.ReplaceAll(w.ledgerFilesPattern(), "{group}", "1")
}

// The scratch ledger, the one file ledgerPath names: one line a task, with its
// checks.
func (w *world) ledgerText() string {
	var b strings.Builder
	for i, task := range w.ledger {
		fmt.Fprintf(&b, "- { id: %s, type: chore, title: %s", task.id, taskTitles[i%len(taskTitles)])
		if len(task.checks) > 0 {
			fmt.Fprintf(&b, ", done_when: [%s]", strings.Join(task.checks, ", "))
		}
		b.WriteString(" }\n")
	}
	return b.String()
}

// Given steps.

// A template's squashed first commit, judged by a header lint it fails: the
// built-in one, config-conventional's rules.
func (w *world) templateRepository(message string) error {
	w.config.headerLintBuiltin = true
	if err := w.startingFiles("T-001"); err != nil {
		return err
	}
	return w.commit(message)
}

// One commit of the one path, changed, whatever else the working tree holds:
// a range that touches only it.
func (w *world) commitTouchingOnly(message, path string) error {
	full := filepath.Join(w.dir, path)
	text, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := w.write(path, string(text)+"A line for "+message+".\n"); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", message, "--", path); err != nil {
		return err
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, sha)
	return nil
}

// The config's ledger footer, the one whose source is the ledger, under this
// key instead of Task.
func (w *world) ledgerFooterIs(key string) error {
	w.config.ledgerFooter = key
	return w.writeConfig()
}

// The scratch ledger moved to the file of phase 1 the pattern names, its tasks
// as they were, and ledger.files following it. The old file is removed, so
// nothing is left where itos read the ledger before; the registry stays where
// it is.
func (w *world) ledgerFilesAre(pattern string) error {
	if err := os.Remove(filepath.Join(w.dir, w.ledgerPath())); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	w.config.ledgerFiles = pattern
	if err := w.write(w.ledgerPath(), w.ledgerText()); err != nil {
		return err
	}
	return w.writeConfig()
}

// A footer of free text under the key, required for the types the
// comma-separated list names.
func (w *world) requiresTextFooter(key, types string) error {
	t := &textFooter{key: key}
	for _, typ := range strings.Split(types, ",") {
		t.types = append(t.types, strings.TrimSpace(typ))
	}
	w.config.textFooter = t
	return w.writeConfig()
}

// The footer of free text required only of the commits after HEAD: its since
// is HEAD's full SHA.
func (w *world) textFooterSinceHead(key string) error {
	t := w.config.textFooter
	if t == nil || t.key != key {
		return fmt.Errorf("the config has no %q footer of free text", key)
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	t.since = head
	return w.writeConfig()
}

func (w *world) proseIs(paths, steps string) error {
	w.config.prosePaths, w.config.proseSteps = paths, steps
	return w.writeConfig()
}

func (w *world) commitOnTop(message string) error {
	if err := w.write("README.md", "# Scratch\n\nA line for "+message+".\n"); err != nil {
		return err
	}
	return w.commit(message)
}

func (w *world) repositoryWithTask(tasks ...string) error {
	if err := w.startingFiles(tasks...); err != nil {
		return err
	}
	return w.commit("chore: start\n\nTask: " + strings.Join(tasks, " ") + "\n")
}

func (w *world) stageChange(path string) error {
	full := filepath.Join(w.dir, path)
	text, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := w.write(path, string(text)+"A staged change.\n"); err != nil {
		return err
	}
	return w.git("add", "--", path)
}

// The ledger's folder, tasks/, gone from the working tree, the index and HEAD:
// a repository whose config has a ledger footer and no ledger. Its removal is
// committed alone, whatever else is staged, so that the commit-msg hook checks
// a commit that stages none of itos's data, and reads no ledger in the index
// or, falling back, in the working tree.
func (w *world) ledgerFolderMissing() error {
	if err := os.RemoveAll(filepath.Join(w.dir, "tasks")); err != nil {
		return err
	}
	if err := w.git("rm", "-r", "-q", "--cached", "--", "tasks"); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", "chore: drop the ledger", "--", "tasks"); err != nil {
		return err
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, sha)
	return nil
}

// commitlint's conventional config is the rules the built-in lint holds,
// under commitlint's rule ids and words, so the scenarios that name it run
// the built-in one: commitlint left this repository when it switched to it
// (T-063), and the scenarios' text stays as it was (PLAN.md's risks).
func (w *world) conventionalHeaderLint() error {
	return w.builtinHeaderLint()
}

func (w *world) sinceFirstCommit() error {
	if len(w.commits) == 0 {
		return errors.New("the repository has no commit yet")
	}
	return w.sinceIs(w.commits[0])
}

func (w *world) sinceIs(value string) error {
	w.config.since = value
	return w.writeConfig()
}

// A clone of the scratch repository one commit deep, as actions/checkout
// makes by default, with the files the repository has not committed (its
// config among them) laid into it: only its history is shorter. itos and every
// later step run in the clone. A plain path clone ignores --depth, so the
// clone is of a file:// URL.
func (w *world) shallowClone() error {
	clone, err := os.MkdirTemp("", "itos-features-clone-")
	if err != nil {
		return err
	}
	if err := w.git("clone", "-q", "--depth", "1", "file://"+w.dir, clone); err != nil {
		os.RemoveAll(clone)
		return err
	}
	if err := w.layUncommitted(clone); err != nil {
		os.RemoveAll(clone)
		return err
	}
	w.origin, w.dir = w.dir, clone
	return nil
}

// The files the scratch repository has not committed, laid into a clone of
// it.
func (w *world) layUncommitted(clone string) error {
	cmd := exec.Command("git", "ls-files", "-z", "--modified", "--others", "--exclude-standard")
	cmd.Dir = w.dir
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	for _, path := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if path == "" {
			continue
		}
		if err := layFile(filepath.Join(w.dir, path), filepath.Join(clone, path)); err != nil {
			return err
		}
	}
	return nil
}

// The file at from, its mode kept, written at to; to removed when from is.
func layFile(from, to string) error {
	info, err := os.Stat(from)
	if errors.Is(err, os.ErrNotExist) {
		return os.RemoveAll(to)
	}
	if err != nil {
		return err
	}
	text, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, text, info.Mode().Perm())
}

func (w *world) recordingRangeCheck() error {
	w.config.rangeCheck = true
	return w.writeConfig()
}

// The recording shell: a script in the support folder that appends each
// command it is given to shell-log beside it, then runs it with sh -c (on
// windows script-exe running it, program_test.go).
func (w *world) recordingShellPath() string {
	return programPath(filepath.Join(w.support, "recording-shell"))
}

func (w *world) recordingShell() error {
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$1\" >> %s\nexec sh -c \"$1\"\n",
		quote(filepath.Join(w.support, "shell-log")))
	if err := w.writeProgram(filepath.Join(w.support, "recording-shell"), script); err != nil {
		return err
	}
	w.config.recordingShell = true
	return w.writeConfig()
}

// The ledger's task, with one check and no cost: of its own, staged: the
// commit-msg hook reads the ledger as the commit will hold it, and itos task
// reads the working tree, which holds the same.
func (w *world) taskHasCheck(task, check string) error {
	return w.stagedChecks(task, fmt.Sprintf("{ run: %q }", check))
}

// A script at the path in the scratch repository that writes the recording
// check's file when it runs, whatever its arguments: a check that calls it
// ran exactly when the file is there.
func (w *world) recordingScript(path string) error {
	script := "#!/bin/sh\nprintf 'ran\\n' > " + quote(filepath.Join(w.dir, recordingCheckFile)) + "\n"
	if err := w.write(path, script); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(w.dir, path), 0o755)
}

// A script at the path in the scratch repository that writes the arguments
// it is given, one a line, to a file in the support folder named after it
// (scriptWasGiven reads them back).
func (w *world) argumentsScript(path string) error {
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + quote(w.argumentsFile(path)) + "\n"
	if err := w.write(path, script); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(w.dir, path), 0o755)
}

func (w *world) argumentsFile(script string) string {
	return filepath.Join(w.support, filepath.Base(script)+".args")
}

// The kind's run.select is the command, {pattern} standing for the selection.
func (w *world) kindRunsSelection(kind, command string) error {
	if got := w.testsKind(); got != kind {
		return fmt.Errorf("the scratch config's kind of named tests is %q, not %q", got, kind)
	}
	w.config.runSelect = command
	return w.writeConfig()
}

// Whether the arguments script was given the argument, as one of its own,
// the last time it ran.
func (w *world) scriptWasGiven(script, arg string, want bool) error {
	text, err := os.ReadFile(w.argumentsFile(script))
	if err != nil {
		return fmt.Errorf("%s did not run: %w\n%s", script, err, w.report())
	}
	args := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	if slices.Contains(args, arg) != want {
		return fmt.Errorf("%s was given %q: %t, not %t\n%s", script, args, !want, want, w.report())
	}
	return nil
}

// The file, in the scratch repository, that the recording step writes.
const recordingStepFile = "step-ran"

// The file, in the scratch repository, that the recording check writes, and
// the check.
const recordingCheckFile = "check-ran"

var recordingCheck = "printf 'ran\\n' > " + recordingCheckFile

// The file, in the scratch repository, that the counting check appends a line
// to each time it runs.
const countingCheckFile = "check-runs"

// The counting check, in the mode given (run or fails): it appends a line to
// its file, then exits 3, so it fails as a run: and passes as a fails:.
func countingCheck(mode string) string {
	return fmt.Sprintf("{ %s: %q }", mode, "printf 'ran\\n' >> "+countingCheckFile+"; exit 3")
}

// A check of the ledger's that says cost: static, which its command alone
// would not make it, as the scratch config has no static patterns.
func staticCheck(command string) string { return fmt.Sprintf("{ run: %q, cost: static }", command) }

// The ledger's task with these checks, its other tasks as they were, staged:
// the commit-msg hook reads the ledger as the commit will hold it. A task the
// ledger does not have is added after the others. The stealth mode's ledger,
// in the git folder, is only written.
func (w *world) stagedChecks(task string, checks ...string) error {
	i := slices.IndexFunc(w.ledger, func(t ledgerTask) bool { return t.id == task })
	if i < 0 {
		w.ledger = append(w.ledger, ledgerTask{id: task})
		i = len(w.ledger) - 1
	}
	w.ledger[i].checks = checks
	if err := w.write(w.data(w.ledgerPath()), w.ledgerText()); err != nil {
		return err
	}
	if w.dataDir != "" {
		// The stealth mode's ledger, in the git folder, which git never stages.
		return nil
	}
	return w.git("add", "--", w.ledgerPath())
}

func (w *world) taskChecksAre(value string) error {
	on := value == "true"
	w.config.taskChecks = &on
	return w.writeConfig()
}

func (w *world) checkTimeoutIs(seconds int) error {
	w.config.checkTimeout = seconds
	return w.writeConfig()
}

func (w *world) statusesAre(list string) error {
	w.config.statuses = strings.Split(list, ", ")
	return w.writeConfig()
}

// The people file the config names, people.yaml beside the config, is not
// there.
func (w *world) peopleFileMissing() error {
	return os.Remove(filepath.Join(w.dir, w.data("people.yaml")))
}

func (w *world) groupsKeyIs(key string) error {
	w.config.groupsKey = key
	return w.writeConfig()
}

// The registry at its default path gives the group to the owner under key,
// and holds one unowned item of that group: a registry read for its owners
// anywhere but key finds the group unlisted. The owner is made one of the
// people, so that owning the group is no problem of its own.
func (w *world) registryGroupOwner(group, owner, key string) error {
	if err := w.write("people.yaml", "- someone\n- "+owner+"\n"); err != nil {
		return err
	}
	return w.write(startingRegistry, fmt.Sprintf(
		"%s: { %s: %s }\nitems:\n  - { id: W-1, title: One, phase: %s, owner: null, status: todo, depends_on: [] }\n",
		key, group, owner, group))
}

// The registry at its default path lists no group under its groups key and
// holds one unowned item of the group: an item whose group it does not list.
func (w *world) registryUnlistedGroup(item, group string) error {
	key := w.config.groupsKey
	if key == "" {
		key = "phases"
	}
	return w.write(startingRegistry, fmt.Sprintf(
		"%s: {}\nitems:\n  - { id: %s, title: %s, phase: %s, owner: null, status: todo, depends_on: [] }\n",
		key, item, item, group))
}

// A feature file under features/ with one live scenario, which the config's
// scenario kind (with a smoke set) reads, staged, so a commit made next adds
// it, as a feat adds the scenario it names.
func (w *world) featureFile(file, id string) error {
	if w.scenarioFiles == nil {
		w.scenarioFiles = map[string]string{}
	}
	w.scenarioFiles[id] = file
	path := filepath.Join("features", file)
	if err := w.write(path, featureText(file, id)); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	w.config.smoke = true
	return w.writeConfig()
}

// The text of a feature file with one live scenario, named after it.
func featureText(file, id string) string {
	return fmt.Sprintf("Feature: %s\n\n  %s\n  Scenario: %s runs\n    When it runs\n", file, id, id)
}

// A feature file under features/ with one live scenario, committed as the
// feat that adds it, with whatever else the working tree holds.
func (w *world) committedFeatureFile(file, id string) error {
	if w.scenarioFiles == nil {
		w.scenarioFiles = map[string]string{}
	}
	w.scenarioFiles[id] = file
	if err := w.write(filepath.Join("features", file), featureText(file, id)); err != nil {
		return err
	}
	return w.commit("feat: add the scenario " + id)
}

// The kind's one range check is the built-in moves rule, leaving alone the
// types the comma-separated list names.
func (w *world) movesRule(types string) error {
	m := &moves{}
	for _, t := range strings.Split(types, ",") {
		m.exceptTypes = append(m.exceptTypes, strings.TrimSpace(t))
	}
	w.config.moves = m
	return w.writeConfig()
}

func (w *world) movesAllowRename(id, name string) error {
	if w.config.moves == nil {
		return errors.New("the kind's range check is not the moves rule")
	}
	w.config.moves.renames = append(w.config.moves.renames, [2]string{id, name})
	return w.writeConfig()
}

// An edit of a scenario's text: its steps changed, or its name.
type scenarioEdit func(text, id string) string

func changeSteps(text, id string) string {
	return strings.Replace(text, "    When it runs\n", "    When it runs twice\n", 1)
}

func rename(name string) scenarioEdit {
	return func(text, id string) string {
		return strings.Replace(text, "Scenario: "+id+" runs\n", "Scenario: "+name+"\n", 1)
	}
}

// The feature file holding the scenario, edited in the working tree.
func (w *world) editScenario(id string, edit scenarioEdit) (string, error) {
	file, ok := w.scenarioFiles[id]
	if !ok {
		return "", fmt.Errorf("no feature file has the scenario %s", id)
	}
	path := filepath.Join("features", file)
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return "", err
	}
	changed := edit(string(text), id)
	if changed == string(text) {
		return "", fmt.Errorf("the edit left %s as it was", path)
	}
	return path, w.write(path, changed)
}

// The scenario's feature file, edited, staged alone.
func (w *world) stageScenarioEdit(id string, edit scenarioEdit) error {
	path, err := w.editScenario(id, edit)
	if err != nil {
		return err
	}
	return w.git("add", "--", path)
}

// The scenario taken out of its feature file, which is deleted, and written
// unchanged in a new one under another header, both staged.
func (w *world) stageScenarioMove(id, to string) error {
	file, ok := w.scenarioFiles[id]
	if !ok {
		return fmt.Errorf("no feature file has the scenario %s", id)
	}
	from := filepath.Join("features", file)
	text, err := os.ReadFile(filepath.Join(w.dir, from))
	if err != nil {
		return err
	}
	_, block, found := strings.Cut(string(text), "\n\n")
	if !found {
		return fmt.Errorf("%s has no scenario after its header", from)
	}
	if err := w.write(filepath.Join("features", to), "Feature: "+to+"\n\n"+block); err != nil {
		return err
	}
	if err := w.git("rm", "-q", "--", from); err != nil {
		return err
	}
	w.scenarioFiles[id] = to
	return w.git("add", "--", filepath.Join("features", to))
}

// A commit naming the task whose one change is the scenario's steps.
func (w *world) commitChangingSteps(message, task, id string) error {
	if _, err := w.editScenario(id, changeSteps); err != nil {
		return err
	}
	return w.commit(message + "\n\nTask: " + task + "\n")
}

// The smoke set holds the one scenario, under the feature file it is in.
func (w *world) smokeSetLists(id string) error {
	file, ok := w.scenarioFiles[id]
	if !ok {
		return fmt.Errorf("no feature file has the scenario %s", id)
	}
	return w.write("features/smoke.yaml", fmt.Sprintf(
		"- file: %s\n  scenarios: [{ id: %q, why: the one listed }]\n", file, id))
}

func (w *world) smokeEveryFileIs(value string) error {
	every := value == "true"
	w.config.smokeEveryFile = &every
	return w.writeConfig()
}

// The scratch config's kind is written without tag_prefix, so the kind takes
// itos's default.
func (w *world) noTagPrefix() error {
	w.config.noTagPrefix = true
	return w.writeConfig()
}

func (w *world) folder(path string) error {
	return os.MkdirAll(filepath.Join(w.dir, path), 0o755)
}

func (w *world) hooksManagerIs(manager string) error {
	w.config.hooksManager = manager
	return w.writeConfig()
}

func (w *world) hooksBinIs(bin string) error {
	w.config.hooksBin = bin
	return w.writeConfig()
}

func (w *world) costStaticIs(pattern string) error {
	w.config.costStatic = []string{pattern}
	return w.writeConfig()
}

func (w *world) coversIs(step, pattern string) error {
	w.config.covers = append(w.config.covers, cover{step, pattern})
	return w.writeConfig()
}

func (w *world) nightlyOnlyIs(command string) error {
	w.config.nightlyOnly = []string{command}
	return w.writeConfig()
}

func (w *world) configSets(key, value string) error {
	w.config.settings = append(w.config.settings, setting{key, value})
	return w.writeConfig()
}

// CI's one step runs the kind's named tests. The kind has a feature file with
// two live scenarios, @ID-A-01 and @ID-A-02, and a smoke set of the second, so
// a push's run selects the smoke set and what a check adds to it.
func (w *world) ciStepsRunTests(kind string) error {
	feature := "Feature: A\n\n  @ID-A-01\n  Scenario: One\n    When one runs\n\n" +
		"  @ID-A-02\n  Scenario: Two\n    When two runs\n"
	if err := w.write("features/a.feature", feature); err != nil {
		return err
	}
	if err := w.write("features/smoke.yaml",
		"- file: a.feature\n  scenarios: [{ id: \"@ID-A-02\", why: the one listed }]\n"); err != nil {
		return err
	}
	w.config.ciTests = kind
	return w.writeConfig()
}

// A check written as this command is read back as the kind's smoke run, the
// run of exactly its smoke set (as: smoke). The kind is the one CI runs.
func (w *world) recognizesSmokeRun(kind, command string) error {
	if kind != w.config.ciTests {
		return fmt.Errorf("the CI steps run the kind %q, not %q", w.config.ciTests, kind)
	}
	w.config.smokeRuns = append(w.config.smokeRuns, command)
	return w.writeConfig()
}

// The smoke set lists no file, so a push selects only what its commits name.
func (w *world) emptySmokeSet() error {
	return w.write("features/smoke.yaml", "[]\n")
}

func (w *world) ciStepsAre(steps ...string) error {
	w.config.ciSteps = steps
	return w.writeConfig()
}

func (w *world) headerLintIs(command string) error {
	w.config.headerLintCommand = command
	return w.writeConfig()
}

func (w *world) builtinHeaderLint() error {
	w.config.headerLintBuiltin = true
	return w.writeConfig()
}

func (w *world) stopAtFirstFailureIs(value string) error {
	stop := value == "true"
	w.config.stopAtFirst = &stop
	return w.writeConfig()
}

func (w *world) registryIs(path string) error {
	w.config.registry = path
	return w.writeConfig()
}

// The repository's one work registry is at path, with one unowned item of
// phase 1: the registry it started with is removed when it is elsewhere, so
// nothing is left where itos would otherwise look. The working tree and the
// index both hold it, so what reads the staged tree reads it too.
func (w *world) registryAt(path, item, status string) error {
	if path != startingRegistry {
		if err := os.Remove(filepath.Join(w.dir, startingRegistry)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := w.workingRegistry(path, item, status); err != nil {
		return err
	}
	return w.git("add", "-A", "--", path, startingRegistry)
}

// The registry at path holds one unowned item of phase 1, in the working tree
// alone: nothing is staged.
func (w *world) workingRegistry(path, item, status string) error {
	return w.workingRegistryOwned(path, item, "null", status)
}

// The registry at path holds one item of phase 1, a group nobody owns, its
// owner written as YAML, in the working tree alone.
func (w *world) workingRegistryOwned(path, item, owner, status string) error {
	return w.write(path, fmt.Sprintf(
		"phases: { 1: null }\nitems:\n  - { id: %s, title: %s, phase: 1, owner: %s, status: %s, depends_on: [] }\n",
		item, item, owner, status))
}

// A folder of the scenario's support folder first on the PATH itos and its
// hooks run with, made and put there once, for the programs a scenario
// stands in.
func (w *world) binOnPath() (string, error) {
	bin := filepath.Join(w.support, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", err
	}
	path := "PATH=" + bin + string(os.PathListSeparator) + w.basePath()
	if !slices.Contains(w.vars, path) {
		w.vars = append(w.vars, path)
	}
	return bin, nil
}

// Every identity lookup fails: the scratch config names no work.identity, so
// its provider is github, and the gh first on the PATH is signed out,
// answering nothing and exiting 4 to anything asked, as gh does (gh.exe on
// windows, which would otherwise find the runner's own gh).
func (w *world) noIdentity() error {
	bin, err := w.binOnPath()
	if err != nil {
		return err
	}
	script := "#!/bin/sh\necho 'not logged in' >&2\nexit 4\n"
	return w.writeProgram(filepath.Join(bin, "gh"), script)
}

// work's text proposal lists the item under "Can start now:", the section of
// the session's own items that can start, by its ID, the first word of its
// line.
func (w *world) proposesToStart(item string) error {
	in := false
	for _, line := range strings.Split(w.stdout, "\n") {
		switch {
		case line == "Can start now:":
			in = true
		case strings.TrimSpace(line) == "":
			in = false
		case in && strings.Fields(line)[0] == item:
			return nil
		}
	}
	return fmt.Errorf("itos does not propose %q to start\n%s", item, w.report())
}

// The registry at path holds these unowned items of phase 1, each with its
// status, in the working tree alone.
func (w *world) workingRegistryItems(path string, items [][2]string) error {
	var b strings.Builder
	b.WriteString("phases: { 1: null }\nitems:\n")
	for _, item := range items {
		fmt.Fprintf(&b, "  - { id: %s, title: %s, phase: 1, owner: null, status: %s, depends_on: [] }\n",
			item[0], item[0], item[1])
	}
	return w.write(path, b.String())
}

// The nightly's one step runs the checks of the tasks whose work item is
// done: every one of them, or only the static ones.
func (w *world) nightlyTasksAre(which string) error {
	w.config.nightlyTasks = which
	return w.writeConfig()
}

// The same registry, staged, with the starting one's removal when it moved.
func (w *world) stagedRegistryAt(path, item, status string) error {
	return w.registryAt(path, item, status)
}

// The same registry, committed past the hooks.
func (w *world) committedRegistryAt(path, item, status string) error {
	if err := w.registryAt(path, item, status); err != nil {
		return err
	}
	return w.commit("docs: a registry")
}

// The scratch repository's itos.yaml with one more top-level key, staged.
func (w *world) stagedConfigKey(key string) error {
	if err := w.writeConfig(); err != nil {
		return err
	}
	text, err := os.ReadFile(filepath.Join(w.dir, "itos.yaml"))
	if err != nil {
		return err
	}
	if err := w.write("itos.yaml", string(text)+key+": true\n"); err != nil {
		return err
	}
	return w.git("add", "--", "itos.yaml")
}

// The ledger's one task with one more key, staged.
func (w *world) stagedLedgerKey(task, key string) error {
	if err := w.write("tasks/phase-1.yaml",
		fmt.Sprintf("- { id: %s, type: chore, title: Tidy, %s: Tidy }\n", task, key)); err != nil {
		return err
	}
	return w.git("add", "--", "tasks/phase-1.yaml")
}

// When steps.

// itos with these arguments, in the scratch repository.
func (w *world) itos(args ...string) error { return w.itosIn(w.dir, args...) }

// itos run in the folder dir.
func (w *world) itosIn(dir string, args ...string) error {
	w.markRun()
	return w.run(dir, w.bin, args...)
}

// A program run in the folder dir, its exit code and output what the Then
// steps read.
func (w *world) run(dir, program string, args ...string) error {
	return w.runWith(dir, nil, program, args...)
}

// A program run in the folder dir with stdin on its standard input (none
// when nil), as run.
func (w *world) runWith(dir string, stdin io.Reader, program string, args ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	cmd.Env = w.env()
	cmd.Stdin = stdin
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	w.stdout, w.stderr = stdout.String(), stderr.String()
	var exit *exec.ExitError
	switch {
	case err == nil:
		w.exit = 0
	case errors.As(err, &exit):
		w.exit = exit.ExitCode()
	default:
		return fmt.Errorf("running %s: %w", program, err)
	}
	return nil
}

func (w *world) commitMsgHook(message string) error {
	file := filepath.Join(w.support, "COMMIT_EDITMSG")
	if err := os.WriteFile(file, []byte(message), 0o644); err != nil {
		return err
	}
	return w.itos("hook", "commit-msg", file)
}

// Then steps.

func (w *world) output() string { return w.stdout + w.stderr }

func (w *world) report() string {
	return fmt.Sprintf("exit %d\n--- stdout\n%s--- stderr\n%s", w.exit, w.stdout, w.stderr)
}

func (w *world) exitsWith(code int) error {
	if w.exit != code {
		return fmt.Errorf("itos exited %d, not %d\n%s", w.exit, code, w.report())
	}
	return nil
}

func (w *world) outputSays(text string) error {
	if !strings.Contains(w.output(), text) {
		return fmt.Errorf("the output does not say %q\n%s", text, w.report())
	}
	return nil
}

func (w *world) outputDoesNotSay(text string) error {
	if strings.Contains(w.output(), text) {
		return fmt.Errorf("the output says %q\n%s", text, w.report())
	}
	return nil
}

func (w *world) outputNamesRule(rule string) error {
	if !strings.Contains(w.output(), "["+rule+"]") {
		return fmt.Errorf("the output does not name the rule [%s]\n%s", rule, w.report())
	}
	return nil
}

// Standard output is one JSON object whose valid is false: a report, not a
// crash's message, that says what it checked is not sound.
func (w *world) invalidJSONReport() error {
	var report struct {
		Valid *bool `json:"valid"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
		return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	if report.Valid == nil || *report.Valid {
		return fmt.Errorf("the JSON report's valid is not false\n%s", w.report())
	}
	return nil
}

// Standard output is one JSON object whose items list the item, by its id,
// with a title that is not empty: what a reader of the registry's titles
// asks work list --json for.
func (w *world) jsonListsItem(id string) error {
	var listing struct {
		Items []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &listing); err != nil {
		return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	for _, item := range listing.Items {
		if item.ID == id && strings.TrimSpace(item.Title) != "" {
			return nil
		}
	}
	return fmt.Errorf("the JSON's items do not list %s with a title\n%s", id, w.report())
}

// A line of the output names the task, as a word of its own, and gives it the
// status, as whole words: the status table's line for the task.
func (w *world) outputLists(task, status string) error {
	word := regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(status) + `(\s|$)`)
	for _, line := range strings.Split(w.output(), "\n") {
		if slices.Contains(strings.Fields(line), task) && word.MatchString(line) {
			return nil
		}
	}
	return fmt.Errorf("the output does not list %s as %q\n%s", task, status, w.report())
}

// The counting check's file has one line a run.
func (w *world) countingCheckRan(times int) error {
	text, err := os.ReadFile(filepath.Join(w.dir, countingCheckFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if runs := strings.Count(string(text), "\n"); runs != times {
		return fmt.Errorf("the counting check ran %d times, not %d\n%s", runs, times, w.report())
	}
	return nil
}

func (w *world) rangeCheckStartedAtFirst() error {
	text, err := os.ReadFile(filepath.Join(w.support, "range-from"))
	if err != nil {
		return fmt.Errorf("the range check did not run: %w\n%s", err, w.report())
	}
	if got := strings.TrimSpace(string(text)); got != w.commits[0] {
		return fmt.Errorf("the range check started at %q, not the first commit %s\n%s", got, w.commits[0], w.report())
	}
	return nil
}

// The commands the recording shell was given, one a line.
func (w *world) shellLog() ([]string, error) {
	text, err := os.ReadFile(filepath.Join(w.support, "shell-log"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(string(text), "\n"), "\n"), err
}

func (w *world) recordingShellRan(command string) error {
	log, err := w.shellLog()
	if err != nil {
		return err
	}
	for _, line := range log {
		if line == command {
			return nil
		}
	}
	return fmt.Errorf("the recording shell did not run %q; it ran %q\n%s", command, log, w.report())
}

// The range check is the only command that names the file it records to.
func (w *world) recordingShellRanRangeCheck() error {
	log, err := w.shellLog()
	if err != nil {
		return err
	}
	file := quote(filepath.Join(w.support, "range-from"))
	for _, line := range log {
		if strings.Contains(line, file) {
			return nil
		}
	}
	return fmt.Errorf("the recording shell did not run the range check; it ran %q\n%s", log, w.report())
}

func (w *world) recordingCheckRan(want bool) error {
	_, err := os.Stat(filepath.Join(w.dir, recordingCheckFile))
	ran := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if ran != want {
		return fmt.Errorf("the recording check ran: %t, not %t\n%s", ran, want, w.report())
	}
	return nil
}

// The file is a hook that hands its work to itos's hook command.
func (w *world) fileCallsItos(path string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w\n%s", path, err, w.report())
	}
	if !strings.Contains(string(text), "itos hook ") {
		return fmt.Errorf("%s does not call itos:\n%s\n%s", path, text, w.report())
	}
	return nil
}

func (w *world) recordingStepRan(want bool) error {
	_, err := os.Stat(filepath.Join(w.dir, recordingStepFile))
	ran := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if ran != want {
		return fmt.Errorf("the recording step ran: %t, not %t\n%s", ran, want, w.report())
	}
	return nil
}
