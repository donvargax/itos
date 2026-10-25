package features

import (
	"flag"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"

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

func TestShard(t *testing.T) {
	for text, ok := range map[string]bool{"1/1": true, "2/3": true, "3/3": true, "0/2": false, "3/2": false, "1": false, "a/b": false, "1/": false, "": false} {
		if _, _, err := parseShard(text); (err == nil) != ok {
			t.Errorf("parseShard(%q): %v", text, err)
		}
	}
}

// Each scenario is in exactly one share, the same one on every call, and the shares are not
// lopsided.
func TestInShardPartitionsTheScenarios(t *testing.T) {
	defer func(i, n int) { shardIndex, shardCount = i, n }(shardIndex, shardCount)
	const count = 3
	var scenarios []*godog.Scenario
	for i := 0; i < 300; i++ {
		scenarios = append(scenarios, &godog.Scenario{
			Uri:   "a.feature",
			Name:  "scenario " + strconv.Itoa(i/2),
			Steps: []*godog.Step{{Text: "step " + strconv.Itoa(i)}},
		})
	}
	sizes := make([]int, count)
	for _, s := range scenarios {
		in := 0
		for index := 1; index <= count; index++ {
			shardIndex, shardCount = index, count
			if inShard(s) {
				in++
				sizes[index-1]++
				if !inShard(s) {
					t.Fatalf("%s is in share %d once and not again", s.Name, index)
				}
			}
		}
		if in != 1 {
			t.Fatalf("%s is in %d shares", s.Name, in)
		}
	}
	for index, size := range sizes {
		if size < 60 || size > 140 {
			t.Errorf("share %d holds %d of %d scenarios", index+1, size, len(scenarios))
		}
	}
	shardIndex, shardCount = 1, 1
	if !inShard(scenarios[0]) {
		t.Error("a run with no shard skips a scenario")
	}
}
