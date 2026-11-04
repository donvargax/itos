package brief

import (
	"strings"
	"testing"
)

// sliceBriefs are the briefs of a slice, by role, as they were before a task
// had texts of its own (slice-220). They are Go strings, not files, so a
// checkout's line endings cannot change them. A change to a slice's or a bug's
// brief changes them on purpose, here.
var sliceBriefs = map[string]string{
	"drafter": "Specify slice-9 (a slice) of project, in the repository at /work/project: Say the next step. The scenarios are @ID-A-01 in features/tagged.feature, all `@wip`. Your job is the spec alone: the `@wip` scenarios the item needs, or a task's ledger entry with its checks, committed as the item's spec and pushed. You write no implementation and no step definitions; an implementing agent writes those later, from your spec. Take nothing: the implementer takes the item.\n\n" +
		"Run `itos guide brief` first, then `itos work show slice-9`, and read AGENTS.md and features/tagged.feature. Read in full the feature file the scenarios go in, the step definitions they will use and the code they reach, so that each step you write either exists or is plainly new.\n\n" +
		"<settled decisions, filled by the coordinator>\n\n" +
		"Every settled decision goes into the scenarios or the comments above them, never re-decided; where the code shows one cannot work as stated, stop and say so in your report rather than inventing around it. A decision the spec leaves open you make yourself, in the spec, and list in your report.\n\n" +
		"Mint every scenario ID with `itos tests next-id`, never by hand, and put each scenario beside its topic's block in the file, never at the file's end. Tag each `@wip @slice-9` with its ID; a comment above the tag line gives its reason when it is not obvious. A Given sets up and never asserts: trace each one to what it leaves in the repository. Every scenario must be able to fail before the work and pass after a correct one; say in the commit body at which step each fails today, and why. Other drafters may be specifying sibling items at the same time: touch only your own feature file.\n\n" +
		"Commit with `itos commit --item slice-9`: the header `docs: specify slice-9, <short rule>`, the body the problem, the rule and each scenario's failing step. Push with `itos push`. Do the work yourself; start no agents. Prefix your scratch files with slice-9: sessions share the folder.\n\n" +
		"Report: the pushed commit, each scenario's ID, title and failing step before the work, the decisions you made yourself, the step definitions the implementer must add, <the item's own questions, filled by the coordinator>, and anything you are unsure of.\n",
	"implementer": "Implement slice-9 (a slice) of project, in the repository at /work/project: Say the next step. The scenarios are @ID-A-01 in features/tagged.feature, all `@wip`. Take it with `itos work take slice-9` and push that first.\n\n" +
		"Run `itos guide work` first, then `itos work show slice-9`, and read AGENTS.md and features/tagged.feature.\n\n" +
		"<settled decisions, filled by the coordinator>\n\n" +
		"Red first: commit the step definitions the scenarios need alone, in a `test` commit made with `itos commit --item slice-9`, the scenarios still `@wip`; run them with `@wip` removed locally and see each fail at the step that checks the behaviour; then build. Writing the missing step definitions is the test commit's job: an undefined step is work, not a reason to stop. Check the scenarios' names against what you build, correcting only a name, never what a scenario checks. Stop only for a scenario that passes before the work, or one that fails in its setup where no step definition can fix it, and propose the change.\n\n" +
		"Before pushing, run what the change can reach beyond what it names: <neighbouring scenarios and checks, filled by the coordinator>.\n\n" +
		"Others push to main while you work: commit with `itos commit`, push with `itos push`, never force (a branch main moved past is brought up with `itos push --onto-main`). Do the work yourself; start no agents. Each commit's body says why, and its footers stand alone in its last paragraph, the one place git reads them. Prefix your scratch files with slice-9: sessions share the folder. Push, wait for CI with `itos push` in the background, and close the item with `itos work done slice-9` once it is green.\n\n" +
		"Report: the commits, the CI run's address, each scenario's failing step before the work, the scenarios turned green, <the item's own questions, filled by the coordinator>, the ideas you added, and any scenario text corrected and why.\n",
	"reviewer": "Review slice-9 (a slice) of project, in the repository at /work/project: Say the next step. The scenarios are @ID-A-01 in features/tagged.feature, all `@wip`. Your job is a verdict on the item's spec and the work on it: you change nothing, commit nothing and push nothing.\n\n" +
		"Run `itos work show slice-9` first, and read AGENTS.md and features/tagged.feature. Read what the change must do before what it does: the scenarios, a task's why and its checks, the plan or decision they name.\n\n" +
		"<settled decisions, filled by the coordinator>\n\n" +
		"For each scenario, say whether it can fail before the work and pass after a correct one, reading the steps it uses and what each Given leaves in the repository; one that passes before the work, passes for the wrong reason, or depends on the environment it runs in instead of setting it up is a P1 finding. Take neither the spec's, a commit message's nor a comment's word for what the code does: for each changed function, try the inputs the spec does not name. Report what the change leaves out or adds beyond its spec, and an error check no run can reach. Do not report formatting, naming or wording no contract fixes; the linters judge those.\n\n" +
		"Report: a verdict, approve or request changes; each finding with its severity (P1 blocks, P2 should be fixed, P3 is a note), where it is and what would fix it; for each scenario, whether it can fail before the work and pass after a correct one; <the item's own questions, filled by the coordinator>; and anything you are unsure of.\n",
}

