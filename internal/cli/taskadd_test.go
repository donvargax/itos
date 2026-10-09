package cli

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
)

func taskAddTestConfig() *config.Loaded {
	cfg := &config.Loaded{}
	cfg.Ledger.Group.Label = "phase"
	return cfg
}

func TestTaskAddArgsParsesChecksAndTimeouts(t *testing.T) {
	n, err := taskAddArgs(taskAddTestConfig(), []string{
		"--phase=1", "--type=chore", "--title=Title", "--why=Reason",
		"--check=true", "--timeout=2", "--check", "echo done",
	})
	if err != nil {
		t.Fatalf("taskAddArgs: %v", err)
	}
	if n.Group != "1" || n.Type != "chore" || n.Title != "Title" || n.Why != "Reason" {
		t.Fatalf("taskAddArgs fields = %#v", n)
	}
	if len(n.Checks) != 2 || n.Checks[0].Run != "true" || n.Checks[1].Run != "echo done" {
		t.Fatalf("taskAddArgs checks = %#v", n.Checks)
	}
	if n.Checks[0].Timeout == nil || *n.Checks[0].Timeout != 2 || n.Checks[1].Timeout != nil {
		t.Fatalf("taskAddArgs timeouts = %#v, want 2 and nil", n.Checks)
	}
}

func TestTaskAddArgsRejectsInvalidCheckAndTimeoutOptions(t *testing.T) {
	base := []string{"--group", "1", "--type", "chore", "--title", "Title", "--why", "Reason"}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing check", args: base, want: taskAddUsage},
		{name: "unknown flag", args: append(append([]string{}, base...), "--unknown", "value"), want: "task add does not take --unknown"},
		{name: "timeout before check", args: append(append([]string{}, base...), "--timeout", "2"), want: "task add takes one --timeout"},
		{name: "empty check", args: append(append([]string{}, base...), "--check", " "), want: "task add --check needs a command"},
		{name: "invalid timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "nope"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "duplicate timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "2", "--timeout", "3"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "explicit id", args: append([]string{"T-004"}, append(append([]string{}, base...), "--check", "true")...), want: "task add mints the task id"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := taskAddArgs(taskAddTestConfig(), test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("taskAddArgs error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
