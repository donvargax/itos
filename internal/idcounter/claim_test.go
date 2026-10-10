package idcounter

import (
	"math"
	"strings"
	"testing"
)

// A claim of three takes three numbers in a row past the higher of the
// repository's highest and the counter, and raises the counter past the
// last, in the file and on the remote alike.
func TestClaimTakesARunAndRaisesTheCounterPastIt(t *testing.T) {
	local := Store{CommonDir: t.TempDir(), LocalOnly: true}
	if first, err := Claim(local, "ID-A", 1, 3); err != nil || first != 2 {
		t.Fatalf("local Claim = (%d, %v), want (2, nil)", first, err)
	}
	if first, err := Claim(local, "ID-A", 1, 2); err != nil || first != 5 {
		t.Fatalf("second local Claim = (%d, %v), want (5, nil)", first, err)
	}
	if n, err := Mint(local, "ID-A", 9); err != nil || n != 10 {
		t.Fatalf("Mint past a higher repository = (%d, %v), want (10, nil)", n, err)
	}
	root, remote := counterRemote(t, "4\n")
	if first, err := Claim(Store{Root: root, Remote: "origin"}, "slice", 2, 3); err != nil || first != 5 {
		t.Fatalf("remote Claim = (%d, %v), want (5, nil)", first, err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "show", "refs/itos/ids:counters/slice")); got != "7" {
		t.Fatalf("remote slice counter = %q, want 7", got)
	}
}

// A count below 1 claims nothing, and a run past the largest int is refused
// before the counter is written.
func TestClaimRefusesAnEmptyRunAndOnePastTheLargestInt(t *testing.T) {
	local := Store{CommonDir: t.TempDir(), LocalOnly: true}
	for _, count := range []int{0, -2} {
		if first, err := Claim(local, "ID-A", 0, count); err == nil || first != 0 {
			t.Errorf("Claim of %d = (%d, %v), want a refusal", count, first, err)
		}
	}
	if first, err := Claim(local, "ID-A", math.MaxInt-1, 2); err == nil || first != 0 || !strings.Contains(err.Error(), "largest number") {
		t.Fatalf("Claim past the largest int = (%d, %v), want a refusal", first, err)
	}
	if first, err := Claim(local, "ID-A", math.MaxInt-2, 2); err != nil || first != math.MaxInt-1 {
		t.Fatalf("Claim up to the largest int = (%d, %v), want (%d, nil)", first, err, math.MaxInt-1)
	}
	root, remote := counterRemote(t, "4\n")
	if first, err := Claim(Store{Root: root, Remote: "origin"}, "slice", math.MaxInt, 1); err == nil || first != 0 {
		t.Fatalf("remote Claim past the largest int = (%d, %v), want a refusal", first, err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "show", "refs/itos/ids:counters/slice")); got != "4" {
		t.Fatalf("remote slice counter = %q after a refused claim, want 4", got)
	}
}
