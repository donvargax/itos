package proptest

import (
	"flag"
	"os"
	"strconv"
	"sync"
	"testing"

	"pgregory.net/rapid"
)

// FixedSeed is the seed every property runs with unless the run names one.
const FixedSeed uint64 = 20261025

var pinOnce sync.Once

// Check runs prop as a rapid property of t with the tier's seed (FixedSeed
// unless RAPID_SEED or -rapid.seed names one) and the options.
func Check(t *testing.T, prop func(*rapid.T), options ...Option) {
	t.Helper()
	pinOnce.Do(func() { pinSeed(flag.CommandLine) })
	for _, option := range options {
		option(t, flag.CommandLine, os.LookupEnv)
	}
	rapid.Check(t, prop)
}

// pinSeed sets rapid's seed flag to FixedSeed when the run left it at 0,
// rapid's "draw a random one".
func pinSeed(fs *flag.FlagSet) {
	if seed := fs.Lookup("rapid.seed"); seed != nil && seed.Value.String() == "0" {
		_ = fs.Set("rapid.seed", strconv.FormatUint(FixedSeed, 10))
	}
}

// Option changes how Check runs one property.
type Option func(t *testing.T, fs *flag.FlagSet, lookup func(string) (string, bool))

// Checks is the number of checks the property runs, unless the run names a
// number (RAPID_CHECKS or -rapid.checks), which the nightly does.
func Checks(n int) Option {
	return func(t *testing.T, fs *flag.FlagSet, lookup func(string) (string, bool)) {
		if _, named := lookup("RAPID_CHECKS"); named {
			return
		}
		flagged := false
		fs.Visit(func(f *flag.Flag) { flagged = flagged || f.Name == "rapid.checks" })
		checks := fs.Lookup("rapid.checks")
		if flagged || checks == nil {
			return
		}
		was := checks.Value.String()
		_ = fs.Set("rapid.checks", strconv.Itoa(n))
		t.Cleanup(func() { _ = fs.Set("rapid.checks", was) })
	}
}
