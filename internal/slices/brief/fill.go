package brief

import (
	"fmt"
	"path"
	"strings"

	guidetext "github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/tests"
)

// facts are what a template is filled with: the item's ID, Role, Kind and
// Title, the repository's Top as git prints it, the Scenarios tagged for
// the item (a task's ledger entry instead, task), what it Depends on, the
// paths Read and what was Omitted, and the repository's own brief Lines.
type facts struct {
	ID, Role, Kind, Title, Top string
	Scenarios                  []tests.TaggedFile
	task                       *ledger.Task
	specFiles                  []string
	Depends                    []dependency
	Reads                      []string
	Omitted                    []Omitted
	Lines                      string
}

// The template's placeholders, as written in the brief procedure's quoted
// block (internal/guide/brief.md) and in the templates beside it, each
// replaced by one fact; the three a person fills are replaced by a marked
// placeholder of the same form.
const (
	itemOfProject = "<item> of <project>"
	item          = "<item>"
	absolutePath  = "<absolute path>"
	oneLine       = "<one line>"
	scenariosLine = "The scenarios are <ids> in <feature file>, all `@wip` (or: the task is <id>)."
	readsLine     = "<the repository's agent instructions and the docs this item rests on>."
	settled       = "<What is settled: the decisions to build to, not to re-decide. Any live scenario this item makes untrue, and that the commit must say so.>"
	neighbours    = "<the neighbouring scenarios and checks; for an item that moves a rule from one command to another, the scenarios of both commands>"
	questions     = "<the item's own questions>"

	settledMark    = "<settled decisions, filled by the coordinator>"
	neighboursMark = "<neighbouring scenarios and checks, filled by the coordinator>"
	questionsMark  = "<the item's own questions, filled by the coordinator>"
)

// fill is the role's template filled with f: each paragraph on one line,
// the placeholders replaced, and the repository's own brief lines after a
// blank line when there are any.
func fill(role string, f facts) string {
	template, _ := guidetext.Template(role)
	r := strings.NewReplacer(
		itemOfProject, fmt.Sprintf("%s (a %s) of %s", f.ID, f.Kind, path.Base(f.Top)),
		item, f.ID,
		absolutePath, f.Top,
		oneLine, f.Title,
		scenariosLine, specSentence(f)+dependsSentence(f.Depends),
		readsLine, readsSentence(f.Reads, f.Omitted),
		settled, settledMark,
		neighbours, neighboursMark,
		questions, questionsMark,
	)
	var paragraphs []string
	for _, p := range strings.Split(strings.TrimSpace(template), "\n\n") {
		paragraphs = append(paragraphs, r.Replace(strings.Join(strings.Fields(p), " ")))
	}
	text := strings.Join(paragraphs, "\n\n") + "\n"
	if f.Lines != "" {
		text += "\n" + f.Lines
	}
	return text
}

// specSentence names the item's spec: a task's checks, the scenarios
// tagged for the item by file, and which are live, or that nothing is
// tagged yet.
func specSentence(f facts) string {
	if f.Kind == "task" {
		switch {
		case f.task == nil:
			return f.ID + " has no ledger entry yet."
		case len(f.task.DoneWhen) == 0:
			return "The task is " + f.ID + ", which has no checks yet."
		}
		var checks []string
		for _, c := range f.task.DoneWhen {
			checks = append(checks, "`"+c.Command()+"`")
		}
		return "The task is " + f.ID + ", whose checks are " + list(checks) + "."
	}
	if len(f.Scenarios) == 0 {
		return "No scenario is tagged @" + f.ID + " yet."
	}
	var files []string
	byFile := map[string][]string{}
	var live []string
	for _, s := range f.Scenarios {
		if _, seen := byFile[s.Path]; !seen {
			files = append(files, s.Path)
		}
		byFile[s.Path] = append(byFile[s.Path], s.ID)
		if s.Live {
			live = append(live, s.ID)
		}
	}
	var parts []string
	for _, file := range files {
		parts = append(parts, list(byFile[file])+" in "+file)
	}
	sentence := "The scenarios are " + strings.Join(parts, " and ")
	switch {
	case len(live) == 0:
		return sentence + ", all `@wip`."
	case len(live) == len(f.Scenarios):
		return sentence + ", all live."
	case len(live) == 1:
		return sentence + "; " + live[0] + " is live."
	}
	return sentence + "; " + list(live) + " are live."
}

// dependsSentence names what the item depends on, with each status, ""
// for nothing.
func dependsSentence(deps []dependency) string {
	if len(deps) == 0 {
		return ""
	}
	var named []string
	for _, d := range deps {
		named = append(named, d.ID+" ("+d.Status+")")
	}
	return " It depends on " + list(named) + "."
}

// readsSentence names the files read, and what was left out and why.
func readsSentence(reads []string, omitted []Omitted) string {
	sentence := "nothing."
	if len(reads) > 0 {
		sentence = list(reads) + "."
	}
	if len(omitted) == 0 {
		return sentence
	}
	var said []string
	for _, o := range omitted {
		said = append(said, o.Path+" "+omittedWhy(o.Reason))
	}
	return sentence + " Left out: " + strings.Join(said, "; ") + "."
}

// omittedWhy is an omission's reason as a sentence says it.
func omittedWhy(reason string) string {
	switch reason {
	case notInTree, symlinked:
		return "is " + reason
	default:
		return "has " + reason
	}
}

// list is the items as prose names them: "a", "a and b", "a, b and c".
func list(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
