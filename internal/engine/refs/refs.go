package refs

import (
	"context"
	"regexp"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// objectName is a commit's full name, as a CI provider gives a range's
// start.
var objectName = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// CheckRange is Check for a range a CI run reads (ci plan, run and scope):
// its start may be a full commit name the repository does not have, a
// history rewritten since the provider saw it, which the plan reads as a
// range it cannot read, and runs everything for (internal/plan).
func CheckRange(ctx context.Context, v vcsport.VCS, command, from, to string, more ...string) error {
	if objectName.MatchString(from) {
		from = ""
	}
	return Check(ctx, v, command, append([]string{from, to}, more...)...)
}

// Check refuses a ref that names no commit, before command uses it: a
// usage error naming the ref, in a line for people rather than git's
// command line (rule 25 of docs/CLI.md). An empty or all-zero start (a new branch,
// which ranges read as everything up to its end) and vcsport.Unpushed name
// none to check; outside a repository nothing is checked, and the command
// says what it says there.
func Check(ctx context.Context, v vcsport.VCS, command string, refs ...string) error {
	for _, ref := range refs {
		if ref == vcsport.Unpushed || config.NewBranch(ref) {
			continue
		}
		if _, err := v.Resolve(ctx, ref); err == nil {
			continue
		}
		if _, err := v.Locate(ctx, ""); err != nil {
			return nil
		}
		return out.Usage("%s: %s is not a commit of this repository", command, ref)
	}
	return nil
}
