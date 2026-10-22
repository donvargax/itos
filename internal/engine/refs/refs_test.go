package refs

import (
	"errors"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

const commit = "1111111111111111111111111111111111111111"

// repository is a repository of one commit, which main names.
func repository() *porttest.VCS {
	return &porttest.VCS{
		Located: map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}},
		History: []porttest.Commit{{Name: commit}},
		Named:   map[string]string{"refs/heads/main": commit},
	}
}

func TestCheckPassesCommitsAndWhatNamesNoneToCheck(t *testing.T) {
	refs := []string{"main", commit, "", "0000000000000000000000000000000000000000", vcsport.Unpushed}
	if err := Check(t.Context(), repository(), "verify", refs...); err != nil {
		t.Fatalf("Check(%q) = %v, want nil", refs, err)
	}
}

func TestCheckRefusesTheFirstRefThatNamesNoCommit(t *testing.T) {
	err := Check(t.Context(), repository(), "verify", "main", "nope", "gone")
	var u out.UsageError
	if !errors.As(err, &u) || err.Error() != "verify: nope is not a commit of this repository" {
		t.Fatalf("Check(main nope gone) = %v, want nope refused", err)
	}
}

func TestCheckChecksNothingOutsideARepository(t *testing.T) {
	if err := Check(t.Context(), &porttest.VCS{}, "verify", "nope"); err != nil {
		t.Fatalf("Check outside a repository = %v, want nil", err)
	}
}

func TestCheckRangeLeavesOutAStartThatIsAFullCommitName(t *testing.T) {
	for _, from := range []string{strings.Repeat("a", 40), strings.Repeat("b", 64)} {
		if err := CheckRange(t.Context(), repository(), "ci plan", from, "main"); err != nil {
			t.Errorf("CheckRange(%s, main) = %v, want nil", from, err)
		}
	}
}

func TestCheckRangeChecksEveryOtherRef(t *testing.T) {
	for _, c := range []struct {
		from, to string
		more     []string
		ref      string
	}{
		{strings.Repeat("a", 39), "main", nil, strings.Repeat("a", 39)},
		{strings.Repeat("a", 41), "main", nil, strings.Repeat("a", 41)},
		{"x" + strings.Repeat("a", 40), "main", nil, "x" + strings.Repeat("a", 40)},
		{strings.Repeat("a", 40) + "x", "main", nil, strings.Repeat("a", 40) + "x"},
		{strings.Repeat("A", 40), "main", nil, strings.Repeat("A", 40)},
		{"nope", "gone", nil, "nope"},
		{"", "gone", nil, "gone"},
		{"main", "main", []string{"data"}, "data"},
	} {
		err := CheckRange(t.Context(), repository(), "ci plan", c.from, c.to, c.more...)
		var u out.UsageError
		if !errors.As(err, &u) || err.Error() != "ci plan: "+c.ref+" is not a commit of this repository" {
			t.Errorf("CheckRange(%s, %s, %q) = %v, want %s refused", c.from, c.to, c.more, err, c.ref)
		}
	}
}
