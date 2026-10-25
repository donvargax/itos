package features

import (
	"flag"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// -shard=i/n runs the i-th of n shares of the scenarios, so a platform whose
// run nears go test's time limit splits the suite across jobs (T-308). A
// scenario belongs to the share its feature file, name and steps hash to, so
// every job of a run agrees on the shares, the scenarios of an outline fall in
// different ones, and a new scenario moves no other.
var shardFlag = flag.String("shard", "", "run only the i-th of n shares of the scenarios, as i/n")

// The run's share, 1 of 1 when -shard is not given.
var shardIndex, shardCount = 1, 1

// parseShard reads "i/n", 1 <= i <= n.
func parseShard(text string) (index, count int, err error) {
	i, n, found := strings.Cut(text, "/")
	index, err1 := strconv.Atoi(i)
	count, err2 := strconv.Atoi(n)
	if !found || err1 != nil || err2 != nil || index < 1 || index > count {
		return 0, 0, fmt.Errorf("-shard=%q is not i/n with 1 <= i <= n", text)
	}
	return index, count, nil
}

// inShard is whether the scenario is in the run's share.
func inShard(s *godog.Scenario) bool {
	if shardCount == 1 {
		return true
	}
	h := fnv.New32a()
	h.Write([]byte(s.Uri + "\x00" + s.Name))
	for _, step := range s.Steps {
		h.Write([]byte("\x00" + step.Text))
	}
	return int(h.Sum32()%uint32(shardCount)) == shardIndex-1
}
