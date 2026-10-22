package cli

import (
	"strings"
	"testing"
)

// A block of rules for agents other than this itos's is one line naming
// itos init --agent-rules, beside the itos line, and --json's agent_rules
// names its files, null with none (slice 124).
func TestStatusAgentRules(t *testing.T) {
	statusRepo(t, map[string]string{
		"work-items.yaml": items("one"),
		"people.yaml":     "[]\n",
		agentsFile:        "Mine.\n\n" + olderBlock,
	})
	code, stdout, stderr := run("status")
	if code != 0 || !strings.Contains(stdout, "\n\n"+staleRulesLine(false)+"\n") {
		t.Errorf("exit %d, the status does not name itos init --agent-rules:\n%s%s", code, stdout, stderr)
	}
	code, stdout, stderr = run("status", "--json")
	if code != 0 || !strings.Contains(stdout, `"agent_rules": {`) || !strings.Contains(stdout, `"stale": [`) {
		t.Errorf("exit %d, --json does not give agent_rules:\n%s%s", code, stdout, stderr)
	}
}
