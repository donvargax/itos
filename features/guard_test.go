// The guard's steps (guard.feature): Claude Code's PreToolUse input, built
// here as Claude Code writes it (the session, the event, the tool's name and
// input, the folder), handed to itos guard claude-code on stdin, run in the
// folder the input names, as Claude Code runs a hook in the session's folder.
// What itos answers is read as Claude Code reads it: a deny is stdout's JSON,
// anything else no answer at all.
package features

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func initializeGuardSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^Claude Code asks itos about the Bash command "([^"]*)"$`, func(command string) error {
		return w.askAboutBash(w.dir, command)
	})
	sc.Step(`^Claude Code asks itos about the Bash command "([^"]*)" run in "([^"]*)"$`, func(command, folder string) error {
		return w.askAboutBash(filepath.Join(w.dir, folder), command)
	})
	sc.Step(`^Claude Code asks itos about the Bash command "([^"]*)" in that repository$`, func(command string) error {
		return w.askAboutBash(w.plain(), command)
	})
	sc.Step(`^Claude Code asks itos about the tool "([^"]*)" on the file "([^"]*)"$`, w.askAboutFileTool)
	sc.Step(`^Claude Code sends itos "([^"]*)" as a PreToolUse input$`, func(input string) error {
		return w.preToolUse(w.dir, input)
	})

	sc.Step(`^itos denies the command, its reason saying "([^"]*)"$`, w.deniesSaying)
	sc.Step(`^itos writes nothing to stdout$`, w.writesNothing)
}

// The input Claude Code sends before the tool runs in the folder dir.
func (w *world) askAbout(dir, tool string, input map[string]any) error {
	raw, err := json.Marshal(map[string]any{
		"session_id":      "features",
		"transcript_path": filepath.Join(w.support, "transcript.jsonl"),
		"cwd":             dir,
		"permission_mode": "default",
		"hook_event_name": "PreToolUse",
		"tool_name":       tool,
		"tool_input":      input,
		"tool_use_id":     "toolu_features",
	})
	if err != nil {
		return err
	}
	return w.preToolUse(dir, string(raw))
}

func (w *world) askAboutBash(dir, command string) error {
	return w.askAbout(dir, "Bash", map[string]any{"command": command, "description": "the scenario's command"})
}

func (w *world) askAboutFileTool(tool, file string) error {
	return w.askAbout(w.dir, tool, map[string]any{
		"file_path":  filepath.Join(w.dir, file),
		"old_string": "git commit",
		"new_string": "git push",
	})
}

// itos guard claude-code run in the folder dir, input on its stdin.
func (w *world) preToolUse(dir, input string) error {
	w.markRun()
	return w.runWith(dir, strings.NewReader(input), w.bin, "guard", "claude-code")
}

// Standard output is Claude Code's deny for a PreToolUse hook, its reason
// saying text.
func (w *world) deniesSaying(text string) error {
	var answer struct {
		Output struct {
			Event    string `json:"hookEventName"`
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &answer); err != nil {
		return fmt.Errorf("stdout is not Claude Code's JSON: %v\n%s", err, w.report())
	}
	switch o := answer.Output; {
	case o.Event != "PreToolUse":
		return fmt.Errorf("the answer is for the event %q, not PreToolUse\n%s", o.Event, w.report())
	case o.Decision != "deny":
		return fmt.Errorf("the answer decides %q, not deny\n%s", o.Decision, w.report())
	case !strings.Contains(o.Reason, text):
		return fmt.Errorf("the reason does not say %q\n%s", text, w.report())
	}
	return nil
}

func (w *world) writesNothing() error {
	if w.stdout != "" {
		return fmt.Errorf("itos wrote to stdout\n%s", w.report())
	}
	return nil
}
