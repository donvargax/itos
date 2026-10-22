package message

import (
	"slices"
	"testing"
)

func TestWithoutFootersDropsItosCommitsOwnWords(t *testing.T) {
	env := []string{"HOME=/h", FootersEnv + "=Task: T-1", AmendEnv + "=1", "ITOS_FOOTERSX=y", "X=" + AmendEnv + "=1"}
	if got := WithoutFooters(env); !slices.Equal(got, []string{"HOME=/h", "ITOS_FOOTERSX=y", "X=ITOS_AMEND=1"}) {
		t.Errorf("WithoutFooters = %q", got)
	}
	if got := WithoutFooters([]string{AmendEnv + "=0"}); got != nil {
		t.Errorf("only ITOS_AMEND: %q", got)
	}
}
