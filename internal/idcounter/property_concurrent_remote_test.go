package idcounter

import (
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/proptest"
	"pgregory.net/rapid"
)

// The counter claimed on the remote holds under claims at once from clones of
// one repository, on real git: every claim answers what a sequential counter
// would, whichever of them lost a race and retried. A claim that lost sixteen
// is the only one allowed to fail, and it takes nothing.
func TestPropertyConcurrentRemoteCounterIsLinearizable(t *testing.T) {
	stores, remote := clonedStores(t, 3)
	var series int
	proptest.Check(t, func(rt *rapid.T) {
		series++
		category := "c" + strconv.Itoa(series)
		workers := rapid.IntRange(2, len(stores)).Draw(rt, "clones")
		history := run(category, stores[:workers], drawPlan(rt, workers))
		counter := strings.TrimSpace(gitAt(t, remote, "show", remoteRef+":counters/"+category))
		stored, err := strconv.Atoi(counter)
		if err != nil {
			rt.Fatalf("the remote's counter of %s reads %q", category, counter)
		}
		holds(rt, history, 0, stored, func(err error) bool { return strings.Contains(err.Error(), "kept advancing") })
	}, proptest.Checks(10))
}
