package initcmd

import (
	"errors"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
)

func TestParseRulesFlagConflictConsumesNoArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--agent-rules", "--no-agent-rules"},
		{"--no-agent-rules", "--agent-rules"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var flag RulesFlag
			if n, err := parseRulesFlag(args, 0, &flag); n != 1 || err != nil {
				t.Fatalf("first flag: consumed %d, %v", n, err)
			}
			want := RulesFlag{Given: true, Write: args[0] == "--agent-rules"}
			if flag != want {
				t.Fatalf("first flag: %+v, want %+v", flag, want)
			}
			n, err := parseRulesFlag(args, 1, &flag)
			var usage out.UsageError
			if n != 0 || !errors.As(err, &usage) || err.Error() != "init takes --agent-rules or --no-agent-rules, not both" || flag != want {
				t.Fatalf("conflicting flag: consumed %d, %v, state %+v; want unchanged %+v", n, err, flag, want)
			}
		})
	}
}
