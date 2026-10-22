package out

import (
	"errors"
	"testing"
)

func TestUsageIsAUsageErrorSayingTheTrimmedText(t *testing.T) {
	err := Usage("  %s takes one %s\n", "followup", "--with")
	var u UsageError
	if !errors.As(err, &u) {
		t.Fatalf("Usage gave %T, want a UsageError", err)
	}
	if got, want := err.Error(), "followup takes one --with"; got != want {
		t.Errorf("Usage said %q, want %q", got, want)
	}
}