// asBug is a slice's brief as the fixture's bug is briefed: its text is the
// slice's, but for what names the item.
var asBug = strings.NewReplacer(
	"slice-9", "bug-7",
	"(a slice)", "(a bug)",
	"Say the next step", "A bug",
	"@ID-A-01 in features/tagged.feature", "@ID-F-01 in features/fixes.feature",
	"features/tagged.feature", "features/fixes.feature",
)

// A task's text leaves no placeholder of a template unfilled: only what a person
// adds, each marked, and the drafter's header, which is the drafter's to write.
func TestTaskBriefsFillEveryPlaceholder(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml":    "phases: { 1: null }\nitems:\n  - { id: T-001, title: Tidy, phase: 1, owner: null, status: todo, kind: task, depends_on: [slice-8] }\n  - { id: slice-8, title: Earlier, phase: 1, owner: null, status: done, depends_on: [] }\n",
		"AGENTS.md":          "Rules.\n",
		"tasks/phase-1.yaml": "- id: T-001\n  type: chore\n  title: Tidy\n  done_when:\n    - { run: go vet ./... }\n",
	})
	marked := strings.NewReplacer(settledMark, "", neighboursMark, "", questionsMark, "", "<short rule>", "")
	for _, role := range []string{"drafter", "implementer", "reviewer"} {
		text := brief(t, p, "T-001", role).Text
		if left := marked.Replace(text); strings.Contains(left, "<") || strings.Contains(left, ">") {
			t.Errorf("the %s's brief keeps a placeholder:\n%s", role, left)
		}
		if !strings.Contains(text, "The task is T-001, whose checks are `go vet ./...`. It depends on slice-8 (done). ") {
			t.Errorf("the %s's brief does not name the task's spec:\n%s", role, text)
		}
	}
}

// A slice's and a bug's briefs are byte for byte what they were before a task
// had texts of its own, for every role: no scenario can pin a whole brief, as
// their output would become released cases (ADR-0054 exempts guide text).
func TestSliceAndBugBriefsGolden(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml":         "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Say the next step, phase: 1, owner: null, status: todo, depends_on: [] }\n  - { id: bug-7, title: A bug, phase: 1, owner: null, status: todo, depends_on: [] }\n",
		"AGENTS.md":               "Rules.\n",
		"features/tagged.feature": "Feature: Tagged\n\n  @ID-A-01 @wip @slice-9\n  Scenario: one\n    When it runs\n",
		"features/fixes.feature":  "Feature: Fixes\n\n  @ID-F-01 @wip @bug-7\n  Scenario: one\n    When it runs\n",
	})
	for role, slice := range sliceBriefs {
		for id, want := range map[string]string{"slice-9": slice, "bug-7": asBug.Replace(slice)} {
			if got := brief(t, p, id, role).Text; got != want {
				t.Errorf("%s's %s brief changed:\n got %q\nwant %q", id, role, got, want)
			}
		}
	}
}
