// The steps of init's rerun status (init-status.feature, bug 114): a
// repository init set up and committed, its AGENTS.md holding a stale block
// or the current one, and the rerun's report read line by line, its first
// line the config's alone and the rules offer's lines after it.
package features

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// What a rerun's first line says of the config it found: kept, and no more.
const configKeptLine = "itos.yaml is there already; init kept it as it is."

func initializeInitStatusSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a repository with a valid existing itos config and installed hooks$`, w.initializedRepository)
	sc.Step(`^all release/update observations use local fixtures$`, w.releasesLocal)
	sc.Step(`^AGENTS\.md has a stale marked itos rules block$`, func() error {
		return w.committedFileWithOlderBlock("AGENTS.md", "Be kind to the build.", "Task:")
	})
	sc.Step(`^AGENTS\.md and its Claude import already contain the current rules$`, w.currentRulesCommitted)
	sc.Step(`^its first existing-config status explicitly says that the config contents were kept$`, func() error {
		if first := w.firstLine(); first != configKeptLine {
			return fmt.Errorf("the first line is %q, not %q\n%s", first, configKeptLine, w.report())
		}
		return nil
	})
	sc.Step(`^it does not say that init as a whole wrote nothing$`, func() error {
		if first := w.firstLine(); strings.Contains(first, "wrote nothing") {
			return fmt.Errorf("the first line says init wrote nothing: %q\n%s", first, w.report())
		}
		return nil
	})
	sc.Step(`^the stale marked rules block is updated using the existing rules behavior$`, w.staleBlockRewritten)
	sc.Step(`^the rules result separately reports what it wrote$`, func() error {
		return w.laterLines("updated AGENTS.md",
			"AGENTS.md holds itos's rules for agents, between <!-- itos:begin --> and <!-- itos:end -->")
	})
	sc.Step(`^its rules result separately says that the block is already current$`, func() error {
		return w.laterLines("AGENTS.md holds this itos's rules for agents already, and CLAUDE.md imports them.")
	})
}

// A repository init set up, away from a terminal and offering nothing, with
// what it wrote committed: its config, ledger and registry, and the hooks it
// declared in the git config.
func (w *world) initializedRepository() error {
	if err := w.repositoryWithoutItos("docs: start"); err != nil {
		return err
	}
	if err := w.alreadyRan("init --plugin no --no-git-shim --no-agent-rules"); err != nil {
		return err
	}
	return w.commit("chore: adopt itos\n\nTask: T-1")
}

// Every run keeps to the scenario's own machine: with no release server of
// the scenario's, launcherEnv gives itos an address nothing listens on and
// ITOS_NO_UPDATE.
func (w *world) releasesLocal() error {
	if w.releases != nil {
		return fmt.Errorf("the scenario started a release server, so its runs are not the default local ones")
	}
	return nil
}

// AGENTS.md holding this itos's block and CLAUDE.md importing it, as init
// --agent-rules writes them, committed.
func (w *world) currentRulesCommitted() error {
	if err := w.alreadyRan("init --agent-rules --plugin no --no-git-shim"); err != nil {
		return err
	}
	return w.commit("docs: add the rules for agents")
}

// AGENTS.md's block is this itos's, the older one's text gone from between
// the markers, the project's line outside them kept, and CLAUDE.md imports
// it.
func (w *world) staleBlockRewritten() error {
	if err := w.saysBetweenMarkers("AGENTS.md", "Task:", false); err != nil {
		return err
	}
	if err := w.saysBetweenMarkers("AGENTS.md", "Start with `itos go`", true); err != nil {
		return err
	}
	if err := w.fileHasLine("AGENTS.md", "Be kind to the build.", ""); err != nil {
		return err
	}
	return w.fileHasLine("CLAUDE.md", "@AGENTS.md", "")
}

// The first line of the last run's standard output.
func (w *world) firstLine() string {
	first, _, _ := strings.Cut(w.stdout, "\n")
	return first
}

// Each text begins a line of the last run's standard output after its
// first, the config's.
func (w *world) laterLines(texts ...string) error {
	lines := strings.Split(w.stdout, "\n")
	for _, text := range texts {
		if !slices.ContainsFunc(lines[1:], func(l string) bool { return strings.HasPrefix(l, text) }) {
			return fmt.Errorf("no line after the first begins %q\n%s", text, w.report())
		}
	}
	return nil
}
